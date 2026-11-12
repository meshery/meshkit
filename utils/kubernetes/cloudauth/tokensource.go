package cloudauth

import (
	"context"
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
	// (an STS AssumeRole call, an instance metadata lookup) it triggers.
	defaultMintTimeout = 30 * time.Second
)

// mintFunc produces a fresh token. now is the token source's clock, so that
// the token's expiry is computed on the same clock that later checks it.
type mintFunc func(ctx context.Context, now time.Time) (*oauth2.Token, error)

// cachingTokenSource is a thread-safe oauth2.TokenSource that returns its
// cached token until skew before the token's expiry and then mints a new
// one. Concurrent callers block on the same mint rather than each minting.
// Unlike oauth2.ReuseTokenSource its clock is injectable, so refresh timing
// can be tested without waiting or network access.
//
// It also implements transport.ResettableTokenSource: when the API server
// answers 401, client-go calls ResetTokenOlderThan and the next request
// mints a fresh token instead of reusing the rejected one.
type cachingTokenSource struct {
	mint    mintFunc
	skew    time.Duration
	timeout time.Duration
	now     func() time.Time

	mu       sync.Mutex
	token    *oauth2.Token
	mintedAt time.Time
}

var _ transport.ResettableTokenSource = (*cachingTokenSource)(nil)

func newCachingTokenSource(mint mintFunc) *cachingTokenSource {
	return &cachingTokenSource{
		mint:    mint,
		skew:    defaultRefreshSkew,
		timeout: defaultMintTimeout,
		now:     time.Now,
	}
}

// Token returns the cached token while it has more than skew left before
// expiry, and otherwise mints, caches and returns a new one. A failed mint
// is returned to the caller and leaves the stale token in place, so the
// next call retries the mint.
func (s *cachingTokenSource) Token() (*oauth2.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	if s.token != nil && now.Before(s.token.Expiry.Add(-s.skew)) {
		return s.token, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	token, err := s.mint(ctx, now)
	if err != nil {
		return nil, err
	}
	s.token = token
	s.mintedAt = now
	return token, nil
}

// ResetTokenOlderThan discards the cached token if it was minted before t.
func (s *cachingTokenSource) ResetTokenOlderThan(t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.token != nil && s.mintedAt.Before(t) {
		s.token = nil
	}
}
