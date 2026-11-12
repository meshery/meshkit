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
//     server must be a GKE control plane endpoint: an IP address (public or
//     private endpoint) with the cluster CA certificate in the kubeconfig, a
//     DNS-based endpoint under ".gke.goog", or Connect Gateway
//     (connectgateway.googleapis.com or <region>-connectgateway.googleapis.com).
//
// For both, the scheme must be https, TLS verification must be on
// (insecure-skip-tls-verify unset), and the kubeconfig must not set
// proxy-url: with a proxy and a CA certificate both chosen by the kubeconfig
// author, the proxy could terminate TLS for a genuine EKS or GKE host name.
//
// Residual risk: an IP address cannot be proven to belong to a GKE cluster
// offline. A kubeconfig author who supplies their own IP address and their
// own CA certificate passes the check, so requiring TLS verification only
// protects against attackers on the network path, not against the
// kubeconfig author. The kubeconfig's cluster name
// (gke_<project>_<location>_<cluster>) is chosen by the same author and so
// proves nothing either. Closing this gap needs a GKE API lookup of the
// cluster's endpoint with the process's credentials, which this package
// deliberately does not make; callers that accept kubeconfigs from
// untrusted parties for GKE IP endpoints should disable in-process minting
// (DisableEnvVar) or use the Meshery Operator.

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

// bindEndpoint checks that config's server may receive tokens minted for
// spec and returns spec bound to it: for EKS, a Region left empty by the
// exec stanza is taken from the endpoint host name. It returns a plain
// error describing the first rule the endpoint breaks.
func bindEndpoint(config *rest.Config, spec Spec) (Spec, error) {
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
		return spec, checkGKEEndpoint(host, config)
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

// checkGKEEndpoint returns an error unless host is a GKE control plane
// endpoint form.
func checkGKEEndpoint(host string, config *rest.Config) error {
	if net.ParseIP(host) != nil {
		if len(config.CAData) == 0 && config.CAFile == "" {
			return fmt.Errorf("the GKE IP endpoint %q has no certificate-authority in the kubeconfig", host)
		}
		return nil
	}
	if strings.HasSuffix(host, gkeDNSEndpointSuffix) && len(host) > len(gkeDNSEndpointSuffix) {
		return nil
	}
	if connectGatewayPattern.MatchString(host) {
		return nil
	}
	return fmt.Errorf("%q is not a GKE control plane endpoint (an IP address, a *.gke.goog DNS endpoint, or Connect Gateway)", host)
}
