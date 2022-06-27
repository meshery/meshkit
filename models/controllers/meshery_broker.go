package controllers

import (
	"context"
	"fmt"

	opClient "github.com/layer5io/meshery-operator/pkg/client"
	mesherykube "github.com/layer5io/meshkit/utils/kubernetes"
	kubeerror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	broker, err := mb.kclient.KubeClient.AppsV1().Deployments("meshery").Get(context.TODO(), "meshery-broker", metav1.GetOptions{})
	// if the deployment is not found, then it is NotDeployed
	if err != nil && !kubeerror.IsNotFound(err) {
		mb.status = NotDeployed
		return mb.status
	}
	mb.status = Deploying
	if mesherykube.IsDeploymentDone(*broker) {
		mb.status = Deployed
	}
	return mb.status
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
	return "", nil
}
