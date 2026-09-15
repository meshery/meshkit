package artifacthub

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
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

// TestUpdatePackageDataMalformedIndex verifies that UpdatePackageData returns an
// error (and does not panic) when the remote helm repository index.yaml has an
// unexpected shape. The chartUrl extraction previously asserted and indexed the
// parsed index without guards, so a malformed index crashed the generator.
func TestUpdatePackageDataMalformedIndex(t *testing.T) {
	serve := func(index string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, index)
		}))
	}

	tests := []struct {
		name      string
		index     string
		expectErr bool
	}{
		{
			name:      "empty version list for the chart",
			index:     "entries:\n  mychart: []\n",
			expectErr: true,
		},
		{
			name:      "chart entry is not a list",
			index:     "entries:\n  mychart:\n    foo: bar\n",
			expectErr: true,
		},
		{
			name:      "version entry has an empty urls list",
			index:     "entries:\n  mychart:\n    - urls: []\n",
			expectErr: true,
		},
		{
			name:      "first url is not a string",
			index:     "entries:\n  mychart:\n    - urls:\n        - 123\n",
			expectErr: true,
		},
		{
			name:      "well-formed index",
			index:     "entries:\n  mychart:\n    - urls:\n        - https://example.com/mychart-1.0.0.tgz\n",
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := serve(tt.index)
			defer srv.Close()

			pkg := AhPackage{Name: "mychart", RepoUrl: srv.URL}
			// Must return an error instead of panicking on malformed index data.
			err := pkg.UpdatePackageData()
			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected an error for a malformed helm index, got nil (ChartUrl=%q)", pkg.ChartUrl)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for a well-formed index: %v", err)
			}
			if pkg.ChartUrl == "" {
				t.Fatal("expected ChartUrl to be populated for a well-formed index")
			}
		})
	}
}
