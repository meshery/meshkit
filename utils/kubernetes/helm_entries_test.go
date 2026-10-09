package kubernetes

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHelmEntriesLookup(t *testing.T) {
	entries := HelmEntries{
		"meshery": {
			{Name: "meshery", Version: "1.2.0", AppVersion: "0.7.0"},
		},
	}

	tests := []struct {
		name   string
		lookup func() (HelmEntryMetadata, bool)
		want   HelmEntryMetadata
	}{
		{
			name: "app version",
			lookup: func() (HelmEntryMetadata, bool) {
				return entries.GetEntryWithAppVersion("meshery", "0.7.0")
			},
			want: HelmEntryMetadata{Name: "meshery", Version: "1.2.0", AppVersion: "0.7.0"},
		},
		{
			name: "chart version",
			lookup: func() (HelmEntryMetadata, bool) {
				return entries.GetEntryWithChartVersion("meshery", "1.2.0")
			},
			want: HelmEntryMetadata{Name: "meshery", Version: "1.2.0", AppVersion: "0.7.0"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.lookup()
			if !ok {
				t.Fatal("lookup reported no match")
			}
			if got != tt.want {
				t.Errorf("lookup = %#v, want %#v", got, tt.want)
			}
		})
	}
	if _, ok := entries.GetEntryWithAppVersion("missing", "0.7.0"); ok {
		t.Error("missing entry unexpectedly found")
	}
}

func TestNormalizeVersion(t *testing.T) {
	if got := normalizeVersion("1.2.3"); got != "v1.2.3" {
		t.Errorf("normalizeVersion() = %q, want v1.2.3", got)
	}
	if got := normalizeVersion("v1.2.3"); got != "v1.2.3" {
		t.Errorf("normalizeVersion() = %q, want v1.2.3", got)
	}
}

func TestHelmVersionLookups(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "apiVersion: v1\nentries:\n  sample:\n  - name: sample\n    version: 2.0.0\n    appVersion: v1.2.3\n")
	}))
	defer server.Close()

	chartVersion, err := HelmAppVersionToChartVersion(server.URL, "sample", "v1.2.3")
	if err != nil || chartVersion != "2.0.0" {
		t.Fatalf("HelmAppVersionToChartVersion() = %q, %v", chartVersion, err)
	}
	appVersion, err := HelmChartVersionToAppVersion(server.URL, "sample", "2.0.0")
	if err != nil || appVersion != "v1.2.3" {
		t.Fatalf("HelmChartVersionToAppVersion() = %q, %v", appVersion, err)
	}
	normalizedChartVersion, err := HelmConvertAppVersionToChartVersion(server.URL, "sample", "1.2.3")
	if err != nil || normalizedChartVersion != "2.0.0" {
		t.Fatalf("HelmConvertAppVersionToChartVersion() = %q, %v", normalizedChartVersion, err)
	}
}

func TestHelmVersionLookupsNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "apiVersion: v1\nentries: {}\n")
	}))
	defer server.Close()
	if _, err := HelmAppVersionToChartVersion(server.URL, "missing", "1.0.0"); err == nil {
		t.Error("HelmAppVersionToChartVersion() error = nil, want missing entry error")
	}
}
