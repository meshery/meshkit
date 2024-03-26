package artifacthub

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"
)

func TestGetChartUrl(t *testing.T) {
	var tests = []struct {
		ahpkg AhPackage
		want  string
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
			if !strings.HasPrefix(tt.ahpkg.ChartUrl, tt.want) {
				t.Errorf("got %v, want %v", tt.ahpkg.ChartUrl, tt.want)
			}

			comps, err := tt.ahpkg.GenerateComponents()
			if err != nil {
				fmt.Println(err)
				t.Errorf("error while generating components: %v", err)
				return
			}
			for _, comp := range comps {
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
