package artifacthub

import (
	"testing"
)

func TestGetPackagesWithName(t *testing.T) {
	var tests = []struct {
		name string
	}{
		{"prometheus"},
		{"crossplane"},
	}
	for _, tt := range tests {
		t.Run("GetPackages", func(t *testing.T) {
			got, err := GetAhPackagesWithName(tt.name)
			if len(got) == 0 || err != nil {
				t.Errorf("got %v, want %v", got, "atleast one package")
			}
		})
	}
}

func Equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func TestFilterPackagesWithCrds(t *testing.T) {
	var tests = []struct {
		input []AhPackage
		want  []string
	}{
		{[]AhPackage{
			{Name: "crossplane-types", Repository: "crossplane", Organization: "", ChartUrl: "https://charts.crossplane.io/master/crossplane-types-0.13.0-rc.98.g1eb0776.tgz"},
			{Name: "crossplane", Repository: "crossplane", Organization: "", ChartUrl: "https://charts.crossplane.io/master/crossplane-1.10.0-rc.0.99.g7f471c48.tgz"},
		},
			[]string{
				"crossplane-types",
			},
		},
	}
	for _, tt := range tests {
		t.Run("FilterPackagesWithCrds", func(t *testing.T) {
			res := FilterPackagesWithCrds(tt.input)
			got := make([]string, 0)
			for _, ap := range res {
				got = append(got, ap.Name)
			}
			if !Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
