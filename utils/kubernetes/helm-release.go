package kubernetes

import (
	"errors"
	"fmt"
	"sync"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
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
	// HelmAlreadyInstalledOtherVersion means the release was already deployed, so nothing was
	// installed, but at a different chart version than the one asked for. An install never
	// changes a deployed release; moving it to the new version takes an explicit upgrade.
	HelmAlreadyInstalledOtherVersion
	// HelmProvidedElsewhere means the release does not exist but the caller reported that
	// what the chart provides is already available (for example an operator installed
	// without Helm), so nothing was installed.
	HelmProvidedElsewhere
)

// helmLock is one lock in the table, with the number of calls holding or waiting for it.
type helmLock struct {
	mu   sync.Mutex
	refs int
}

var (
	// helmLocks serializes InstallHelmChartOnce. An entry exists only while a call holds or waits
	// for it, so the table is bounded by the number of calls in flight and does not grow with the
	// number of releases or charts ever touched.
	helmLocks   = map[string]*helmLock{}
	helmLocksMu sync.Mutex
)

// lockHelm takes the lock for key and returns the function that releases it.
func lockHelm(key string) func() {
	helmLocksMu.Lock()
	l, ok := helmLocks[key]
	if !ok {
		l = &helmLock{}
		helmLocks[key] = l
	}
	l.refs++
	helmLocksMu.Unlock()

	l.mu.Lock()
	return func() {
		l.mu.Unlock()
		helmLocksMu.Lock()
		l.refs--
		if l.refs == 0 {
			delete(helmLocks, key)
		}
		helmLocksMu.Unlock()
	}
}

// helmLockHost identifies the cluster a client talks to, for the lock keys. A nil client or an
// empty host share one identity, which only happens in tests.
func helmLockHost(c *Client) string {
	if c == nil {
		return ""
	}
	return c.RestConfig.Host
}

// lockChartSource serializes fetching and loading one chart source, so that two calls for the same
// source do not download the cached chart file at the same time.
func lockChartSource(c *Client, cfg ApplyHelmChartConfig) func() {
	identity := cfg.LocalPath
	switch {
	case identity != "":
	case cfg.URL != "":
		identity = cfg.URL
	default:
		identity = fmt.Sprintf("%s/%s@%s", cfg.ChartLocation.Repository, cfg.ChartLocation.Chart, cfg.ChartLocation.Version)
	}
	return lockHelm(fmt.Sprintf("source|%s|%s", helmLockHost(c), identity))
}

// lockHelmRelease serializes the lookup and the install of one release in one cluster and
// namespace. The name is the resolved release name, so two chart sources that resolve to the same
// release (two versions of one chart, say) share the lock.
func lockHelmRelease(c *Client, namespace, name string) func() {
	return lockHelm(fmt.Sprintf("release|%s|%s|%s", helmLockHost(c), namespace, name))
}

// InstallHelmChartOnce installs the chart unless its release is already there, and is safe to
// call once per component of a model that shares one chart.
//
// ApplyHelmChart with INSTALL either always installs, and Helm refuses a second install of the
// same release with "cannot reuse a name that is still in use", or, with UpgradeIfInstalled,
// upgrades the release when it is there. This function looks at the release first, while
// holding a lock for the release (by its resolved name, so two chart URLs for one chart share it)
// so that concurrent callers cannot both find it absent, and loads the chart once for both the
// lookup and the install. The release name is cfg.ReleaseName,
// or the chart's own name when that is empty, as in ApplyHelmChart. Action and UpgradeIfInstalled
// are ignored: this is always an install. Use ApplyHelmChart to upgrade.
//
// What it does for each state of the release (its latest revision):
//   - not found: provided is asked (it may be nil). If it reports true the install is skipped
//     (HelmProvidedElsewhere), otherwise the chart is installed (HelmInstalled).
//   - deployed: nothing is installed (HelmAlreadyInstalled), or HelmAlreadyInstalledOtherVersion
//     when the deployed chart version differs from the one asked for, so the caller can say that
//     the release was left as it is. Upgrading is never implicit: it would add a revision for
//     every component of a model and reset the values the operator was installed with.
//   - failed, pending, uninstalling or uninstalled: an error that names the state. A fresh
//     install cannot succeed over such a release, and replacing or rolling it back on the
//     caller's behalf could hide a problem, so it is left for an operator to resolve.
//   - the lookup itself failed: ErrHelmReleaseStorage (or ErrHelmClusterUnreachable when the
//     cluster could not be reached at all), since the install needs the same access.
//
// Unlike ApplyHelmChart with UpgradeIfInstalled, it never recovers a pending release, even an
// abandoned one: recovering ends in an upgrade, which this function never performs.
//
// The lock is in-process. With several server replicas, Helm's storage is the only protection,
// and the loser gets Helm's own error.
func (c *Client) InstallHelmChartOnce(cfg ApplyHelmChartConfig, provided func() bool) (HelmInstallOutcome, error) {
	setupDefaults(&cfg)
	cfg.Action = INSTALL
	cfg.UpgradeIfInstalled = false

	// The release name is only known once the chart is loaded, so the chart is fetched under a lock
	// for its source, and the lookup and the install then run under a lock for the resolved release.
	// The source lock is let go before the release lock is taken, so the two never nest.
	helmChart, err := c.fetchChart(&cfg)
	if err != nil {
		return HelmInstalled, ErrHelmChartFetch(err)
	}
	if cfg.ReleaseName == "" {
		cfg.ReleaseName = helmChart.Name()
	}
	if err = checkIfInstallable(helmChart); err != nil {
		return HelmInstalled, err
	}

	unlock := lockHelmRelease(c, cfg.Namespace, cfg.ReleaseName)
	defer unlock()

	actionConfig, err := newHelmActionConfig(c, cfg, c.getRESTClientGetter())
	if err != nil {
		return HelmInstalled, ErrHelmReleaseStorage(cfg.ReleaseName, cfg.Namespace, err)
	}
	if err = actionConfig.KubeClient.IsReachable(); err != nil {
		return HelmInstalled, ErrHelmClusterUnreachable(cfg.ReleaseName, cfg.Namespace, err)
	}

	existing, err := latestRelease(actionConfig, cfg.ReleaseName)
	if err != nil {
		return HelmInstalled, ErrHelmReleaseStorage(cfg.ReleaseName, cfg.Namespace, err)
	}
	switch {
	case existing != nil && existing.Info.Status == release.StatusDeployed:
		if chartVersionDiffers(existing, helmChart) {
			return HelmAlreadyInstalledOtherVersion, nil
		}
		return HelmAlreadyInstalled, nil
	case existing != nil:
		return HelmInstalled, ErrApplyHelmChart(fmt.Errorf(
			"helm release %q in namespace %q is %s, so installing the chart again would fail; resolve it first, for example with helm uninstall",
			cfg.ReleaseName, cfg.Namespace, existing.Info.Status))
	}

	if provided != nil && provided() {
		return HelmProvidedElsewhere, nil
	}

	if err := runHelmAction(actionConfig, cfg, helmReleasePlan{action: INSTALL}, helmChart); err != nil {
		return HelmInstalled, err
	}
	return HelmInstalled, nil
}

// fetchChart resolves the chart version, downloads the chart (or reuses the cached file) and loads
// it, holding the lock for the chart source meanwhile.
func (c *Client) fetchChart(cfg *ApplyHelmChartConfig) (*chart.Chart, error) {
	unlock := lockChartSource(c, *cfg)
	defer unlock()

	if err := setupChartVersion(cfg); err != nil {
		return nil, err
	}
	localPath, err := getHelmLocalPath(*cfg)
	if err != nil {
		return nil, err
	}
	return loader.Load(localPath)
}

// latestRelease returns the latest revision of the named release in the namespace the action
// configuration is bound to, or nil when the release does not exist.
func latestRelease(actionConfig *action.Configuration, name string) (*release.Release, error) {
	rel, err := actionConfig.Releases.Last(name)
	if err != nil {
		if errors.Is(err, driver.ErrReleaseNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if rel == nil || rel.Info == nil {
		return nil, fmt.Errorf("helm release %q has no status", name)
	}
	return rel, nil
}

// chartVersionDiffers reports whether the deployed release was installed from a different chart
// version than want. When either version is unknown it reports false: a difference is only
// claimed when it can be shown.
func chartVersionDiffers(deployed *release.Release, want *chart.Chart) bool {
	if deployed.Chart == nil || deployed.Chart.Metadata == nil || want == nil || want.Metadata == nil {
		return false
	}
	have, wanted := deployed.Chart.Metadata.Version, want.Metadata.Version
	return have != "" && wanted != "" && have != wanted
}
