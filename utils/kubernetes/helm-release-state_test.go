package kubernetes

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	meshkiterrors "github.com/meshery/meshkit/errors"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	kubefake "helm.sh/helm/v3/pkg/kube/fake"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage"
	"helm.sh/helm/v3/pkg/storage/driver"
	helmtime "helm.sh/helm/v3/pkg/time"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// The tests below run Helm's real Install, Upgrade and Uninstall actions against Helm's in-memory
// release storage (driver.NewMemory) and a fake Kubernetes client. The served chart is testchart
// 0.1.0 (see onceFixture); seeded revisions record chart version 0.0.1, so the revision an upgrade
// writes is told apart by its chart version.

// revision is one seeded revision of the release testchart.
type revision struct {
	status release.Status
	// age is how long before now the revision's operation started (Info.LastDeployed). A negative
	// age leaves LastDeployed unset.
	age time.Duration
}

// seedRevisions stores the revisions of testchart in the default namespace, numbered from 1.
func seedRevisions(t *testing.T, s *storage.Storage, revisions ...revision) {
	t.Helper()
	for i, r := range revisions {
		info := &release.Info{Status: r.status, Description: "seeded"}
		if r.age >= 0 {
			info.LastDeployed = helmtime.Time{Time: time.Now().Add(-r.age)}
		}
		rel := &release.Release{
			Name: "testchart", Namespace: "default", Version: i + 1, Info: info,
			Chart: &chart.Chart{Metadata: &chart.Metadata{Name: "testchart", Version: "0.0.1", Type: "application"}},
		}
		if err := s.Create(rel); err != nil {
			t.Fatalf("seeding revision %d: %v", i+1, err)
		}
	}
}

// historyOf returns the status of every revision of name, indexed by revision number (index 0 is
// unused), and fails the test if the revisions are not numbered 1..n without gaps.
func historyOf(t *testing.T, s *storage.Storage, name string) []release.Status {
	t.Helper()
	h, err := s.History(name)
	if errors.Is(err, driver.ErrReleaseNotFound) {
		return []release.Status{""}
	}
	if err != nil {
		t.Fatalf("reading history of %s: %v", name, err)
	}
	statuses := make([]release.Status, len(h)+1)
	for _, r := range h {
		if r.Version < 1 || r.Version > len(h) {
			t.Fatalf("history of %s has revision %d among %d revisions", name, r.Version, len(h))
		}
		statuses[r.Version] = r.Info.Status
	}
	return statuses
}

func upgradeIfInstalled(cfg ApplyHelmChartConfig) ApplyHelmChartConfig {
	cfg.Action = INSTALL
	cfg.UpgradeIfInstalled = true
	return cfg
}

func TestPlanInstallOrUpgradeCoversEveryReleaseState(t *testing.T) {
	const timeout = 10 * time.Minute
	stale, fresh := 2*timeout, timeout/2
	cases := []struct {
		name        string
		revisions   []revision
		dryRun      bool
		wantAction  HelmChartAction
		wantReplace bool
		wantAbandon bool
		wantErrCode string
	}{
		{name: "not installed", wantAction: INSTALL},
		{name: "deployed", revisions: []revision{{release.StatusDeployed, stale}}, wantAction: UPGRADE},
		{name: "failed first install", revisions: []revision{{release.StatusFailed, stale}}, wantAction: UPGRADE},
		{name: "failed upgrade", revisions: []revision{{release.StatusDeployed, stale}, {release.StatusFailed, stale}}, wantAction: UPGRADE},
		{name: "superseded", revisions: []revision{{release.StatusSuperseded, stale}}, wantAction: UPGRADE},
		{name: "uninstalled with history kept", revisions: []revision{{release.StatusUninstalled, stale}}, wantAction: INSTALL, wantReplace: true},
		{name: "pending-install abandoned", revisions: []revision{{release.StatusPendingInstall, stale}}, wantAction: UPGRADE, wantAbandon: true},
		{name: "pending-upgrade abandoned", revisions: []revision{{release.StatusDeployed, stale}, {release.StatusPendingUpgrade, stale}}, wantAction: UPGRADE, wantAbandon: true},
		{name: "pending-rollback abandoned", revisions: []revision{{release.StatusDeployed, stale}, {release.StatusPendingRollback, stale}}, wantAction: UPGRADE, wantAbandon: true},
		{name: "pending without a start time", revisions: []revision{{release.StatusPendingInstall, -1}}, wantAction: UPGRADE, wantAbandon: true},
		{name: "pending-install in progress", revisions: []revision{{release.StatusPendingInstall, fresh}}, wantErrCode: ErrHelmReleasePendingCode},
		{name: "pending-upgrade in progress", revisions: []revision{{release.StatusDeployed, stale}, {release.StatusPendingUpgrade, fresh}}, wantErrCode: ErrHelmReleasePendingCode},
		{name: "pending-rollback in progress", revisions: []revision{{release.StatusDeployed, stale}, {release.StatusPendingRollback, fresh}}, wantErrCode: ErrHelmReleasePendingCode},
		{name: "pending started just now", revisions: []revision{{release.StatusPendingInstall, 0}}, wantErrCode: ErrHelmReleasePendingCode},
		{name: "abandoned but dry run", revisions: []revision{{release.StatusPendingInstall, stale}}, dryRun: true, wantErrCode: ErrHelmReleasePendingCode},
		{name: "uninstalling", revisions: []revision{{release.StatusDeployed, stale}, {release.StatusUninstalling, stale}}, wantErrCode: ErrHelmReleaseStateCode},
		{name: "unknown status", revisions: []revision{{release.StatusUnknown, stale}}, wantErrCode: ErrHelmReleaseStateCode},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actionConfig, s := memoryActionConfig()
			seedRevisions(t, s, tc.revisions...)
			cfg := ApplyHelmChartConfig{ReleaseName: "testchart", Namespace: "default", PendingReleaseTimeout: timeout, DryRun: tc.dryRun}

			plan, err := planInstallOrUpgrade(actionConfig, cfg, time.Now())
			if tc.wantErrCode != "" {
				if got := meshkiterrors.GetCode(err); got != tc.wantErrCode {
					t.Fatalf("error code = %q (%v), want %q", got, err, tc.wantErrCode)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			if plan.action != tc.wantAction || plan.replace != tc.wantReplace || (plan.abandoned != nil) != tc.wantAbandon {
				t.Fatalf("plan = {action %v, replace %v, abandoned %v}, want {action %v, replace %v, abandoned %v}",
					plan.action, plan.replace, plan.abandoned != nil, tc.wantAction, tc.wantReplace, tc.wantAbandon)
			}
			if tc.wantAbandon && plan.abandoned.Version != len(tc.revisions) {
				t.Fatalf("abandoned revision = %d, want the latest, %d", plan.abandoned.Version, len(tc.revisions))
			}
		})
	}
}

func TestPlanInstallOrUpgradeIgnoresOtherReleasesAndNamespaces(t *testing.T) {
	actionConfig, s := memoryActionConfig()
	mem := s.Driver.(*driver.Memory)
	// The same name in another namespace, and another name in this one, are both pending.
	for _, rel := range []*release.Release{
		{Name: "testchart", Namespace: "other", Version: 1, Info: &release.Info{Status: release.StatusPendingInstall, LastDeployed: helmtime.Now()}},
		{Name: "otherchart", Namespace: "default", Version: 1, Info: &release.Info{Status: release.StatusPendingInstall, LastDeployed: helmtime.Now()}},
	} {
		if err := s.Create(rel); err != nil {
			t.Fatalf("seeding %s/%s: %v", rel.Namespace, rel.Name, err)
		}
	}
	// Creating a release binds the memory driver to its namespace; bind it back, as Helm's
	// Secrets driver is bound to the namespace it was initialized with.
	mem.SetNamespace("default")

	plan, err := planInstallOrUpgrade(actionConfig, ApplyHelmChartConfig{ReleaseName: "testchart", Namespace: "default"}, time.Now())
	if err != nil || plan.action != INSTALL || plan.replace || plan.abandoned != nil {
		t.Fatalf("plan = %+v, %v, want a plain install", plan, err)
	}
}

func TestPendingReleaseTimeoutIsConfigurable(t *testing.T) {
	actionConfig, s := memoryActionConfig()
	seedRevisions(t, s, revision{release.StatusPendingInstall, 2 * time.Minute})

	_, err := planInstallOrUpgrade(actionConfig, ApplyHelmChartConfig{ReleaseName: "testchart", Namespace: "default"}, time.Now())
	if got := meshkiterrors.GetCode(err); got != ErrHelmReleasePendingCode {
		t.Fatalf("with the default timeout: error code = %q (%v), want %q", got, err, ErrHelmReleasePendingCode)
	}
	plan, err := planInstallOrUpgrade(actionConfig, ApplyHelmChartConfig{ReleaseName: "testchart", Namespace: "default", PendingReleaseTimeout: time.Minute}, time.Now())
	if err != nil || plan.abandoned == nil {
		t.Fatalf("with a 1m timeout: plan = %+v, %v, want the revision treated as abandoned", plan, err)
	}
}

func TestPendingReleaseStartedInTheFutureIsInProgress(t *testing.T) {
	// The clock of the host that started the operation is ahead of this one: wait, never recover.
	actionConfig, s := memoryActionConfig()
	rel := &release.Release{Name: "testchart", Namespace: "default", Version: 1,
		Info: &release.Info{Status: release.StatusPendingUpgrade, LastDeployed: helmtime.Time{Time: time.Now().Add(time.Hour)}}}
	if err := s.Create(rel); err != nil {
		t.Fatalf("seeding: %v", err)
	}
	_, err := planInstallOrUpgrade(actionConfig, ApplyHelmChartConfig{ReleaseName: "testchart", Namespace: "default", PendingReleaseTimeout: time.Nanosecond}, time.Now())
	if got := meshkiterrors.GetCode(err); got != ErrHelmReleasePendingCode {
		t.Fatalf("error code = %q (%v), want %q", got, err, ErrHelmReleasePendingCode)
	}
}

// TestApplyHelmChartUpgradeIfInstalledReachesDeployed runs the whole call for every state that it
// recovers from, and checks the release history it leaves behind.
func TestApplyHelmChartUpgradeIfInstalledReachesDeployed(t *testing.T) {
	stale := 2 * DefaultPendingReleaseTimeout
	// When no revision was ever deployed, Helm upgrades from the failed one and then marks it
	// superseded (keeping its description), so a failed or abandoned first install ends up
	// superseded rather than failed.
	cases := []struct {
		name      string
		revisions []revision
		want      []release.Status // by revision number, from 1
	}{
		{"not installed", nil, []release.Status{release.StatusDeployed}},
		{"deployed", []revision{{release.StatusDeployed, stale}}, []release.Status{release.StatusSuperseded, release.StatusDeployed}},
		{"failed first install", []revision{{release.StatusFailed, stale}}, []release.Status{release.StatusSuperseded, release.StatusDeployed}},
		{"failed upgrade", []revision{{release.StatusDeployed, stale}, {release.StatusFailed, stale}},
			[]release.Status{release.StatusSuperseded, release.StatusFailed, release.StatusDeployed}},
		{"superseded", []revision{{release.StatusSuperseded, stale}}, []release.Status{release.StatusSuperseded, release.StatusDeployed}},
		{"uninstalled with history kept", []revision{{release.StatusUninstalled, stale}}, []release.Status{release.StatusSuperseded, release.StatusDeployed}},
		{"pending-install abandoned", []revision{{release.StatusPendingInstall, stale}}, []release.Status{release.StatusSuperseded, release.StatusDeployed}},
		{"pending-upgrade abandoned", []revision{{release.StatusDeployed, stale}, {release.StatusPendingUpgrade, stale}},
			[]release.Status{release.StatusSuperseded, release.StatusFailed, release.StatusDeployed}},
		{"pending-rollback abandoned", []revision{{release.StatusSuperseded, stale}, {release.StatusDeployed, stale}, {release.StatusPendingRollback, stale}},
			[]release.Status{release.StatusSuperseded, release.StatusSuperseded, release.StatusFailed, release.StatusDeployed}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, cfg, _ := onceFixture(t)
			seedRevisions(t, s, tc.revisions...)
			var logged []string
			cfg.Logger = func(format string, v ...interface{}) { logged = append(logged, fmt.Sprintf(format, v...)) }

			if err := (&Client{}).ApplyHelmChart(upgradeIfInstalled(cfg)); err != nil {
				t.Fatalf("ApplyHelmChart() error = %v", err)
			}

			got := historyOf(t, s, "testchart")[1:]
			if fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Fatalf("history = %v, want %v", got, tc.want)
			}
			latest, err := s.Last("testchart")
			if err != nil || latest.Chart.Metadata.Version != "0.1.0" {
				t.Fatalf("latest revision = %+v, %v, want the requested chart 0.1.0", latest, err)
			}

			// An abandoned revision says why it failed, in the history and in the log.
			if len(tc.revisions) > 0 && tc.revisions[len(tc.revisions)-1].status.IsPending() {
				abandoned, err := s.Get("testchart", len(tc.revisions))
				if err != nil {
					t.Fatalf("reading the abandoned revision: %v", err)
				}
				if !strings.Contains(abandoned.Info.Description, "abandoned") {
					t.Fatalf("abandoned revision description = %q, want it to say it was abandoned", abandoned.Info.Description)
				}
				if !containsAll(strings.Join(logged, "\n"), "testchart", "marking the revision failed") {
					t.Fatalf("log %q does not report the recovery", logged)
				}
			}
		})
	}
}

// TestApplyHelmChartUpgradeIfInstalledLeavesBlockedReleases checks that a release that is not
// recovered is left exactly as it was, with the error that names its state.
func TestApplyHelmChartUpgradeIfInstalledLeavesBlockedReleases(t *testing.T) {
	stale := 2 * DefaultPendingReleaseTimeout
	cases := []struct {
		name      string
		revisions []revision
		dryRun    bool
		wantCode  string
	}{
		{"pending-install in progress", []revision{{release.StatusPendingInstall, time.Minute}}, false, ErrHelmReleasePendingCode},
		{"pending-upgrade in progress", []revision{{release.StatusDeployed, stale}, {release.StatusPendingUpgrade, time.Minute}}, false, ErrHelmReleasePendingCode},
		{"pending-rollback in progress", []revision{{release.StatusDeployed, stale}, {release.StatusPendingRollback, time.Minute}}, false, ErrHelmReleasePendingCode},
		{"abandoned but dry run", []revision{{release.StatusPendingInstall, stale}}, true, ErrHelmReleasePendingCode},
		{"uninstalling", []revision{{release.StatusDeployed, stale}, {release.StatusUninstalling, stale}}, false, ErrHelmReleaseStateCode},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, cfg, _ := onceFixture(t)
			seedRevisions(t, s, tc.revisions...)
			before := historyOf(t, s, "testchart")
			cfg.DryRun = tc.dryRun

			err := (&Client{}).ApplyHelmChart(upgradeIfInstalled(cfg))
			if got := meshkiterrors.GetCode(err); got != tc.wantCode {
				t.Fatalf("error code = %q (%v), want %q", got, err, tc.wantCode)
			}
			latest := tc.revisions[len(tc.revisions)-1].status
			if !containsAll(err.Error(), `"testchart"`, `"default"`, string(latest)) {
				t.Fatalf("error %q should name the release, its namespace and its state", err)
			}
			if after := historyOf(t, s, "testchart"); fmt.Sprint(after) != fmt.Sprint(before) {
				t.Fatalf("history changed from %v to %v", before, after)
			}
		})
	}
}

func TestApplyHelmChartUpgradesTheNamedRelease(t *testing.T) {
	s, cfg, _ := onceFixture(t)
	rel := &release.Release{
		Name: "custom-name", Namespace: "default", Version: 1,
		Info:  &release.Info{Status: release.StatusDeployed},
		Chart: &chart.Chart{Metadata: &chart.Metadata{Name: "testchart", Version: "0.0.1"}},
	}
	if err := s.Create(rel); err != nil {
		t.Fatalf("seeding: %v", err)
	}
	cfg.ReleaseName = "custom-name"

	if err := (&Client{}).ApplyHelmChart(upgradeIfInstalled(cfg)); err != nil {
		t.Fatalf("ApplyHelmChart() error = %v", err)
	}
	if got := historyOf(t, s, "custom-name")[1:]; fmt.Sprint(got) != fmt.Sprint([]release.Status{release.StatusSuperseded, release.StatusDeployed}) {
		t.Fatalf("history of custom-name = %v, want it upgraded", got)
	}
	if _, err := s.History("testchart"); !errors.Is(err, driver.ErrReleaseNotFound) {
		t.Fatalf("a release named after the chart was touched: %v", err)
	}
}

func TestApplyHelmChartPlainInstallIsUnchanged(t *testing.T) {
	s, cfg, _ := onceFixture(t)
	seedRevisions(t, s, revision{release.StatusDeployed, time.Hour})
	cfg.Action = INSTALL

	err := (&Client{}).ApplyHelmChart(cfg)
	if got := meshkiterrors.GetCode(err); got != ErrApplyHelmChartCode || !strings.Contains(err.Error(), "cannot re-use a name") {
		t.Fatalf("error = %q (%v), want Helm's refusal as %q", got, err, ErrApplyHelmChartCode)
	}
}

func TestApplyHelmChartKeepsBoundedHistory(t *testing.T) {
	cases := []struct {
		name       string
		maxHistory int
		upgrades   int
		want       int
	}{
		{"default limit", 0, DefaultHelmMaxHistory + 3, DefaultHelmMaxHistory},
		{"configured limit", 3, 6, 3},
		{"negative keeps everything", -1, DefaultHelmMaxHistory + 3, DefaultHelmMaxHistory + 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, cfg, _ := onceFixture(t)
			cfg.MaxHistory = tc.maxHistory
			for i := 0; i <= tc.upgrades; i++ { // the first call installs
				if err := (&Client{}).ApplyHelmChart(upgradeIfInstalled(cfg)); err != nil {
					t.Fatalf("call %d: %v", i+1, err)
				}
			}
			h, err := s.History("testchart")
			if err != nil || len(h) != tc.want {
				t.Fatalf("history has %d revisions (%v), want %d", len(h), err, tc.want)
			}
			if latest, err := s.Last("testchart"); err != nil || latest.Version != tc.upgrades+1 || latest.Info.Status != release.StatusDeployed {
				t.Fatalf("latest revision = %+v, %v, want revision %d deployed", latest, err, tc.upgrades+1)
			}
		})
	}
}

func TestApplyHelmChartConcurrentInstallsSerialize(t *testing.T) {
	s, cfg, _ := onceFixture(t)
	const callers = 8
	var wg sync.WaitGroup
	errs := make([]error, callers)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = (&Client{}).ApplyHelmChart(upgradeIfInstalled(cfg))
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("caller %d: %v", i, err)
		}
	}
	// One install and an upgrade per other caller, never two installs.
	statuses := historyOf(t, s, "testchart")[1:]
	deployed := 0
	for _, st := range statuses {
		if st == release.StatusDeployed {
			deployed++
		}
	}
	if len(statuses) != callers || deployed != 1 {
		t.Fatalf("history = %v, want %d revisions with exactly one deployed", statuses, callers)
	}
}

// unreachableKubeClient is a fake Kubernetes client whose cluster cannot be reached.
type unreachableKubeClient struct {
	kubefake.PrintingKubeClient
	err error
}

func (c *unreachableKubeClient) IsReachable() error { return c.err }

// faultyDriver is Helm's in-memory storage with injectable query and update failures.
type faultyDriver struct {
	*driver.Memory
	queryErr, updateErr error
}

func (d *faultyDriver) Query(labels map[string]string) ([]*release.Release, error) {
	if d.queryErr != nil {
		return nil, d.queryErr
	}
	return d.Memory.Query(labels)
}

func (d *faultyDriver) Update(key string, rls *release.Release) error {
	if d.updateErr != nil {
		return d.updateErr
	}
	return d.Memory.Update(key, rls)
}

// useActionConfig makes ApplyHelmChart use actionConfig, or fail with err when it is not nil.
func useActionConfig(t *testing.T, actionConfig *action.Configuration, err error) {
	t.Helper()
	orig := newHelmActionConfig
	t.Cleanup(func() { newHelmActionConfig = orig })
	newHelmActionConfig = func(*Client, ApplyHelmChartConfig, genericclioptions.RESTClientGetter) (*action.Configuration, error) {
		return actionConfig, err
	}
}

func TestApplyHelmChartClassifiesFailures(t *testing.T) {
	t.Run("cluster unreachable", func(t *testing.T) {
		s, cfg, _ := onceFixture(t)
		actionConfig, _ := memoryActionConfig()
		actionConfig.Releases = s
		actionConfig.KubeClient = &unreachableKubeClient{PrintingKubeClient: kubefake.PrintingKubeClient{Out: io.Discard}, err: errors.New("dial tcp: connection refused")}
		useActionConfig(t, actionConfig, nil)

		err := (&Client{}).ApplyHelmChart(upgradeIfInstalled(cfg))
		if got := meshkiterrors.GetCode(err); got != ErrHelmClusterUnreachableCode || !strings.Contains(err.Error(), "connection refused") {
			t.Fatalf("error = %q (%v), want %q carrying the cause", got, err, ErrHelmClusterUnreachableCode)
		}
		if _, err := s.History("testchart"); !errors.Is(err, driver.ErrReleaseNotFound) {
			t.Fatalf("something was stored: %v", err)
		}
	})

	t.Run("action configuration fails", func(t *testing.T) {
		_, cfg, _ := onceFixture(t)
		useActionConfig(t, nil, errors.New(`unknown driver "bogus"`))

		err := (&Client{}).ApplyHelmChart(upgradeIfInstalled(cfg))
		if got := meshkiterrors.GetCode(err); got != ErrHelmReleaseStorageCode {
			t.Fatalf("error = %q (%v), want %q", got, err, ErrHelmReleaseStorageCode)
		}
	})

	t.Run("release history cannot be read", func(t *testing.T) {
		_, cfg, _ := onceFixture(t)
		actionConfig, _ := memoryActionConfig()
		actionConfig.Releases = storage.Init(&faultyDriver{Memory: driver.NewMemory(), queryErr: errors.New(`secrets is forbidden: cannot list resource "secrets"`)})
		useActionConfig(t, actionConfig, nil)

		err := (&Client{}).ApplyHelmChart(upgradeIfInstalled(cfg))
		if got := meshkiterrors.GetCode(err); got != ErrHelmReleaseStorageCode || !strings.Contains(err.Error(), "forbidden") {
			t.Fatalf("error = %q (%v), want %q carrying the cause", got, err, ErrHelmReleaseStorageCode)
		}
	})

	t.Run("abandoned revision cannot be marked failed", func(t *testing.T) {
		_, cfg, _ := onceFixture(t)
		mem := driver.NewMemory()
		mem.SetNamespace("default")
		faulty := &faultyDriver{Memory: mem}
		actionConfig, _ := memoryActionConfig()
		actionConfig.Releases = storage.Init(faulty)
		seedRevisions(t, actionConfig.Releases, revision{release.StatusPendingInstall, time.Hour})
		faulty.updateErr = errors.New(`secrets "sh.helm.release.v1.testchart.v1" is forbidden`)
		useActionConfig(t, actionConfig, nil)

		err := (&Client{}).ApplyHelmChart(upgradeIfInstalled(cfg))
		if got := meshkiterrors.GetCode(err); got != ErrHelmReleaseStorageCode {
			t.Fatalf("error = %q (%v), want %q", got, err, ErrHelmReleaseStorageCode)
		}
		if h := historyOf(t, actionConfig.Releases, "testchart"); len(h) != 2 {
			t.Fatalf("history = %v, want the seeded revision only, with no upgrade attempted", h[1:])
		}
	})

	t.Run("chart cannot be fetched", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(http.NotFound))
		t.Cleanup(ts.Close)
		err := (&Client{}).ApplyHelmChart(upgradeIfInstalled(ApplyHelmChartConfig{URL: ts.URL + "/missing-0.1.0.tgz", DownloadLocation: t.TempDir()}))
		if got := meshkiterrors.GetCode(err); got != ErrHelmChartFetchCode {
			t.Fatalf("error = %q (%v), want %q", got, err, ErrHelmChartFetchCode)
		}
	})

	t.Run("helm action fails, wrapped once", func(t *testing.T) {
		s, cfg, _ := onceFixture(t)
		actionConfig, _ := memoryActionConfig()
		actionConfig.Releases = s
		actionConfig.KubeClient = &kubefake.FailingKubeClient{PrintingKubeClient: kubefake.PrintingKubeClient{Out: io.Discard}, CreateError: errors.New("admission webhook denied the request")}
		useActionConfig(t, actionConfig, nil)
		cfg.CreateNamespace = true // the fake builds no chart resources; the namespace is what it creates

		err := (&Client{}).ApplyHelmChart(upgradeIfInstalled(cfg))
		if got := meshkiterrors.GetCode(err); got != ErrApplyHelmChartCode {
			t.Fatalf("error = %q (%v), want %q", got, err, ErrApplyHelmChartCode)
		}
		if n := strings.Count(err.Error(), "Error applying helm chart"); n > 1 {
			t.Fatalf("error %q wraps ErrApplyHelmChart %d times", err, n)
		}
	})
}

func TestApplyHelmChartUninstallByNameSkipsChartFetch(t *testing.T) {
	s, _, _ := onceFixture(t)
	seedRevisions(t, s, revision{release.StatusDeployed, time.Hour})
	var downloads int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&downloads, 1)
		http.NotFound(w, r)
	}))
	t.Cleanup(ts.Close)

	err := (&Client{}).ApplyHelmChart(ApplyHelmChartConfig{
		URL: ts.URL + "/testchart-0.1.0.tgz", DownloadLocation: t.TempDir(),
		ReleaseName: "testchart", Namespace: "default", Action: UNINSTALL,
	})
	if err != nil {
		t.Fatalf("ApplyHelmChart() uninstall error = %v", err)
	}
	if downloads != 0 {
		t.Fatalf("the chart was requested %d times for an uninstall by release name", downloads)
	}
	if _, err := s.History("testchart"); !errors.Is(err, driver.ErrReleaseNotFound) {
		t.Fatalf("release still present after uninstall: %v", err)
	}
}

func TestFetchHelmChartNeverLeavesAPartialChart(t *testing.T) {
	full := createTestChartArchive(t, "testchart", "0.1.0")
	var truncate atomic.Bool
	truncate.Store(true)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if truncate.Load() {
			// Promise the whole archive, send half of it and drop the connection.
			w.Header().Set("Content-Length", fmt.Sprint(len(full)))
			_, _ = w.Write(full[:len(full)/2])
			return
		}
		_, _ = w.Write(full)
	}))
	t.Cleanup(ts.Close)
	dir := t.TempDir()
	url := ts.URL + "/testchart-0.1.0.tgz"

	if _, err := fetchHelmChart(url, dir); err == nil {
		t.Fatal("expected the truncated download to fail")
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
		t.Fatalf("download directory holds %v (%v) after a failed download, want nothing", entries, err)
	}

	// The next call downloads again instead of reusing a partial file, and gets a loadable chart.
	truncate.Store(false)
	path, err := fetchHelmChart(url, dir)
	if err != nil {
		t.Fatalf("fetchHelmChart() error = %v", err)
	}
	if path != filepath.Join(dir, "testchart-0.1.0.tgz") {
		t.Fatalf("path = %q", path)
	}
	if got, err := os.ReadFile(path); err != nil || len(got) != len(full) {
		t.Fatalf("downloaded %d bytes (%v), want %d", len(got), err, len(full))
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Fatalf("download directory holds %d entries, want only the chart", len(entries))
	}
}

func TestHelmReleaseErrorsAreValid(t *testing.T) {
	for name, err := range map[string]error{
		"ErrHelmChartFetch":         ErrHelmChartFetch(errors.New("x")),
		"ErrHelmClusterUnreachable": ErrHelmClusterUnreachable("r", "ns", errors.New("x")),
		"ErrHelmReleaseStorage":     ErrHelmReleaseStorage("r", "ns", errors.New("x")),
		"ErrHelmReleasePending":     ErrHelmReleasePending("r", "ns", "pending-install", 1, time.Minute, time.Hour, false),
		"ErrHelmReleaseState":       ErrHelmReleaseState("r", "ns", "uninstalling", 2),
	} {
		if verr := meshkiterrors.ValidateCauses(meshkiterrors.GetCauses(err)); verr != nil {
			t.Errorf("%s: invalid causes: %v", name, verr)
		}
	}
}

func TestInstanceDescriptionDropsInnerCauseAndRemedy(t *testing.T) {
	inner := ErrHelmReleaseState("r", "ns", "uninstalling", 2)
	lines := instanceDescription(inner)
	if len(lines) != 1 || !strings.Contains(lines[0], "uninstalling") {
		t.Fatalf("instanceDescription() = %q, want the inner long description only", lines)
	}
	if got := instanceDescription(errors.New("plain")); len(got) != 1 || got[0] != "plain" {
		t.Fatalf("instanceDescription(plain error) = %q", got)
	}
	if got := instanceDescription(nil); got != nil {
		t.Fatalf("instanceDescription(nil) = %q, want nil", got)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
