package kubernetes

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chartutil"
	kubefake "helm.sh/helm/v3/pkg/kube/fake"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func memoryActionConfig() (*action.Configuration, *storage.Storage) {
	memDriver := driver.NewMemory()
	memDriver.SetNamespace("default")
	memStorage := storage.Init(memDriver)
	return &action.Configuration{
		Releases:     memStorage,
		KubeClient:   &kubefake.PrintingKubeClient{Out: io.Discard},
		Capabilities: chartutil.DefaultCapabilities,
		Log:          func(format string, v ...interface{}) {},
	}, memStorage
}

func storeRelease(t *testing.T, s *storage.Storage, name string, status release.Status) {
	t.Helper()
	rel := &release.Release{Name: name, Namespace: "default", Version: 1, Info: &release.Info{Status: status}}
	if err := s.Create(rel); err != nil {
		t.Fatalf("storing release: %v", err)
	}
}

func TestLatestReleaseStatus(t *testing.T) {
	cases := []struct {
		name      string
		stored    map[string]release.Status
		query     string
		wantFound bool
		want      release.Status
	}{
		{"no releases", nil, "testchart", false, ""},
		{"deployed release", map[string]release.Status{"testchart": release.StatusDeployed}, "testchart", true, release.StatusDeployed},
		{"failed release", map[string]release.Status{"testchart": release.StatusFailed}, "testchart", true, release.StatusFailed},
		{"pending release", map[string]release.Status{"testchart": release.StatusPendingInstall}, "testchart", true, release.StatusPendingInstall},
		{"other release only", map[string]release.Status{"other": release.StatusDeployed}, "testchart", false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, s := memoryActionConfig()
			for name, status := range tc.stored {
				storeRelease(t, s, name, status)
			}
			got, found, err := latestReleaseStatus(cfg, tc.query)
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			if found != tc.wantFound || got != tc.want {
				t.Fatalf("latestReleaseStatus() = %q, %v, want %q, %v", got, found, tc.want, tc.wantFound)
			}
		})
	}
}

func TestLatestReleaseStatusUsesNewestRevision(t *testing.T) {
	cfg, s := memoryActionConfig()
	for i, status := range []release.Status{release.StatusSuperseded, release.StatusFailed} {
		rel := &release.Release{Name: "testchart", Namespace: "default", Version: i + 1, Info: &release.Info{Status: status}}
		if err := s.Create(rel); err != nil {
			t.Fatalf("storing release: %v", err)
		}
	}
	got, found, err := latestReleaseStatus(cfg, "testchart")
	if err != nil || !found || got != release.StatusFailed {
		t.Fatalf("latestReleaseStatus() = %q, %v, %v, want failed, true, nil", got, found, err)
	}
}

// onceFixture serves a one-chart archive, points Helm at an in-memory store and returns the
// store, the config to install with and the number of times the chart was downloaded.
func onceFixture(t *testing.T) (*storage.Storage, ApplyHelmChartConfig, *int32) {
	t.Helper()
	actionConfig, s := memoryActionConfig()
	orig := newHelmActionConfig
	t.Cleanup(func() { newHelmActionConfig = orig })
	newHelmActionConfig = func(*Client, ApplyHelmChartConfig, genericclioptions.RESTClientGetter) (*action.Configuration, error) {
		return actionConfig, nil
	}

	var downloads int32
	chartData := createTestChartArchive(t, "testchart", "0.1.0")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&downloads, 1)
		_, _ = w.Write(chartData)
	}))
	t.Cleanup(ts.Close)

	return s, ApplyHelmChartConfig{
		URL:              ts.URL + "/testchart-0.1.0.tgz",
		DownloadLocation: t.TempDir(),
		Namespace:        "default",
	}, &downloads
}

func TestInstallHelmChartOnceInstallsThenSkips(t *testing.T) {
	s, cfg, downloads := onceFixture(t)
	client := &Client{}

	outcome, err := client.InstallHelmChartOnce(cfg, nil)
	if err != nil || outcome != HelmInstalled {
		t.Fatalf("first call = %v, %v, want HelmInstalled, nil", outcome, err)
	}
	if _, err := s.Get("testchart", 1); err != nil {
		t.Fatalf("release was not stored: %v", err)
	}

	outcome, err = client.InstallHelmChartOnce(cfg, nil)
	if err != nil || outcome != HelmAlreadyInstalled {
		t.Fatalf("second call = %v, %v, want HelmAlreadyInstalled, nil", outcome, err)
	}
	if _, err := s.Get("testchart", 2); err == nil {
		t.Fatal("second call created a new revision")
	}
	if *downloads != 1 {
		t.Fatalf("chart downloads = %d, want 1", *downloads)
	}
}

func TestInstallHelmChartOnceIgnoresUpgradeAndAction(t *testing.T) {
	s, cfg, _ := onceFixture(t)
	storeRelease(t, s, "testchart", release.StatusDeployed)
	cfg.UpgradeIfInstalled = true
	cfg.Action = UPGRADE

	outcome, err := (&Client{}).InstallHelmChartOnce(cfg, nil)
	if err != nil || outcome != HelmAlreadyInstalled {
		t.Fatalf("call = %v, %v, want HelmAlreadyInstalled, nil", outcome, err)
	}
	if _, err := s.Get("testchart", 2); err == nil {
		t.Fatal("an upgrade was performed")
	}
}

func TestInstallHelmChartOnceProvidedElsewhere(t *testing.T) {
	s, cfg, _ := onceFixture(t)
	client := &Client{}

	outcome, err := client.InstallHelmChartOnce(cfg, func() bool { return true })
	if err != nil || outcome != HelmProvidedElsewhere {
		t.Fatalf("call = %v, %v, want HelmProvidedElsewhere, nil", outcome, err)
	}
	if _, err := s.Get("testchart", 1); err == nil {
		t.Fatal("the chart was installed although the caller said it was provided")
	}

	// A deployed release wins: the caller is not even asked.
	storeRelease(t, s, "testchart", release.StatusDeployed)
	asked := false
	outcome, err = client.InstallHelmChartOnce(cfg, func() bool { asked = true; return false })
	if err != nil || outcome != HelmAlreadyInstalled || asked {
		t.Fatalf("call = %v, %v (asked %v), want HelmAlreadyInstalled, nil, not asked", outcome, err, asked)
	}
}

func TestInstallHelmChartOnceRefusesUnusableRelease(t *testing.T) {
	for _, status := range []release.Status{
		release.StatusFailed,
		release.StatusPendingInstall,
		release.StatusPendingUpgrade,
		release.StatusPendingRollback,
		release.StatusUninstalling,
		release.StatusUninstalled,
	} {
		t.Run(string(status), func(t *testing.T) {
			s, cfg, _ := onceFixture(t)
			storeRelease(t, s, "testchart", status)

			asked := false
			_, err := (&Client{}).InstallHelmChartOnce(cfg, func() bool { asked = true; return true })
			if err == nil {
				t.Fatalf("expected an error for a %s release", status)
			}
			if !strings.Contains(err.Error(), string(status)) || !strings.Contains(err.Error(), "testchart") {
				t.Fatalf("error %q should name the release and its state %q", err, status)
			}
			if asked {
				t.Fatal("the caller was asked although the release is in the way")
			}
			if _, err := s.Get("testchart", 2); err == nil {
				t.Fatal("a new revision was created over an unusable release")
			}
		})
	}
}

func TestInstallHelmChartOnceReportsUnloadableChart(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	t.Cleanup(ts.Close)

	_, err := (&Client{}).InstallHelmChartOnce(ApplyHelmChartConfig{
		URL:              ts.URL + "/missing-0.1.0.tgz",
		DownloadLocation: t.TempDir(),
	}, nil)
	if err == nil {
		t.Fatal("expected an error when the chart cannot be fetched")
	}
}

func TestInstallHelmChartOnceConcurrentCallsInstallOnce(t *testing.T) {
	s, cfg, _ := onceFixture(t)
	client := &Client{}

	const callers = 8
	var wg sync.WaitGroup
	outcomes := make([]HelmInstallOutcome, callers)
	errs := make([]error, callers)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// The pause widens the window between "release absent" and the install, which is
			// where unsynchronized callers would all decide to install.
			outcomes[i], errs[i] = client.InstallHelmChartOnce(cfg, func() bool {
				time.Sleep(20 * time.Millisecond)
				return false
			})
		}(i)
	}
	wg.Wait()

	installed := 0
	for i := range outcomes {
		if errs[i] != nil {
			t.Fatalf("caller %d: unexpected error %v", i, errs[i])
		}
		switch outcomes[i] {
		case HelmInstalled:
			installed++
		case HelmAlreadyInstalled:
		default:
			t.Fatalf("caller %d: outcome %v", i, outcomes[i])
		}
	}
	if installed != 1 {
		t.Fatalf("callers that installed = %d, want 1", installed)
	}
	if _, err := s.Get("testchart", 2); err == nil {
		t.Fatal("more than one revision exists")
	}
}

func TestLockHelmReleaseIsPerRelease(t *testing.T) {
	c := &Client{}
	cfg := ApplyHelmChartConfig{URL: "https://example.com/a-0.1.0.tgz", Namespace: "ns"}
	unlock := lockHelmRelease(c, cfg)

	acquired := func(c *Client, cfg ApplyHelmChartConfig) bool {
		done := make(chan struct{})
		go func() {
			defer close(done)
			lockHelmRelease(c, cfg)()
		}()
		select {
		case <-done:
			return true
		case <-time.After(100 * time.Millisecond):
			return false
		}
	}

	other := cfg
	other.Namespace = "elsewhere"
	if !acquired(c, other) {
		t.Fatal("a release in another namespace must not wait")
	}
	named := cfg
	named.ReleaseName = "other"
	if !acquired(c, named) {
		t.Fatal("a differently named release must not wait")
	}
	if acquired(c, cfg) {
		t.Fatal("the same release must wait while the lock is held")
	}
	unlock()
	if !acquired(c, cfg) {
		t.Fatal("the release must be free after unlock")
	}
}
