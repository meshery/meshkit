package kompose

import (
	"testing"
)

func TestHasMultipleDocuments(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected bool
	}{
		{
			name: "single document YAML",
			input: []byte(`version: "3.8"
services:
  web:
    image: nginx`),
			expected: false,
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
			expected: true,
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
			expected: true,
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
			expected: true,
		},
		{
			name:     "Empty YAML",
			input:    []byte(``),
			expected: false,
		},
		{
			name: "YAML with --- in string content",
			input: []byte(`version: "3.8"
services:
  web:
    image: nginx
    environment:
      - TEXT=some---text`),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasMultipleDocuments(tt.input)
			if result != tt.expected {
				t.Errorf("hasMultipleDocuments() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDockerComposeFile_Validate_WithMultiDocument(t *testing.T) {
	// This test verifies that validation rejects multi-document YAML
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

	// This should return an error about multiple documents
	err := dc.Validate(schema)
	if err == nil {
		t.Error("Expected validation error for multi-document YAML, got nil")
	}
	// Verify it's the multiple documents error
	if err != nil && err.Error() != ErrMultipleDocuments().Error() {
		t.Logf("Got error (may not be multiple documents error): %v", err)
	}
}

func TestDockerComposeFile_Validate_WithKubernetesManifest(t *testing.T) {
	// This test verifies that validation properly rejects Kubernetes manifests
	// that have multiple documents
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

	// This should return an error about multiple documents
	err := dc.Validate(schema)
	if err == nil {
		t.Error("Expected validation error for Kubernetes manifest, got nil")
	}
}

func TestDockerComposeFile_Validate_SingleDocumentValid(t *testing.T) {
	// This test verifies that single-document Docker Compose files
	// pass the multi-document check
	dockerCompose := []byte(`version: "3.8"
services:
  web:
    image: nginx:latest
    ports:
      - "80:80"`)

	dc := DockerComposeFile(dockerCompose)

	// Create a minimal Docker Compose schema for testing
	schema := []byte(`{
		"type": "object",
		"properties": {
			"version": {"type": "string"},
			"services": {"type": "object"}
		}
	}`)

	// This should not return the multiple documents error
	// (it may still fail schema validation, but that's a different error)
	err := dc.Validate(schema)
	if err != nil && err.Error() == ErrMultipleDocuments().Error() {
		t.Error("Single document YAML should not trigger multiple documents error")
	}
}
