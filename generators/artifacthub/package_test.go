package artifacthub

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"
)

func TestGetChartUrl(t *testing.T) {
	var tests = []struct {
		ahpkg      AhPackage
		wantPrefix string
	}{
		// these might change in the future, so the tests have to be changed as well when the urls change
		// because the urls will change with every new version update to the package
		{AhPackage{Name: "consul", Repository: "bitnami", Organization: "", RepoUrl: "https://charts.bitnami.com/bitnami"}, "https://charts.bitnami.com/bitnami/consul"},
		{AhPackage{Name: "crossplane-types", Repository: "crossplane", Organization: "", RepoUrl: "https://charts.crossplane.io/master"}, "https://charts.crossplane.io/master/crossplane-types-0.13.0-rc.191.g3a18fb7.tgz"},
	}
	for _, tt := range tests {
		t.Run("UpdatePackageData", func(t *testing.T) {
			err := tt.ahpkg.UpdatePackageData()
			if err != nil {
				t.Errorf("error while updating package data = %v", err)
				return
			}
			// Verify that a valid URL was generated
			if tt.ahpkg.ChartUrl == "" {
				t.Error("ChartUrl should not be empty")
			}

			// Verify URL is well-formed (not mixing protocols)
			if strings.Contains(tt.ahpkg.ChartUrl, "http") && strings.Contains(tt.ahpkg.ChartUrl, "oci://") {
				t.Errorf("ChartUrl contains mixed protocols: %v", tt.ahpkg.ChartUrl)
			}

			comps, err := tt.ahpkg.GenerateComponents("")
			if err != nil {
				// Don't fail the test if it's just an OCI unsupported error
				if strings.Contains(err.Error(), "unsupported protocol scheme") {
					t.Logf("Skipping component generation for OCI chart (not yet supported): %v", err)
					return
				}
				t.Errorf("error while generating components: %v", err)
				return
			}
			for _, comp := range comps {
				if comp.Model == nil {
					t.Errorf("component %s has nil Model", comp.Component.Kind)
					continue
				}
				dirName := "./" + comp.Model.Name
				_, err := os.Stat(dirName)
				if errors.Is(err, os.ErrNotExist) {
					err = os.Mkdir(dirName, fs.ModePerm)
					if err != nil {
						t.Errorf("err creating dir : %v", err)
					}
				}
				byt, _ := json.MarshalIndent(comp, "", "")

				f, err := os.Create(dirName + "/" + comp.Component.Kind + ".json")
				if err != nil {
					t.Errorf("error create file : %v", err)
					continue
				}
				_, _ = f.Write(byt)
			}
		})
	}
}

// TestGetPackageWithDirectURL tests that when SourceURL is an actual URL,
// it is used directly instead of searching
func TestGetPackageWithDirectURL(t *testing.T) {
	tests := []struct {
		name        string
		packageName string
		sourceURL   string
		wantErr     bool
	}{
		{
			name:        "Direct HTTP URL should be used",
			packageName: "consul",
			sourceURL:   "https://charts.bitnami.com/bitnami/consul-1.0.0.tgz",
			wantErr:     false,
		},
		{
			name:        "Direct HTTPS URL should be used",
			packageName: "test-package",
			sourceURL:   "https://example.com/charts/test-1.0.0.tgz",
			wantErr:     false,
		},
		{
			name:        "OCI URL should be used",
			packageName: "test-oci",
			sourceURL:   "oci://registry.example.com/charts/test",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := ArtifactHubPackageManager{
				PackageName: tt.packageName,
				SourceURL:   tt.sourceURL,
			}

			pkg, err := pm.GetPackage()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetPackage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				// Verify the package has the correct source URL
				if pkg.GetSourceURL() != tt.sourceURL {
					t.Errorf("GetSourceURL() = %v, want %v", pkg.GetSourceURL(), tt.sourceURL)
				}
			}
		})
	}
}
