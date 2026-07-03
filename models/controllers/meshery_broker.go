package controllers

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	opClient "github.com/meshery/meshery-operator/pkg/client"
	"github.com/meshery/meshkit/utils"
	mesherykube "github.com/meshery/meshkit/utils/kubernetes"
	v1 "k8s.io/api/core/v1"
	kubeerror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubectl/pkg/polymorphichelpers"
)

var (
	brokerMonitoringPortName = "monitor"
)

type mesheryBroker struct {
	name    string
	status  MesheryControllerStatus
	kclient *mesherykube.Client
}

func NewMesheryBrokerHandler(kubernetesClient *mesherykube.Client) IMesheryController {
	return &mesheryBroker{
		name:    "MesheryBroker",
		status:  Unknown,
		kclient: kubernetesClient,
	}
}

func (mb *mesheryBroker) GetName() string {
	return mb.name
}

func (mb *mesheryBroker) GetStatus() MesheryControllerStatus {
	operatorClient, err := opClient.New(&mb.kclient.RestConfig)
	if err != nil || operatorClient == nil {
		return Unknown
	}
	// TODO: Confirm if the presence of operator is needed to use the operator client sdk
	_, err = operatorClient.CoreV1Alpha1().Brokers("meshery").Get(context.TODO(), "meshery-broker", metav1.GetOptions{})
	if err == nil {
		var monitoringEndpoint string
		monitoringEndpoint, err = mb.GetEndpointForPort(brokerMonitoringPortName)
		if err == nil {
			if ConnectivityTest(MesheryServer, monitoringEndpoint) {
				mb.status = Connected
				return mb.status
			}
		}
		mb.status = Deployed
		return mb.status
	} else {
		if kubeerror.IsNotFound(err) {
			if mb.status != Undeployed {
				mb.status = Undeployed
			}
			return mb.status
		}
		// when operatorClient is not able to get meshesry-broker, we try again with kubernetes client as a fallback
		broker, err := mb.kclient.DynamicKubeClient.Resource(schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}).Namespace("meshery").Get(context.TODO(), MesheryBroker, metav1.GetOptions{})
		if err != nil {
			// if the resource is not found, then it is NotDeployed
			if kubeerror.IsNotFound(err) {
				mb.status = Undeployed
				return mb.status
			}
			return Unknown
		}
		mb.status = Deploying
		sv, er := polymorphichelpers.StatusViewerFor(broker.GroupVersionKind().GroupKind())
		if er != nil {
			mb.status = Unknown
			return mb.status
		}
		_, done, statusErr := sv.Status(broker, 0)
		if statusErr != nil {
			mb.status = Unknown
			return mb.status
		}
		if done {
			mb.status = Deployed
		}
		return mb.status
	}
}

func (mb *mesheryBroker) Deploy(force bool) error {
	// deploying the operator will deploy broker. Right now, we don't need to implement this functionality. But we may implement in the future
	return nil
}
func (mb *mesheryBroker) Undeploy() error {
	// currently we do not allow the manual undeployment of broker
	return nil
}

func (mb *mesheryBroker) GetPublicEndpoint() (string, error) {
	operatorClient, err := opClient.New(&mb.kclient.RestConfig)
	if err != nil {
		return "", ErrGetControllerPublicEndpoint(err)
	}
	broker, err := operatorClient.CoreV1Alpha1().Brokers("meshery").Get(context.TODO(), MesheryBroker, metav1.GetOptions{})
	if broker.Status.Endpoint.External == "" {
		if err == nil {
			err = fmt.Errorf("Could not get the External endpoint for meshery-broker")
		}
		// broker is not available
		return "", ErrGetControllerPublicEndpoint(err)
	}

	return GetBrokerEndpoint(mb.kclient, broker), nil
}

func (mb *mesheryBroker) GetVersion() (string, error) {
	statefulSet, err := mb.kclient.KubeClient.AppsV1().StatefulSets("meshery").Get(context.TODO(), MesheryBroker, metav1.GetOptions{})
	if kubeerror.IsNotFound(err) {
		return "", err
	}
	return getImageVersionOfContainer(statefulSet.Spec.Template, "nats"), nil
}

func (mb *mesheryBroker) GetEndpointForPort(portName string) (string, error) {
	endpoint, err := mesherykube.GetServiceEndpoint(context.TODO(), mb.kclient.KubeClient, &mesherykube.ServiceOptions{
		Name:         "meshery-broker",
		Namespace:    "meshery",
		PortSelector: portName,
	})
	if err != nil {
		return "", ErrGetControllerEndpointForPort(err)
	}

	// Probe the broker the same way the data connection does (see
	// GetBrokerEndpoint), so a status check can't report "not connected" while
	// the data path is actually up. Candidates, in order of how Meshery is
	// commonly deployed relative to the cluster:
	//   1. internal            — Meshery running in-cluster
	//   2. external            — Meshery out-of-cluster, broker exposed (NodePort/LB)
	//   3. host.docker.internal— Meshery in Docker Desktop reaching the host
	//   4. API server host     — reuse the kube API host with the broker port
	// The former "external first, internal only" ordering both preferred the
	// wrong endpoint in-cluster and gave up before trying the Docker/API-host
	// fallbacks, which is the main source of false "not connected" reports.
	candidates := []*utils.HostPort{endpoint.Internal, endpoint.External}

	var port int32
	if endpoint.External != nil && endpoint.External.Port != 0 {
		port = endpoint.External.Port
	} else if endpoint.Internal != nil {
		port = endpoint.Internal.Port
	}
	if port != 0 {
		candidates = append(candidates, &utils.HostPort{Address: "host.docker.internal", Port: port})
		if u, perr := url.Parse(mb.kclient.RestConfig.Host); perr == nil && u.Hostname() != "" {
			candidates = append(candidates, &utils.HostPort{Address: u.Hostname(), Port: port})
		}
	}

	for _, ep := range candidates {
		if ep != nil && ep.Address != "" {
			if utils.TcpCheck(ep, nil) {
				return ep.String(), nil
			}
		}
	}

	return "", ErrGetControllerEndpointForPort(
		fmt.Errorf("no reachable endpoint (internal/external/host.docker.internal/api-host) for meshery-broker port %s", portName),
	)
}

func getImageVersionOfContainer(container v1.PodTemplateSpec, containerName string) string {
	var version string
	for _, container := range container.Spec.Containers {
		if strings.Compare(container.Name, containerName) == 0 {
			versionTag := strings.Split(container.Image, ":")
			if len(versionTag) > 1 {
				version = versionTag[1]
			}
		}
	}
	return version
}
