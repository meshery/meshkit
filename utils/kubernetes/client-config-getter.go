package kubernetes

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

type clientConfigRESTClientGetter struct {
	clientConfig clientcmd.ClientConfig
}

var _ genericclioptions.RESTClientGetter = (*clientConfigRESTClientGetter)(nil)

func newClientConfigRESTClientGetter(clientConfig clientcmd.ClientConfig) genericclioptions.RESTClientGetter {
	return &clientConfigRESTClientGetter{clientConfig: clientConfig}
}

func (g *clientConfigRESTClientGetter) ToRESTConfig() (*rest.Config, error) {
	return g.clientConfig.ClientConfig()
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
