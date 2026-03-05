package component

import (
	"fmt"
	"io"
	"net/http"
	"testing"
)

// TestGetResolvedManifest_RealKubernetesSpec downloads the actual Kubernetes
// OpenAPI spec and runs it through getResolvedManifest to verify that circular
// references (e.g. JSONSchemaProps) do not cause a stack overflow.
func TestGetResolvedManifest_RealKubernetesSpec(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// apiextensions spec contains JSONSchemaProps which references itself circularly.
	const specURL = "https://raw.githubusercontent.com/kubernetes/kubernetes/release-1.32/api/openapi-spec/v3/apis__apiextensions.k8s.io__v1_openapi.json"
	resp, err := http.Get(specURL)
	if err != nil {
		t.Fatalf("failed to fetch kubernetes spec: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	out, err := getResolvedManifest(string(body))
	if err != nil {
		t.Fatalf("getResolvedManifest failed: %v", err)
	}

	if len(out) == 0 {
		t.Fatal("expected non-empty output")
	}
	fmt.Printf("resolved manifest length: %d bytes\n", len(out))
}
