package artifacthub

import (
	"reflect"
	"testing"
)

func TestSortPackagesWithScore(t *testing.T) {
	var tests = []struct {
		pkgs []AhPackage
		want []AhPackage
	}{
		{[]AhPackage{
			{Official: false, VerifiedPublisher: false},
			{Official: false, VerifiedPublisher: true},
			{Official: true, VerifiedPublisher: true},
		}, []AhPackage{
			{Official: true, VerifiedPublisher: true},
			{Official: false, VerifiedPublisher: true},
			{Official: false, VerifiedPublisher: false},
		}},
	}
	for _, tt := range tests {
		t.Run("SortPackages", func(t *testing.T) {
			got := SortPackagesWithScore(tt.pkgs)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
