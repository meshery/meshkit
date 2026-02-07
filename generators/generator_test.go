package generators

import (
	"testing"

	"github.com/meshery/meshkit/generators/github"
)

func TestNewGeneratorWithOptions(t *testing.T) {
	tests := []struct {
		name        string
		registrant  string
		url         string
		packageName string
		opts        GeneratorOptions
		wantRec     bool
		wantDepth   int
	}{
		{
			name:        "Github Recursive",
			registrant:  "github",
			url:         "https://github.com/owner/repo",
			packageName: "test",
			opts: GeneratorOptions{
				Recursive: true,
				MaxDepth:  5,
			},
			wantRec:   true,
			wantDepth: 5,
		},
		{
			name:        "Github Default",
			registrant:  "github",
			url:         "https://github.com/owner/repo",
			packageName: "test",
			opts: GeneratorOptions{
				Recursive: false,
				MaxDepth:  0,
			},
			wantRec:   false,
			wantDepth: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm, err := NewGeneratorWithOptions(tt.registrant, tt.url, tt.packageName, tt.opts)
			if err != nil {
				t.Fatalf("NewGeneratorWithOptions() error = %v", err)
			}

			// Type assertion to access fields
			if ghpm, ok := pm.(github.GitHubPackageManager); ok {
				if ghpm.Recursive != tt.wantRec {
					t.Errorf("NewGeneratorWithOptions() Recursive = %v, want %v", ghpm.Recursive, tt.wantRec)
				}
				if ghpm.MaxDepth != tt.wantDepth {
					t.Errorf("NewGeneratorWithOptions() MaxDepth = %v, want %v", ghpm.MaxDepth, tt.wantDepth)
				}
			} else {
				t.Errorf("NewGeneratorWithOptions() returned unexpected type")
			}
		})
	}
}
