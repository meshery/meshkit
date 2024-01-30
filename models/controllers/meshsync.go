package controllers

import (
	"context"

	opClient "github.com/layer5io/meshery-operator/pkg/client"
	mesherykube "github.com/layer5io/meshkit/utils/kubernetes"
	v1 "k8s.io/api/core/v1"
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
	_, err := operatorClient.CoreV1Alpha1().MeshSyncs("meshery").Get(context.TODO(), "meshery-meshsync", metav1.GetOptions{})

	if err == nil {
		ms.status = Enabled
		meshSyncPod, errMeshery := ms.kclient.KubeClient.CoreV1().Pods("meshery").List(context.TODO(), metav1.ListOptions{
			LabelSelector: "component=meshsync",
		})

		if len(meshSyncPod.Items) == 0 || kubeerror.IsNotFound(errMeshery) {
			return ms.status
		}
		for _, pod := range meshSyncPod.Items {
			switch pod.Status.Phase {
			case v1.PodRunning:
				ms.status = Running
				broker := NewMesheryBrokerHandler(ms.kclient)
				brokerEndpoint, err := broker.GeEndpointForPort(brokerMonitoringPortName)
				if err != nil {
					return ms.status
				}
				isConnected := ConnectivityTest(MeshSync, brokerEndpoint)
				if isConnected {
					ms.status = Connected
				}
				return ms.status
			default:
				ms.status = Deployed
			}
		}
	} else {
		if kubeerror.IsNotFound(err) {
			if ms.status != Undeployed {
				ms.status = Undeployed
			}
			return ms.status
		}
		// when we are not able to get meshSync resource from OperatorClient, we try to get it using kubernetes client
		meshSync, err := ms.kclient.DynamicKubeClient.Resource(schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}).Namespace("meshery").Get(context.TODO(), "meshery-meshsync", metav1.GetOptions{})
		if err != nil {
			// if the resource is not found, then it is NotDeployed
			if kubeerror.IsNotFound(err) {
				ms.status = Undeployed
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
	return ms.status
}

func (ms *meshsync) Deploy(force bool) error {
	// deploying the operator will deploy meshsync. Right now, we don't need to implement this functionality. But we may implement in the future
	return nil
}
func (ms *meshsync) Undeploy() error {
	// currently we do not allow the manual undeployment of meshsync
	return nil
}

func (ms *meshsync) GetPublicEndpoint() (string, error) {
	return "", nil
}

func (ms *meshsync) GetVersion() (string, error) {
	meshsyncclient, err := opClient.New(&ms.kclient.RestConfig)
	if err != nil {
		return "", err
	}

	meshsyncresource, err := meshsyncclient.CoreV1Alpha1().MeshSyncs("meshery").Get(context.TODO(), "meshery-meshsync", metav1.GetOptions{})

	if err != nil {
		return "", err
	}

	return meshsyncresource.Spec.Version, nil
}

func (mb *meshsync) GeEndpointForPort(portName string) (string, error) {
	return "", nil
}
