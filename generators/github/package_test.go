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
		{ // Source pointing to a directly downloadable file (not a repo per se)
			ghPackageManager: GitHubPackageManager{
				PackageName: "kuberntes",
				SourceURL:   "https://raw.githubusercontent.com/kubernetes/kubernetes/master/api/openapi-spec/v3/apis__autoscaling__v2_openapi.json/v1.29.2",
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
			comps, err := pkg.GenerateComponents()
			if err != nil {
				fmt.Println(err)
				t.Errorf("error while generating components: %v", err)
				return
			}
			for _, comp := range comps {
				currentDirectory, _ := os.Getwd()
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
