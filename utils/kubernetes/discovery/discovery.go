package discovery

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Config struct {
	Namespace string
	Resource  string
	Codec     runtime.ParameterCodec
}

// Client will implement discovery functions for kubernetes resources
type Client struct {
	restClient rest.Interface
}

// NewKubeClientForConfig constructor
func NewClient(clientset *kubernetes.Clientset) (*Client, error) {
	return &Client{
		restClient: clientset.Discovery().RESTClient(),
	}, nil
}
