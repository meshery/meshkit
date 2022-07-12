package controllers

import (
	"context"

	// opClient "github.com/layer5io/meshery-operator/pkg/client"
	opClient "github.com/layer5io/meshery-operator/pkg/client"
	mesherykube "github.com/layer5io/meshkit/utils/kubernetes"
	kubeerror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubectl/pkg/polymorphichelpers"
)

type meshsync struct {
	name    string
	status  MesheryControllerStatus
	kclient *mesherykube.Client
}

func NewMeshsyncHandler(kubernetesClient *mesherykube.Client) IMesheryController {
	return &meshsync{
		name:    "MeshSync",
		status:  Unknown,
		kclient: kubernetesClient,
	}
}

func (ms *meshsync) GetName() string {
	return ms.name
}

func (ms *meshsync) GetStatus() MesheryControllerStatus {
	operatorClient, _ := opClient.New(&ms.kclient.RestConfig)
	// TODO: Confirm if the presence of operator is needed to use the operator client sdk
	meshSync, err := operatorClient.CoreV1Alpha1().MeshSyncs("meshery").Get(context.TODO(), "meshery-meshsync", metav1.GetOptions{})
	if err == nil {
		if meshSync.Status.PublishingTo != "" {
			ms.status = Deployed
			return ms.status
		}
		ms.status = NotDeployed
		return ms.status
	} else {
		if kubeerror.IsNotFound(err) {
			ms.status = NotDeployed
			return ms.status
		}
		// when we are not able to get meshSync resource from OperatorClient, we try to get it using kubernetes client
		meshSync, err := ms.kclient.DynamicKubeClient.Resource(schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}).Namespace("meshery").Get(context.TODO(), "meshery-meshsync", metav1.GetOptions{})
		if err != nil {
			// if the resource is not found, then it is NotDeployed
			if kubeerror.IsNotFound(err) {
				ms.status = NotDeployed
				return ms.status
			}
			return Unknown
		}
		ms.status = Deploying
		sv, err := polymorphichelpers.StatusViewerFor(meshSync.GroupVersionKind().GroupKind())
		if err != nil {
			ms.status = Unknown
			return ms.status
		}
		_, done, err := sv.Status(meshSync, 0)
		if err != nil {
			ms.status = Unknown
			return ms.status
		}
		if done {
			ms.status = Deployed
		}
		return ms.status
	}
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
