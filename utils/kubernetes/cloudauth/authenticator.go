package cloudauth

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
)

// DisableEnvVar names the environment variable that, when set to a true
// value (as parsed by strconv.ParseBool), disables in-process token minting
// so every exec stanza is left for client-go to run.
const DisableEnvVar = "MESHKIT_DISABLE_INPROCESS_CLOUD_AUTH"

// TokenSourceFactory builds the token source for a recognized Spec,
// verifying up front that the credentials it needs are available. ctx bounds
// only that setup; the returned source must keep working after ctx ends.
type TokenSourceFactory func(ctx context.Context, spec Spec) (oauth2.TokenSource, error)

// DefaultTokenSourceFactory mints EKS tokens with NewEKSTokenSource and GKE
// tokens with NewGKETokenSource.
func DefaultTokenSourceFactory(ctx context.Context, spec Spec) (oauth2.TokenSource, error) {
	switch spec.Provider {
	case ProviderEKS:
		return NewEKSTokenSource(ctx, spec.EKS)
	case ProviderGKE:
		return NewGKETokenSource(ctx)
	default:
		return nil, ErrUnsupportedCloudProvider(spec.Provider)
	}
}

// Option configures an Authenticator.
type Option func(*Authenticator)

// WithTokenSourceFactory replaces DefaultTokenSourceFactory, for example to
// mint with credentials stored with a cloud connection, or in tests.
func WithTokenSourceFactory(factory TokenSourceFactory) Option {
	return func(a *Authenticator) { a.factory = factory }
}

// WithLookPath replaces exec.LookPath when deciding whether the exec plugin
// binary is installed and can serve as a fallback.
func WithLookPath(lookPath func(file string) (string, error)) Option {
	return func(a *Authenticator) { a.lookPath = lookPath }
}

// WithDisabled disables in-process minting regardless of DisableEnvVar.
func WithDisabled(disabled bool) Option {
	return func(a *Authenticator) { a.disabled = disabled }
}

// WithFallbackObserver registers a callback invoked once per cluster
// identity when in-process minting fails but the exec plugin binary is on
// PATH, so the stanza is left for client-go to run. It receives the error
// that prevented in-process minting, so the fallback is never silent to a
// caller that wants to log it.
func WithFallbackObserver(observe func(spec Spec, err error)) Option {
	return func(a *Authenticator) { a.onFallback = observe }
}

// Authenticator replaces recognized cloud exec stanzas in rest.Configs with
// in-process token minting. It caches one token source per cluster
// identity, so every rest.Config it touches for the same cluster shares one
// token and one refresh schedule. It is safe for concurrent use.
//
// A cached source is set up again whenever it fails to produce a token or
// the API server rejects one of its tokens, so credentials rewritten on disk
// (a refreshed ~/.aws/credentials or gcloud ADC file) are picked up without
// a restart. A decision to fall back to the exec plugin is reconsidered
// after fallbackRetryInterval.
type Authenticator struct {
	factory    TokenSourceFactory
	lookPath   func(string) (string, error)
	getenv     func(string) string
	now        func() time.Time
	disabled   bool
	onFallback func(Spec, error)

	mu      sync.Mutex
	entries map[string]*authEntry
}

// fallbackRetryInterval is how long a decision to fall back to the exec
// plugin is reused before in-process setup is tried again, so a transient
// setup failure does not disable in-process minting for the process
// lifetime.
const fallbackRetryInterval = 5 * time.Minute

// authEntry is the shared state for one cluster identity, and the token
// source installed into every rest.Config for it. Setup runs under the
// entry's own lock so that a slow credential check for one cluster does not
// block requests for another.
type authEntry struct {
	auth *Authenticator
	spec Spec

	mu sync.Mutex
	// source is the in-process token source; nil until set up, and again
	// after it fails or is reset.
	source oauth2.TokenSource
	// setupAt is when source was set up.
	setupAt time.Time
	// fallbackErr is the setup error behind a cached decision to fall back
	// to the exec plugin, made at fallbackAt; nil when there is none.
	fallbackErr error
	fallbackAt  time.Time
}

var _ transport.ResettableTokenSource = (*authEntry)(nil)

// NewAuthenticator returns an Authenticator using DefaultTokenSourceFactory
// and exec.LookPath unless overridden by opts.
func NewAuthenticator(opts ...Option) *Authenticator {
	a := &Authenticator{
		factory:  DefaultTokenSourceFactory,
		lookPath: exec.LookPath,
		getenv:   os.Getenv,
		now:      time.Now,
		entries:  make(map[string]*authEntry),
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Apply rewrites config to mint bearer tokens in process when its
// ExecProvider is a recognized cloud plugin: ExecProvider is cleared, so
// client-go never forks the plugin, and the transport is wrapped to inject
// the token on every request. No static BearerToken is set.
//
// config is left unchanged when it has no ExecProvider, the plugin is not
// recognized, minting is disabled, or credentials are unavailable but the
// plugin binary is on PATH. When credentials are unavailable and the binary
// is not on PATH, Apply returns ErrInProcessCloudAuthUnavailable.
func (a *Authenticator) Apply(config *rest.Config) error {
	if config == nil || config.ExecProvider == nil || a.isDisabled() {
		return nil
	}
	spec, ok := Recognize(config.ExecProvider)
	if !ok {
		return nil
	}

	source, err := a.tokenSource(spec)
	if err != nil {
		return err
	}
	if source == nil {
		return nil
	}
	InstallTokenSource(config, source)
	return nil
}

// InstallTokenSource clears config.ExecProvider and wraps config's transport
// so every request carries a bearer token from source. When source
// implements transport.ResettableTokenSource, a 401 response discards the
// cached token so the next request mints a fresh one.
func InstallTokenSource(config *rest.Config, source oauth2.TokenSource) {
	config.ExecProvider = nil
	if resettable, ok := source.(transport.ResettableTokenSource); ok {
		config.Wrap(transport.ResettableTokenSourceWrapTransport(resettable))
		return
	}
	config.Wrap(transport.TokenSourceWrapTransport(source))
}

func (a *Authenticator) isDisabled() bool {
	if a.disabled {
		return true
	}
	disabled, err := strconv.ParseBool(a.getenv(DisableEnvVar))
	return err == nil && disabled
}

// tokenSource returns the shared token source for spec, setting one up when
// none is cached. It returns (nil, nil) when the exec plugin should be used
// instead. Successful setups and fallback decisions are cached; errors are
// not, so a later call retries once credentials appear. A cached fallback
// decision is only reused when spec.Command is itself on PATH, because
// stanzas naming different commands share one cluster identity.
func (a *Authenticator) tokenSource(spec Spec) (oauth2.TokenSource, error) {
	key := spec.cacheKey()
	a.mu.Lock()
	entry, ok := a.entries[key]
	if !ok {
		entry = &authEntry{auth: a, spec: spec}
		a.entries[key] = entry
	}
	a.mu.Unlock()

	entry.mu.Lock()
	defer entry.mu.Unlock()
	now := a.now()
	setupErr := entry.fallbackErr
	retry := setupErr == nil || now.Sub(entry.fallbackAt) >= fallbackRetryInterval
	if entry.source == nil && retry {
		entry.fallbackErr = nil
		setupErr = entry.setupLocked()
	}
	if entry.source != nil {
		return entry, nil
	}

	if _, lookErr := a.lookPath(spec.Command); lookErr != nil {
		return nil, ErrInProcessCloudAuthUnavailable(setupErr, spec.Provider, spec.Command)
	}
	if retry {
		entry.fallbackErr = setupErr
		entry.fallbackAt = now
		if a.onFallback != nil {
			a.onFallback(spec, setupErr)
		}
	}
	return nil, nil
}

// setupLocked builds a new in-process source for the entry. The caller
// holds e.mu.
func (e *authEntry) setupLocked() error {
	source, err := e.auth.factory(context.Background(), e.spec)
	if err != nil {
		return err
	}
	e.source = source
	e.setupAt = e.auth.now()
	e.fallbackErr = nil
	return nil
}

// Token returns a token from the entry's in-process source, setting the
// source up again first when a previous failure or reset discarded it. A
// failed token discards the source, so the next call re-reads credentials.
func (e *authEntry) Token() (*oauth2.Token, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.source == nil {
		if err := e.setupLocked(); err != nil {
			return nil, err
		}
	}
	token, err := e.source.Token()
	if err != nil {
		e.source = nil
		return nil, err
	}
	return token, nil
}

// ResetTokenOlderThan is called by client-go when the API server answers
// 401. It discards the source if it was set up before t, so the next token
// comes from freshly loaded credentials rather than the rejected ones.
func (e *authEntry) ResetTokenOlderThan(t time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.source != nil && e.setupAt.Before(t) {
		e.source = nil
	}
}
