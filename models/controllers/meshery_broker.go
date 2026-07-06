package controllers

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	opClient "github.com/meshery/meshery-operator/pkg/client"
	"github.com/meshery/meshkit/logger"
	"github.com/meshery/meshkit/utils"
	mesherykube "github.com/meshery/meshkit/utils/kubernetes"
	v1 "k8s.io/api/core/v1"
	kubeerror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubectl/pkg/polymorphichelpers"
)

var (
	brokerMonitoringPortName = "monitor"

	// natsResourceNames lists the names the operator may use for the NATS
	// broker's StatefulSet/Service, newest first. Older operators deployed a
	// StatefulSet/Service literally named "meshery-broker"; operator >= 1.0.2
	// deploys NATS via the upstream Helm chart, naming them "meshery-nats".
	natsResourceNames = []string{MesheryBroker, "meshery-nats"}
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
	if mb.kclient == nil {
		return Unknown
	}
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
		// when operatorClient is not able to get meshery-broker, we try again with
		// kubernetes client as a fallback, trying each known NATS StatefulSet name.
		stsClient := mb.kclient.DynamicKubeClient.Resource(schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}).Namespace("meshery")
		var broker *unstructured.Unstructured
		var stsErr error
		for _, name := range natsResourceNames {
			broker, stsErr = stsClient.Get(context.TODO(), name, metav1.GetOptions{})
			if stsErr == nil {
				break
			}
		}
		if stsErr != nil {
			// if the resource is not found, then it is NotDeployed
			if kubeerror.IsNotFound(stsErr) {
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
	if err != nil {
		return "", ErrGetControllerPublicEndpoint(err)
	}
	// Newer operators deploy NATS as a ClusterIP-only Service, so the Broker CRD
	// publishes only an internal endpoint (External is empty). Requiring an
	// external endpoint here made Meshery give up before even trying the
	// internal / host.docker.internal / API-host fallbacks in GetBrokerEndpoint,
	// so a host-run Meshery could never obtain a broker endpoint. Only fail when
	// the broker has published no endpoint at all.
	if broker.Status.Endpoint.Internal == "" && broker.Status.Endpoint.External == "" {
		return "", ErrGetControllerPublicEndpoint(fmt.Errorf("broker has no published endpoint (neither internal nor external)"))
	}

	endpoint := GetBrokerEndpoint(mb.kclient, broker)
	if endpoint == "" {
		return "", ErrGetControllerPublicEndpoint(fmt.Errorf(
			"no reachable endpoint for meshery-broker (published internal=%q external=%q)",
			broker.Status.Endpoint.Internal, broker.Status.Endpoint.External,
		))
	}
	return endpoint, nil
}

func (mb *mesheryBroker) GetVersion() (string, error) {
	if mb.kclient == nil {
		return "", fmt.Errorf("kubernetes client is not initialized")
	}
	var lastErr error
	for _, name := range natsResourceNames {
		statefulSet, err := mb.kclient.KubeClient.AppsV1().StatefulSets("meshery").Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			lastErr = err
			continue
		}
		return getImageVersionOfContainer(statefulSet.Spec.Template, "nats"), nil
	}
	return "", lastErr
}

// natsAuthSecretName / natsAuthTokenKey follow the operator's convention for the
// NATS token secret (operator >= 1.0.2 provisions NATS with token auth:
// `authorization { token: $NATS_TOKEN }`, token stored in this secret).
const (
	natsAuthSecretName = "meshery-nats-auth"
	natsAuthTokenKey   = "token"
)

// GetToken returns the NATS authorization token the operator provisioned for the
// broker (secret meshery-nats-auth, key token). It returns ("", nil) when the
// secret is absent — an unauthenticated broker — so callers stay backward
// compatible with tokenless deployments.
func (mb *mesheryBroker) GetToken() (string, error) {
	if mb.kclient == nil {
		return "", fmt.Errorf("kubernetes client is not initialized")
	}
	secret, err := mb.kclient.KubeClient.CoreV1().Secrets("meshery").Get(context.TODO(), natsAuthSecretName, metav1.GetOptions{})
	if err != nil {
		if kubeerror.IsNotFound(err) {
			return "", nil
		}
		return "", err
	}
	return string(secret.Data[natsAuthTokenKey]), nil
}

// natsClientPort is the NATS client port to port-forward to.
const natsClientPort = 4222

// GetPortForwarder returns a self-healing port-forwarder to the broker's NATS
// pod, so an out-of-cluster Meshery can reach a ClusterIP-only broker without a
// manual `kubectl port-forward`. The caller owns Start()/Stop(). The pod is
// selected by the upstream NATS chart label and re-resolved on reconnect.
func (mb *mesheryBroker) GetPortForwarder(log logger.Handler) (*mesherykube.PortForwarder, error) {
	return mesherykube.NewPortForwarder(mb.kclient, mesherykube.PortForwardTarget{
		Namespace:  "meshery",
		PodLabels:  map[string]string{"app.kubernetes.io/name": "nats"},
		RemotePort: natsClientPort,
	}, log)
}

func (mb *mesheryBroker) GetEndpointForPort(portName string) (string, error) {
	if mb.kclient == nil {
		return "", ErrGetControllerEndpointForPort(fmt.Errorf("kubernetes client is not initialized"))
	}
	// Resolve the broker Service under each known NATS Service name. Older
	// operators named it "meshery-broker"; operator >= 1.0.2 names it
	// "meshery-nats". Looking up only "meshery-broker" made the monitoring-port
	// (and thus the connectivity) lookup fail against newer deployments.
	var endpoint *utils.Endpoint
	var err error
	for _, name := range natsResourceNames {
		endpoint, err = mesherykube.GetServiceEndpoint(context.TODO(), mb.kclient.KubeClient, &mesherykube.ServiceOptions{
			Name:         name,
			Namespace:    "meshery",
			PortSelector: portName,
		})
		if err == nil {
			break
		}
	}
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
