package artifacthub

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/meshery/meshkit/utils/manifests"
)

var AhApiSearchParams = map[string]string{
	"offset": "0",
	"limit":  "10",
	"facets": "false",
	"kind":   "0", // represents HELM types
	"sort":   "relevance",
}

const (
	AhTextSearchQueryFieldName = "ts_query_web"
	maxRetries                 = 5
	initialBackoff             = 1 * time.Second
	maxBackoff                 = 16 * time.Second
)

// httpClient with an explicit timeout prevents hanging requests in CI
var artifactHubHTTPClient = &http.Client{
	Timeout: 15 * time.Second,
}

func GetAhPackagesWithName(name string) ([]AhPackage, error) {
	return getAhPackagesWithNameFromEndpoint(ArtifactHubAPIEndpoint, name)
}

func getAhPackagesWithNameFromEndpoint(endpoint, name string) ([]AhPackage, error) {
	pkgs := make([]AhPackage, 0)

	// 1. Build the search URL safely with encoded query parameters
	baseURL, err := url.Parse(fmt.Sprintf("%s/packages/search", endpoint))
	if err != nil {
		return nil, ErrGetAhPackage(err)
	}

	query := url.Values{}
	query.Add(AhTextSearchQueryFieldName, name)
	for key, val := range AhApiSearchParams {
		query.Add(key, val)
	}
	baseURL.RawQuery = query.Encode()
	finalURL := baseURL.String()

	// 2. Fetch the URL using our retry + exponential backoff helper
	resp, err := getWithRetry(finalURL)
	if err != nil {
		return nil, ErrGetAhPackage(err)
	}
	defer func() { _ = resp.Body.Close() }()

	// 3. Parse the JSON response into AhPackage structs
	var res map[string][]map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, ErrGetAhPackage(err)
	}

	for _, pkg := range res["packages"] {
		pkgs = append(pkgs, *parseArtifacthubResponse(pkg))
	}
	return pkgs, nil
}

// getWithRetry performs an HTTP GET and retries transient 429 and 5xx responses
// using exponential backoff with jitter.
func getWithRetry(targetURL string) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		resp, err := artifactHubHTTPClient.Get(targetURL)
		if err != nil {
			lastErr = err
		} else if resp.StatusCode == http.StatusOK {
			// Success! Return the open response so the caller can read and close it.
			return resp, nil
		} else {
			lastErr = fmt.Errorf("status code %d for %s", resp.StatusCode, targetURL)
			// Only retry on 429 (rate limit) or 5xx (temporary server errors)
			shouldRetry := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
			retryAfterHeader := resp.Header.Get("Retry-After")
			_ = resp.Body.Close() // Always close body before retrying or returning

			if !shouldRetry {
				return nil, lastErr
			}

			if attempt < maxRetries {
				wait := computeBackoff(attempt, retryAfterHeader)
				fmt.Printf("WARN: ArtifactHub rate-limited/unavailable (%v). Retrying in %v (attempt %d/%d)...\n",
					lastErr, wait, attempt+1, maxRetries)
				time.Sleep(wait)
				continue
			}
			return nil, lastErr
		}

		// Network-level error retry path
		if attempt < maxRetries {
			wait := computeBackoff(attempt, "")
			fmt.Printf("WARN: ArtifactHub request error (%v). Retrying in %v (attempt %d/%d)...\n",
				lastErr, wait, attempt+1, maxRetries)
			time.Sleep(wait)
		}
	}

	return nil, lastErr
}

// computeBackoff honors the Retry-After header if present; otherwise it calculates
// exponential backoff (1s, 2s, 4s, 8s, 16s) plus random jitter up to 500ms.
func computeBackoff(attempt int, retryAfter string) time.Duration {
	if retryAfter != "" {
		if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}

	wait := initialBackoff
	for i := 0; i < attempt; i++ {
		wait *= 2
	}
	if wait > maxBackoff {
		wait = maxBackoff
	}

	jitter := time.Duration(rand.IntN(500)) * time.Millisecond
	return wait + jitter
}

func FilterPackagesWithCrds(pkgs []AhPackage) []AhPackage {
	out := make([]AhPackage, 0)
	for _, ap := range pkgs {
		crds, err := manifests.GetCrdsFromHelm(ap.ChartUrl)
		if err == nil && len(crds) != 0 {
			out = append(out, ap)
		}
	}
	return out
}
