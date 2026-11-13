package cloudauth

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"

	"k8s.io/client-go/rest"
)

// Endpoint binding.
//
// An in-process token carries the Meshery process's own cloud identity, and
// a kubeconfig is often supplied by someone else (uploaded to Meshery,
// pasted into a connection). Before a token source is installed, the
// cluster endpoint is checked so that a kubeconfig pointing at an arbitrary
// server cannot collect the process's credentials:
//
//   - EKS tokens are presigned STS GetCallerIdentity URLs. Anyone holding one
//     can replay it, for its 15-minute lifetime, against the EKS cluster
//     whose name is signed in, or present it to STS to learn the process's
//     IAM identity. The server must be an EKS API endpoint,
//     https://<id>.<cell>.<region>.eks.amazonaws.com (".amazonaws.com.cn" in
//     the China partition), or the IPv6 form
//     https://<id>.<region>.api.aws (".api.amazonwebservices.com.cn" in
//     China), and when the endpoint names a region, the token
//     is presigned for that region.
//   - GKE tokens are Google OAuth2 access tokens for the cloud-platform
//     scope, usable against every Google API the identity can reach. The
//     server must be a GKE control plane endpoint: a DNS-based endpoint
//     under ".gke.goog", Connect Gateway (connectgateway.googleapis.com or
//     <region>-connectgateway.googleapis.com), or an IP address (public or
//     private) with the cluster CA certificate in the kubeconfig. An IP
//     endpoint is only marked here. The Authenticator confirms the IP and
//     the CA against the GKE Container API before it installs a token, and
//     it sends nothing to the kubeconfig IP until that check succeeds.
//
// For both, the scheme must be https, TLS verification must be on
// (insecure-skip-tls-verify unset), and the kubeconfig must not set
// proxy-url: with a proxy and a CA certificate both chosen by the kubeconfig
// author, the proxy could terminate TLS for a genuine EKS or GKE host name.
//
// There is no static GKE IP list. Google does not publish the control plane
// addresses, and https://www.gstatic.com/ipranges/cloud.json lists every
// Google Cloud customer prefix, so a VM in any customer project would match.
// That file also changes often, and a private control plane uses an address
// the customer chooses, often from an RFC1918 range. The kubeconfig is
// trusted to name the cluster (the gke_<project>_<location>_<cluster> context
// or cluster name is only a hint for container.clusters.get). The IP and the
// CA must both match a cluster the process credentials can see through
// container.clusters.get or container.clusters.list. If they do not, or the
// API cannot be reached, the endpoint is refused.

var (
	// eksHostSuffixes are the EKS API endpoint domains per AWS partition:
	// aws and aws-us-gov use amazonaws.com, aws-cn uses amazonaws.com.cn.
	// IPv6 clusters created after October 2024 use the api.aws form
	// (<id>.<region>.api.aws, or .api.amazonwebservices.com.cn in China)
	// instead of the eks.amazonaws.com form.
	eksHostSuffixes = []string{".eks.amazonaws.com", ".eks.amazonaws.com.cn", ".api.aws", ".api.amazonwebservices.com.cn"}
	// awsRegionPattern matches AWS region names such as us-west-2,
	// us-gov-west-1 and cn-north-1.
	awsRegionPattern = regexp.MustCompile(`^[a-z]{2}(-[a-z]+)+-[0-9]+$`)
	// eksHostLabelPattern matches one DNS label of an EKS endpoint host
	// name before the region: the cluster's endpoint ID and its cell.
	eksHostLabelPattern = regexp.MustCompile(`^[a-z0-9]+$`)
	// gkeDNSEndpointSuffix is the domain of GKE DNS-based control plane
	// endpoints.
	gkeDNSEndpointSuffix = ".gke.goog"
	// connectGatewayPattern matches the global and regional Connect Gateway
	// hosts.
	connectGatewayPattern = regexp.MustCompile(`^([a-z0-9-]+-)?connectgateway\.googleapis\.com$`)
)

// clusterHint carries the kubeconfig context name and that context's
// cluster name. Both are optional. They are not authenticated; they only
// choose the first container.clusters.get URL for a GKE IP endpoint.
type clusterHint struct {
	contextName string
	clusterName string
}

// gkeIPEndpoint is a GKE control plane addressed by IP. bindEndpoint records
// it; the Authenticator verifies it inside setup, after the Google token
// source exists and before installTokenSource.
type gkeIPEndpoint struct {
	// Host is the server IP, canonicalized with net.IP.String, without a port.
	Host string
	// CA is the kubeconfig certificate-authority-data, already decoded.
	// It is empty when the kubeconfig names a file instead; CAFile is read
	// at verification time.
	CA []byte
	// CAFile is the kubeconfig certificate-authority path when CA is empty.
	CAFile string
	// ContextName and ClusterName are the kubeconfig hint, either of which
	// may be empty.
	ContextName string
	ClusterName string
}

// bindEndpoint checks that config's server may receive tokens minted for
// spec and returns spec bound to it: for EKS, a Region left empty by the
// exec stanza is taken from the endpoint host name; for a GKE IP, spec is
// marked for Container API verification. It returns a plain error describing
// the first rule the endpoint breaks. It does not contact the server or the
// GKE API.
func bindEndpoint(config *rest.Config, spec Spec, hint clusterHint) (Spec, error) {
	server, err := url.Parse(config.Host)
	if err != nil {
		return spec, fmt.Errorf("the server URL %q cannot be parsed: %w", config.Host, err)
	}
	if !strings.EqualFold(server.Scheme, "https") {
		return spec, fmt.Errorf("the server URL %q does not use https", config.Host)
	}
	host := strings.TrimSuffix(strings.ToLower(server.Hostname()), ".")
	if host == "" {
		return spec, fmt.Errorf("the server URL %q has no host", config.Host)
	}
	if config.Insecure {
		return spec, fmt.Errorf("TLS verification is disabled for %q (insecure-skip-tls-verify)", host)
	}
	if config.Proxy != nil {
		return spec, fmt.Errorf("the kubeconfig routes %q through proxy-url", host)
	}

	switch spec.Provider {
	case ProviderEKS:
		region, err := eksEndpointRegion(host)
		if err != nil {
			return spec, err
		}
		switch {
		case spec.EKS.Region == "":
			spec.EKS.Region = region
		case spec.EKS.Region != region:
			return spec, fmt.Errorf("the exec stanza mints for region %q but the EKS endpoint %q is in region %q", spec.EKS.Region, host, region)
		}
		return spec, nil
	case ProviderGKE:
		return bindGKEEndpoint(host, config, spec, hint)
	default:
		return spec, fmt.Errorf("no endpoint rules exist for provider %q", spec.Provider)
	}
}

// eksEndpointRegion returns the region named by an EKS API endpoint host
// name, or an error when host is not one.
func eksEndpointRegion(host string) (string, error) {
	notEKS := fmt.Errorf("%q is not an EKS API endpoint (<id>.<cell>.<region>.eks.amazonaws.com)", host)
	for _, suffix := range eksHostSuffixes {
		prefix, ok := strings.CutSuffix(host, suffix)
		if !ok {
			continue
		}
		labels := strings.Split(prefix, ".")
		if len(labels) < 2 || len(labels) > 3 {
			return "", notEKS
		}
		region := labels[len(labels)-1]
		if !awsRegionPattern.MatchString(region) {
			return "", notEKS
		}
		for _, label := range labels[:len(labels)-1] {
			if !eksHostLabelPattern.MatchString(label) {
				return "", notEKS
			}
		}
		return region, nil
	}
	return "", notEKS
}

// bindGKEEndpoint accepts a DNS endpoint or Connect Gateway as-is, and marks
// an IP endpoint for API verification when the kubeconfig has a CA. The CA
// file is not read here, so a path that exists only at verification time is
// still accepted by the form check.
func bindGKEEndpoint(host string, config *rest.Config, spec Spec, hint clusterHint) (Spec, error) {
	if ip := net.ParseIP(host); ip != nil {
		if len(config.CAData) == 0 && config.CAFile == "" {
			return spec, fmt.Errorf("the GKE IP endpoint %q has no certificate-authority in the kubeconfig", host)
		}
		marked := &gkeIPEndpoint{
			Host:        ip.String(),
			ContextName: hint.contextName,
			ClusterName: hint.clusterName,
		}
		if len(config.CAData) > 0 {
			marked.CA = append([]byte(nil), config.CAData...)
		} else {
			marked.CAFile = config.CAFile
		}
		spec.gkeIP = marked
		return spec, nil
	}
	if strings.HasSuffix(host, gkeDNSEndpointSuffix) && len(host) > len(gkeDNSEndpointSuffix) {
		return spec, nil
	}
	if connectGatewayPattern.MatchString(host) {
		return spec, nil
	}
	return spec, fmt.Errorf("%q is not a GKE control plane endpoint (an IP address, a *.gke.goog DNS endpoint, or Connect Gateway)", host)
}
