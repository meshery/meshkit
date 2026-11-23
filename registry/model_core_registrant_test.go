package registry

import (
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
