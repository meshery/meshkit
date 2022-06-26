package controllers

import (
	"context"
	"fmt"

	mesherykube "github.com/layer5io/meshkit/utils/kubernetes"
	kubeerror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type mesheryOperator struct {
	name           string
	status         MesheryControllerStatus
	client         *mesherykube.Client
	deploymentConf OperatorDeploymentConfig
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
	// check if the deployment exists
	deployment, err := mo.client.KubeClient.AppsV1().Deployments("meshery").Get(context.TODO(), "meshery-operator", metav1.GetOptions{})
	if err != nil {
		if kubeerror.IsNotFound(err) {
			mo.status = NotDeployed
			return mo.status
		}
		return Unknown
	}

	mo.status = Deploying
	if mesherykube.IsDeploymentDone(*deployment) {
		mo.status = Deployed
	}
	return mo.status
}

func (mo *mesheryOperator) Deploy() error {
	if mo.status == Deploying {
		return ErrDeployController(fmt.Errorf("Already a Meshery Operator is being deployed."))
	}
	err := applyOperatorHelmChart(mo.deploymentConf.HelmChartRepo, *mo.client, mo.deploymentConf.MesheryReleaseVersion, false, mo.deploymentConf.GetHelmOverrides(false))
	if err != nil {
		return ErrDeployController(err)
	}
	return nil
}

func (mo *mesheryOperator) GetPublicEndpoint() (string, error) {
	return "", nil
}

func (mo *mesheryOperator) GetVersion() (string, error) {
	return "", nil
}
