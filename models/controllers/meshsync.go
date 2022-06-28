package controllers

import (
	"context"

	// opClient "github.com/layer5io/meshery-operator/pkg/client"
	mesherykube "github.com/layer5io/meshkit/utils/kubernetes"
	kubeerror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type meshsync struct {
	name    string
	status  MesheryControllerStatus
	kclient *mesherykube.Client
}

func NewMeshsyncHandler(kubernetesClient *mesherykube.Client) IMesheryController {
	return &meshsync{
		name:    "Meshsync",
		status:  Unknown,
		kclient: kubernetesClient,
	}
}

func (ms *meshsync) GetName() string {
	return ms.name
}

func (ms *meshsync) GetStatus() MesheryControllerStatus {
	meshsync, err := ms.kclient.KubeClient.AppsV1().Deployments("meshery").Get(context.TODO(), "meshery-meshsync", metav1.GetOptions{})
	// if the deployment is not found, then it is NotDeployed
	if err != nil && !kubeerror.IsNotFound(err) {
		ms.status = NotDeployed
		return ms.status
	}
	ms.status = Deploying
	if mesherykube.IsDeploymentDone(*meshsync) {
		ms.status = Deployed
	}
	return ms.status
}

func (ms *meshsync) Deploy() error {
	// deploying the operator will deploy meshsync. Right now, we don't need to implement this functionality. But we may implement in the future
	return nil
}

func (ms *meshsync) GetPublicEndpoint() (string, error) {
	return "", nil
}

func (ms *meshsync) GetVersion() (string, error) {
	return "", nil
}
