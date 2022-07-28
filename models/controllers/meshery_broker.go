package controllers

import (
	"context"
	"fmt"
	"strings"

	opClient "github.com/layer5io/meshery-operator/pkg/client"
	mesherykube "github.com/layer5io/meshkit/utils/kubernetes"
	v1 "k8s.io/api/core/v1"
	kubeerror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubectl/pkg/polymorphichelpers"
)

type mesheryBroker struct {
	name    string
	status  MesheryControllerStatus
	kclient *mesherykube.Client
	version string
}

func NewMesheryBrokerHandler(kubernetesClient *mesherykube.Client) IMesheryController {
	return &mesheryBroker{
		name:    "MesheryBroker",
		status:  Unknown,
		kclient: kubernetesClient,
		version: "",
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
	broker, err := operatorClient.CoreV1Alpha1().Brokers("meshery").Get(context.TODO(), "meshery-broker", metav1.GetOptions{})
	if err == nil {
		if broker.Status.Endpoint.External != "" {
			mb.status = Deployed
			return mb.status
		}
		mb.status = NotDeployed
		return mb.status
	} else {
		if kubeerror.IsNotFound(err) {
			mb.status = NotDeployed
			return mb.status
		}
		// when operatorClient is not able to get meshesry-broker, we try again with kubernetes client as a fallback
		broker, err := mb.kclient.DynamicKubeClient.Resource(schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}).Namespace("meshery").Get(context.TODO(), "meshery-broker", metav1.GetOptions{})
		if err != nil {
			// if the resource is not found, then it is NotDeployed
			if kubeerror.IsNotFound(err) {
				mb.status = NotDeployed
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

func (mb *mesheryBroker) Deploy() error {
	// deploying the operator will deploy broker. Right now, we don't need to implement this functionality. But we may implement in the future
	return nil
}

func (mb *mesheryBroker) GetPublicEndpoint() (string, error) {
	operatorClient, err := opClient.New(&mb.kclient.RestConfig)
	if err != nil {
		return "", ErrGetControllerPublicEndpoint(err)
	}
	broker, err := operatorClient.CoreV1Alpha1().Brokers("meshery").Get(context.TODO(), "meshery-broker", metav1.GetOptions{})
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
	if len(mb.version) == 0 {
		statefulSet, err := mb.kclient.KubeClient.AppsV1().StatefulSets("meshery").Get(context.TODO(), "meshery-broker", metav1.GetOptions{})
		if kubeerror.IsNotFound(err) {
			return "", err
		}
		return getImageVersionOfContainer(statefulSet.Spec.Template, "nats"), nil
	}
	return mb.version, nil
}

func getImageVersionOfContainer(container v1.PodTemplateSpec, containerName string) string {
	var version string
	for _, container := range container.Spec.Containers {
		if strings.Compare(container.Name, containerName) == 0 {
			version = strings.Split(container.Image, ":")[1]
		}
	}
	return version
}
