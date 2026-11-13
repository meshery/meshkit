// Package kubernetes provides a thin wrapper around client-go/Helm to discover
// a cluster's kubeconfig, build a Kubernetes/dynamic client, and apply
// manifests and Helm charts against that cluster.
//
// Authentication is resolved through a genericclioptions.RESTClientGetter
// (see client-config-getter.go) rather than a static rest.Config wherever
// possible, so that authentication mechanisms which require re-resolution on
// every request - most notably exec-based credential plugins such as
// `aws eks get-token` - keep working for both direct Kubernetes API calls and
// Helm operations (ApplyHelmChart).
//
// When the exec plugin is a known cloud plugin (aws eks get-token,
// aws-iam-authenticator, or gke-gcloud-auth-plugin
// --use_application_default_credentials) and the cluster endpoint belongs
// to that cloud, the getter mints the bearer token in process with the
// cloud SDK instead of forking the binary, so Meshery works where the plugin
// is not installed. The first token is minted when the kubeconfig is loaded
// or the client is built. Later requests reuse the cached token and mint
// again only near expiry or after the API server answers 401. One source is
// cached per cluster identity and shared by every client in the process.
// A GKE IP endpoint is cached per verified endpoint: the IP and the CA must
// match a cluster in the ADC project or GOOGLE_CLOUD_PROJECT or
// GOOGLE_PROJECT through the GKE API (container.clusters.get or
// container.clusters.list). The kubeconfig cannot add a project to search.
// No static IP list is used, and a refusal is reused for five minutes. If
// cloud credentials are unavailable, or the endpoint is not one the
// process's credentials may be sent to (including a GKE IP the API does not
// confirm), the exec plugin is used (and the fallback is logged) when it is
// on PATH, and a descriptive error is returned otherwise.
// Every other exec plugin, and any config that already carries a token,
// basic auth, or a client certificate, is left to client-go.
// Set MESHKIT_DISABLE_INPROCESS_CLOUD_AUTH=true to turn in-process minting
// off for the whole process. SetInProcessCloudAuthForContext does the same
// for a kubeconfig context's user, which is the switch mesheryctl and Meshery
// Server use; every context that shares that user switches with it. See
// package utils/kubernetes/cloudauth for supported plugins and the
// credential chains.
package kubernetes

import (
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Client wraps the Kubernetes and dynamic clientsets for a single cluster,
// along with the REST configuration and, when available, the
// genericclioptions.RESTClientGetter used to (re)resolve credentials for
// operations - such as Helm chart installation - that need more than a
// static rest.Config.
type Client struct {
	RestConfig        rest.Config           `json:"restconfig,omitempty"`
	KubeClient        *kubernetes.Clientset `json:"kubeclient,omitempty"`
	DynamicKubeClient dynamic.Interface     `json:"dynamicKubeClient,omitempty"`
	restClientGetter  genericclioptions.RESTClientGetter
}

// New detects the kubeconfig for the target cluster (from the supplied
// bytes, in-cluster config, $KUBECONFIG, or the default kubeconfig path),
// builds the Kubernetes and dynamic clientsets, and returns the resulting
// Client. When the kubeconfig is loaded from a clientcmd.ClientConfig
// (i.e. not an in-cluster config), the loader is retained on the Client so
// that consumers such as ApplyHelmChart can re-resolve credentials -
// including exec-based credential plugins - on every request instead of
// relying on a single, possibly stale, rest.Config.
func New(kubeconfig []byte) (*Client, error) {
	restConfig, kubeConfigLoader, err := detectKubeConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	var restClientGetter genericclioptions.RESTClientGetter
	if kubeConfigLoader != nil {
		restClientGetter = newClientConfigRESTClientGetter(kubeConfigLoader)
	} else {
		restClientGetter = newRESTConfigRESTClientGetter(restConfig)
	}
	restConfig, err = restClientGetter.ToRESTConfig()
	if err != nil {
		return nil, err
	}

	// if insecure variable is kept true, allow that
	if restConfig.TLSClientConfig.Insecure { //nolint:staticcheck
		restConfig.TLSClientConfig.Insecure = true //nolint:staticcheck
	}

	// Configure kubeclient
	kclient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, ErrNewKubeClient(err)
	}

	// Configure dynamic kubeclient
	dyclient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, ErrNewDynClient(err)
	}

	return &Client{
		RestConfig:        *restConfig,
		DynamicKubeClient: dyclient,
		KubeClient:        kclient,
		restClientGetter:  restClientGetter,
	}, nil
}

// configureRESTConfig applies meshkit's default QPS/Burst rate limits to
// config, without overriding values a caller has already set explicitly.
func configureRESTConfig(config *rest.Config) {
	if config.QPS == 0 {
		config.QPS = float32(50)
	}
	if config.Burst == 0 {
		config.Burst = int(100)
	}
}

// getRESTClientGetter returns the genericclioptions.RESTClientGetter used to
// resolve credentials for operations such as Helm chart installation. When
// the Client was constructed via New, this is the getter backed by the
// original kubeconfig loader (preserving exec-based credential plugins).
// For Clients built directly (e.g. &Client{RestConfig: cfg}) without going
// through New, it falls back to a getter derived from RestConfig so that
// callers retain compatibility with the previous, RestConfig-only behavior.
func (c *Client) getRESTClientGetter() genericclioptions.RESTClientGetter {
	if c.restClientGetter != nil {
		return c.restClientGetter
	}

	// Preserve compatibility for clients constructed directly instead of through New.
	return newRESTConfigRESTClientGetter(&c.RestConfig)
}
