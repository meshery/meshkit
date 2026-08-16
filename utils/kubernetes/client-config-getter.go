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

type clientConfigRESTClientGetter struct {
	clientConfig clientcmd.ClientConfig
}

type restConfigClientConfig struct {
	restConfig *rest.Config
}

var _ genericclioptions.RESTClientGetter = (*clientConfigRESTClientGetter)(nil)
var _ clientcmd.ClientConfig = (*restConfigClientConfig)(nil)

func newClientConfigRESTClientGetter(clientConfig clientcmd.ClientConfig) genericclioptions.RESTClientGetter {
	return &clientConfigRESTClientGetter{clientConfig: clientConfig}
}

func newRESTConfigRESTClientGetter(config *rest.Config) genericclioptions.RESTClientGetter {
	return newClientConfigRESTClientGetter(&restConfigClientConfig{
		restConfig: rest.CopyConfig(config),
	})
}

func (g *clientConfigRESTClientGetter) ToRESTConfig() (*rest.Config, error) {
	config, err := g.clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	configureRESTConfig(config)
	return config, nil
}

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

func (g *clientConfigRESTClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	discoveryClient, err := g.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	return restmapper.NewShortcutExpander(mapper, discoveryClient, func(string) {}), nil
}

func (g *clientConfigRESTClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	return g.clientConfig
}

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

func (c *restConfigClientConfig) ClientConfig() (*rest.Config, error) {
	return rest.CopyConfig(c.restConfig), nil
}

func (c *restConfigClientConfig) Namespace() (string, bool, error) {
	return "default", false, nil
}

func (c *restConfigClientConfig) ConfigAccess() clientcmd.ConfigAccess {
	return nil
}
