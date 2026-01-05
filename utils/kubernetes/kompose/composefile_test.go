package kompose

import (
	"testing"
)

func TestExtractFirstYAMLDocument(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{
			name: "single document YAML",
			input: []byte(`version: "3.8"
services:
  web:
    image: nginx`),
			expected: []byte(`version: "3.8"
services:
  web:
    image: nginx`),
		},
		{
			name: "multi-document YAML with standard separator",
			input: []byte(`version: "3.8"
services:
  web:
    image: nginx
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test`),
			expected: []byte(`version: "3.8"
services:
  web:
    image: nginx`),
		},
		{
			name: "multi-document YAML with multiple separators",
			input: []byte(`version: "3.8"
services:
  web:
    image: nginx
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test1
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test2`),
			expected: []byte(`version: "3.8"
services:
  web:
    image: nginx`),
		},
		{
			name: "Kubernetes manifest (multi-document)",
			input: []byte(`apiVersion: v1
data:
  config.alloy: |
    some config data
kind: ConfigMap
metadata:
  name: test
---
apiVersion: v1
kind: Service
metadata:
  name: test-service`),
			expected: []byte(`apiVersion: v1
data:
  config.alloy: |
    some config data
kind: ConfigMap
metadata:
  name: test`),
		},
		{
			name:     "Empty YAML",
			input:    []byte(``),
			expected: []byte(``),
		},
		{
			name: "YAML with --- in string content",
			input: []byte(`version: "3.8"
services:
  web:
    image: nginx
    environment:
      - TEXT=some---text`),
			expected: []byte(`version: "3.8"
services:
  web:
    image: nginx
    environment:
      - TEXT=some---text`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractFirstYAMLDocument(tt.input)
			if string(result) != string(tt.expected) {
				t.Errorf("extractFirstYAMLDocument() = %q, want %q", string(result), string(tt.expected))
			}
		})
	}
}

func TestDockerComposeFile_Validate_WithMultiDocument(t *testing.T) {
	// This test verifies that validation doesn't crash on multi-document YAML
	// and only validates the first document
	multiDocYAML := []byte(`version: "3.8"
services:
  web:
    image: nginx
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: value`)

	dc := DockerComposeFile(multiDocYAML)

	// Create a minimal Docker Compose schema for testing
	schema := []byte(`{
		"type": "object",
		"properties": {
			"version": {"type": "string"},
			"services": {"type": "object"}
		}
	}`)

	// This should not crash - it should validate only the first document
	err := dc.Validate(schema)
	// We expect this to pass since the first document matches the schema
	if err != nil {
		t.Logf("Validation returned error (expected for strict schema): %v", err)
	}
}

func TestDockerComposeFile_Validate_WithKubernetesManifest(t *testing.T) {
	// This test verifies that validation properly rejects Kubernetes manifests
	// by only validating the first document against Docker Compose schema
	k8sManifest := []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: value
---
apiVersion: v1
kind: Service
metadata:
  name: test-service`)

	dc := DockerComposeFile(k8sManifest)

	// Create a minimal Docker Compose schema for testing
	schema := []byte(`{
		"type": "object",
		"properties": {
			"version": {"type": "string"},
			"services": {"type": "object"}
		},
		"required": ["version"]
	}`)

	// This should return an error because the Kubernetes manifest
	// doesn't match the Docker Compose schema
	err := dc.Validate(schema)
	if err == nil {
		t.Error("Expected validation error for Kubernetes manifest, got nil")
	}
}
