package github

import (
	"fmt"
	"testing"
)

func TestGenerateCompFromGitHub(t *testing.T) {
	var tests = []struct {
		ghPackageManager GitHubPackageManager
		want             int
	}{
		{
			ghPackageManager: GitHubPackageManager{
				PackageName: "k8s-config-connector",
				SourceURL:   "git://github.com/GoogleCloudPlatform/k8s-config-connector/master/v1.112.0/crds/",
			},
			want: 100,
		},
	}

	for _, test := range tests {
		t.Run("GenerateComponents", func(t *testing.T) {

			pkg, err := test.ghPackageManager.GetPackage()
			t.Logf("ERROR :: %v", err)
			if err != nil {
				t.Errorf("error while getting package: %v", err)
				return
			}
			comps, err := pkg.GenerateComponents()
			t.Logf("ERROR 22222 :: %v", err)
			if err != nil {
				fmt.Println(err)
				t.Errorf("error while generating components: %v", err)
				return
			}
			t.Log("GOT ", len(comps), "WANT ", test.want)
			if len(comps) != test.want {
				t.Errorf("generated %d, want %d", len(comps), test.want)
				return
			}
		})
	}
}
