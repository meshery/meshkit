package kubernetes

import (
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/release"
)

// HelmReleaseDeployed reports whether the release that ApplyHelmChart would create for cfg
// is already deployed in cfg.Namespace. The release name is cfg.ReleaseName, or the chart's
// own name when that is empty, which is how ApplyHelmChart names it.
//
// It lets callers that apply the same chart more than once skip the install, which Helm
// refuses with "cannot reuse a name that is still in use" while the release exists.
// Only a release in the deployed state counts: a failed or pending release is not usable.
func (c *Client) HelmReleaseDeployed(cfg ApplyHelmChartConfig) (bool, error) {
	setupDefaults(&cfg)

	if err := setupChartVersion(&cfg); err != nil {
		return false, ErrApplyHelmChart(err)
	}

	localPath, err := getHelmLocalPath(cfg)
	if err != nil {
		return false, ErrApplyHelmChart(err)
	}

	helmChart, err := loader.Load(localPath)
	if err != nil {
		return false, ErrApplyHelmChart(err)
	}
	if cfg.ReleaseName == "" {
		cfg.ReleaseName = helmChart.Name()
	}

	actionConfig, err := newHelmActionConfig(c, cfg, c.getRESTClientGetter())
	if err != nil {
		return false, ErrApplyHelmChart(err)
	}

	deployed, err := releaseDeployed(actionConfig, cfg.ReleaseName)
	if err != nil {
		return false, ErrApplyHelmChart(err)
	}
	return deployed, nil
}

// releaseDeployed reports whether a release with the given name is in the deployed state in
// the namespace the action configuration is bound to.
func releaseDeployed(actionConfig *action.Configuration, name string) (bool, error) {
	list := action.NewList(actionConfig)
	// NewList defaults to deployed and failed releases; only deployed ones count here.
	list.StateMask = action.ListDeployed
	releases, err := list.Run()
	if err != nil {
		return false, err
	}
	for _, r := range releases {
		if r.Name == name && r.Info != nil && r.Info.Status == release.StatusDeployed {
			return true, nil
		}
	}
	return false, nil
}
