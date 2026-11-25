package kubernetes

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestGetGVRForCustomResourcesHandlesMissingVersions(t *testing.T) {
	tests := []struct {
		name string
		crd  *CRDItem
		want *schema.GroupVersionResource
	}{
		{name: "nil CRD"},
		{
			name: "no versions",
			crd: &CRDItem{Spec: Spec{
				Group: "example.com",
				Names: names{ResourceName: "widgets"},
			}},
			want: &schema.GroupVersionResource{Group: "example.com", Resource: "widgets"},
		},
		{
			name: "first version",
			crd: &CRDItem{Spec: Spec{
				Group: "example.com",
				Names: names{ResourceName: "widgets"},
				Versions: []struct {
					Name string `json:"name"`
				}{{Name: "v1beta1"}, {Name: "v1"}},
			}},
			want: &schema.GroupVersionResource{Group: "example.com", Version: "v1beta1", Resource: "widgets"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetGVRForCustomResources(tt.crd)
			if got == tt.want {
				return
			}
			if got == nil || tt.want == nil || *got != *tt.want {
				t.Errorf("GetGVRForCustomResources() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
