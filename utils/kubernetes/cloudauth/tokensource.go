package cloudauth

import (
	"context"
	"net/http"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"k8s.io/client-go/transport"
)

const (
	defaultExpiryDelta = 2 * time.Minute
	defaultMintTimeout = 30 * time.Second
)

// MintFunc produces a fresh OAuth2 bearer token.
type MintFunc func(ctx context.Context) (*oauth2.Token, error)

// RefreshingTokenSource caches bearer tokens in memory and refreshes them
// synchronously under a mutex whenever the current token is missing, invalid,
// or within expiryDelta of expiration.
//
// It implements golang.org/x/oauth2.TokenSource and
// k8s.io/client-go/transport.ResettableTokenSource (evicting stale tokens on
// HTTP 401 responses) and deliberately avoids writing tokens to disk or logging
// token values.
type RefreshingTokenSource struct {
	mu          sync.Mutex
	mint        MintFunc
	onReset     func()
	expiryDelta time.Duration
	mintTimeout time.Duration
	now         func() time.Time
	cached      *oauth2.Token
	mintedAt    time.Time
}

var (
	_ oauth2.TokenSource              = (*RefreshingTokenSource)(nil)
	_ transport.ResettableTokenSource = (*RefreshingTokenSource)(nil)
)

// NewRefreshingTokenSource builds a thread-safe caching TokenSource around mint.
// A positive expiryDelta ensures tokens are proactively refreshed before they
// expire (defaulting to 2m if expiryDelta <= 0).
func NewRefreshingTokenSource(mint MintFunc, expiryDelta time.Duration) *RefreshingTokenSource {
	if expiryDelta <= 0 {
		expiryDelta = defaultExpiryDelta
	}
	return &RefreshingTokenSource{
		mint:        mint,
		expiryDelta: expiryDelta,
		mintTimeout: defaultMintTimeout,
		now:         time.Now,
	}
}

// Token returns a cached token when still valid beyond expiryDelta, or mints a
// fresh token using a context bounded by mintTimeout.
func (s *RefreshingTokenSource) Token() (*oauth2.Token, error) {
	return s.TokenWithContext(context.Background())
}

// TokenWithContext returns a cached token when valid beyond expiryDelta, or
// calls mint with a deadline-bounded context under the mutex so concurrent
// callers coalesce onto a single refresh.
func (s *RefreshingTokenSource) TokenWithContext(ctx context.Context) (*oauth2.Token, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isValidLocked(s.cached) {
		return cloneToken(s.cached), nil
	}

	mintCtx := ctx
	var cancel context.CancelFunc
	if s.mintTimeout > 0 {
		mintCtx, cancel = context.WithTimeout(ctx, s.mintTimeout)
		defer cancel()
	}

	now := s.now()
	tok, err := s.mint(mintCtx)
	if err != nil {
		if ctx.Err() == nil && s.isUnexpiredLocked(s.cached, now) {
			return cloneToken(s.cached), nil
		}
		return nil, err
	}
	if tok == nil || tok.AccessToken == "" {
		if ctx.Err() == nil && s.isUnexpiredLocked(s.cached, now) {
			return cloneToken(s.cached), nil
		}
		return nil, errEmptyToken
	}
	s.cached = cloneToken(tok)
	s.mintedAt = now
	return cloneToken(s.cached), nil
}

// Invalidate drops any cached token so the next request mints a new one.
func (s *RefreshingTokenSource) Invalidate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clearLocked()
}

// ResetTokenOlderThan implements client-go/transport.ResettableTokenSource,
// evicting the cached token when an API request sent before t receives 401
// Unauthorized (without dropping a newer token minted by a concurrent caller).
func (s *RefreshingTokenSource) ResetTokenOlderThan(t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cached != nil && s.mintedAt.Before(t) {
		s.clearLocked()
	}
}

func (s *RefreshingTokenSource) clearLocked() {
	s.cached = nil
	s.mintedAt = time.Time{}
	if s.onReset != nil {
		s.onReset()
	}
}

// WrapTransport returns an http.RoundTripper wrapper suitable for
// rest.Config.WrapTransport that attaches a fresh Bearer token to each request
// and resets the cached token if the API server responds 401 Unauthorized.
func (s *RefreshingTokenSource) WrapTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return transport.ResettableTokenSourceWrapTransport(s)(base)
}

func (s *RefreshingTokenSource) isValidLocked(t *oauth2.Token) bool {
	if t == nil || t.AccessToken == "" {
		return false
	}
	if t.Expiry.IsZero() {
		return true
	}
	return s.now().Add(s.expiryDelta).Before(t.Expiry)
}

func (s *RefreshingTokenSource) isUnexpiredLocked(t *oauth2.Token, now time.Time) bool {
	if t == nil || t.AccessToken == "" {
		return false
	}
	if t.Expiry.IsZero() {
		return true
	}
	return now.Before(t.Expiry)
}

func cloneToken(t *oauth2.Token) *oauth2.Token {
	if t == nil {
		return nil
	}
	cp := *t
	return &cp
}

type staticSentinelError string

func (e staticSentinelError) Error() string { return string(e) }

const errEmptyToken = staticSentinelError("minted token is empty")
