package cloudauth

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"sync"

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
type Authenticator struct {
	factory    TokenSourceFactory
	lookPath   func(string) (string, error)
	getenv     func(string) string
	disabled   bool
	onFallback func(Spec, error)

	mu      sync.Mutex
	entries map[string]*authEntry
}

// authEntry is the cached outcome of setting up one cluster identity.
// Setup runs under the entry's own lock so that a slow credential check for
// one cluster does not block requests for another.
type authEntry struct {
	mu       sync.Mutex
	resolved bool
	// source is the in-process token source; nil after resolution means
	// "fall back to the exec plugin".
	source oauth2.TokenSource
}

// NewAuthenticator returns an Authenticator using DefaultTokenSourceFactory
// and exec.LookPath unless overridden by opts.
func NewAuthenticator(opts ...Option) *Authenticator {
	a := &Authenticator{
		factory:  DefaultTokenSourceFactory,
		lookPath: exec.LookPath,
		getenv:   os.Getenv,
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

// tokenSource returns the cached token source for spec, setting one up on
// first use. It returns (nil, nil) when the exec plugin should be used
// instead. Successful setups and fallback decisions are cached; errors are
// not, so a later call retries once credentials appear.
func (a *Authenticator) tokenSource(spec Spec) (oauth2.TokenSource, error) {
	key := spec.cacheKey()
	a.mu.Lock()
	entry, ok := a.entries[key]
	if !ok {
		entry = &authEntry{}
		a.entries[key] = entry
	}
	a.mu.Unlock()

	entry.mu.Lock()
	defer entry.mu.Unlock()
	if entry.resolved {
		return entry.source, nil
	}

	source, err := a.factory(context.Background(), spec)
	if err != nil {
		if _, lookErr := a.lookPath(spec.Command); lookErr != nil {
			return nil, ErrInProcessCloudAuthUnavailable(err, spec.Provider, spec.Command)
		}
		if a.onFallback != nil {
			a.onFallback(spec, err)
		}
		entry.resolved = true
		return nil, nil
	}

	entry.resolved = true
	entry.source = source
	return source, nil
}
