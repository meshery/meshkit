package registry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// GenerateDefsForCoreRegistrant reads the model version from the SourceURL:
// protocol://github.com/owner/repo/tree/definitions/{model-name}/version/components.
// Split on "/", the version is element 8, so a URL with fewer than 9 elements
// has none. It must be rejected with an error, not crash the generator by
// indexing past the end.
func TestGenerateDefsForCoreRegistrant_SourceURLWithoutVersion(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"empty", ""},
		{"host only", "https://github.com"},
		{"stops at the model name (8 elements)", "https://github.com/meshery/schemas/tree/definitions/aws-ec2"},
		{"stops at the definitions folder (7 elements)", "https://github.com/meshery/schemas/tree/definitions"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("GenerateDefsForCoreRegistrant panicked on SourceURL %q: %v", tt.url, r)
				}
			}()

			err := GenerateDefsForCoreRegistrant(ModelCSV{SourceURL: tt.url}, nil, t.TempDir(), "aws-ec2")
			if err == nil {
				t.Fatalf("expected an error for SourceURL %q, got nil", tt.url)
			}
			if !strings.Contains(err.Error(), "invalid SourceURL format") {
				t.Errorf("error = %q, want it to report an invalid SourceURL format", err)
			}
		})
	}
}

// The other side of the boundary: a SourceURL that does carry a version must get
// past the guard and use element 8 as the version. With no components in the
// helper the function stops after creating the versioned directories, with a
// "no components found" error, which is not the invalid SourceURL error.
func TestGenerateDefsForCoreRegistrant_SourceURLWithVersion(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"ends at the version (9 elements)", "https://github.com/meshery/schemas/tree/definitions/aws-ec2/v1.2.3"},
		{"ends at the components folder (10 elements)", "https://github.com/meshery/schemas/tree/definitions/aws-ec2/v1.2.3/components"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("GenerateDefsForCoreRegistrant panicked on SourceURL %q: %v", tt.url, r)
				}
			}()

			path := t.TempDir()
			model := ModelCSV{Model: "aws-ec2", SourceURL: tt.url}
			helper := &ComponentCSVHelper{Components: map[string]map[string][]ComponentCSV{}}

			err := GenerateDefsForCoreRegistrant(model, helper, path, "aws-ec2")
			if err != nil && strings.Contains(err.Error(), "invalid SourceURL format") {
				t.Fatalf("SourceURL %q carries a version but was rejected: %v", tt.url, err)
			}

			versionDir := filepath.Join(path, "aws-ec2", "v1.2.3")
			info, statErr := os.Stat(versionDir)
			if statErr != nil || !info.IsDir() {
				t.Errorf("expected element 8 of the URL (v1.2.3) to name the version directory %s: %v", versionDir, statErr)
			}
		})
	}
}
