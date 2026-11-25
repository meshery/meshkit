package github

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateCompFromGitHub(t *testing.T) {
	var tests = []struct {
		ghPackageManager GitHubPackageManager
		want             int
	}{
		// { // Source pointing to a directory
		// 	ghPackageManager: GitHubPackageManager{
		// 		PackageName: "k8s-config-connector",
		// 		SourceURL:   "git://github.com/GoogleCloudPlatform/k8s-config-connector/master/crds/",
		// 	},
		// 	want: 337,
		// },
		// { // Source pointing to a file in a repo
		// ghPackageManager: GitHubPackageManager{
		// 		PackageName: "k8s-config-connector",
		// 		SourceURL:   "git://github.com/GoogleCloudPlatform/k8s-config-connector/master/crds/accesscontextmanager_v1alpha1_accesscontextmanageraccesslevelcondition.yaml",
		// 	},
		// 	want: 1,
		// },

		// { // Source pointing to a directly downloadable file (not a repo per se)
		// 	ghPackageManager: GitHubPackageManager{
		// 		PackageName: "k8s-config-connector",
		// 		SourceURL:   "git://github.com/GoogleCloudPlatform/k8s-config-connector/master/crds/accesscontextmanager_v1alpha1_accesscontextmanageraccesslevelcondition.yaml",
		// 	},
		// 	want: 1,
		// },

		{ // Source pointing to a directly downloadable file (not a repo per se)
			ghPackageManager: GitHubPackageManager{
				PackageName: "gateway-api",
				SourceURL:   "https://raw.githubusercontent.com/kubernetes-sigs/gateway-api/main/config/crd/standard/gateway.networking.k8s.io_gatewayclasses.yaml",
			},
			want: 1,
		},

		// { // Source pointing to a directory containing helm chart
		// 	ghPackageManager: GitHubPackageManager{
		// 		PackageName: "acm-controller",
		// 		SourceURL:   "https://meshery.github.io/meshery.io/charts/meshery-v0.7.12.tgz/v0.7.12",
		// 	},
		// 	want: 2,
		// },
		{ // Source pointing to a zip containing manifests but no CRDs
			ghPackageManager: GitHubPackageManager{
				PackageName: "acm-controller",
				SourceURL:   "https://github.com/MUzairS15/WASM-filters/raw/main/test.tar.gz",
			},
			want: 0,
		},
		{ // Source pointing to a zip containing CRDs
			ghPackageManager: GitHubPackageManager{
				PackageName: "acm-controller",
				SourceURL:   "git://github.com/MUzairS15/WASM-filters/main/chart.tar.gz",
			},
			want: 14,
		},
		// { // Source pointing to a dir containing CRDs
		// 	ghPackageManager: GitHubPackageManager{
		// 		PackageName: "acm-controller",
		// 		SourceURL:   "git://github.com/meshery/meshery/master/install/kubernetes/helm/meshery-operator",
		// 	},
		// 	want: 2,
		// },
		{ // Source pointing to a directly downloadable file (not a repo per se), this package contains a single CRD with comments
			ghPackageManager: GitHubPackageManager{
				PackageName: "channels",
				SourceURL:   "https://raw.githubusercontent.com/knative/eventing/refs/heads/main/config/core/resources/channel.yaml",
			},
			want: 1,
		},
	}

	for _, test := range tests {
		t.Run("GenerateComponents", func(t *testing.T) {

			pkg, err := test.ghPackageManager.GetPackage()
			if err != nil {
				t.Errorf("error while getting package: %v", err)
				return
			}
			comps, err := pkg.GenerateComponents("")
			if err != nil {
				fmt.Println(err)
				t.Errorf("error while generating components: %v", err)
				return
			}
			// Write each component to a temporary directory: the test checks that
			// generation and serialization succeed, and must never rewrite files
			// in the source tree.
			outDir := t.TempDir()
			for _, comp := range comps {
				if comp.Model == nil {
					t.Errorf("component %s has nil Model", comp.Component.Kind)
					continue
				}
				dirName := filepath.Join(outDir, comp.Model.Name)
				if err := os.MkdirAll(dirName, fs.ModePerm); err != nil {
					t.Errorf("error creating dir at %s: %v", dirName, err)
					return
				}
				byt, err := json.MarshalIndent(comp, "", "")
				if err != nil {
					t.Errorf("error marshaling %s: %v", comp.Component.Kind, err)
					continue
				}
				if err := os.WriteFile(filepath.Join(dirName, comp.Component.Kind+".json"), byt, 0o600); err != nil {
					t.Errorf("error writing file for %s: %v", comp.Component.Kind, err)
				}
			}
			t.Log("generated ", len(comps), "want: ", test.want)
			if len(comps) != test.want {
				t.Errorf("generated %d, want %d", len(comps), test.want)
				return
			}
		})
	}
}
