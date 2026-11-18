package kubernetes

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	helmkubefake "helm.sh/helm/v3/pkg/kube/fake"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage"
	"helm.sh/helm/v3/pkg/storage/driver"
)

func TestSetupDefaults(t *testing.T) {
	cfg := ApplyHelmChartConfig{}
	setupDefaults(&cfg)

	if cfg.ChartLocation.Repository != Stable || cfg.ChartLocation.Version != Latest {
		t.Errorf("chart defaults = (%q, %q), want (%q, %q)", cfg.ChartLocation.Repository, cfg.ChartLocation.Version, Stable, Latest)
	}
	if cfg.HelmDriver != Secret || cfg.Namespace != "default" || cfg.Logger == nil {
		t.Errorf("general defaults not populated: %#v", cfg)
	}

	custom := ApplyHelmChartConfig{URL: "https://example.test/chart.tgz", Namespace: "custom", LocalPath: "chart.tgz"}
	setupDefaults(&custom)
	if custom.ChartLocation.Repository != "" || custom.ChartLocation.Version != "" {
		t.Errorf("chart repository defaults applied with explicit URL: %#v", custom.ChartLocation)
	}
	if custom.Namespace != "custom" {
		t.Errorf("Namespace = %q, want custom", custom.Namespace)
	}
}

func TestFetchHelmChartDownloadsAndReusesFile(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		_, _ = w.Write([]byte("chart archive"))
	}))
	defer server.Close()
	downloadDir := t.TempDir()

	path, err := fetchHelmChart(server.URL+"/sample.tgz", downloadDir)
	if err != nil {
		t.Fatalf("fetchHelmChart() error = %v", err)
	}
	contents, err := os.ReadFile(path)
	if err != nil || string(contents) != "chart archive" {
		t.Fatalf("downloaded chart = %q, %v", contents, err)
	}
	if _, err := fetchHelmChart(server.URL+"/sample.tgz", downloadDir); err != nil {
		t.Fatalf("fetchHelmChart() cached call error = %v", err)
	}
	if requests != 1 {
		t.Errorf("chart download requests = %d, want 1", requests)
	}
}

func TestApplyHelmChartInvalidLocalChart(t *testing.T) {
	err := (&Client{}).ApplyHelmChart(ApplyHelmChartConfig{LocalPath: filepath.Join(t.TempDir(), "missing")})
	if err == nil {
		t.Error("ApplyHelmChart() error = nil, want chart load error")
	}
}

func TestGetHelmLocalPathAndURL(t *testing.T) {
	localPath := filepath.Join(t.TempDir(), "chart.tgz")
	path, err := getHelmLocalPath(ApplyHelmChartConfig{LocalPath: localPath})
	if err != nil || path != localPath {
		t.Fatalf("getHelmLocalPath() = %q, %v, want %q", path, err, localPath)
	}

	url, err := getHelmChartURL(ApplyHelmChartConfig{URL: "https://example.test/chart.tgz"})
	if err != nil || url != "https://example.test/chart.tgz" {
		t.Fatalf("getHelmChartURL() = %q, %v", url, err)
	}
}

func TestCheckIfInstallable(t *testing.T) {
	for _, chartType := range []string{"", "application"} {
		if err := checkIfInstallable(&chart.Chart{Metadata: &chart.Metadata{Type: chartType}}); err != nil {
			t.Errorf("checkIfInstallable(%q) error = %v", chartType, err)
		}
	}
	if err := checkIfInstallable(&chart.Chart{Metadata: &chart.Metadata{Type: "library"}}); err == nil {
		t.Error("checkIfInstallable(library) error = nil, want error")
	}
}

func TestUpdateActionIfReleaseFound(t *testing.T) {
	tests := []struct {
		name       string
		seed       bool
		wantAction HelmChartAction
	}{
		{name: "release absent", wantAction: INSTALL},
		{name: "release present", seed: true, wantAction: UPGRADE},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actionConfig := &action.Configuration{
				KubeClient: &helmkubefake.PrintingKubeClient{Out: io.Discard},
				Releases:   storage.Init(driver.NewMemory()),
			}
			if tt.seed {
				if err := actionConfig.Releases.Create(&release.Release{Name: "sample", Namespace: "default", Version: 1, Info: &release.Info{Status: release.StatusDeployed}}); err != nil {
					t.Fatalf("seed release: %v", err)
				}
			}
			cfg := ApplyHelmChartConfig{ReleaseName: "sample", Action: INSTALL}
			if err := updateActionIfReleaseFound(actionConfig, &cfg, chart.Chart{}); err != nil {
				t.Fatalf("updateActionIfReleaseFound() error = %v", err)
			}
			if cfg.Action != tt.wantAction {
				t.Errorf("Action = %v, want %v", cfg.Action, tt.wantAction)
			}
		})
	}
}

func TestGenerateActionInstallDryRun(t *testing.T) {
	chart, err := loader.Load(writeTestChart(t))
	if err != nil {
		t.Fatalf("loader.Load() error = %v", err)
	}
	actionConfig := &action.Configuration{
		KubeClient:   &helmkubefake.PrintingKubeClient{Out: io.Discard},
		Releases:     storage.Init(driver.NewMemory()),
		Capabilities: chartutil.DefaultCapabilities,
		Log:          func(string, ...interface{}) {},
	}
	err = generateAction(actionConfig, ApplyHelmChartConfig{
		Action: INSTALL, ReleaseName: "sample", Namespace: "default", DryRun: true,
	})(chart)
	if err != nil {
		t.Fatalf("generateAction(install dry-run) error = %v", err)
	}
}
