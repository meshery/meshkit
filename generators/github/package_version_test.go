package github

import (
	"os"
	"path/filepath"
	"testing"
)

const widgetCRD = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: widgets.example.com
spec:
  group: example.com
  names:
    kind: Widget
    plural: widgets
    singular: widget
  scope: Namespaced
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
`

// A GitHub source's version is its release tag or asset file name. The model
// version, its directory and every component's model version must carry the
// version that name holds, not the tag or file name (meshery/meshery#23094).
func TestGitHubPackageVersionIsTheVersionItsTagCarries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "widgets.yaml")
	if err := os.WriteFile(path, []byte(widgetCRD), 0o644); err != nil {
		t.Fatal(err)
	}
	pkg := GitHubPackage{Name: "widgets", filePath: path, version: "release-v1.7.6-rc1"}

	if got := pkg.GetVersion(); got != "v1.7.6-rc1" {
		t.Errorf("GetVersion() = %q, want %q", got, "v1.7.6-rc1")
	}

	comps, err := pkg.GenerateComponents("")
	if err != nil {
		t.Fatalf("GenerateComponents: %v", err)
	}
	if len(comps) == 0 {
		t.Fatal("GenerateComponents returned no components")
	}
	for _, comp := range comps {
		if comp.Model == nil {
			t.Fatalf("component %s has no model", comp.Component.Kind)
		}
		if comp.Model.Version != "v1.7.6-rc1" {
			t.Errorf("component %s model version = %q, want %q", comp.Component.Kind, comp.Model.Version, "v1.7.6-rc1")
		}
	}
}
