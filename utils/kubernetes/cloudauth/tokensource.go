package cloudauth

import (
	"context"
	"errors"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"k8s.io/client-go/transport"
)

const (
	// defaultRefreshSkew is how long before a token's expiry the caching
	// token source mints a replacement, so that a request never leaves with
	// a token that expires in flight.
	defaultRefreshSkew = time.Minute
	// defaultMintTimeout bounds one mint, including any credential refresh
	// (an STS AssumeRole call, an instance metadata lookup, a Google OAuth2
	// token exchange) it triggers.
	defaultMintTimeout = 30 * time.Second
)

// errEmptyToken is the cause reported when a mint succeeds without
// producing an access token.
var errEmptyToken = errors.New("the credential source returned an empty access token")

// requireAccessToken passes through a failed mint and turns a successful
// one without an access token into errEmptyToken, so that an empty bearer
// is never cached or sent. Every path that obtains a token applies it.
func requireAccessToken(token *oauth2.Token, err error) (*oauth2.Token, error) {
	if err != nil {
		return nil, err
	}
	if token == nil || token.AccessToken == "" {
		return nil, errEmptyToken
	}
	return token, nil
}

// mintFunc produces a fresh token. now is the token source's clock, so that
// the token's expiry is computed on the same clock that later checks it.
// ctx carries the mint deadline; a mintFunc must return once ctx ends, so a
// call that cannot take a context (oauth2.TokenSource.Token) must be wrapped
// in runBounded and given its own transport timeout.
type mintFunc func(ctx context.Context, now time.Time) (*oauth2.Token, error)

// cachingTokenSource is a thread-safe oauth2.TokenSource that returns its
// cached token until skew before the token's expiry and then mints a new
// one. Its lock is never held while minting: concurrent callers that need a
// new token join one in-flight mint and share its result, success or
// failure, and every mint is bounded by timeout. Unlike
// oauth2.ReuseTokenSource its clock is injectable, so refresh timing and
// timeouts can be tested without waiting or network access.
//
// When a refresh fails while the cached token has not yet expired (it is
// only inside the refresh skew), the cached token is returned instead of
// the error, and the next call tries to refresh again.
//
// It also implements transport.ResettableTokenSource: when the API server
// answers 401, client-go calls ResetTokenOlderThan and the next request
// mints a fresh token instead of reusing the rejected one.
type cachingTokenSource struct {
	mint    mintFunc
	skew    time.Duration
	timeout time.Duration
	clock   clock

	mu       sync.Mutex
	token    *oauth2.Token
	mintedAt time.Time
	inflight *flight[*oauth2.Token]
}

var _ transport.ResettableTokenSource = (*cachingTokenSource)(nil)

func newCachingTokenSource(mint mintFunc) *cachingTokenSource {
	return &cachingTokenSource{
		mint:    mint,
		skew:    defaultRefreshSkew,
		timeout: defaultMintTimeout,
		clock:   realClock{},
	}
}

// Token returns the cached token while it has more than skew left before
// expiry, and otherwise mints, caches and returns a new one. A token with a
// zero Expiry never expires, as in package oauth2.
func (s *cachingTokenSource) Token() (*oauth2.Token, error) {
	s.mu.Lock()
	now := s.clock.Now()
	if s.token != nil && (s.token.Expiry.IsZero() || now.Before(s.token.Expiry.Add(-s.skew))) {
		token := s.token
		s.mu.Unlock()
		return token, nil
	}
	if f := s.inflight; f != nil {
		f.waiters++
		s.mu.Unlock()
		return f.wait()
	}
	f := newFlight[*oauth2.Token]()
	s.inflight = f
	stale := s.token
	s.mu.Unlock()

	token, err := s.mintBounded(now)

	s.mu.Lock()
	s.inflight = nil
	switch {
	case err == nil:
		s.token = token
		s.mintedAt = now
	case stale != nil && s.token == stale && s.clock.Now().Before(stale.Expiry):
		// The refresh failed but the cached token is still accepted, and no
		// 401 discarded it meanwhile: keep serving it.
		token, err = stale, nil
	}
	s.mu.Unlock()

	f.finish(token, err)
	return token, err
}

// mintBounded runs one mint under the mint deadline.
func (s *cachingTokenSource) mintBounded(now time.Time) (*oauth2.Token, error) {
	ctx, cancel := s.clock.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	return requireAccessToken(s.mint(ctx, now))
}

// ResetTokenOlderThan discards the cached token if it was minted before t.
// It never waits for an in-flight mint.
func (s *cachingTokenSource) ResetTokenOlderThan(t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.token != nil && s.mintedAt.Before(t) {
		s.token = nil
	}
}
