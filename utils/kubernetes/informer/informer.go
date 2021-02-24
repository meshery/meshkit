package informer

import (
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

type Config struct {
	Group    string
	Version  string
	Resource string
}

type Client struct {
	informerFactory informers.SharedInformerFactory
}

func NewClient(clientset *kubernetes.Clientset) (*Client, error) {
	informerFactory := informers.NewSharedInformerFactory(clientset, 100*time.Second)

	return &Client{
		informerFactory: informerFactory,
	}, nil
}
