package utils

import (
	"os"
	"path/filepath"
	"testing"
)

// Model version directories are named after whatever the source published:
// release tags ("release-v1.7.6-rc1"), chart names ("helm-k6-operator-3.13.1")
// and release asset file names
// ("azureserviceoperator_customresourcedefinitions_v2.14.0.yaml"). The newest
// directory must be the one carrying the highest version, not the one whose
// name happens to start with a letter (meshery/meshery#23094).
func TestGetAllVersionDirsSortedDescRanksTheVersionANameCarries(t *testing.T) {
	tests := []struct {
		name   string
		dirs   []string
		newest string
	}{
		{
			name:   "tag-prefixed pre-release does not outrank a newer one",
			dirs:   []string{"release-v1.7.6-rc1", "r-v1.7.6-rc2", "v1.7.6-rc6", "v1.7.7-rc1", "v1.7.3"},
			newest: "v1.7.7-rc1",
		},
		{
			name:   "file-named version does not outrank a newer version",
			dirs:   []string{"azureserviceoperator_customresourcedefinitions_v2.14.0.yaml", "v2.15.0"},
			newest: "v2.15.0",
		},
		{
			name:   "file-named versions order by the version they carry",
			dirs:   []string{"azureserviceoperator_customresourcedefinitions_v2.9.0.yaml", "azureserviceoperator_customresourcedefinitions_v2.14.0.yaml"},
			newest: "azureserviceoperator_customresourcedefinitions_v2.14.0.yaml",
		},
		{
			name:   "a release outranks its own pre-releases",
			dirs:   []string{"v1.8.0-rc1", "v1.8.0", "v1.8.0-rc2"},
			newest: "v1.8.0",
		},
		{
			name:   "pre-release numbers order numerically",
			dirs:   []string{"v2.0.0-rc2", "v2.0.0-rc10"},
			newest: "v2.0.0-rc10",
		},
		{
			name:   "a plain version outranks the same version under a tag prefix",
			dirs:   []string{"release-v1.7.6-rc1", "v1.7.6-rc1"},
			newest: "v1.7.6-rc1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, d := range tt.dirs {
				if err := os.Mkdir(filepath.Join(dir, d), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			sorted, err := GetAllVersionDirsSortedDesc(dir)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := filepath.Base(sorted[0]); got != tt.newest {
				t.Errorf("newest version directory = %q, want %q", got, tt.newest)
			}
		})
	}
}

// Inputs are version directory names found in meshery/meshery's models/.
func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		// release tags and chart names: keep the version they carry
		{"release-v1.7.6-rc1", "v1.7.6-rc1"},
		{"r-v1.7.6-rc2", "v1.7.6-rc2"},
		{"release-0.25.6", "0.25.6"},
		{"knative-v1.24.0", "v1.24.0"},
		{"metallb-chart-0.16.2", "0.16.2"},
		{"datadog-3.270.1", "3.270.1"},
		{"jaeger-3.4.1", "3.4.1"},
		{"helm-k6-operator-3.13.1", "3.13.1"},
		// release asset file names: drop the name and the extension
		{"azureserviceoperator_customresourcedefinitions_v2.14.0.yaml", "v2.14.0"},
		{"ondat-operator-0.7.4.tgz", "0.7.4"},
		// already versions: unchanged
		{"v1.7.7-rc1", "v1.7.7-rc1"},
		{"1.18.1", "1.18.1"},
		{"v0.9.8.4", "v0.9.8.4"},
		{"2025.6.30", "2025.6.30"},
		{"v1.2.3-alpha.1", "v1.2.3-alpha.1"},
		// release channels that are part of the upstream version name: kept
		{"edge-26.11.1", "edge-26.11.1"},
		{"stable-2.14.10", "stable-2.14.10"},
		// no version to extract: unchanged
		{"f5-bigip-ctlr-customresourcedefinitions.yml", "f5-bigip-ctlr-customresourcedefinitions.yml"},
		{"master", "master"},
	}
	for _, tt := range tests {
		if got := NormalizeVersion(tt.in); got != tt.want {
			t.Errorf("NormalizeVersion(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
