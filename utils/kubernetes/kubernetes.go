package kubernetes

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Config struct {
	RestConfig rest.Config          `json:"restconfig,omitempty"`
	Clientset  kubernetes.Interface `json:"clientset,omitempty"`
}

func New(clientset kubernetes.Interface, cfg rest.Config) (*Config, error) {
	return &Config{
		Clientset:  clientset,
		RestConfig: cfg,
	}, nil
}
