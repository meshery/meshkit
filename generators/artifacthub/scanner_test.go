package artifacthub

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
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

func TestComputeBackoff(t *testing.T) {
	var tests = []struct {
		name       string
		attempt    int
		retryAfter string
		wantMin    time.Duration
		wantMax    time.Duration
	}{
		{"retry-after seconds honored", 0, "1", 1 * time.Second, 1 * time.Second},
		{"retry-after seconds capped at maxRetryAfter", 0, "3600", maxRetryAfter, maxRetryAfter},
		{"invalid retry-after falls back to exponential", 0, "not-a-delay", 1 * time.Second, 1500 * time.Millisecond},
		{"no header uses exponential backoff", 0, "", 1 * time.Second, 1500 * time.Millisecond},
		{"backoff doubles per attempt", 2, "", 4 * time.Second, 4500 * time.Millisecond},
		{"backoff capped at maxBackoff", 10, "", maxBackoff, maxBackoff + 500*time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := computeBackoff(tt.attempt, tt.retryAfter); got < tt.wantMin || got > tt.wantMax {
				t.Errorf("computeBackoff(%d, %q) = %v, want in [%v, %v]", tt.attempt, tt.retryAfter, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestComputeBackoff_RetryAfterHttpDate(t *testing.T) {
	// HTTP-date form of Retry-After, a few seconds in the future.
	future := time.Now().Add(5 * time.Second).UTC().Format(http.TimeFormat)
	got := computeBackoff(0, future)
	if got < 3*time.Second || got > 5*time.Second {
		t.Errorf("computeBackoff(0, %q) = %v, want roughly 5s", future, got)
	}

	// A date in the past cannot produce a delay, so fall back to exponential.
	past := time.Now().Add(-1 * time.Hour).UTC().Format(http.TimeFormat)
	got = computeBackoff(0, past)
	if got < 1*time.Second || got > 1500*time.Millisecond {
		t.Errorf("computeBackoff(0, %q) = %v, want exponential fallback in [1s, 1.5s]", past, got)
	}
}
