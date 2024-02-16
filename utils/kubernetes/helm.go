package kubernetes

import (
	"github.com/layer5io/meshkit/utils/helm"
	"helm.sh/helm/v3/pkg/chart/loader"
)

// Though we are using the same config that is used for installing/uninstalling helm charts.
// We will only make use of URL/ChartLocation/LocalPath to get and load the helm chart
func ConvertHelmChartToK8sManifest(cfg ApplyHelmChartConfig) (manifest []byte, err error) {
	setupDefaults(&cfg)
	if err = setupChartVersion(&cfg); err != nil {
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

	return helm.DryRunHelmChart(helmChart)
}
