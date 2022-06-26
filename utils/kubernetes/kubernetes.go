package kubernetes

import (
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	deploymentutil "k8s.io/kubectl/pkg/util/deployment"
)

type Client struct {
	RestConfig        rest.Config           `json:"restconfig,omitempty"`
	KubeClient        *kubernetes.Clientset `json:"kubeclient,omitempty"`
	DynamicKubeClient dynamic.Interface     `json:"dynamic_kubeclient,omitempty"`
}

func New(kubeconfig []byte) (*Client, error) {
	restConfig, err := DetectKubeConfig(kubeconfig)
	if err != nil {
		return nil, err
	}
	restConfig.QPS = float32(50)
	restConfig.Burst = int(100)

	// Configure kubeclient
	kclient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, ErrNewKubeClient(err)
	}

	// Configure dynamic kubeclient
	dyclient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, ErrNewDynClient(err)
	}

	return &Client{
		RestConfig:        *restConfig,
		DynamicKubeClient: dyclient,
		KubeClient:        kclient,
	}, nil
}

// checks if deployment is done.
func IsDeploymentDone(deployment appsv1.Deployment) bool {
	status := false
	if deployment.Generation <= deployment.Status.ObservedGeneration {
		cond := deploymentutil.GetDeploymentCondition(deployment.Status, appsv1.DeploymentProgressing)
		if cond != nil && cond.Reason == deploymentutil.TimedOutReason {
			// deployment exceeded its progress deadline
			return status
		}
		if deployment.Spec.Replicas != nil && deployment.Status.UpdatedReplicas < *deployment.Spec.Replicas {
			// Waiting for deployment rollout to finish...
			return status
		}
		if deployment.Status.Replicas > deployment.Status.UpdatedReplicas {
			// Waiting for deployment rollout to finish...
			return status
		}
		if deployment.Status.AvailableReplicas < deployment.Status.UpdatedReplicas {
			// Waiting for deployment rollout to finish...
			return status
		}
		status = true
	}
	return status
}
