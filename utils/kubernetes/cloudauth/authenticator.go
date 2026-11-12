package cloudauth

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/meshery/meshkit/logger"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
)

// DisableEnvVar names the environment variable that, when set to a true
// value (as parsed by strconv.ParseBool), disables in-process token minting
// so every exec stanza is left for client-go to run.
const DisableEnvVar = "MESHKIT_DISABLE_INPROCESS_CLOUD_AUTH"

const (
	// fallbackRetryInterval is how long a decision to fall back to the exec
	// plugin is reused before in-process setup is tried again, so a
	// transient setup failure does not disable in-process minting for the
	// process lifetime.
	fallbackRetryInterval = 5 * time.Minute
	// defaultSetupTimeout bounds one token source setup (credential
	// discovery plus the up-front credential check), whether it runs in
	// Apply or again after a 401 or a failed token.
	defaultSetupTimeout = 30 * time.Second
)

// tokenSourceFactory builds the token source for a recognized Spec; see
// WithTokenSourceFactory.
type tokenSourceFactory func(ctx context.Context, spec Spec) (oauth2.TokenSource, error)

// defaultTokenSourceFactory mints EKS tokens with setupEKSTokenSource and GKE
// tokens with setupGKETokenSource.
func defaultTokenSourceFactory(ctx context.Context, spec Spec) (oauth2.TokenSource, error) {
	switch spec.Provider {
	case ProviderEKS:
		return setupEKSTokenSource(ctx, spec.EKS)
	case ProviderGKE:
		return setupGKETokenSource(ctx)
	default:
		return nil, ErrUnsupportedCloudProvider(spec.Provider)
	}
}

// Option configures an Authenticator.
type Option func(*Authenticator)

// WithTokenSourceFactory replaces the default factory, which mints with the
// Meshery process's own AWS and Google credentials. The factory builds the
// token source for a recognized Spec, verifying up front that the
// credentials it needs are available; ctx bounds only that setup and is
// cancelled when the setup deadline passes, and the returned source must
// keep working after ctx ends. Sources the factory returns are attached only
// to endpoints the Authenticator trusts for spec.Provider. A source that was
// not built by this package is called under the mint deadline in its own
// goroutine, since oauth2.TokenSource takes no context.
func WithTokenSourceFactory(factory func(ctx context.Context, spec Spec) (oauth2.TokenSource, error)) Option {
	return func(a *Authenticator) { a.factory = factory }
}

// Authenticator replaces recognized cloud exec stanzas in rest.Configs with
// in-process token minting. It caches one token source per cluster
// identity, so every rest.Config it touches for the same cluster shares one
// token and one refresh schedule. It is safe for concurrent use, and never
// holds a lock across network I/O: concurrent callers that need the same
// setup or mint share one bounded attempt and its result.
//
// A cached source is set up again whenever it fails to produce a token or
// the API server rejects one of its tokens, so credentials rewritten on disk
// (a refreshed ~/.aws/credentials or gcloud ADC file) are picked up without
// a restart. A decision to fall back to the exec plugin is reconsidered
// after fallbackRetryInterval, or at once when the plugin binary has since
// disappeared.
//
// Each decision to fall back to the exec plugin is logged as a warning: one
// carrying ErrInProcessCloudAuthFallback when in-process setup failed (at
// most once per cluster identity per fallbackRetryInterval), and one
// carrying ErrUntrustedClusterEndpointFallback when the cluster endpoint may
// not receive in-process tokens (once per provider and endpoint).
type Authenticator struct {
	factory      tokenSourceFactory
	lookPath     func(string) (string, error)
	getenv       func(string) string
	clock        clock
	setupTimeout time.Duration
	mintTimeout  time.Duration
	disabled     bool
	log          logger.Handler

	mu      sync.Mutex
	entries map[string]*authEntry
	// warnedEndpoints records the provider and endpoint pairs whose
	// untrusted-endpoint fallback has been logged.
	warnedEndpoints map[string]struct{}
}

// NewAuthenticator returns an Authenticator that mints with the Meshery
// process's own cloud credentials, looks fallback plugins up with
// exec.LookPath, and logs to standard error through a MeshKit logger.
func NewAuthenticator(opts ...Option) *Authenticator {
	a := &Authenticator{
		factory:         defaultTokenSourceFactory,
		lookPath:        exec.LookPath,
		getenv:          os.Getenv,
		clock:           realClock{},
		setupTimeout:    defaultSetupTimeout,
		mintTimeout:     defaultMintTimeout,
		entries:         make(map[string]*authEntry),
		warnedEndpoints: make(map[string]struct{}),
	}
	if log, err := logger.New("meshkit-cloudauth", logger.Options{
		Format:   logger.TerminalLogFormat,
		LogLevel: int(logrus.InfoLevel),
		Output:   os.Stderr,
	}); err == nil {
		a.log = log
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Apply rewrites config to mint bearer tokens in process when its
// ExecProvider is a recognized cloud plugin (see Recognize) and its server
// is an endpoint of that cloud (see bindEndpoint): ExecProvider is cleared,
// so client-go never forks the plugin, and the transport is wrapped to
// inject the token on every request. No static BearerToken is set.
//
// config is left unchanged when it has no ExecProvider, the plugin is not
// recognized, minting is disabled, config already carries credentials that
// client-go prefers over an exec plugin (a bearer token or token file,
// basic auth, or a client certificate and key), or in-process minting
// cannot be used (credentials are unavailable, or the endpoint is not one
// the process's credentials may be sent to) but the plugin binary is on
// PATH. When in-process minting cannot be used and the binary is not on
// PATH either, Apply returns ErrInProcessCloudAuthUnavailable or
// ErrUntrustedClusterEndpoint.
func (a *Authenticator) Apply(config *rest.Config) error {
	if config == nil || config.ExecProvider == nil || a.isDisabled() || hasPrecedingCredentials(config) {
		return nil
	}
	spec, ok := Recognize(config.ExecProvider)
	if !ok {
		return nil
	}

	spec, err := bindEndpoint(config, spec)
	if err != nil {
		return a.refuseEndpoint(spec, config.Host, err)
	}

	source, err := a.tokenSource(spec)
	if err != nil {
		return err
	}
	if source == nil {
		return nil
	}
	installTokenSource(config, source)
	return nil
}

// hasPrecedingCredentials reports whether config carries credentials that
// client-go's exec authenticator gives precedence to, in which case client-go
// never runs the plugin and Apply must not intercept it either. It mirrors
// the HasTokenAuth, HasBasicAuth and HasCertAuth checks client-go makes
// before installing the exec plugin.
func hasPrecedingCredentials(config *rest.Config) bool {
	hasToken := config.BearerToken != "" || config.BearerTokenFile != ""
	hasBasic := config.Username != ""
	hasCert := (len(config.CertData) != 0 || config.CertFile != "") &&
		(len(config.KeyData) != 0 || config.KeyFile != "")
	return hasToken || hasBasic || hasCert
}

// refuseEndpoint handles a recognized stanza whose endpoint may not receive
// in-process tokens: the exec plugin runs as before when it is on PATH (the
// first such decision per provider and endpoint is logged), and otherwise
// Apply fails with ErrUntrustedClusterEndpoint.
func (a *Authenticator) refuseEndpoint(spec Spec, host string, cause error) error {
	if _, err := a.lookPath(spec.Command); err != nil {
		return ErrUntrustedClusterEndpoint(cause, spec.Provider, host, spec.Command)
	}
	key := string(spec.Provider) + "\x00" + host
	a.mu.Lock()
	_, warned := a.warnedEndpoints[key]
	a.warnedEndpoints[key] = struct{}{}
	a.mu.Unlock()
	if !warned && a.log != nil {
		a.log.Warn(ErrUntrustedClusterEndpointFallback(cause, spec.Provider, host, spec.Command))
	}
	return nil
}

// installTokenSource clears config.ExecProvider and wraps config's transport
// so every request carries a bearer token from source. When source
// implements transport.ResettableTokenSource, a 401 response discards the
// cached token so the next request mints a fresh one.
func installTokenSource(config *rest.Config, source oauth2.TokenSource) {
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

// entry returns the shared entry for spec's cluster identity.
func (a *Authenticator) entry(spec Spec) *authEntry {
	key := spec.cacheKey()
	a.mu.Lock()
	defer a.mu.Unlock()
	entry, ok := a.entries[key]
	if !ok {
		entry = &authEntry{auth: a, spec: spec}
		a.entries[key] = entry
	}
	return entry
}

// tokenSource returns the shared token source for spec, setting one up when
// none is cached. It returns (nil, nil) when the exec plugin should be used
// instead. Successful setups are cached until their source fails; fallback
// decisions are cached for fallbackRetryInterval; errors are not cached, so
// a later call retries once credentials appear. A cached fallback decision
// is only reused while spec.Command is itself on PATH, both because stanzas
// naming different commands share one cluster identity and because a
// plugin removed since the decision can no longer be fallen back to. Each
// new fallback decision is logged once.
func (a *Authenticator) tokenSource(spec Spec) (oauth2.TokenSource, error) {
	entry := a.entry(spec)
	now := a.clock.Now()
	_, lookErr := a.lookPath(spec.Command)
	onPath := lookErr == nil

	entry.mu.Lock()
	ready := entry.source != nil
	setupErr := entry.fallbackErr
	retry := setupErr == nil || !onPath || now.Sub(entry.fallbackAt) >= fallbackRetryInterval
	entry.mu.Unlock()
	if ready {
		return entry, nil
	}

	var attempt uint64
	if retry {
		result, err := entry.acquire()
		if err == nil {
			return entry, nil
		}
		setupErr, attempt = err, result.attempt
	}
	if !onPath {
		return nil, ErrInProcessCloudAuthUnavailable(setupErr, spec.Provider, spec.Command)
	}
	if retry && entry.recordFallback(setupErr, attempt, now) && a.log != nil {
		a.log.Warn(ErrInProcessCloudAuthFallback(setupErr, spec.Provider, spec.Command))
	}
	return nil, nil
}

// authEntry is the shared state for one cluster identity, and the token
// source installed into every rest.Config for it. Its lock guards only the
// fields below and is never held across setup or minting.
type authEntry struct {
	auth *Authenticator
	spec Spec

	mu sync.Mutex
	// source is the in-process token source; nil until set up, and again
	// after it fails or is reset.
	source oauth2.TokenSource
	// generation counts successful setups; it identifies source, so a
	// failure only discards the source it came from.
	generation uint64
	// setupAt is when source was set up.
	setupAt time.Time
	// attempts counts setup attempts; it identifies a setup flight.
	attempts uint64
	// setupFlight is the in-progress setup, if any.
	setupFlight *flight[setupResult]
	// tokenFlight is the in-progress Token call, if any.
	tokenFlight *flight[*oauth2.Token]
	// fallbackErr is the setup error behind a cached decision to fall back
	// to the exec plugin, made at fallbackAt by setup attempt
	// fallbackAttempt; nil when there is none.
	fallbackErr     error
	fallbackAt      time.Time
	fallbackAttempt uint64
}

// setupResult is the outcome of one setup flight.
type setupResult struct {
	source     oauth2.TokenSource
	generation uint64
	attempt    uint64
}

var _ transport.ResettableTokenSource = (*authEntry)(nil)

// acquire returns the entry's source, setting it up first when there is
// none. Concurrent callers share one setup attempt, run without the entry
// lock and bounded by the setup timeout; its failure is returned to all of
// them and is not cached, so the next call starts a new attempt.
func (e *authEntry) acquire() (setupResult, error) {
	e.mu.Lock()
	if e.source != nil {
		result := setupResult{source: e.source, generation: e.generation}
		e.mu.Unlock()
		return result, nil
	}
	if f := e.setupFlight; f != nil {
		f.waiters++
		e.mu.Unlock()
		return f.wait()
	}
	f := newFlight[setupResult]()
	e.setupFlight = f
	e.attempts++
	result := setupResult{attempt: e.attempts}
	e.mu.Unlock()

	source, err := e.runSetup()

	e.mu.Lock()
	e.setupFlight = nil
	if err == nil {
		e.generation++
		e.source = source
		e.setupAt = e.auth.clock.Now()
		e.fallbackErr = nil
		result.source, result.generation = source, e.generation
	}
	e.mu.Unlock()

	f.finish(result, err)
	return result, err
}

// runSetup calls the factory under the setup deadline. A factory that
// overruns the deadline is abandoned and reported as
// ErrCloudAuthTimeout; its late result is discarded.
func (e *authEntry) runSetup() (oauth2.TokenSource, error) {
	ctx, cancel := e.auth.clock.WithTimeout(context.Background(), e.auth.setupTimeout)
	defer cancel()
	source, err := runBounded(ctx, func(ctx context.Context) (oauth2.TokenSource, error) {
		return e.auth.factory(ctx, e.spec)
	})
	if err != nil && err == context.Cause(ctx) {
		return nil, ErrCloudAuthTimeout(err, e.spec.Provider, "setting up credentials", e.auth.setupTimeout)
	}
	return source, err
}

// recordFallback caches the decision to fall back after setup attempt
// attempt failed with err, and reports whether this call recorded it (so
// that concurrent callers sharing one failed attempt log it once).
func (e *authEntry) recordFallback(err error, attempt uint64, now time.Time) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.fallbackErr != nil && e.fallbackAttempt == attempt {
		return false
	}
	e.fallbackErr = err
	e.fallbackAt = now
	e.fallbackAttempt = attempt
	return true
}

// Token returns a token from the entry's in-process source, setting the
// source up again first when a previous failure or reset discarded it.
// Concurrent callers share one call and its result. A failed token
// discards the source it came from, so the next call re-reads credentials.
func (e *authEntry) Token() (*oauth2.Token, error) {
	e.mu.Lock()
	if f := e.tokenFlight; f != nil {
		f.waiters++
		e.mu.Unlock()
		return f.wait()
	}
	f := newFlight[*oauth2.Token]()
	e.tokenFlight = f
	e.mu.Unlock()

	token, err := e.token()

	e.mu.Lock()
	e.tokenFlight = nil
	e.mu.Unlock()
	f.finish(token, err)
	return token, err
}

// token runs one Token call for the leader of a token flight.
func (e *authEntry) token() (*oauth2.Token, error) {
	current, err := e.acquire()
	if err != nil {
		return nil, err
	}
	token, err := e.boundedToken(current.source)
	if err != nil {
		e.mu.Lock()
		if e.generation == current.generation {
			e.source = nil
		}
		e.mu.Unlock()
		return nil, err
	}
	return token, nil
}

// boundedToken calls source.Token under the mint deadline. Sources built by
// this package bound and coalesce their own mints and are called directly;
// any other source runs in a goroutine that is abandoned at the deadline,
// and an empty token from it is rejected as the package's own sources do.
func (e *authEntry) boundedToken(source oauth2.TokenSource) (*oauth2.Token, error) {
	if own, ok := source.(*cachingTokenSource); ok {
		return own.Token()
	}
	ctx, cancel := e.auth.clock.WithTimeout(context.Background(), e.auth.mintTimeout)
	defer cancel()
	token, err := runBounded(ctx, func(context.Context) (*oauth2.Token, error) {
		return requireAccessToken(source.Token())
	})
	if err != nil && err == context.Cause(ctx) {
		return nil, ErrCloudAuthTimeout(err, e.spec.Provider, "minting a token", e.auth.mintTimeout)
	}
	return token, err
}

// ResetTokenOlderThan is called by client-go when the API server answers
// 401. It discards the source if it was set up before t, so the next token
// comes from freshly loaded credentials rather than the rejected ones. It
// never waits for an in-flight setup or mint.
func (e *authEntry) ResetTokenOlderThan(t time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.source != nil && e.setupAt.Before(t) {
		e.source = nil
	}
}
