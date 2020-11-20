package kubernetes

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Client struct {
	RestConfig rest.Config          `json:"restconfig,omitempty"`
	Clientset  kubernetes.Interface `json:"clientset,omitempty"`
}

func New(clientset kubernetes.Interface, cfg rest.Config) (*Client, error) {
	return &Client{
		Clientset:  clientset,
		RestConfig: cfg,
	}, nil
}
