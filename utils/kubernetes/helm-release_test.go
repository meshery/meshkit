package kubernetes

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

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

func TestReleaseDeployed(t *testing.T) {
	cases := []struct {
		name   string
		stored map[string]release.Status
		query  string
		want   bool
	}{
		{"no releases", nil, "testchart", false},
		{"deployed release", map[string]release.Status{"testchart": release.StatusDeployed}, "testchart", true},
		{"failed release does not count", map[string]release.Status{"testchart": release.StatusFailed}, "testchart", false},
		{"pending release does not count", map[string]release.Status{"testchart": release.StatusPendingInstall}, "testchart", false},
		{"other release only", map[string]release.Status{"other": release.StatusDeployed}, "testchart", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, s := memoryActionConfig()
			for name, status := range tc.stored {
				storeRelease(t, s, name, status)
			}
			got, err := releaseDeployed(cfg, tc.query)
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			if got != tc.want {
				t.Fatalf("releaseDeployed() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHelmReleaseDeployedUsesChartNameAsReleaseName(t *testing.T) {
	actionConfig, s := memoryActionConfig()
	orig := newHelmActionConfig
	t.Cleanup(func() { newHelmActionConfig = orig })
	newHelmActionConfig = func(*Client, ApplyHelmChartConfig, genericclioptions.RESTClientGetter) (*action.Configuration, error) {
		return actionConfig, nil
	}

	chartData := createTestChartArchive(t, "testchart", "0.1.0")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(chartData)
	}))
	t.Cleanup(ts.Close)

	cfg := ApplyHelmChartConfig{
		URL:              ts.URL + "/testchart-0.1.0.tgz",
		DownloadLocation: t.TempDir(),
		Namespace:        "default",
	}
	client := &Client{}

	got, err := client.HelmReleaseDeployed(cfg)
	if err != nil || got {
		t.Fatalf("before install: HelmReleaseDeployed() = %v, %v, want false, nil", got, err)
	}

	storeRelease(t, s, "testchart", release.StatusDeployed)
	got, err = client.HelmReleaseDeployed(cfg)
	if err != nil || !got {
		t.Fatalf("after install: HelmReleaseDeployed() = %v, %v, want true, nil", got, err)
	}

	// An explicit release name wins over the chart name.
	cfg.ReleaseName = "renamed"
	got, err = client.HelmReleaseDeployed(cfg)
	if err != nil || got {
		t.Fatalf("custom release name: HelmReleaseDeployed() = %v, %v, want false, nil", got, err)
	}
}

func TestHelmReleaseDeployedReportsUnloadableChart(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	t.Cleanup(ts.Close)

	_, err := (&Client{}).HelmReleaseDeployed(ApplyHelmChartConfig{
		URL:              ts.URL + "/missing-0.1.0.tgz",
		DownloadLocation: t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected an error when the chart cannot be fetched")
	}
}
