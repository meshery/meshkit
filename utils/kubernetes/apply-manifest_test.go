package kubernetes

import (
	"net/http"
	"testing"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func TestGetObjectFromManifest(t *testing.T) {
	object, unstructuredObject, err := GetObjectFromManifest(`apiVersion: v1
kind: ConfigMap
metadata:
  name: settings
  namespace: default
data:
  key: value`)
	if err != nil {
		t.Fatalf("GetObjectFromManifest() error = %v", err)
	}
	if object.GetObjectKind().GroupVersionKind().Kind != "ConfigMap" {
		t.Errorf("object kind = %q, want ConfigMap", object.GetObjectKind().GroupVersionKind().Kind)
	}
	if unstructuredObject.GetName() != "settings" || unstructuredObject.GetNamespace() != "default" {
		t.Errorf("metadata = %q/%q, want default/settings", unstructuredObject.GetNamespace(), unstructuredObject.GetName())
	}
}

func TestGetObjectFromManifestInvalid(t *testing.T) {
	if _, _, err := GetObjectFromManifest("kind: ["); err == nil {
		t.Error("GetObjectFromManifest() error = nil, want invalid manifest error")
	}
}

func TestCreateNamespaceIfNotExist(t *testing.T) {
	client := fake.NewSimpleClientset()
	if err := createNamespaceIfNotExist(t.Context(), client, "meshery"); err != nil {
		t.Fatalf("createNamespaceIfNotExist() error = %v", err)
	}
	if err := createNamespaceIfNotExist(t.Context(), client, "meshery"); err != nil {
		t.Fatalf("createNamespaceIfNotExist() second call error = %v", err)
	}
}

func TestApplyManifestCreate(t *testing.T) {
	server := newKubernetesAPIServer(t)
	defer server.Close()
	config := rest.Config{Host: server.URL}
	clientset, err := kubernetes.NewForConfig(&config)
	if err != nil {
		t.Fatalf("NewForConfig() error = %v", err)
	}
	client := &Client{RestConfig: config, KubeClient: clientset}
	manifest := []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: settings\n  namespace: default\ndata:\n  key: value\n")
	if err := client.ApplyManifest(manifest, ApplyOptions{}); err != nil {
		t.Fatalf("ApplyManifest() error = %v", err)
	}
}

func TestApplyManifestDelete(t *testing.T) {
	server := newKubernetesAPIServer(t)
	defer server.Close()
	config := rest.Config{Host: server.URL}
	clientset, err := kubernetes.NewForConfig(&config)
	if err != nil {
		t.Fatalf("NewForConfig() error = %v", err)
	}
	client := &Client{RestConfig: config, KubeClient: clientset}
	manifest := []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: settings\n  namespace: default\n")
	if err := client.ApplyManifest(manifest, ApplyOptions{Delete: true}); err != nil {
		t.Fatalf("ApplyManifest(delete) error = %v", err)
	}
}

func TestApplyManifestUpdateExisting(t *testing.T) {
	server := newKubernetesAPIServerWithConfigMapCreateStatus(t, http.StatusConflict)
	defer server.Close()
	config := rest.Config{Host: server.URL}
	clientset, err := kubernetes.NewForConfig(&config)
	if err != nil {
		t.Fatalf("NewForConfig() error = %v", err)
	}
	client := &Client{RestConfig: config, KubeClient: clientset}
	manifest := []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: settings\n  namespace: default\ndata:\n  key: updated\n")
	if err := client.ApplyManifest(manifest, ApplyOptions{Update: true}); err != nil {
		t.Fatalf("ApplyManifest(update) error = %v", err)
	}
}
