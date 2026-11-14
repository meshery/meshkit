package kubernetes

import (
	"context"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"
)

func TestGetGVRForCustomResources(t *testing.T) {
	crd := &CRDItem{}
	crd.Spec.Group = "example.meshery.io"
	crd.Spec.Names.ResourceName = "widgets"
	crd.Spec.Versions = []struct {
		Name string `json:"name"`
	}{{Name: "v1alpha1"}}

	want := &schema.GroupVersionResource{Group: "example.meshery.io", Version: "v1alpha1", Resource: "widgets"}
	if got := GetGVRForCustomResources(crd); !reflect.DeepEqual(got, want) {
		t.Fatalf("GetGVRForCustomResources() = %#v, want %#v", got, want)
	}
}

func TestIsCRD(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
		want     bool
	}{
		{name: "custom resource definition", manifest: "apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\nmetadata:\n  name: widgets.example.io", want: true},
		{name: "other resource", manifest: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: config", want: false},
		{name: "invalid yaml", manifest: "kind: [", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsCRD(tt.manifest); got != tt.want {
				t.Errorf("IsCRD() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetAllCustomResourcesInCluster(t *testing.T) {
	server := newTestRESTServer(t, `{"items":[{"spec":{"group":"example.meshery.io","names":{"plural":"widgets"},"versions":[{"name":"v1"}]}}]}`)
	defer server.Close()

	config := rest.Config{
		Host: server.URL,
		ContentConfig: rest.ContentConfig{
			GroupVersion:         &schema.GroupVersion{Version: "v1"},
			NegotiatedSerializer: serializer.NewCodecFactory(runtime.NewScheme()),
		},
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		t.Fatalf("rest.RESTClientFor() error = %v", err)
	}

	got, err := GetAllCustomResourcesInCluster(context.Background(), client)
	if err != nil {
		t.Fatalf("GetAllCustomResourcesInCluster() error = %v", err)
	}
	want := []*schema.GroupVersionResource{{Group: "example.meshery.io", Version: "v1", Resource: "widgets"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetAllCustomResourcesInCluster() = %#v, want %#v", got, want)
	}
}
