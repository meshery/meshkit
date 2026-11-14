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
		{
			name:   "build metadata is part of the version, not a second version",
			dirs:   []string{"release-v1.2.3+build.4.5", "v1.3.0"},
			newest: "v1.3.0",
		},
		{
			name:   "a single-number version is a version",
			dirs:   []string{"v2-rc1", "v2", "v1.5.0"},
			newest: "v2",
		},
		{
			name:   "a name that carries a version outranks one that carries none",
			dirs:   []string{"master", "release-v1.0.0", "v2.0.0"},
			newest: "v2.0.0",
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

// compareVersions must be a strict weak ordering, or sort.Slice can rank
// version directories inconsistently. That includes names with and without a
// version mixed together. Names are passed as GetAllVersionDirsSortedDesc
// passes them, without a leading "v".
func TestCompareVersionsIsAStrictWeakOrder(t *testing.T) {
	names := []string{
		"2.0.0", "release-v1.0.0", "master", "latest",
		"f5-bigip-ctlr-customresourcedefinitions.yml",
		"1.7.6-rc1", "release-v1.7.6-rc1", "1.7.7-rc1", "1.8.0", "1.8", "1.8.0-rc2",
		"2.0.0-rc2", "2.0.0-rc10", "azureserviceoperator_customresourcedefinitions_v2.14.0.yaml",
		"0.3.4", "0.3.4+f2ca803", "release-v1.2.3+build.4.5", "2", "2-rc1", "edge-26.11.1",
	}
	sign := func(c int) int {
		switch {
		case c > 0:
			return 1
		case c < 0:
			return -1
		}
		return 0
	}
	for _, a := range names {
		for _, b := range names {
			ab := sign(compareVersions(a, b))
			if ab != -sign(compareVersions(b, a)) {
				t.Fatalf("compareVersions(%q, %q) and compareVersions(%q, %q) disagree", a, b, b, a)
			}
			for _, c := range names {
				bc, ac := sign(compareVersions(b, c)), sign(compareVersions(a, c))
				if ab >= 0 && bc >= 0 && (ac < 0 || (ab > 0 || bc > 0) && ac == 0) {
					t.Fatalf("compareVersions is not transitive: %q vs %q = %d, %q vs %q = %d, but %q vs %q = %d", a, b, ab, b, c, bc, a, c, ac)
				}
			}
		}
	}
}

// Inputs are version directory names found in meshery/meshery's models/, and
// names that carry no version.
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
		// build metadata (newrelic-operator): kept
		{"0.3.4+f2ca803", "0.3.4+f2ca803"},
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
