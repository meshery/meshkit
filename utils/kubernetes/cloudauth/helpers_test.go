package cloudauth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/meshery/meshkit/logger"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// TestMain clears ambient proxy variables before the tests run. The
// transports under test dial httptest servers directly, and with a proxy
// set client-go (Proxy nil) sends a plaintext CONNECT to the test server
// instead of a TLS handshake. The proxy check in bindEndpoint rejects a
// non-nil rest.Config.Proxy, so the variables are cleared here rather than
// by setting a no-proxy function on the test configs.
func TestMain(m *testing.M) {
	for _, name := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy"} {
		_ = os.Unsetenv(name)
	}
	os.Exit(m.Run())
}

// withLookPath replaces exec.LookPath when deciding whether the exec plugin
// binary is installed and can serve as a fallback.
func withLookPath(lookPath func(file string) (string, error)) Option {
	return func(a *Authenticator) { a.lookPath = lookPath }
}

// withDisabled disables in-process minting regardless of DisableEnvVar.
func withDisabled(disabled bool) Option {
	return func(a *Authenticator) { a.disabled = disabled }
}

// withLogger replaces the Authenticator's default MeshKit logger.
func withLogger(log logger.Handler) Option {
	return func(a *Authenticator) { a.log = log }
}

// fakeClock is a manually advanced clock. Deadlines created with
// WithTimeout fire only when Advance moves past them, so timeout behavior is
// tested without sleeping.
type fakeClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []fakeTimer
}

type fakeTimer struct {
	deadline time.Time
	cancel   context.CancelCauseFunc
}

var _ clock = (*fakeClock)(nil)

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) WithTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancelCause(ctx)
	c.mu.Lock()
	c.timers = append(c.timers, fakeTimer{deadline: c.now.Add(d), cancel: cancel})
	c.mu.Unlock()
	return ctx, func() { cancel(context.Canceled) }
}

// Advance moves the clock forward and fires every deadline it passes.
func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	var fired []fakeTimer
	pending := c.timers[:0]
	for _, timer := range c.timers {
		if timer.deadline.After(c.now) {
			pending = append(pending, timer)
		} else {
			fired = append(fired, timer)
		}
	}
	c.timers = pending
	c.mu.Unlock()
	for _, timer := range fired {
		timer.cancel(context.DeadlineExceeded)
	}
}

// eventually fails the test unless cond becomes true within a generous
// real-time bound. It only guards against deadlock; the conditions it waits
// for are reached as soon as the goroutines involved are scheduled.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

// returnsPromptly fails the test unless fn returns within a generous
// real-time bound, proving it does not wait for a blocked mint or setup.
func returnsPromptly(t *testing.T, what string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		fn()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatalf("%s blocked behind an in-flight operation", what)
	}
}

const (
	// testEKSHost is a well-formed EKS API endpoint.
	testEKSHost = "https://0123456789ABCDEF0123456789ABCDEF.gr7.us-west-2.eks.amazonaws.com"
	// testGKEDNSHost is a well-formed GKE DNS-based control plane endpoint.
	testGKEDNSHost = "https://gke-0123456789abcdef0123456789abcdef-123456789012.us-central1.gke.goog"
)

// redirectedConfig returns a rest.Config whose Host is host but whose
// connections go to server, verified against the server's own certificate
// (issued for example.com), so endpoint rules can be exercised end to end
// without DNS or the network.
func redirectedConfig(server *httptest.Server, host string, exec *clientcmdapi.ExecConfig) *rest.Config {
	return &rest.Config{
		Host:         host,
		ExecProvider: exec,
		TLSClientConfig: rest.TLSClientConfig{
			CAData:     pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}),
			ServerName: "example.com",
		},
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, network, server.Listener.Addr().String())
		},
	}
}

// eksConfig is a rest.Config for an EKS endpoint with exec, for tests that
// never send a request.
func eksConfig(exec *clientcmdapi.ExecConfig) *rest.Config {
	return &rest.Config{Host: testEKSHost, ExecProvider: exec}
}

// fakeServiceAccountKey returns a syntactically valid service-account key
// with a freshly generated RSA key. It is not registered anywhere.
func fakeServiceAccountKey(t *testing.T, fields map[string]string) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey() error = %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey() error = %v", err)
	}
	values := map[string]string{
		"type":           "service_account",
		"project_id":     "meshkit-test",
		"private_key_id": "0123456789abcdef",
		"private_key":    string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})),
		"client_email":   "meshkit-test@meshkit-test.iam.gserviceaccount.com",
		"client_id":      "123456789",
		"token_uri":      googleTokenURL,
	}
	for k, v := range fields {
		if v == "" {
			delete(values, k)
			continue
		}
		values[k] = v
	}
	data, err := json.Marshal(values)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return data
}

// googleTokenEndpoint stands in for https://oauth2.googleapis.com/token by
// replacing googleTransport for the duration of the test.
type googleTokenEndpoint struct {
	requests atomic.Int32
	mu       sync.Mutex
	scopes   []string
	// respond answers one token request.
	respond func(r *http.Request) (*http.Response, error)
}

// useGoogleTokenEndpoint installs an endpoint answering every token request
// with status and body.
func useGoogleTokenEndpoint(t *testing.T, status int, body string) *googleTokenEndpoint {
	t.Helper()
	endpoint := &googleTokenEndpoint{}
	endpoint.respond = func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    r,
		}, nil
	}
	googleTransport = endpoint
	t.Cleanup(func() { googleTransport = nil })
	return endpoint
}

func (e *googleTokenEndpoint) RoundTrip(r *http.Request) (*http.Response, error) {
	e.requests.Add(1)
	if r.URL.String() != googleTokenURL {
		return nil, &unexpectedRequestError{r.URL.String()}
	}
	if r.Body != nil {
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		e.recordScopes(string(body))
	}
	return e.respond(r)
}

// recordScopes extracts the scope claim from a JWT bearer grant.
func (e *googleTokenEndpoint) recordScopes(form string) {
	for _, field := range strings.Split(form, "&") {
		value, ok := strings.CutPrefix(field, "assertion=")
		if !ok {
			continue
		}
		parts := strings.Split(value, ".")
		if len(parts) < 2 {
			return
		}
		payload, err := base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			return
		}
		var claims struct {
			Scope string `json:"scope"`
		}
		if json.Unmarshal(payload, &claims) == nil {
			e.mu.Lock()
			e.scopes = strings.Fields(claims.Scope)
			e.mu.Unlock()
		}
	}
}

func (e *googleTokenEndpoint) requestedScopes() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.scopes
}

// unexpectedRequestError reports a token request sent anywhere but
// googleTokenURL.
type unexpectedRequestError struct{ url string }

func (e *unexpectedRequestError) Error() string { return "unexpected token request to " + e.url }

// useADCFile points GOOGLE_APPLICATION_CREDENTIALS at data.
func useADCFile(t *testing.T, data []byte) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "adc.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", path)
}

const validGoogleToken = `{"access_token":"adc-token","token_type":"Bearer","expires_in":3600}`
