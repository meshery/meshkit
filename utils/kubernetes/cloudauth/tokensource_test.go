package cloudauth

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/meshery/meshkit/errors"
	"golang.org/x/oauth2"
)

// fakeClock is a manually advanced clock for cachingTokenSource.now.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func TestEKSTokenFormat(t *testing.T) {
	const (
		region    = "us-west-2"
		clusterID = "demo-cluster"
	)
	clock := &fakeClock{now: time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)}
	source := newEKSTokenSource(aws.Config{
		Region:      region,
		Credentials: credentials.NewStaticCredentialsProvider("AKIDEXAMPLE", "secret", ""),
	}, clusterID)
	source.now = clock.Now

	token, err := source.Token()
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	if token.Type() != "Bearer" {
		t.Fatalf("token type = %q, want Bearer", token.Type())
	}
	if want := clock.Now().Add(15 * time.Minute); !token.Expiry.Equal(want) {
		t.Fatalf("token expiry = %v, want %v", token.Expiry, want)
	}

	presignedURL := decodeEKSToken(t, token.AccessToken)
	if presignedURL.Scheme != "https" {
		t.Fatalf("presigned URL scheme = %q, want https", presignedURL.Scheme)
	}
	if want := "sts." + region + ".amazonaws.com"; presignedURL.Host != want {
		t.Fatalf("presigned URL host = %q, want regional endpoint %q", presignedURL.Host, want)
	}

	query := presignedURL.Query()
	if got := query.Get("Action"); got != "GetCallerIdentity" {
		t.Fatalf("Action = %q, want GetCallerIdentity", got)
	}
	if got := query.Get("X-Amz-Expires"); got != "60" {
		t.Fatalf("X-Amz-Expires = %q, want 60", got)
	}
	signedHeaders := strings.Split(query.Get("X-Amz-SignedHeaders"), ";")
	if !contains(signedHeaders, "x-k8s-aws-id") || !contains(signedHeaders, "host") {
		t.Fatalf("X-Amz-SignedHeaders = %v, want host and x-k8s-aws-id", signedHeaders)
	}
	if got := query.Get("X-Amz-Credential"); !strings.HasPrefix(got, "AKIDEXAMPLE/") || !strings.Contains(got, "/"+region+"/sts/aws4_request") {
		t.Fatalf("X-Amz-Credential = %q, want static key scoped to %s/sts", got, region)
	}
	if query.Get("X-Amz-Signature") == "" {
		t.Fatal("presigned URL carries no X-Amz-Signature")
	}
	// The cluster ID must be signed, not sent in the clear as a parameter.
	if query.Has("x-k8s-aws-id") {
		t.Fatal("x-k8s-aws-id leaked into the query string; it must be a signed header")
	}
}

func decodeEKSToken(t *testing.T, token string) *url.URL {
	t.Helper()
	encoded, ok := strings.CutPrefix(token, "k8s-aws-v1.")
	if !ok {
		t.Fatalf("token %q lacks the k8s-aws-v1. prefix", token)
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("token payload is not base64 RawURLEncoding: %v", err)
	}
	presignedURL, err := url.Parse(string(raw))
	if err != nil {
		t.Fatalf("token payload %q is not a URL: %v", raw, err)
	}
	return presignedURL
}

func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

// countingSource returns a cachingTokenSource on clock whose tokens live for
// lifetime and are numbered by mint order.
func countingSource(clock *fakeClock, lifetime time.Duration, mints *atomic.Int32) *cachingTokenSource {
	source := newCachingTokenSource(func(_ context.Context, now time.Time) (*oauth2.Token, error) {
		n := mints.Add(1)
		return &oauth2.Token{AccessToken: fmt.Sprintf("token-%d", n), Expiry: now.Add(lifetime)}, nil
	})
	source.now = clock.Now
	return source
}

func TestCachingTokenSourceRefreshesBeforeExpiry(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)}
	var mints atomic.Int32
	source := countingSource(clock, 15*time.Minute, &mints)

	assertToken := func(step, want string) {
		t.Helper()
		token, err := source.Token()
		if err != nil {
			t.Fatalf("%s: Token() error = %v", step, err)
		}
		if token.AccessToken != want {
			t.Fatalf("%s: token = %q, want %q", step, token.AccessToken, want)
		}
	}

	assertToken("first call", "token-1")
	clock.Advance(13*time.Minute + 59*time.Second)
	assertToken("inside the refresh window", "token-1")
	clock.Advance(time.Second)
	assertToken("at expiry minus skew", "token-2")
	assertToken("after refresh", "token-2")
	clock.Advance(20 * time.Minute)
	assertToken("after expiry", "token-3")

	if got := mints.Load(); got != 3 {
		t.Fatalf("mint count = %d, want 3", got)
	}
}

func TestCachingTokenSourceConcurrentCallsShareOneMint(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)}
	var mints atomic.Int32
	source := countingSource(clock, 15*time.Minute, &mints)

	run := func() {
		var wg sync.WaitGroup
		for i := 0; i < 64; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, err := source.Token(); err != nil {
					t.Errorf("Token() error = %v", err)
				}
			}()
		}
		wg.Wait()
	}

	run()
	if got := mints.Load(); got != 1 {
		t.Fatalf("mint count after concurrent first use = %d, want 1", got)
	}
	clock.Advance(14 * time.Minute)
	run()
	if got := mints.Load(); got != 2 {
		t.Fatalf("mint count after concurrent refresh = %d, want 2", got)
	}
}

func TestCachingTokenSourceRetriesFailedMint(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)}
	var calls atomic.Int32
	source := newCachingTokenSource(func(_ context.Context, now time.Time) (*oauth2.Token, error) {
		if calls.Add(1) == 1 {
			return nil, fmt.Errorf("transient failure")
		}
		return &oauth2.Token{AccessToken: "recovered", Expiry: now.Add(15 * time.Minute)}, nil
	})
	source.now = clock.Now

	if _, err := source.Token(); err == nil {
		t.Fatal("Token() succeeded, want the mint error")
	}
	token, err := source.Token()
	if err != nil {
		t.Fatalf("Token() after a failed mint error = %v", err)
	}
	if token.AccessToken != "recovered" {
		t.Fatalf("token = %q, want %q", token.AccessToken, "recovered")
	}
}

func TestCachingTokenSourceResetTokenOlderThan(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)}
	var mints atomic.Int32
	source := countingSource(clock, 15*time.Minute, &mints)

	if _, err := source.Token(); err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	// A reset older than the cached token keeps it.
	source.ResetTokenOlderThan(clock.Now().Add(-time.Second))
	if token, _ := source.Token(); token.AccessToken != "token-1" {
		t.Fatalf("token after stale reset = %q, want token-1", token.AccessToken)
	}
	// A 401 seen after the token was minted discards it.
	source.ResetTokenOlderThan(clock.Now().Add(time.Second))
	if token, _ := source.Token(); token.AccessToken != "token-2" {
		t.Fatalf("token after reset = %q, want token-2", token.AccessToken)
	}
}

// isolateAWSEnvironment points the AWS default credential chain at nothing,
// so tests exercise the missing-credentials path without touching the
// developer's AWS configuration or the network.
func isolateAWSEnvironment(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("AWS_CONFIG_FILE", dir+"/config")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", dir+"/credentials")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	for _, name := range []string{
		"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN",
		"AWS_PROFILE", "AWS_DEFAULT_PROFILE", "AWS_REGION", "AWS_DEFAULT_REGION",
		"AWS_ROLE_ARN", "AWS_WEB_IDENTITY_TOKEN_FILE",
		"AWS_CONTAINER_CREDENTIALS_RELATIVE_URI", "AWS_CONTAINER_CREDENTIALS_FULL_URI",
	} {
		t.Setenv(name, "")
	}
}

func TestNewEKSTokenSourceMissingCredentials(t *testing.T) {
	isolateAWSEnvironment(t)

	_, err := NewEKSTokenSource(context.Background(), EKSConfig{ClusterID: "demo", Region: "us-west-2"})
	if err == nil {
		t.Fatal("NewEKSTokenSource() succeeded without credentials")
	}
	if got := errors.GetCode(err); got != ErrAWSCredentialsUnavailableCode {
		t.Fatalf("error code = %q, want %q (error: %v)", got, ErrAWSCredentialsUnavailableCode, err)
	}
}

func TestNewEKSTokenSourceMissingRegion(t *testing.T) {
	isolateAWSEnvironment(t)

	_, err := NewEKSTokenSource(context.Background(), EKSConfig{ClusterID: "demo", AccessKeyID: "AKID", SecretAccessKey: "secret"})
	if got := errors.GetCode(err); got != ErrEKSRegionUnresolvedCode {
		t.Fatalf("error code = %q, want %q (error: %v)", got, ErrEKSRegionUnresolvedCode, err)
	}
}

func TestNewEKSTokenSourceWithExecEnvStaticCredentials(t *testing.T) {
	isolateAWSEnvironment(t)

	source, err := NewEKSTokenSource(context.Background(), EKSConfig{
		ClusterID: "demo", Region: "eu-central-1", AccessKeyID: "AKIDEXEC", SecretAccessKey: "secret",
	})
	if err != nil {
		t.Fatalf("NewEKSTokenSource() error = %v", err)
	}
	token, err := source.Token()
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	presignedURL := decodeEKSToken(t, token.AccessToken)
	if presignedURL.Host != "sts.eu-central-1.amazonaws.com" {
		t.Fatalf("presigned URL host = %q, want sts.eu-central-1.amazonaws.com", presignedURL.Host)
	}
	if got := presignedURL.Query().Get("X-Amz-Credential"); !strings.HasPrefix(got, "AKIDEXEC/") {
		t.Fatalf("X-Amz-Credential = %q, want the exec env access key", got)
	}
}
