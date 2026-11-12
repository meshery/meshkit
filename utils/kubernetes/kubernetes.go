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
package kubernetes

import (
	"github.com/meshery/meshkit/utils/kubernetes/cloudauth"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// ClientOption configures optional settings when constructing a Kubernetes Client.
type ClientOption func(*clientOptions)

type clientOptions struct {
	authOpts []cloudauth.Option
}

// WithCloudCredentials attaches explicit AWS or GCP credentials (in-memory only)
// for in-process EKS/GKE token minting instead of relying on host environment files.
func WithCloudCredentials(creds cloudauth.Credentials) ClientOption {
	return func(o *clientOptions) {
		o.authOpts = append(o.authOpts, cloudauth.WithCredentials(creds))
	}
}

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
// builds the Kubernetes and dynamic clientsets using default in-process cloud
// authentication settings, and returns the resulting Client. When the
// kubeconfig is loaded from a clientcmd.ClientConfig (i.e. not an in-cluster
// config), the loader is retained on the Client so that consumers such as
// ApplyHelmChart can re-resolve credentials - including exec-based credential
// plugins - on every request instead of relying on a single, possibly stale,
// rest.Config.
func New(kubeconfig []byte) (*Client, error) {
	return NewWithOptions(kubeconfig)
}

// NewWithOptions creates a new Kubernetes client from kubeconfig bytes with
// optional configuration overrides such as explicit cloud credentials for
// in-process EKS and GKE authentication.
func NewWithOptions(kubeconfig []byte, opts ...ClientOption) (*Client, error) {
	var cfg clientOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	restConfig, kubeConfigLoader, err := detectKubeConfig(kubeconfig, cfg.authOpts...)
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
