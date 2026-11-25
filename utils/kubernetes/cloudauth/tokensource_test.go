package cloudauth

import (
	"context"
	"encoding/base64"
	stderrors "errors"
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

func TestEKSTokenFormat(t *testing.T) {
	const (
		region    = "us-west-2"
		clusterID = "demo-cluster"
	)
	clock := newFakeClock()
	source := newEKSTokenSource(aws.Config{
		Region:      region,
		Credentials: credentials.NewStaticCredentialsProvider("AKIDEXAMPLE", "secret", ""),
	}, clusterID)
	source.clock = clock

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
	source.clock = clock
	return source
}

func TestCachingTokenSourceRefreshesBeforeExpiry(t *testing.T) {
	clock := newFakeClock()
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
	clock := newFakeClock()
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
	clock := newFakeClock()
	var calls atomic.Int32
	source := newCachingTokenSource(func(_ context.Context, now time.Time) (*oauth2.Token, error) {
		if calls.Add(1) == 1 {
			return nil, fmt.Errorf("transient failure")
		}
		return &oauth2.Token{AccessToken: "recovered", Expiry: now.Add(15 * time.Minute)}, nil
	})
	source.clock = clock

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

func TestCachingTokenSourceKeepsUnexpiredTokenWhenRefreshFails(t *testing.T) {
	clock := newFakeClock()
	var calls atomic.Int32
	source := newCachingTokenSource(func(_ context.Context, now time.Time) (*oauth2.Token, error) {
		if calls.Add(1) == 1 {
			return &oauth2.Token{AccessToken: "first", Expiry: now.Add(15 * time.Minute)}, nil
		}
		return nil, fmt.Errorf("STS unavailable")
	})
	source.clock = clock

	if _, err := source.Token(); err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	// Inside the refresh skew the token still works; a failed refresh must
	// not fail the request.
	clock.Advance(14*time.Minute + 30*time.Second)
	token, err := source.Token()
	if err != nil || token.AccessToken != "first" {
		t.Fatalf("Token() inside the skew with a failing refresh = (%v, %v), want the cached token", token, err)
	}
	// Once the token has expired the failure surfaces.
	clock.Advance(time.Minute)
	if _, err := source.Token(); err == nil {
		t.Fatal("Token() after expiry with a failing refresh succeeded")
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("mint calls = %d, want 3 (each call past the skew retries the refresh)", got)
	}
}

func TestCachingTokenSourceRejectsEmptyToken(t *testing.T) {
	source := newCachingTokenSource(func(context.Context, time.Time) (*oauth2.Token, error) {
		return &oauth2.Token{}, nil
	})
	if _, err := source.Token(); !stderrors.Is(err, errEmptyToken) {
		t.Fatalf("Token() error = %v, want errEmptyToken", err)
	}
}

func TestCachingTokenSourceResetTokenOlderThan(t *testing.T) {
	clock := newFakeClock()
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

// blockingMint is a mint that signals started, then waits for release (and
// returns result) or for its deadline (and records the cancellation).
type blockingMint struct {
	calls     atomic.Int32
	started   chan struct{}
	release   chan struct{}
	cancelled chan error
	result    func(now time.Time) (*oauth2.Token, error)
}

func newBlockingMint(result func(now time.Time) (*oauth2.Token, error)) *blockingMint {
	return &blockingMint{
		started:   make(chan struct{}, 16),
		release:   make(chan struct{}),
		cancelled: make(chan error, 16),
		result:    result,
	}
}

func (m *blockingMint) mint(ctx context.Context, now time.Time) (*oauth2.Token, error) {
	m.calls.Add(1)
	m.started <- struct{}{}
	select {
	case <-m.release:
		return m.result(now)
	case <-ctx.Done():
		m.cancelled <- context.Cause(ctx)
		return nil, context.Cause(ctx)
	}
}

func waitersOf(source *cachingTokenSource) int {
	source.mu.Lock()
	defer source.mu.Unlock()
	if source.inflight == nil {
		return 0
	}
	return source.inflight.waiters
}

func TestCachingTokenSourceDoesNotHoldLockWhileMinting(t *testing.T) {
	m := newBlockingMint(func(now time.Time) (*oauth2.Token, error) {
		return &oauth2.Token{AccessToken: "minted", Expiry: now.Add(15 * time.Minute)}, nil
	})
	source := newCachingTokenSource(m.mint)
	source.clock = newFakeClock()

	results := make(chan error, 9)
	go func() {
		_, err := source.Token()
		results <- err
	}()
	<-m.started

	// The mint is blocked, yet the 401 reset path takes the lock and returns.
	returnsPromptly(t, "ResetTokenOlderThan", func() { source.ResetTokenOlderThan(time.Now()) })

	for i := 0; i < 8; i++ {
		go func() {
			_, err := source.Token()
			results <- err
		}()
	}
	eventually(t, "8 callers to join the in-flight mint", func() bool { return waitersOf(source) == 8 })

	close(m.release)
	for i := 0; i < 9; i++ {
		if err := <-results; err != nil {
			t.Fatalf("Token() error = %v", err)
		}
	}
	if got := m.calls.Load(); got != 1 {
		t.Fatalf("mint calls = %d, want 1 shared by all callers", got)
	}
}

func TestCachingTokenSourceWaitersShareFailedMint(t *testing.T) {
	m := newBlockingMint(func(time.Time) (*oauth2.Token, error) {
		return nil, fmt.Errorf("STS throttled")
	})
	source := newCachingTokenSource(m.mint)
	source.clock = newFakeClock()

	const callers = 16
	results := make(chan error, callers)
	for i := 0; i < callers; i++ {
		go func() {
			_, err := source.Token()
			results <- err
		}()
	}
	<-m.started
	eventually(t, "every other caller to join the failing mint", func() bool { return waitersOf(source) == callers-1 })

	close(m.release)
	for i := 0; i < callers; i++ {
		if err := <-results; err == nil || !strings.Contains(err.Error(), "STS throttled") {
			t.Fatalf("Token() error = %v, want the shared mint failure", err)
		}
	}
	if got := m.calls.Load(); got != 1 {
		t.Fatalf("mint calls = %d, want 1: waiters must share the failure, not retry it serially", got)
	}

	// The failure is not cached: the next call mints again.
	go func() { _, _ = source.Token() }()
	<-m.started
	if got := m.calls.Load(); got != 2 {
		t.Fatalf("mint calls after the shared failure = %d, want 2", got)
	}
}

func TestCachingTokenSourceMintTimeoutCancels(t *testing.T) {
	clock := newFakeClock()
	m := newBlockingMint(func(time.Time) (*oauth2.Token, error) {
		t.Error("the mint was released, want it cancelled by the deadline")
		return nil, nil
	})
	source := newCachingTokenSource(m.mint)
	source.clock = clock

	result := make(chan error, 1)
	go func() {
		_, err := source.Token()
		result <- err
	}()
	<-m.started

	clock.Advance(defaultMintTimeout - time.Second)
	select {
	case err := <-result:
		t.Fatalf("Token() returned %v before the mint deadline", err)
	default:
	}
	clock.Advance(time.Second)

	if cause := <-m.cancelled; !stderrors.Is(cause, context.DeadlineExceeded) {
		t.Fatalf("mint context cause = %v, want context.DeadlineExceeded", cause)
	}
	if err := <-result; !stderrors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Token() error = %v, want context.DeadlineExceeded", err)
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

	_, err := setupEKSTokenSource(context.Background(), EKSConfig{ClusterID: "demo", Region: "us-west-2"})
	if err == nil {
		t.Fatal("setupEKSTokenSource() succeeded without credentials")
	}
	if got := errors.GetCode(err); got != ErrAWSCredentialsUnavailableCode {
		t.Fatalf("error code = %q, want %q (error: %v)", got, ErrAWSCredentialsUnavailableCode, err)
	}
}

func TestNewEKSTokenSourceMissingRegion(t *testing.T) {
	isolateAWSEnvironment(t)
	t.Setenv("AWS_ACCESS_KEY_ID", "AKID")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")

	_, err := setupEKSTokenSource(context.Background(), EKSConfig{ClusterID: "demo"})
	if got := errors.GetCode(err); got != ErrEKSRegionUnresolvedCode {
		t.Fatalf("error code = %q, want %q (error: %v)", got, ErrEKSRegionUnresolvedCode, err)
	}
}

func TestNewEKSTokenSourceWithProcessEnvCredentials(t *testing.T) {
	isolateAWSEnvironment(t)
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIDPROCESS")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")

	source, err := setupEKSTokenSource(context.Background(), EKSConfig{ClusterID: "demo", Region: "eu-central-1"})
	if err != nil {
		t.Fatalf("setupEKSTokenSource() error = %v", err)
	}
	token, err := source.Token()
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	presignedURL := decodeEKSToken(t, token.AccessToken)
	if presignedURL.Host != "sts.eu-central-1.amazonaws.com" {
		t.Fatalf("presigned URL host = %q, want sts.eu-central-1.amazonaws.com", presignedURL.Host)
	}
	if got := presignedURL.Query().Get("X-Amz-Credential"); !strings.HasPrefix(got, "AKIDPROCESS/") {
		t.Fatalf("X-Amz-Credential = %q, want the process env access key", got)
	}
}
