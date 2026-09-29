package kubernetes

import (
	"fmt"
	"net/http"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// clientConfigRESTClientGetter adapts a clientcmd.ClientConfig into a
// genericclioptions.RESTClientGetter, the interface Helm (and other
// client-go-based tooling) uses to resolve a REST config, discovery client,
// and REST mapper on demand. Because it defers to the underlying
// clientConfig on every call rather than caching a resolved *rest.Config, it
// allows authentication mechanisms that must be re-resolved per request -
// most notably exec-based credential plugins (e.g. `aws eks get-token`) - to
// keep working.
type clientConfigRESTClientGetter struct {
	clientConfig clientcmd.ClientConfig
}

// restConfigClientConfig adapts a *rest.Config into a clientcmd.ClientConfig
// by synthesizing an equivalent in-memory clientcmdapi.Config on demand. It
// exists so that Clients constructed directly from a rest.Config (i.e. not
// via New, so no kubeconfig loader is available) can still be driven through
// a clientConfigRESTClientGetter and, in particular, preserve any
// rest.Config.ExecProvider that would otherwise be silently dropped.
type restConfigClientConfig struct {
	restConfig *rest.Config
}

var _ genericclioptions.RESTClientGetter = (*clientConfigRESTClientGetter)(nil)
var _ clientcmd.ClientConfig = (*restConfigClientConfig)(nil)

// newClientConfigRESTClientGetter returns a genericclioptions.RESTClientGetter
// backed by clientConfig, preserving whatever authentication mechanism
// clientConfig resolves to - including exec-based credential plugins - on
// every call.
func newClientConfigRESTClientGetter(clientConfig clientcmd.ClientConfig) genericclioptions.RESTClientGetter {
	return &clientConfigRESTClientGetter{clientConfig: clientConfig}
}

// newRESTConfigRESTClientGetter returns a genericclioptions.RESTClientGetter
// derived directly from a *rest.Config, for use when no kubeconfig loader is
// available (e.g. a Client built without going through New). config is
// copied so that later mutations of the caller's rest.Config do not affect
// the returned getter.
func newRESTConfigRESTClientGetter(config *rest.Config) genericclioptions.RESTClientGetter {
	return newClientConfigRESTClientGetter(&restConfigClientConfig{
		restConfig: rest.CopyConfig(config),
	})
}

// ToRESTConfig resolves the underlying clientConfig into a *rest.Config,
// applying meshkit's default QPS/Burst rate limits via configureRESTConfig.
func (g *clientConfigRESTClientGetter) ToRESTConfig() (*rest.Config, error) {
	config, err := g.clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	configureRESTConfig(config)
	return config, nil
}

// ToDiscoveryClient builds a memory-cached discovery client from the
// resolved REST config.
func (g *clientConfigRESTClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	config, err := g.ToRESTConfig()
	if err != nil {
		return nil, err
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}

	return memory.NewMemCacheClient(discoveryClient), nil
}

// ToRESTMapper builds a deferred discovery REST mapper, with shortcut
// expansion, from the discovery client.
func (g *clientConfigRESTClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	discoveryClient, err := g.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	return restmapper.NewShortcutExpander(mapper, discoveryClient, func(string) {}), nil
}

// ToRawKubeConfigLoader returns the underlying clientcmd.ClientConfig, giving
// callers (e.g. Helm) access to the raw kubeconfig loader so that they can
// re-resolve credentials - including invoking exec-based credential plugins
// for renewal - on their own schedule rather than only once, at getter
// construction time.
func (g *clientConfigRESTClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	return g.clientConfig
}

// RawConfig synthesizes a clientcmdapi.Config from restConfig, carrying over
// server, TLS, proxy, basic/bearer/impersonation auth, and any ExecProvider
// so that consumers of the raw kubeconfig loader (e.g. Helm) observe the same
// authentication settings as restConfig itself, including exec-based
// credential plugins that a plain rest.Config-based RESTClientGetter would
// otherwise ignore.
func (c *restConfigClientConfig) RawConfig() (clientcmdapi.Config, error) {
	const connectionName = "meshkit-connection"

	config := clientcmdapi.NewConfig()
	cluster := &clientcmdapi.Cluster{
		Server:                   c.restConfig.Host,
		TLSServerName:            c.restConfig.ServerName,
		InsecureSkipTLSVerify:    c.restConfig.Insecure,
		CertificateAuthority:     c.restConfig.CAFile,
		CertificateAuthorityData: c.restConfig.CAData,
		DisableCompression:       c.restConfig.DisableCompression,
	}
	if c.restConfig.Proxy != nil {
		request, err := http.NewRequest(http.MethodGet, c.restConfig.Host, nil)
		if err != nil {
			return clientcmdapi.Config{}, fmt.Errorf("failed to create request for Kubernetes proxy resolution: %w", err)
		}
		proxyURL, err := c.restConfig.Proxy(request)
		if err != nil {
			return clientcmdapi.Config{}, fmt.Errorf("failed to resolve Kubernetes proxy: %w", err)
		}
		if proxyURL != nil {
			cluster.ProxyURL = proxyURL.String()
		}
	}
	config.Clusters[connectionName] = cluster.DeepCopy()
	config.AuthInfos[connectionName] = (&clientcmdapi.AuthInfo{
		ClientCertificate:     c.restConfig.CertFile,
		ClientCertificateData: c.restConfig.CertData,
		ClientKey:             c.restConfig.KeyFile,
		ClientKeyData:         c.restConfig.KeyData,
		Token:                 c.restConfig.BearerToken,
		TokenFile:             c.restConfig.BearerTokenFile,
		Impersonate:           c.restConfig.Impersonate.UserName,
		ImpersonateUID:        c.restConfig.Impersonate.UID,
		ImpersonateGroups:     c.restConfig.Impersonate.Groups,
		ImpersonateUserExtra:  c.restConfig.Impersonate.Extra,
		Username:              c.restConfig.Username,
		Password:              c.restConfig.Password,
		AuthProvider:          c.restConfig.AuthProvider,
		Exec:                  c.restConfig.ExecProvider,
	}).DeepCopy()
	config.Contexts[connectionName] = &clientcmdapi.Context{
		Cluster:  connectionName,
		AuthInfo: connectionName,
	}
	config.CurrentContext = connectionName
	return *config, nil
}

// ClientConfig returns a copy of restConfig, so that callers cannot mutate
// the value backing this ClientConfig.
func (c *restConfigClientConfig) ClientConfig() (*rest.Config, error) {
	return rest.CopyConfig(c.restConfig), nil
}

// Namespace returns "default" with explicit set to false, since restConfig
// carries no namespace information of its own.
func (c *restConfigClientConfig) Namespace() (string, bool, error) {
	return "default", false, nil
}

// ConfigAccess returns nil, as there is no on-disk kubeconfig backing a
// restConfigClientConfig for callers to read or persist changes to.
func (c *restConfigClientConfig) ConfigAccess() clientcmd.ConfigAccess {
	return nil
}
