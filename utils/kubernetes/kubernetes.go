package kubernetes

import (
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	RestConfig        rest.Config           `json:"restconfig,omitempty"`
	KubeClient        *kubernetes.Clientset `json:"kubeclient,omitempty"`
	DynamicKubeClient dynamic.Interface     `json:"dynamicKubeClient,omitempty"`
	kubeConfigLoader  clientcmd.ClientConfig
}

func New(kubeconfig []byte) (*Client, error) {
	restConfig, kubeConfigLoader, err := detectKubeConfig(kubeconfig)
	if err != nil {
		return nil, err
	}
	restConfig.QPS = float32(50)
	restConfig.Burst = int(100)

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
		kubeConfigLoader:  kubeConfigLoader,
	}, nil
}

func (c *Client) rawKubeConfigLoader() clientcmd.ClientConfig {
	return c.kubeConfigLoader
}
