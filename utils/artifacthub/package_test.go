package artifacthub

import (
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
		})
	}
}
