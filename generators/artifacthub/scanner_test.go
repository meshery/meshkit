package artifacthub

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestGetPackagesWithName(t *testing.T) {
	var tests = []struct {
		name string
	}{
		{"prometheus"},
		{"crossplane"},
	}
	for _, tt := range tests {
		t.Run("GetPackages", func(t *testing.T) {
			got, err := GetAhPackagesWithName(tt.name)
			if len(got) == 0 || err != nil {
				t.Errorf("got %v, want %v", got, "atleast one package")
			}
		})
	}
}

func Equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func TestFilterPackagesWithCrds(t *testing.T) {
	var tests = []struct {
		input []AhPackage
		want  []string
	}{
		{[]AhPackage{
			{Name: "crossplane-types", Repository: "crossplane", Organization: "", ChartUrl: "https://charts.crossplane.io/master/crossplane-types-0.13.0-rc.98.g1eb0776.tgz"},
			{Name: "crossplane", Repository: "crossplane", Organization: "", ChartUrl: "https://charts.crossplane.io/master/crossplane-1.10.0-rc.0.99.g7f471c48.tgz"},
		},
			[]string{
				"crossplane-types",
			},
		},
	}
	for _, tt := range tests {
		t.Run("FilterPackagesWithCrds", func(t *testing.T) {
			res := FilterPackagesWithCrds(tt.input)
			got := make([]string, 0)
			for _, ap := range res {
				got = append(got, ap.Name)
			}
			if !Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetAhPackagesWithName_RetriesOn429(t *testing.T) {
	var attempts atomic.Int32

	// Spin up a local fake ArtifactHub HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := attempts.Add(1)
		if current <= 2 {
			// Simulate rate limiting on the first 2 attempts
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		// Succeed on the 3rd attempt with valid ArtifactHub JSON
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"packages": [
				{
					"name": "slack-operator",
					"version": "1.0.0",
					"repository": {
						"name": "slack-repo",
						"url": "https://charts.example.com",
						"verified_publisher": true,
						"official": false,
						"cncf": false
					}
				}
			]
		}`))
	}))
	defer server.Close()

	pkgs, err := getAhPackagesWithNameFromEndpoint(server.URL, "slack-operator")
	if err != nil {
		t.Fatalf("expected request to succeed after retries, got error: %v", err)
	}
	if attempts.Load() != 3 {
		t.Fatalf("expected 3 total HTTP attempts, got %d", attempts.Load())
	}
	if len(pkgs) != 1 || pkgs[0].Name != "slack-operator" {
		t.Fatalf("unexpected packages returned: %+v", pkgs)
	}
}

func TestGetAhPackagesWithName_FailsImmediatelyOn404(t *testing.T) {
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	_, err := getAhPackagesWithNameFromEndpoint(server.URL, "nonexistent-pkg")
	if err == nil {
		t.Fatal("expected error for 404 response, got nil")
	}
	if attempts.Load() != 1 {
		t.Fatalf("expected 1 attempt without retrying 404, got %d", attempts.Load())
	}
}
