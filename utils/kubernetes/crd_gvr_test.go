package kubernetes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
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

// A CRD whose spec.versions (or plural name) is empty cannot be addressed through
// the Kubernetes API, so it must not surface as a GVR callers can list through.
func TestGetAllCustomResourcesInClusterSkipsUnaddressableCRDs(t *testing.T) {
	const crdPath = "/apis/apiextensions.k8s.io/v1/customresourcedefinitions"
	const body = `{
		"items": [
			{"spec": {"group": "example.com", "names": {"plural": "widgets"}, "versions": [{"name": "v1"}]}},
			{"spec": {"group": "example.com", "names": {"plural": "gadgets"}}},
			{"spec": {"group": "example.com", "names": {"plural": ""}, "versions": [{"name": "v1"}]}},
			{"spec": {"group": "example.com", "names": {"plural": "sprockets"}, "versions": [{"name": "v1beta1"}, {"name": "v1"}]}}
		]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != crdPath {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	restClient, err := rest.RESTClientFor(&rest.Config{
		Host: server.URL,
		ContentConfig: rest.ContentConfig{
			GroupVersion:         &schema.GroupVersion{},
			NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
		},
	})
	if err != nil {
		t.Fatalf("building the REST client: %v", err)
	}

	got, err := GetAllCustomResourcesInCluster(context.Background(), restClient)
	if err != nil {
		t.Fatalf("GetAllCustomResourcesInCluster() error = %v", err)
	}

	want := []*schema.GroupVersionResource{
		{Group: "example.com", Version: "v1", Resource: "widgets"},
		{Group: "example.com", Version: "v1beta1", Resource: "sprockets"},
	}
	if len(got) != len(want) {
		t.Fatalf("GetAllCustomResourcesInCluster() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] == nil || *got[i] != *want[i] {
			t.Errorf("GetAllCustomResourcesInCluster()[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}
