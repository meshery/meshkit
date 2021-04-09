package kubernetes

import (
	"context"
	"net/url"

	"github.com/layer5io/meshkit/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ServiceOptions give control of which service to discover and which port to discover.
type ServiceOptions struct {
	Name         string // Name of the kubernetes service
	Namespace    string // Namespace of the kubernetes service
	PortSelector string // To specify the name of the kubernetes service port
	APIServerURL string // Kubernetes api-server URL (Used in-case of minikube)
}

// GetServiceEndpoint returns the endpoint for the given service
func GetServiceEndpoint(ctx context.Context, client *kubernetes.Clientset, opts *ServiceOptions) (*utils.Endpoint, error) {
	obj, err := client.CoreV1().Services(opts.Namespace).Get(ctx, opts.Name, metav1.GetOptions{})
	if err != nil {
		return nil, ErrServiceDiscovery(err)
	}

	return GetEndpoint(ctx, opts, obj)
}

// GetEndpoint returns those endpoints in the given service which match the selector. Eg: service name = "client"
func GetEndpoint(ctx context.Context, opts *ServiceOptions, obj *corev1.Service) (*utils.Endpoint, error) {
	var nodePort, clusterPort int32
	endpoint := utils.Endpoint{}

	for _, port := range obj.Spec.Ports {
		if opts.PortSelector != "" && port.Name == opts.PortSelector {
			nodePort = port.NodePort
			clusterPort = port.Port
			break
		} else {
			nodePort = port.NodePort
			clusterPort = port.Port
		}
	}

	// get clusterip endpoint
	endpoint.Internal = &utils.HostPort{
		Address: obj.Spec.ClusterIP,
		Port:    clusterPort,
	}

	endpoint.External = &utils.HostPort{
		Address: "localhost",
		Port:    clusterPort,
	}

	if obj.Status.Size() > 0 && obj.Status.LoadBalancer.Size() > 0 && len(obj.Status.LoadBalancer.Ingress) > 0 && obj.Status.LoadBalancer.Ingress[0].Size() > 0 {
		if obj.Status.LoadBalancer.Ingress[0].IP == "" {
			if obj.Status.LoadBalancer.Ingress[0].Hostname == "localhost" {
				endpoint.External.Address = "host.docker.internal"
			} else {
				endpoint.External.Address = obj.Status.LoadBalancer.Ingress[0].Hostname
			}
		} else if obj.Status.LoadBalancer.Ingress[0].IP == obj.Spec.ClusterIP {
			endpoint.External.Port = nodePort
			url, err := url.Parse(opts.APIServerURL)
			if err != nil {
				return nil, ErrInvalidAPIServer
			}
			endpoint.External.Address = url.Host
		} else {
			endpoint.External.Address = obj.Status.LoadBalancer.Ingress[0].IP
		}
	}

	if !utils.TcpCheck(endpoint.External) {
		url, err := url.Parse(opts.APIServerURL)
		if err != nil {
			return nil, ErrInvalidAPIServer
		}
		endpoint.External.Address = url.Host
		if !utils.TcpCheck(endpoint.External) {
			endpoint.External.Port = nodePort
			if !utils.TcpCheck(endpoint.External) {
				return nil, ErrEndpointNotFound
			}
		}
	}

	return &endpoint, nil
}
