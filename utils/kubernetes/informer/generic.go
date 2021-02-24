package informer

// import (
// 	"fmt"

// 	"k8s.io/apimachinery/pkg/runtime/schema"
// 	"k8s.io/client-go/tools/cache"
// )

// func (c *Client) Get(config Config) error {
// 	genericInformer, err := c.informerFactory.ForResource(schema.GroupVersionResource{
// 		Group:    config.Group,
// 		Version:  config.Version,
// 		Resource: config.Resource,
// 	})
// 	if err != nil {
// 		return err
// 	}

// 	stopCh := make(chan struct{})
// 	go startWatching(stopCh, genericInformer.Informer())

// 	return nil
// }

// func startWatching(stopCh <-chan struct{}, s cache.SharedIndexInformer) {
// 	handlers := cache.ResourceEventHandlerFuncs{
// 		AddFunc: func(obj interface{}) {
// 			fmt.Println("received add event!")
// 		},
// 		UpdateFunc: func(oldObj, obj interface{}) {
// 			fmt.Println("received update event!")
// 		},
// 		DeleteFunc: func(obj interface{}) {
// 			fmt.Println("received update event!")
// 		},
// 	}
// 	s.AddEventHandler(handlers)
// 	s.Run(stopCh)
// }
