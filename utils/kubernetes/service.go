package kubernetes

import (
	"context"
	"net"
	"net/url"

	"github.com/meshery/meshkit/utils"
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
	WorkerNodeIP string // Kubernetes worker node IP address (Any), in case of a kubeadm based cluster orchestration
	Mock         *utils.MockOptions
}

// GetServiceEndpoint returns the endpoint for the given service
func GetServiceEndpoint(ctx context.Context, client kubernetes.Interface, opts *ServiceOptions) (*utils.Endpoint, error) {
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
	if opts.WorkerNodeIP == "" {
		opts.WorkerNodeIP = "localhost"
	}
	for _, port := range obj.Spec.Ports {
		nodePort = port.NodePort
		clusterPort = port.Port
		if opts.PortSelector != "" && port.Name == opts.PortSelector {
			break
		}
	}
	// get clusterip endpoint
	endpoint.Internal = &utils.HostPort{
		Address: obj.Spec.ClusterIP,
		Port:    clusterPort,
	}
	// Initialize nodePort type endpoint
	endpoint.External = &utils.HostPort{
		Address: opts.WorkerNodeIP,
		Port:    nodePort,
	}
	if obj.Status.Size() > 0 && obj.Status.LoadBalancer.Size() > 0 && len(obj.Status.LoadBalancer.Ingress) > 0 && obj.Status.LoadBalancer.Ingress[0].Size() > 0 {
		if obj.Status.LoadBalancer.Ingress[0].IP == "" { //nolint:staticcheck
			endpoint.External.Address = obj.Status.LoadBalancer.Ingress[0].Hostname
			endpoint.External.Port = clusterPort
		} else if obj.Status.LoadBalancer.Ingress[0].IP == obj.Spec.ClusterIP || obj.Status.LoadBalancer.Ingress[0].IP == "<pending>" {
			if opts.APIServerURL != "" {
				url, err := url.Parse(opts.APIServerURL)
				if err != nil {
					return nil, ErrInvalidAPIServer
				}
				host, _, err := net.SplitHostPort(url.Host)
				if err != nil {
					return nil, ErrInvalidAPIServer
				}
				endpoint.External.Address = host
				endpoint.External.Port = nodePort
			} else {
				endpoint.External.Address = obj.Spec.ClusterIP
				endpoint.External.Port = clusterPort
			}
		} else {
			endpoint.External.Address = obj.Status.LoadBalancer.Ingress[0].IP
			endpoint.External.Port = clusterPort
		}
	}
	// Service Type ClusterIP
	if endpoint.External.Port == 0 {
		return &utils.Endpoint{
			Internal: endpoint.Internal,
		}, nil
	}
	// If external endpoint not reachable
	if !utils.TcpCheck(endpoint.External, opts.Mock) && endpoint.External.Address != "localhost" {
		url, err := url.Parse(opts.APIServerURL)
		if err != nil {
			return &endpoint, ErrInvalidAPIServer
		}
		host, _, err := net.SplitHostPort(url.Host)
		if err != nil {
			return &endpoint, ErrInvalidAPIServer
		}
		// Set to APIServer host (For minikube specific clusters)
		endpoint.External.Address = host
		// If still unable to reach, change to resolve to clusterPort
		if !utils.TcpCheck(endpoint.External, opts.Mock) && endpoint.External.Address != "localhost" {
			endpoint.External.Port = nodePort
			if !utils.TcpCheck(endpoint.External, opts.Mock) {
				return &endpoint, ErrEndpointNotFound
			}
		}
	}
	return &endpoint, nil
}
