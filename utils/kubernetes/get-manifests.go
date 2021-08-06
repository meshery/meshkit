package kubernetes

import (
	"fmt"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func GetManifestsFromHelm(client *Client, url string) (string, error) {
	chartLocation, err := fetchHelmChart(url)
	if err != nil {
		return "", nil
	}
	chart, err := loader.Load(chartLocation)
	if err != nil {
		return "", err
	}
	if err := checkIfInstallable(chart); err != nil {
		return "", ErrApplyHelmChart(err)
	}
	// KubeConfig setup
	kubeConfig := genericclioptions.NewConfigFlags(false)
	kubeConfig.APIServer = &client.RestConfig.Host
	kubeConfig.BearerToken = &client.RestConfig.BearerToken
	kubeConfig.CAFile = &client.RestConfig.CAFile

	nopLogger := func(_ string, _ ...interface{}) {} // Dummy logger for helm packages

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(kubeConfig, "default", "", nopLogger); err != nil {
		return "", ErrApplyHelmChart(err)
	}
	p := getter.All(cli.New())
	valueOpts := values.Options{FileValues: []string{}}
	vals, err := valueOpts.MergeValues(p)
	if err != nil {
		return "", err
	}
	ccli := action.NewInstall(actionConfig)
	ccli.ReleaseName = "test" //To be changed to something unique dynamically. Because everytime a new relase name is expected by helm
	rel, err := ccli.Run(chart, vals)
	if err != nil {
		fmt.Println(err.Error())
	}
	return rel.Manifest, nil
}
