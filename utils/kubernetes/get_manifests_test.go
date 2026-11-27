package kubernetes

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
)

func TestGetManifestsFromHelm(t *testing.T) {
	chartDir := writeTestChart(t)
	crdDir := filepath.Join(chartDir, "crds")
	if err := os.MkdirAll(crdDir, 0700); err != nil {
		t.Fatal(err)
	}
	crd := "apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\nmetadata:\n  name: widgets.example.io\n"
	if err := os.WriteFile(filepath.Join(crdDir, "widget.yaml"), []byte(crd), 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := loader.Load(chartDir)
	if err != nil {
		t.Fatalf("loader.Load() error = %v", err)
	}
	archivePath, err := chartutil.Save(loaded, t.TempDir())
	if err != nil {
		t.Fatalf("chartutil.Save() error = %v", err)
	}
	archive, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		_, _ = w.Write(archive)
	}))
	defer server.Close()

	got, err := GetManifestsFromHelm(server.URL + "/sample-0.1.0.tgz")
	if err != nil {
		t.Fatalf("GetManifestsFromHelm() error = %v", err)
	}
	if !strings.Contains(got, "kind: CustomResourceDefinition") {
		t.Errorf("GetManifestsFromHelm() = %q, want CRD manifest", got)
	}
}
