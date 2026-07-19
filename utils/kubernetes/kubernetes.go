package kubernetes

import (
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Client struct {
	RestConfig        rest.Config           `json:"restconfig,omitempty"`
	KubeClient        *kubernetes.Clientset `json:"kubeclient,omitempty"`
	DynamicKubeClient dynamic.Interface     `json:"dynamicKubeClient,omitempty"`
	restClientGetter  genericclioptions.RESTClientGetter
}

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

func configureRESTConfig(config *rest.Config) {
	config.QPS = float32(50)
	config.Burst = int(100)
}

func (c *Client) getRESTClientGetter() genericclioptions.RESTClientGetter {
	if c.restClientGetter != nil {
		return c.restClientGetter
	}

	// Preserve compatibility for clients constructed directly instead of through New.
	return newRESTConfigRESTClientGetter(&c.RestConfig)
}
