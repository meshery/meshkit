package kubernetes

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/kube"
	kubefake "helm.sh/helm/v3/pkg/kube/fake"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func TestWithHelmSQLConnectionStringRestoresExistingValue(t *testing.T) {
	t.Setenv(helmDriverSQLConnectionStringEnv, "existing-value")

	err := withHelmSQLConnectionString("temporary-value", func() error {
		if got := os.Getenv(helmDriverSQLConnectionStringEnv); got != "temporary-value" {
			t.Fatalf("SQL connection string during initialization = %q, want %q", got, "temporary-value")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("withHelmSQLConnectionString() error = %v", err)
	}
	if got := os.Getenv(helmDriverSQLConnectionStringEnv); got != "existing-value" {
		t.Fatalf("restored SQL connection string = %q, want %q", got, "existing-value")
	}
}

func TestWithHelmSQLConnectionStringRestoresAfterInitializationError(t *testing.T) {
	t.Setenv(helmDriverSQLConnectionStringEnv, "existing-value")
	wantErr := errors.New("initialization failed")

	err := withHelmSQLConnectionString("temporary-value", func() error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("withHelmSQLConnectionString() error = %v, want %v", err, wantErr)
	}
	if got := os.Getenv(helmDriverSQLConnectionStringEnv); got != "existing-value" {
		t.Fatalf("restored SQL connection string = %q, want %q", got, "existing-value")
	}
}

func TestWithHelmSQLConnectionStringChecksSetenvError(t *testing.T) {
	called := false
	err := withHelmSQLConnectionString("invalid\x00value", func() error {
		called = true
		return nil
	})
	if err == nil {
		t.Fatal("withHelmSQLConnectionString() error = nil, want invalid environment value error")
	}
	if called {
		t.Fatal("initializer was called after os.Setenv failed")
	}
}

func TestCreateHelmActionConfigDoesNotSetSQLConnectionStringForNonSQLDriver(t *testing.T) {
	t.Setenv(helmDriverSQLConnectionStringEnv, "existing-value")

	_, err := (&Client{}).createHelmActionConfig(ApplyHelmChartConfig{
		HelmDriver:          Secret,
		SQLConnectionString: "unused-sensitive-value",
	}, nil)
	if err != nil {
		t.Fatalf("createHelmActionConfig() error = %v", err)
	}
	if got := os.Getenv(helmDriverSQLConnectionStringEnv); got != "existing-value" {
		t.Fatalf("SQL connection string = %q, want unchanged value %q", got, "existing-value")
	}
}

func createTestChartArchive(t *testing.T, chartName, version string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	chartYAML := fmt.Sprintf("apiVersion: v2\nname: %s\nversion: %s\ntype: application\n", chartName, version)
	cmYAML := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test-cm\ndata:\n  key: value\n"

	files := []struct {
		name string
		body string
	}{
		{name: chartName + "/Chart.yaml", body: chartYAML},
		{name: chartName + "/templates/configmap.yaml", body: cmYAML},
	}

	for _, file := range files {
		hdr := &tar.Header{
			Name: file.name,
			Mode: 0644,
			Size: int64(len(file.body)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("failed to write tar header: %v", err)
		}
		if _, err := tw.Write([]byte(file.body)); err != nil {
			t.Fatalf("failed to write tar body: %v", err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("failed to close tar writer: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("failed to close gzip writer: %v", err)
	}

	return buf.Bytes()
}

func TestSetKubeClientNamespace(t *testing.T) {
	t.Run("sets namespace on real kube.Client", func(t *testing.T) {
		kc := kube.New(genericclioptions.NewConfigFlags(false))
		actionConfig := &action.Configuration{
			KubeClient: kc,
		}
		setKubeClientNamespace(actionConfig, "custom-namespace")
		if kc.Namespace != "custom-namespace" {
			t.Fatalf("kc.Namespace = %q, want %q", kc.Namespace, "custom-namespace")
		}
	})

	t.Run("safely handles nil action configuration", func(t *testing.T) {
		setKubeClientNamespace(nil, "custom-namespace")
	})

	t.Run("does nothing for empty namespace", func(t *testing.T) {
		kc := kube.New(genericclioptions.NewConfigFlags(false))
		kc.Namespace = "existing-namespace"
		actionConfig := &action.Configuration{
			KubeClient: kc,
		}
		setKubeClientNamespace(actionConfig, "")
		if kc.Namespace != "existing-namespace" {
			t.Fatalf("kc.Namespace = %q, want unchanged %q", kc.Namespace, "existing-namespace")
		}
	})

	t.Run("safely leaves fake and non-concrete KubeClient unchanged", func(t *testing.T) {
		fakeClient := &kubefake.PrintingKubeClient{Out: io.Discard}
		actionConfig := &action.Configuration{
			KubeClient: fakeClient,
		}
		setKubeClientNamespace(actionConfig, "custom-namespace")
		if actionConfig.KubeClient != fakeClient {
			t.Fatal("expected KubeClient reference to be unchanged")
		}
	})
}

func TestApplyHelmChart_InMemoryInstallSuccess(t *testing.T) {
	memDriver := driver.NewMemory()
	memDriver.SetNamespace("default")
	memStorage := storage.Init(memDriver)
	actionConfig := &action.Configuration{
		Releases:     memStorage,
		KubeClient:   &kubefake.PrintingKubeClient{Out: io.Discard},
		Capabilities: chartutil.DefaultCapabilities,
		Log:          func(format string, v ...interface{}) {},
	}

	orig := newHelmActionConfig
	t.Cleanup(func() { newHelmActionConfig = orig })
	newHelmActionConfig = func(c *Client, cfg ApplyHelmChartConfig, getter genericclioptions.RESTClientGetter) (*action.Configuration, error) {
		return actionConfig, nil
	}

	chartData := createTestChartArchive(t, "testchart", "0.1.0")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(chartData)
	}))
	t.Cleanup(ts.Close)

	client := &Client{}
	err := client.ApplyHelmChart(ApplyHelmChartConfig{
		URL:              ts.URL + "/testchart-0.1.0.tgz",
		DownloadLocation: t.TempDir(),
		Action:           INSTALL,
		Namespace:        "default",
	})
	if err != nil {
		t.Fatalf("ApplyHelmChart() unexpected error: %v", err)
	}

	rel, err := memStorage.Get("testchart", 1)
	if err != nil {
		t.Fatalf("failed to retrieve installed release from storage: %v", err)
	}
	if rel.Name != "testchart" {
		t.Fatalf("installed release name = %q, want %q", rel.Name, "testchart")
	}
}

func TestApplyHelmChart_InMemoryInstallFailure(t *testing.T) {
	createErr := errors.New("simulated cluster failure during create")
	failingClient := &kubefake.FailingKubeClient{
		CreateError: createErr,
	}
	memDriver := driver.NewMemory()
	memDriver.SetNamespace("default")
	memStorage := storage.Init(memDriver)
	actionConfig := &action.Configuration{
		Releases:     memStorage,
		KubeClient:   failingClient,
		Capabilities: chartutil.DefaultCapabilities,
		Log:          func(format string, v ...interface{}) {},
	}

	orig := newHelmActionConfig
	t.Cleanup(func() { newHelmActionConfig = orig })
	newHelmActionConfig = func(c *Client, cfg ApplyHelmChartConfig, getter genericclioptions.RESTClientGetter) (*action.Configuration, error) {
		return actionConfig, nil
	}

	chartData := createTestChartArchive(t, "testchart", "0.1.0")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(chartData)
	}))
	t.Cleanup(ts.Close)

	client := &Client{}
	err := client.ApplyHelmChart(ApplyHelmChartConfig{
		URL:              ts.URL + "/testchart-0.1.0.tgz",
		DownloadLocation: t.TempDir(),
		Action:           INSTALL,
		CreateNamespace:  true,
		Namespace:        "default",
	})
	if err == nil {
		t.Fatal("expected ApplyHelmChart() to fail with create error, got nil")
	}
	if !strings.Contains(err.Error(), "simulated cluster failure during create") {
		t.Fatalf("error = %v, want error containing simulated failure", err)
	}
}

func TestApplyHelmChart_InMemoryUpgrade(t *testing.T) {
	memDriver := driver.NewMemory()
	memDriver.SetNamespace("default")
	memStorage := storage.Init(memDriver)

	existingRel := &release.Release{
		Name:      "testchart",
		Namespace: "default",
		Version:   1,
		Info: &release.Info{
			Status: release.StatusDeployed,
		},
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{
				Name:    "testchart",
				Version: "0.1.0",
				Type:    "application",
			},
		},
	}
	if err := memStorage.Create(existingRel); err != nil {
		t.Fatalf("failed to seed existing release: %v", err)
	}

	actionConfig := &action.Configuration{
		Releases:     memStorage,
		KubeClient:   &kubefake.PrintingKubeClient{Out: io.Discard},
		Capabilities: chartutil.DefaultCapabilities,
		Log:          func(format string, v ...interface{}) {},
	}

	orig := newHelmActionConfig
	t.Cleanup(func() { newHelmActionConfig = orig })
	newHelmActionConfig = func(c *Client, cfg ApplyHelmChartConfig, getter genericclioptions.RESTClientGetter) (*action.Configuration, error) {
		return actionConfig, nil
	}

	chartData := createTestChartArchive(t, "testchart", "0.2.0")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(chartData)
	}))
	t.Cleanup(ts.Close)

	client := &Client{}
	err := client.ApplyHelmChart(ApplyHelmChartConfig{
		URL:                ts.URL + "/testchart-0.2.0.tgz",
		DownloadLocation:   t.TempDir(),
		Action:             INSTALL,
		UpgradeIfInstalled: true,
		Namespace:          "default",
	})
	if err != nil {
		t.Fatalf("ApplyHelmChart() upgrade unexpected error: %v", err)
	}

	rel, err := memStorage.Get("testchart", 2)
	if err != nil {
		t.Fatalf("failed to retrieve upgraded revision 2: %v", err)
	}
	if rel.Chart.Metadata.Version != "0.2.0" {
		t.Fatalf("upgraded chart version = %q, want %q", rel.Chart.Metadata.Version, "0.2.0")
	}
}

func TestApplyHelmChart_InMemoryUninstall(t *testing.T) {
	memDriver := driver.NewMemory()
	memDriver.SetNamespace("default")
	memStorage := storage.Init(memDriver)

	existingRel := &release.Release{
		Name:      "testchart",
		Namespace: "default",
		Version:   1,
		Info: &release.Info{
			Status: release.StatusDeployed,
		},
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{
				Name:    "testchart",
				Version: "0.1.0",
				Type:    "application",
			},
		},
	}
	if err := memStorage.Create(existingRel); err != nil {
		t.Fatalf("failed to seed existing release: %v", err)
	}

	actionConfig := &action.Configuration{
		Releases:     memStorage,
		KubeClient:   &kubefake.PrintingKubeClient{Out: io.Discard},
		Capabilities: chartutil.DefaultCapabilities,
		Log:          func(format string, v ...interface{}) {},
	}

	orig := newHelmActionConfig
	t.Cleanup(func() { newHelmActionConfig = orig })
	newHelmActionConfig = func(c *Client, cfg ApplyHelmChartConfig, getter genericclioptions.RESTClientGetter) (*action.Configuration, error) {
		return actionConfig, nil
	}

	chartData := createTestChartArchive(t, "testchart", "0.1.0")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(chartData)
	}))
	t.Cleanup(ts.Close)

	client := &Client{}
	err := client.ApplyHelmChart(ApplyHelmChartConfig{
		URL:              ts.URL + "/testchart-0.1.0.tgz",
		DownloadLocation: t.TempDir(),
		Action:           UNINSTALL,
		Namespace:        "default",
	})
	if err != nil {
		t.Fatalf("ApplyHelmChart() uninstall unexpected error: %v", err)
	}

	_, err = memStorage.Get("testchart", 1)
	if !errors.Is(err, driver.ErrReleaseNotFound) {
		t.Fatalf("expected release to be purged (ErrReleaseNotFound), got %v", err)
	}
}
