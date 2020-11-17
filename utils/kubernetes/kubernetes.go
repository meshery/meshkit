package kubernetes

import (
	"context"

	"github.com/layer5io/meshkit/errors"
	"github.com/layer5io/meshkit/utils"
	coreV1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// GetServiceEndpoint returns the endpoint for the given service
func (cfg *Config) GetServiceEndpoint(ctx context.Context, svcName, namespace string) (*utils.Endpoint, error) {
	svc, err := cfg.Clientset.CoreV1().Services(namespace).Get(ctx, svcName, v1.GetOptions{})
	if err != nil {
		return nil, errors.NewDefault(errors.ErrServiceDiscovery, "Error finding the service"+err.Error())
	}

	// Try loadbalancer endpoint
	if endpoint := extractLoadBalancerEndpoint(svc); endpoint != nil {
		return endpoint, nil
	}

	// Try nodeport endpoint
	nodes, err := cfg.Clientset.CoreV1().Nodes().List(ctx, v1.ListOptions{})
	if err != nil {
		return nil, errors.NewDefault(errors.ErrServiceDiscovery, "Error getting the cluster nodes"+err.Error())
	}
	if endpoint := extractNodePortEndpoint(svc, nodes); endpoint != nil {
		return endpoint, nil
	}

	// Try clusterip endpoint
	if endpoint := extractClusterIPEndpoint(svc); endpoint != nil {
		return endpoint, nil
	}

	return nil, err
}

// extractLoadBalancerEndpoint extracts loadbalancer based endpoint, if any.
// It returns the nil if no valid endpoint is extracted
func extractLoadBalancerEndpoint(svc *coreV1.Service) *utils.Endpoint {
	ports := svc.Spec.Ports
	ingresses := svc.Status.LoadBalancer.Ingress

	for _, ingress := range ingresses {
		var address string
		if ingress.Hostname != "" && ingress.Hostname != "None" {
			address = ingress.Hostname
		} else if ingress.IP != "" {
			address = ingress.IP
		} else {
			// If no valid ip address and hostname is found
			// then move on to the next ingress
			continue
		}

		for _, port := range ports {
			return &utils.Endpoint{
				Name:    svc.GetName(),
				Address: address,
				Port:    port.Port,
			}
		}
	}

	return nil
}

// extractNodePortEndpoint extracts nodeport based endpoint, if any.
// It returns the nil if no valid endpoint is extracted
func extractNodePortEndpoint(svc *coreV1.Service, nl *coreV1.NodeList) *utils.Endpoint {
	ports := svc.Spec.Ports

	for _, node := range nl.Items {
		for _, addressData := range node.Status.Addresses {
			if addressData.Type == "InternalIP" {
				address := addressData.Address

				for _, port := range ports {
					// nodeport 0 is an invalid nodeport
					if port.NodePort != 0 {
						return &utils.Endpoint{
							Name:    svc.GetName(),
							Address: address,
							Port:    port.NodePort,
						}
					}
				}
			}
		}
	}
	return nil
}

// extractClusterIPEndpoint extracts clusterIP based endpoint, if any.
// It returns the nil if no valid endpoint is extracted
func extractClusterIPEndpoint(svc *coreV1.Service) *utils.Endpoint {
	ports := svc.Spec.Ports
	clusterIP := svc.Spec.ClusterIP

	for _, port := range ports {
		return &utils.Endpoint{
			Name:    svc.GetName(),
			Address: clusterIP,
			Port:    port.Port,
		}
	}
	return nil
}
