package kubernetes

import (
	"github.com/layer5io/meshkit/utils/kubernetes/discovery"
	"github.com/layer5io/meshkit/utils/kubernetes/informer"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Client struct {
	RestConfig        rest.Config           `json:"restconfig,omitempty"`
	DiscoveryClient   *discovery.Client     `json:"discovery_client,omitempty"`
	InformerClient    *informer.Client      `json:"informer_client,omitempty"`
	KubeClient        *kubernetes.Clientset `json:"kubeclient,omitempty"`
	DynamicKubeClient dynamic.Interface     `json:"dynamic_kubeclient,omitempty"`
}

func New(kubeconfig []byte) (*Client, error) {

	restConfig, err := DetectKubeConfig(kubeconfig)
	if err != nil {
		return nil, err
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

	// Configure discovery client
	dclient, err := discovery.NewClient(kclient)
	if err != nil {
		return nil, ErrNewDiscovery(err)
	}

	// Configure informer client
	iclient, err := informer.NewClient(kclient)
	if err != nil {
		return nil, ErrNewInformer(err)
	}

	return &Client{
		RestConfig:        *restConfig,
		DynamicKubeClient: dyclient,
		DiscoveryClient:   dclient,
		InformerClient:    iclient,
		KubeClient:        kclient,
	}, nil
}
