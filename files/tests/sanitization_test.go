package files_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/layer5io/meshkit/errors"
	"github.com/layer5io/meshkit/files"
	coreV1 "github.com/meshery/schemas/models/v1alpha1/core"
)

func TestSanitizeFile(t *testing.T) {
	testCases := []struct {
		name            string
		filePath        string
		expectedExt     string
		expectError     bool
		expectedErrCode string
		expectedContent map[string]interface{}
		expectedType    coreV1.IaCFileTypes
	}{
		{
			name:         "Valid JSON",
			filePath:     "./samples/valid.json",
			expectedExt:  ".json",
			expectedType: "",
		},
		{
			name:        "Invalid JSON",
			filePath:    "./samples/invalid.json",
			expectError: true,
		},
		{
			name:        "Valid YAML",
			filePath:    "./samples/valid.yml",
			expectedExt: ".yml",
		},
		{
			name:         "Invalid YAML",
			filePath:     "./samples/invalid.yml",
			expectError:  true,
			expectedType: "",
		},
		{
			name:            "Unsupported extension",
			filePath:        "./samples/valid.txt",
			expectError:     true,
			expectedErrCode: files.ErrUnsupportedExtensionCode,
		},
		{
			name:        "Valid compressed extension",
			filePath:    "./samples/valid.tar.gz",
			expectedExt: ".gz",
		},
		{
			name:        "Valid Nested Tar",
			filePath:    "./samples/nested.tar.gz",
			expectedExt: ".gz",
		},
		{
			name:        "Empty file",
			filePath:    "./samples/empty.json",
			expectError: true,
		},
		{
			name:        "invalid tar.gz",
			filePath:    "./samples/invalid.tar.gz",
			expectError: true,
		},

		{
			name:         "Can Identify Design",
			filePath:     "./samples/valid_design.yml",
			expectedExt:  ".yml",
			expectedType: coreV1.MesheryDesign,
		},

		{
			name:         "Can Identify Designs packaged as OCI images",
			filePath:     "./samples/valid-design-oci.tar",
			expectedExt:  ".tar",
			expectedType: coreV1.MesheryDesign,
		},
		{
			name:         "Can Identify Kubernetes Manifest",
			filePath:     "./samples/valid_manifest.yml",
			expectedExt:  ".yml",
			expectedType: coreV1.MesheryDesign,
		},

		{
			name:         "Can Identify Kubernetes Manifest With Crds",
			filePath:     "./samples/manifest-with-crds.yml",
			expectedExt:  ".yml",
			expectedType: coreV1.MesheryDesign,
		},

		{
			name:         "Can Identify HelmChart",
			filePath:     "./samples/valid-helm.tgz",
			expectedExt:  ".tgz",
			expectedType: coreV1.HelmChart,
		},
		{
			name:         "Can Identify Kustomize archive (tar.gz)",
			filePath:     "./samples/wordpress-kustomize.tar.gz",
			expectedExt:  ".gz",
			expectedType: coreV1.K8sKustomize,
		},
		{
			name:         "Can Identify Kustomize archive (zip)",
			filePath:     "./samples/wordpress-kustomize.zip",
			expectedExt:  ".zip",
			expectedType: coreV1.K8sKustomize,
		},
		{
			name:         "Can Identify Docker Compose",
			filePath:     "./samples/valid-docker-compose.yml",
			expectedExt:  ".yml",
			expectedType: coreV1.DockerCompose,
		},

		{
			name:         "Can Identify Docker Compose v2",
			filePath:     "./samples/valid-compose-2.yml",
			expectedExt:  ".yml",
			expectedType: coreV1.DockerCompose,
		},

		// {
		// 	name:         "Can Identify Docker Compose without version",
		// 	filePath:     "./samples/valid-compose-3.yml",
		// 	expectedExt:  ".yml",
		// 	expectedType: files.DOCKER_COMPOSE,
		// },
	}

	validExts := map[string]bool{
		".json":   true,
		".yml":    true,
		".yaml":   true,
		".tar":    true,
		".tar.gz": true,
		".tgz":    true,
		".zip":    true,
	}

	tempDir, _ := os.MkdirTemp(" ", "temp-tests")
	defer os.RemoveAll(tempDir)
	// tempDir := "./temp"

	for _, tc := range testCases {

		filename := filepath.Base(tc.filePath)
		// Read file bytes
		data, err := os.ReadFile(tc.filePath)
		if err != nil {
			t.Error("Error reading file:", err)
			continue
		}

		t.Run(tc.name, func(t *testing.T) {
			result, err := files.SanitizeFile(data, filename, tempDir, validExts)

			if tc.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tc.expectedErrCode != "" && errors.GetCode(err) != tc.expectedErrCode {
					t.Errorf("Expected error message %q, got %q", tc.expectedErrCode, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result.FileExt != tc.expectedExt {
				t.Errorf("Expected file extension %q, got %q", tc.expectedExt, result.FileExt)
			}

			if tc.expectedType != "" {
				identified_file, err := files.IdentifyFile(result)

				if (err != nil || identified_file.Type != tc.expectedType) && !tc.expectError {
					t.Errorf("Failed To Identify File as %s , got %s, errors %s", tc.expectedType, identified_file.Type, err)
				}

			}

		})
	}
}
