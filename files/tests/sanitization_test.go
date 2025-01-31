package files_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/layer5io/meshkit/files"
)

func TestSanitizeFile(t *testing.T) {
	testCases := []struct {
		name            string
		filePath        string
		expectedExt     string
		expectError     bool
		expectedErrMsg  string
		expectedContent map[string]interface{}
		expectedType    string
	}{
		{
			name:        "Valid JSON",
			filePath:    "./samples/valid.json",
			expectedExt: ".json",
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
			name:        "Invalid YAML",
			filePath:    "./samples/invalid.yml",
			expectError: true,
		},
		{
			name:           "Unsupported extension",
			filePath:       "./samples/valid.txt",
			expectError:    true,
			expectedErrMsg: fmt.Sprintf("unsupported file extension: %s", ".txt"),
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
			expectedType: files.MESHERY_DESIGN,
		},
		{
			name:         "Can Identify Kubernetes Manifest",
			filePath:     "./samples/valid_manifest.yml",
			expectedExt:  ".yml",
			expectedType: files.KUBERNETES_MANIFEST,
		},

		{
			name:         "Can Identify HelmChart",
			filePath:     "./samples/valid-helm.tgz",
			expectedExt:  ".tgz",
			expectedType: files.HELM_CHART,
		},
		{
			name:         "Can Identify Kustomize archive",
			filePath:     "./samples/wordpress-kustomize.tar.gz",
			expectedExt:  ".gz",
			expectedType: files.KUSTOMIZATION,
		},
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
			result, err := files.SanitizeFile(data, filename, tempDir)

			if tc.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tc.expectedErrMsg != "" && err.Error() != tc.expectedErrMsg {
					t.Errorf("Expected error message %q, got %q", tc.expectedErrMsg, err.Error())
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
