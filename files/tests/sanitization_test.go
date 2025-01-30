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
	}{
		{
			name:        "Valid JSON",
			filePath:    "./samples/valid.json",
			expectedExt: ".json",
			expectedContent: map[string]interface{}{
				"hello": "world",
			},
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
			expectedContent: map[string]interface{}{
				"hello": "world",
			},
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
			name:        "Empty file",
			filePath:    "./samples/empty.json",
			expectError: true,
		},
		{
			name:        "invalid tar.gz",
			filePath:    "./samples/invalid.tar.gz",
			expectError: true,
		},
	}

	for _, tc := range testCases {

		filename := filepath.Base(tc.filePath)
		// Read file bytes
		data, err := os.ReadFile(tc.filePath)
		if err != nil {
			t.Error("Error reading file:", err)
			continue
		}

		t.Run(tc.name, func(t *testing.T) {
			result, err := files.SanitizeFile(data, filename, "./temp")

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

			if tc.expectedContent != nil {
				unmarshalled, ok := result.UnmarshalledFile.(map[string]interface{})
				if !ok {
					t.Fatalf("Expected unmarshalled file to be a map, got %T", result.UnmarshalledFile)
				}

				for key, expectedValue := range tc.expectedContent {
					actualValue, exists := unmarshalled[key]
					if !exists {
						t.Errorf("Key %q not found in unmarshalled content", key)
						continue
					}
					if actualValue != expectedValue {
						t.Errorf("For key %q, expected value %v, got %v", key, expectedValue, actualValue)
					}
				}
			}
		})
	}
}
