package cloudauth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestRefreshingTokenSourceCachingAndProactiveRefresh(t *testing.T) {
	base := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	now := base
	var calls atomic.Int32

	ts := NewRefreshingTokenSource(func(_ context.Context) (*oauth2.Token, error) {
		n := calls.Add(1)
		return &oauth2.Token{
			AccessToken: fmt.Sprintf("token-%d", n),
			TokenType:   "Bearer",
			Expiry:      now.Add(15 * time.Minute),
		}, nil
	}, 2*time.Minute)
	ts.now = func() time.Time { return now }

	// First call mints token-1.
	tok, err := ts.Token()
	if err != nil {
		t.Fatalf("Token() error: %v", err)
	}
	if tok.AccessToken != "token-1" || calls.Load() != 1 {
		t.Fatalf("got token=%q calls=%d, want token-1 calls=1", tok.AccessToken, calls.Load())
	}

	// Still outside the 2m proactive expiry window -> cached hit.
	now = base.Add(12*time.Minute + 59*time.Second)
	tok, err = ts.Token()
	if err != nil {
		t.Fatalf("Token() error: %v", err)
	}
	if tok.AccessToken != "token-1" || calls.Load() != 1 {
		t.Fatalf("expected cached token-1, got token=%q calls=%d", tok.AccessToken, calls.Load())
	}

	// Entering the 2m proactive refresh window -> mints token-2.
	now = base.Add(13 * time.Minute)
	tok, err = ts.Token()
	if err != nil {
		t.Fatalf("Token() error: %v", err)
	}
	if tok.AccessToken != "token-2" || calls.Load() != 2 {
		t.Fatalf("expected refreshed token-2, got token=%q calls=%d", tok.AccessToken, calls.Load())
	}

	// Invalidate forces immediate re-mint.
	ts.Invalidate()
	tok, err = ts.Token()
	if err != nil {
		t.Fatalf("Token() after Invalidate error: %v", err)
	}
	if tok.AccessToken != "token-3" || calls.Load() != 3 {
		t.Fatalf("expected token-3 after Invalidate, got token=%q calls=%d", tok.AccessToken, calls.Load())
	}
}

func TestRefreshingTokenSourceConcurrentCoalescing(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	var calls atomic.Int32

	ts := NewRefreshingTokenSource(func(_ context.Context) (*oauth2.Token, error) {
		n := calls.Add(1)
		time.Sleep(10 * time.Millisecond)
		return &oauth2.Token{
			AccessToken: fmt.Sprintf("concurrent-%d", n),
			TokenType:   "Bearer",
			Expiry:      now.Add(10 * time.Minute),
		}, nil
	}, time.Minute)
	ts.now = func() time.Time { return now }

	const workers = 16
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			tok, err := ts.Token()
			if err != nil {
				t.Errorf("concurrent Token() error: %v", err)
				return
			}
			if tok.AccessToken != "concurrent-1" {
				t.Errorf("got %q, want concurrent-1", tok.AccessToken)
			}
		}()
	}
	wg.Wait()

	if calls.Load() != 1 {
		t.Fatalf("mint calls = %d, want 1", calls.Load())
	}
}

func TestRefreshingTokenSourceEmptyTokenError(t *testing.T) {
	ts := NewRefreshingTokenSource(func(_ context.Context) (*oauth2.Token, error) {
		return &oauth2.Token{}, nil
	}, time.Minute)

	if _, err := ts.Token(); err == nil {
		t.Fatal("expected error when mint returns empty AccessToken")
	}
}

func TestRefreshingTokenSourceWrapTransport(t *testing.T) {
	base := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	now := base
	var calls atomic.Int32

	ts := NewRefreshingTokenSource(func(_ context.Context) (*oauth2.Token, error) {
		n := calls.Add(1)
		return &oauth2.Token{
			AccessToken: fmt.Sprintf("bearer-%d", n),
			TokenType:   "Bearer",
			Expiry:      now.Add(15 * time.Minute),
		}, nil
	}, 2*time.Minute)
	ts.now = func() time.Time { return now }

	var seenAuths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuths = append(seenAuths, r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := &http.Client{Transport: ts.WrapTransport(http.DefaultTransport)}

	req1, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if _, err := client.Do(req1); err != nil {
		t.Fatalf("req1 error: %v", err)
	}
	// Ensure WrapTransport cloned the request instead of mutating req1 in place.
	if req1.Header.Get("Authorization") != "" {
		t.Fatalf("WrapTransport mutated caller request Authorization header: %q", req1.Header.Get("Authorization"))
	}

	now = base.Add(14 * time.Minute)
	if _, err := client.Get(srv.URL); err != nil {
		t.Fatalf("req2 error: %v", err)
	}

	if len(seenAuths) != 2 || seenAuths[0] != "Bearer bearer-1" || seenAuths[1] != "Bearer bearer-2" {
		t.Fatalf("seen Authorization headers = %v", seenAuths)
	}
}

func TestRefreshingTokenSourceResettableOn401(t *testing.T) {
	var mints atomic.Int32
	ts := NewRefreshingTokenSource(func(_ context.Context) (*oauth2.Token, error) {
		n := mints.Add(1)
		return &oauth2.Token{
			AccessToken: fmt.Sprintf("tok-%d", n),
			TokenType:   "Bearer",
			Expiry:      time.Now().Add(15 * time.Minute),
		}, nil
	}, 2*time.Minute)

	// Guard against resetting a fresher token when ResetTokenOlderThan receives
	// an older timestamp from an earlier concurrent request.
	if _, err := ts.Token(); err != nil {
		t.Fatalf("initial Token(): %v", err)
	}
	beforeMint := time.Now().Add(-time.Second)
	ts.ResetTokenOlderThan(beforeMint)
	if mints.Load() != 1 {
		t.Fatalf("unexpected mint count %d", mints.Load())
	}
	if tok, err := ts.Token(); err != nil || tok.AccessToken != "tok-1" {
		t.Fatalf("expected cached tok-1, got %v err=%v", tok, err)
	}

	// Simulate an API server rejecting tok-1 with 401 while accepting tok-2.
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		seen = append(seen, auth)
		if auth == "Bearer tok-1" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := &http.Client{Transport: ts.WrapTransport(http.DefaultTransport)}
	time.Sleep(5 * time.Millisecond)

	resp1, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("req1 error: %v", err)
	}
	_ = resp1.Body.Close()
	if resp1.StatusCode != http.StatusUnauthorized {
		t.Fatalf("req1 status = %d, want 401", resp1.StatusCode)
	}

	// Next request sees 401 reset and mints tok-2 cleanly.
	resp2, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("req2 error: %v", err)
	}
	_ = resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("req2 status = %d, want 200", resp2.StatusCode)
	}
	if mints.Load() != 2 {
		t.Fatalf("mints = %d, want 2 after 401 reset", mints.Load())
	}
	if len(seen) != 2 || seen[0] != "Bearer tok-1" || seen[1] != "Bearer tok-2" {
		t.Fatalf("seen headers = %v", seen)
	}
}

func TestSecretRedaction(t *testing.T) {
	creds := Credentials{
		AWS: &AWSCredentials{
			AccessKeyID:     "AKIASECRETKEY123456",
			SecretAccessKey: "super-secret-aws-key",
			SessionToken:    "aws-session-secret",
			Region:          "us-east-1",
		},
		GCP: &GCPCredentials{
			ServiceAccountJSON: []byte(`{"private_key":"-----BEGIN PRIVATE KEY-----\nSECRET\n"}`),
		},
	}
	matched := MatchedConfig{
		Provider:    ProviderEKS,
		Command:     "aws",
		ClusterName: "demo",
		Region:      "us-east-1",
		Env: map[string]string{
			"AWS_SECRET_ACCESS_KEY": "env-secret-value",
		},
	}

	formatted := fmt.Sprintf("%v %#v %v %#v %v %#v %v %#v",
		creds, creds,
		*creds.AWS, *creds.AWS,
		*creds.GCP, *creds.GCP,
		matched, matched,
	)

	for _, secret := range []string{
		"AKIASECRETKEY123456",
		"super-secret-aws-key",
		"aws-session-secret",
		"BEGIN PRIVATE KEY",
		"env-secret-value",
	} {
		if strings.Contains(formatted, secret) {
			t.Fatalf("formatted output leaked secret %q: %s", secret, formatted)
		}
	}
}
