package kubernetes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestChart(t *testing.T) string {
	t.Helper()
	chartDir := filepath.Join(t.TempDir(), "sample")
	if err := os.MkdirAll(filepath.Join(chartDir, "templates"), 0700); err != nil {
		t.Fatal(err)
	}
	chartYAML := "apiVersion: v2\nname: sample\nversion: 0.1.0\nappVersion: \"1.2.3\"\n"
	manifest := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: sample\ndata:\n  key: value\n"
	if err := os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte(chartYAML), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chartDir, "templates", "configmap.yaml"), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	return chartDir
}

func TestConvertHelmChartToK8sManifest(t *testing.T) {
	chartDir := writeTestChart(t)
	manifest, err := ConvertHelmChartToK8sManifest(ApplyHelmChartConfig{LocalPath: chartDir})
	if err != nil {
		t.Fatalf("ConvertHelmChartToK8sManifest() error = %v", err)
	}
	if !strings.Contains(string(manifest), "kind: ConfigMap") || !strings.Contains(string(manifest), "name: sample") {
		t.Errorf("converted manifest does not contain expected ConfigMap: %s", manifest)
	}
}

func TestConvertHelmChartToK8sManifestInvalidPath(t *testing.T) {
	if _, err := ConvertHelmChartToK8sManifest(ApplyHelmChartConfig{LocalPath: filepath.Join(t.TempDir(), "missing")}); err == nil {
		t.Error("ConvertHelmChartToK8sManifest() error = nil, want chart load error")
	}
}
