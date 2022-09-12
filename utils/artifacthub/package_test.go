package artifacthub

import "testing"

func TestGetChartUrl(t *testing.T) {
	var tests = []struct {
		ahpkg AhPackage
		want  string
	}{
		// these might change in the future, so the tests have to be changed as well when the urls change
		// because the urls will change with every new version update to the package
		{AhPackage{Name: "consul", Repository: "bitnami", Organization: ""}, "https://charts.bitnami.com/bitnami/consul-10.8.1.tgz"},
		{AhPackage{Name: "crossplane-types", Repository: "crossplane", Organization: ""}, "https://charts.crossplane.io/master/crossplane-types-0.13.0-rc.98.g1eb0776.tgz"},
	}
	for _, tt := range tests {
		t.Run("UpdatePackageData", func(t *testing.T) {
			err := tt.ahpkg.UpdatePackageData()
			if err != nil {
				t.Errorf("error while updating package data = %v", err)
				return
			}
			if tt.ahpkg.Url != tt.want {
				t.Errorf("got %v, want %v", tt.ahpkg.Url, tt.want)
			}
		})
	}
}
