package controllers

import (
	"context"
	"fmt"
	"sync"

	mesherykube "github.com/layer5io/meshkit/utils/kubernetes"
	kubeerror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubectl/pkg/polymorphichelpers"
)

type mesheryOperator struct {
	name           string
	status         MesheryControllerStatus
	client         *mesherykube.Client
	deploymentConf OperatorDeploymentConfig
	mx             sync.Mutex
}

type OperatorDeploymentConfig struct {
	GetHelmOverrides      func(delete bool) map[string]interface{}
	HelmChartRepo         string
	MesheryReleaseVersion string
}

func NewMesheryOperatorHandler(client *mesherykube.Client, deploymentConf OperatorDeploymentConfig) IMesheryController {
	return &mesheryOperator{
		name:           "MesheryOperator",
		status:         Unknown,
		client:         client,
		deploymentConf: deploymentConf,
	}
}

func (mo *mesheryOperator) GetName() string {
	return mo.name
}

func (mo *mesheryOperator) GetStatus() MesheryControllerStatus {
	if mo.status == Undeployed {
		return Undeployed
	}
	// check if the deployment exists
	deployment, err := mo.client.DynamicKubeClient.Resource(schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}).Namespace("meshery").Get(context.TODO(), "meshery-operator", metav1.GetOptions{})
	if err != nil {
		if kubeerror.IsNotFound(err) {
			mo.setStatus(NotDeployed)
			return mo.status
		}
		return Unknown
	}

	sv, err := polymorphichelpers.StatusViewerFor(deployment.GroupVersionKind().GroupKind())
	if err != nil {
		mo.setStatus(Unknown)
		return mo.status
	}
	_, done, err := sv.Status(deployment, 0)
	if err != nil {
		mo.setStatus(Unknown)
		return mo.status
	}
	if done {
		mo.setStatus(Deployed)
	} else {
		mo.setStatus(Deploying)
	}
	return mo.status
}

func (mo *mesheryOperator) Deploy(force bool) error {
	status := mo.GetStatus()
	if status == Undeployed && !force {
		return nil
	}
	if status == Deploying {
		return ErrDeployController(fmt.Errorf("Already a Meshery Operator is being deployed."))
	}
	err := applyOperatorHelmChart(mo.deploymentConf.HelmChartRepo, *mo.client, mo.deploymentConf.MesheryReleaseVersion, false, mo.deploymentConf.GetHelmOverrides(false))
	if err != nil {
		return ErrDeployController(err)
	}
	mo.setStatus(Deployed)
	return nil
}
func (mo *mesheryOperator) Undeploy() error {
	err := applyOperatorHelmChart(mo.deploymentConf.HelmChartRepo, *mo.client, mo.deploymentConf.MesheryReleaseVersion, true, mo.deploymentConf.GetHelmOverrides(false))
	if err != nil {
		return ErrDeployController(err)
	}
	mo.setStatus(Undeployed)
	return nil
}

func (mo *mesheryOperator) GetPublicEndpoint() (string, error) {
	return "", nil
}

func (mo *mesheryOperator) GetVersion() (string, error) {
	deployment, err := mo.client.KubeClient.AppsV1().Deployments("meshery").Get(context.TODO(), "meshery-operator", metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return getImageVersionOfContainer(deployment.Spec.Template, "manager"), nil
}

func (mo *mesheryOperator) setStatus(st MesheryControllerStatus) {
	mo.mx.Lock()
	defer mo.mx.Unlock()
	mo.status = st
}

func (mo *mesheryOperator) GeEndpointForPort(portName string) (string, error) {
	return "", nil
}