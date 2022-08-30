package kubernetes

import (
	"bytes"
	"fmt"
	"strings"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
)

// Though we are using the same config that is used for installing/uninstalling helm charts.
// We will only make use of URL/ChartLocation/LocalPath to get and load the helm chart
func ConvertHelmChartToK8sManifest(cfg ApplyHelmChartConfig) (manifest []byte, err error) {
	setupDefaults(&cfg)
	if err := setupChartVersion(&cfg); err != nil {
		return nil, ErrApplyHelmChart(err)
	}

	localPath, err := getHelmLocalPath(cfg)
	if err != nil {
		return nil, ErrApplyHelmChart(err)
	}

	helmChart, err := loader.Load(localPath)
	if err != nil {
		return nil, ErrApplyHelmChart(err)
	}
	actconfig := new(action.Configuration)
	act := action.NewInstall(actconfig)
	act.ReleaseName = "test-release"
	act.CreateNamespace = true
	act.Namespace = "default"
	act.DryRun = true
	act.IncludeCRDs = true
	act.ClientOnly = true
	rel, err := act.Run(helmChart, nil)
	if err != nil {
		return nil, ErrApplyHelmChart(err)
	}
	var manifests bytes.Buffer
	fmt.Fprintln(&manifests, strings.TrimSpace(rel.Manifest))
	manifest = manifests.Bytes()
	return
}
