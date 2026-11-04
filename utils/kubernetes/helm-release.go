package kubernetes

import (
	"errors"
	"fmt"
	"sync"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
)

// HelmInstallOutcome says what InstallHelmChartOnce did.
type HelmInstallOutcome int

const (
	// HelmInstalled means the chart was installed by this call.
	HelmInstalled HelmInstallOutcome = iota
	// HelmAlreadyInstalled means the release was already deployed, so nothing was installed.
	HelmAlreadyInstalled
	// HelmProvidedElsewhere means the release does not exist but the caller reported that
	// what the chart provides is already available (for example an operator installed
	// without Helm), so nothing was installed.
	HelmProvidedElsewhere
)

// helmReleaseLocks serializes InstallHelmChartOnce per cluster, namespace and release.
var helmReleaseLocks sync.Map

// lockHelmRelease takes the lock for one release and returns the function that releases it.
//
// The release name is only known once the chart is loaded, so the key uses the release name
// when the caller set one and the chart source otherwise. The chart name comes from the chart
// itself, so two sources for the same chart are not serialized against each other; Helm's own
// storage still rejects the second install in that case.
func lockHelmRelease(c *Client, cfg ApplyHelmChartConfig) func() {
	host := ""
	if c != nil {
		host = c.RestConfig.Host
	}
	identity := cfg.ReleaseName
	if identity == "" {
		switch {
		case cfg.LocalPath != "":
			identity = cfg.LocalPath
		case cfg.URL != "":
			identity = cfg.URL
		default:
			identity = fmt.Sprintf("%s/%s@%s", cfg.ChartLocation.Repository, cfg.ChartLocation.Chart, cfg.ChartLocation.Version)
		}
	}
	key := fmt.Sprintf("%s|%s|%s", host, cfg.Namespace, identity)
	mu, _ := helmReleaseLocks.LoadOrStore(key, &sync.Mutex{})
	mu.(*sync.Mutex).Lock()
	return mu.(*sync.Mutex).Unlock
}

// InstallHelmChartOnce installs the chart unless its release is already there, and is safe to
// call once per component of a model that shares one chart.
//
// ApplyHelmChart always installs, and Helm refuses a second install of the same release with
// "cannot reuse a name that is still in use". This function looks at the release first, while
// holding a lock for the release so that concurrent callers cannot both find it absent, and
// loads the chart once for both the lookup and the install. The release name is cfg.ReleaseName,
// or the chart's own name when that is empty, as in ApplyHelmChart. Action and UpgradeIfInstalled
// are ignored: this is always an install. Use ApplyHelmChart to upgrade.
//
// What it does for each state of the release (its latest revision):
//   - not found: provided is asked (it may be nil). If it reports true the install is skipped
//     (HelmProvidedElsewhere), otherwise the chart is installed (HelmInstalled).
//   - deployed: nothing is installed (HelmAlreadyInstalled).
//   - failed, pending, uninstalling or uninstalled: an error that names the state. A fresh
//     install cannot succeed over such a release, and replacing or rolling it back on the
//     caller's behalf could hide a problem, so it is left for an operator to resolve.
//   - the lookup itself failed: that error, since the install needs the same storage access.
//
// The lock is in-process. With several server replicas, Helm's storage is the only protection,
// and the loser gets Helm's own error.
func (c *Client) InstallHelmChartOnce(cfg ApplyHelmChartConfig, provided func() bool) (HelmInstallOutcome, error) {
	setupDefaults(&cfg)
	cfg.Action = INSTALL
	cfg.UpgradeIfInstalled = false

	unlock := lockHelmRelease(c, cfg)
	defer unlock()

	if err := setupChartVersion(&cfg); err != nil {
		return HelmInstalled, ErrApplyHelmChart(err)
	}
	localPath, err := getHelmLocalPath(cfg)
	if err != nil {
		return HelmInstalled, ErrApplyHelmChart(err)
	}
	helmChart, err := loader.Load(localPath)
	if err != nil {
		return HelmInstalled, ErrApplyHelmChart(err)
	}
	if cfg.ReleaseName == "" {
		cfg.ReleaseName = helmChart.Name()
	}
	if err = checkIfInstallable(helmChart); err != nil {
		return HelmInstalled, ErrApplyHelmChart(err)
	}

	actionConfig, err := newHelmActionConfig(c, cfg, c.getRESTClientGetter())
	if err != nil {
		return HelmInstalled, ErrApplyHelmChart(err)
	}

	status, found, err := latestReleaseStatus(actionConfig, cfg.ReleaseName)
	if err != nil {
		return HelmInstalled, ErrApplyHelmChart(err)
	}
	switch {
	case found && status == release.StatusDeployed:
		return HelmAlreadyInstalled, nil
	case found:
		return HelmInstalled, ErrApplyHelmChart(fmt.Errorf(
			"helm release %q in namespace %q is %s, so installing the chart again would fail; resolve it first, for example with helm uninstall",
			cfg.ReleaseName, cfg.Namespace, status))
	}

	if provided != nil && provided() {
		return HelmProvidedElsewhere, nil
	}

	if err := generateAction(actionConfig, cfg)(helmChart); err != nil {
		return HelmInstalled, ErrApplyHelmChart(err)
	}
	return HelmInstalled, nil
}

// latestReleaseStatus returns the status of the latest revision of the named release in the
// namespace the action configuration is bound to, and whether the release exists at all.
func latestReleaseStatus(actionConfig *action.Configuration, name string) (status release.Status, found bool, err error) {
	rel, err := actionConfig.Releases.Last(name)
	if err != nil {
		if errors.Is(err, driver.ErrReleaseNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	if rel == nil || rel.Info == nil {
		return "", false, fmt.Errorf("helm release %q has no status", name)
	}
	return rel.Info.Status, true, nil
}
