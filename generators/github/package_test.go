package github

import (
	"encoding/json"
	"errors"
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
				PackageName: "k8s-config-connector",
				SourceURL:   "https://raw.githubusercontent.com/GoogleCloudPlatform/k8s-config-connector/master/crds/alloydb_v1beta1_alloydbbackup.yaml",
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

		// This test case is to solve the ExtractZip function's root path handling
		{ // Source pointing to a zip containing CRDs
			ghPackageManager: GitHubPackageManager{
				PackageName: "acm-controller",
				SourceURL:   "git://github.com/muhammadolammi/meshcrds/main/crds.zip",
			},
			want: 2,
		},
		// This test case is for the feature where root is empty in the source URL
		{ // Source pointing to a git with just branch containing crds in root and a zip
			ghPackageManager: GitHubPackageManager{
				PackageName: "acm-controller",
				SourceURL:   "git://github.com/muhammadolammi/meshcrds/main/",
			},
			want: 5,
		},
		// This test case is for the feature where root and branch is empty in the source URL
		{ // Source pointing to a git with no branch containing crds in root and a zip
			ghPackageManager: GitHubPackageManager{
				PackageName: "acm-controller",
				SourceURL:   "git://github.com/muhammadolammi/meshcrds/",
			},
			want: 5,
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
			for _, comp := range comps {
				currentDirectory, _ := os.Getwd()
				if comp.Model == nil {
					t.Errorf("component %s has nil Model", comp.Component.Kind)
					continue
				}
				dirName := filepath.Join(currentDirectory, comp.Model.Name)
				_, err := os.Stat(dirName)
				if errors.Is(err, os.ErrNotExist) {
					err = os.Mkdir(dirName, fs.ModePerm)
					if err != nil {
						t.Errorf("error creating dir at %s: %v", dirName, err)
						return
					}
				}
				byt, _ := json.MarshalIndent(comp, "", "")

				f, err := os.Create(fmt.Sprintf("%s/%s%s", dirName, comp.Component.Kind, ".json"))
				if err != nil {
					t.Errorf("error creating file for %s: %v", comp.Component.Kind, err)
					continue
				}
				_, _ = f.Write(byt)
			}

			t.Log("generated ", len(comps), "want: ", test.want)

			if len(comps) != test.want {
				t.Errorf("generated %d, want %d", len(comps), test.want)
				return
			}

		})
	}
}
