package cloudauth

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/meshery/meshkit/errors"
	"github.com/meshery/meshkit/logger"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func eksExecConfig() *clientcmdapi.ExecConfig {
	return &clientcmdapi.ExecConfig{
		APIVersion:      "client.authentication.k8s.io/v1beta1",
		Command:         "aws",
		Args:            []string{"--region", "us-west-2", "eks", "get-token", "--cluster-name", "demo"},
		InteractiveMode: clientcmdapi.NeverExecInteractiveMode,
	}
}

func gkeExecConfig() *clientcmdapi.ExecConfig {
	return &clientcmdapi.ExecConfig{
		APIVersion:      "client.authentication.k8s.io/v1beta1",
		Command:         "gke-gcloud-auth-plugin",
		Args:            []string{"--use_application_default_credentials"},
		InteractiveMode: clientcmdapi.NeverExecInteractiveMode,
	}
}

func staticFactory(token string, calls *atomic.Int32) tokenSourceFactory {
	return func(context.Context, Spec) (oauth2.TokenSource, error) {
		calls.Add(1)
		return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token, TokenType: "Bearer"}), nil
	}
}

func failingFactory(calls *atomic.Int32) tokenSourceFactory {
	return func(context.Context, Spec) (oauth2.TokenSource, error) {
		calls.Add(1)
		return nil, ErrAWSCredentialsUnavailable(fmt.Errorf("no credentials in chain"), "demo")
	}
}

func notOnPath(string) (string, error) { return "", fmt.Errorf("executable file not found in $PATH") }

func onPath(file string) (string, error) { return "/usr/local/bin/" + file, nil }

// authorizationRecorder is a TLS test server that records the Authorization
// header of every request and answers 200 only to want.
type authorizationRecorder struct {
	*httptest.Server
	seen atomic.Value
	hits atomic.Int32
}

func newAuthorizationRecorder(t *testing.T, want string) *authorizationRecorder {
	t.Helper()
	recorder := &authorizationRecorder{}
	recorder.seen.Store("")
	recorder.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder.hits.Add(1)
		got := r.Header.Get("Authorization")
		recorder.seen.Store(got)
		if got != "Bearer "+want {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		_, _ = io.WriteString(w, "ok")
	}))
	t.Cleanup(recorder.Close)
	return recorder
}

func getStatus(t *testing.T, config *rest.Config) int {
	t.Helper()
	client, err := rest.HTTPClientFor(config)
	if err != nil {
		t.Fatalf("HTTPClientFor() error = %v", err)
	}
	resp, err := client.Get(config.Host)
	if err != nil {
		return 0
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}

func TestAuthenticatorInstallsBearerTransport(t *testing.T) {
	server := newAuthorizationRecorder(t, "in-process-token")

	var calls atomic.Int32
	auth := NewAuthenticator(WithTokenSourceFactory(staticFactory("in-process-token", &calls)), withLookPath(notOnPath))

	config := redirectedConfig(server.Server, testEKSHost, eksExecConfig())
	if err := auth.Apply(config); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if config.ExecProvider != nil {
		t.Fatal("Apply() left ExecProvider set; client-go would still fork the plugin")
	}
	if config.BearerToken != "" || config.BearerTokenFile != "" {
		t.Fatalf("Apply() set a static bearer token (%q, %q)", config.BearerToken, config.BearerTokenFile)
	}
	if got := getStatus(t, config); got != http.StatusOK {
		t.Fatalf("GET status = %d (Authorization %q), want 200", got, server.seen.Load())
	}

	// A second rest.Config for the same cluster reuses the cached source.
	if err := auth.Apply(eksConfig(eksExecConfig())); err != nil {
		t.Fatalf("second Apply() error = %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("factory calls = %d, want 1 (token source must be cached per cluster)", got)
	}

	// A different cluster gets its own source.
	other := eksExecConfig()
	other.Args = []string{"eks", "get-token", "--cluster-name", "other", "--region", "us-west-2"}
	if err := auth.Apply(eksConfig(other)); err != nil {
		t.Fatalf("Apply() for another cluster error = %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("factory calls = %d, want 2 after a second cluster", got)
	}
}

func TestAuthenticatorBindsEKSRegionToEndpoint(t *testing.T) {
	var got Spec
	auth := NewAuthenticator(WithTokenSourceFactory(func(_ context.Context, spec Spec) (oauth2.TokenSource, error) {
		got = spec
		return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "t"}), nil
	}))
	exec := eksExecConfig()
	exec.Args = []string{"eks", "get-token", "--cluster-name", "demo"}
	if err := auth.Apply(eksConfig(exec)); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got.EKS.Region != "us-west-2" {
		t.Fatalf("factory region = %q, want us-west-2 from the endpoint host", got.EKS.Region)
	}
}

func TestAuthenticatorLeavesUnrecognizedExecUntouched(t *testing.T) {
	var calls atomic.Int32
	auth := NewAuthenticator(WithTokenSourceFactory(staticFactory("unused", &calls)))

	for _, exec := range []*clientcmdapi.ExecConfig{
		{Command: "kubelogin", Args: []string{"get-token"}},
		{Command: "/usr/local/bin/custom"},
		{Command: "gke-gcloud-auth-plugin"},
		{Command: "gcloud", Args: []string{"config", "config-helper"}},
	} {
		config := &rest.Config{Host: testGKEDNSHost, ExecProvider: exec}
		if err := auth.Apply(config); err != nil {
			t.Fatalf("Apply(%q) error = %v", exec.Command, err)
		}
		if config.ExecProvider != exec || config.WrapTransport != nil {
			t.Fatalf("Apply(%q %v) modified an exec stanza it must leave to client-go", exec.Command, exec.Args)
		}
	}
	if err := auth.Apply(&rest.Config{Host: testEKSHost}); err != nil {
		t.Fatalf("Apply() without exec error = %v", err)
	}
	if err := auth.Apply(nil); err != nil {
		t.Fatalf("Apply(nil) error = %v", err)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("factory calls = %d, want 0", got)
	}
}

func TestAuthenticatorPreservesClientGoCredentialPrecedence(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*rest.Config)
	}{
		{name: "bearer token", mutate: func(c *rest.Config) { c.BearerToken = "static" }},
		{name: "bearer token file", mutate: func(c *rest.Config) { c.BearerTokenFile = "/var/run/token" }},
		{name: "basic auth", mutate: func(c *rest.Config) { c.Username, c.Password = "admin", "secret" }},
		{name: "client certificate data", mutate: func(c *rest.Config) { c.CertData, c.KeyData = []byte("cert"), []byte("key") }},
		{name: "client certificate files", mutate: func(c *rest.Config) { c.CertFile, c.KeyFile = "/tls/crt", "/tls/key" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			// Neither credentials nor the plugin exist; intercepting would
			// turn a working config into an error.
			auth := NewAuthenticator(WithTokenSourceFactory(failingFactory(&calls)), withLookPath(notOnPath))
			exec := eksExecConfig()
			config := eksConfig(exec)
			tt.mutate(config)
			if err := auth.Apply(config); err != nil {
				t.Fatalf("Apply() error = %v, want the existing credentials to take precedence", err)
			}
			if config.ExecProvider != exec || config.WrapTransport != nil || calls.Load() != 0 {
				t.Fatal("Apply() intercepted an exec stanza client-go would not run")
			}
		})
	}

	t.Run("a certificate without a key does not take precedence", func(t *testing.T) {
		var calls atomic.Int32
		auth := NewAuthenticator(WithTokenSourceFactory(staticFactory("minted", &calls)))
		config := eksConfig(eksExecConfig())
		config.CertData = []byte("cert")
		if err := auth.Apply(config); err != nil {
			t.Fatalf("Apply() error = %v", err)
		}
		if config.ExecProvider != nil || calls.Load() != 1 {
			t.Fatal("Apply() did not intercept although client-go would run the plugin")
		}
	})
}

func TestAuthenticatorRefusesUntrustedEndpoints(t *testing.T) {
	proxy := func(*http.Request) (*url.URL, error) { return url.Parse("http://proxy.attacker.example:3128") }
	tests := []struct {
		name   string
		config func() *rest.Config
	}{
		{name: "EKS stanza against an attacker host", config: func() *rest.Config {
			return &rest.Config{Host: "https://attacker.example.com", ExecProvider: eksExecConfig()}
		}},
		{name: "EKS stanza against an attacker IP", config: func() *rest.Config {
			return &rest.Config{Host: "https://203.0.113.7", ExecProvider: eksExecConfig(), TLSClientConfig: rest.TLSClientConfig{CAData: testCAData}}
		}},
		{name: "EKS stanza with TLS verification disabled", config: func() *rest.Config {
			config := eksConfig(eksExecConfig())
			config.Insecure = true
			return config
		}},
		{name: "EKS stanza over http", config: func() *rest.Config {
			return &rest.Config{Host: strings.Replace(testEKSHost, "https", "http", 1), ExecProvider: eksExecConfig()}
		}},
		{name: "EKS stanza through a kubeconfig proxy", config: func() *rest.Config {
			config := eksConfig(eksExecConfig())
			config.Proxy = proxy
			return config
		}},
		{name: "EKS stanza whose region does not match the endpoint", config: func() *rest.Config {
			exec := eksExecConfig()
			exec.Args = []string{"--region", "eu-west-1", "eks", "get-token", "--cluster-name", "demo"}
			return eksConfig(exec)
		}},
		{name: "GKE stanza against an attacker host", config: func() *rest.Config {
			return &rest.Config{Host: "https://attacker.example.com", ExecProvider: gkeExecConfig(), TLSClientConfig: rest.TLSClientConfig{CAData: testCAData}}
		}},
		{name: "GKE stanza against an IP without a CA", config: func() *rest.Config {
			return &rest.Config{Host: "https://34.118.10.20", ExecProvider: gkeExecConfig()}
		}},
		{name: "GKE stanza with TLS verification disabled", config: func() *rest.Config {
			return &rest.Config{Host: testGKEDNSHost, ExecProvider: gkeExecConfig(), TLSClientConfig: rest.TLSClientConfig{Insecure: true}}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			factory := staticFactory("process-credential", &calls)

			// Without the plugin: a clear error and nothing minted.
			auth := NewAuthenticator(WithTokenSourceFactory(factory), withLookPath(notOnPath))
			config := tt.config()
			exec := config.ExecProvider
			err := auth.Apply(config)
			if got := errors.GetCode(err); got != ErrUntrustedClusterEndpointCode {
				t.Fatalf("Apply() error = %v, want code %q", err, ErrUntrustedClusterEndpointCode)
			}
			if config.ExecProvider != exec || config.WrapTransport != nil {
				t.Fatal("Apply() modified the config of an untrusted endpoint")
			}

			// With the plugin: the plugin is used, logged once, nothing minted.
			log, logs := bufferLogger(t)
			auth = NewAuthenticator(WithTokenSourceFactory(factory), withLookPath(onPath), withLogger(log))
			for i := 0; i < 2; i++ {
				config = tt.config()
				exec = config.ExecProvider
				if err := auth.Apply(config); err != nil {
					t.Fatalf("Apply() with the plugin on PATH error = %v", err)
				}
				if config.ExecProvider != exec || config.WrapTransport != nil {
					t.Fatal("Apply() installed in-process minting for an untrusted endpoint")
				}
			}
			if got := calls.Load(); got != 0 {
				t.Fatalf("factory calls = %d, want 0: no token may be minted for an untrusted endpoint", got)
			}
			if lines := strings.Split(strings.TrimSpace(logs.String()), "\n"); len(lines) != 1 ||
				!strings.Contains(lines[0], ErrUntrustedClusterEndpointFallbackCode) {
				t.Fatalf("logs = %q, want one %s warning", logs.String(), ErrUntrustedClusterEndpointFallbackCode)
			}
		})
	}
}

func TestAuthenticatorSendsNoCredentialsToAttackerHost(t *testing.T) {
	attacker := newAuthorizationRecorder(t, "process-credential")
	var calls atomic.Int32
	auth := NewAuthenticator(WithTokenSourceFactory(staticFactory("process-credential", &calls)), withLookPath(onPath))

	// The attacker serves valid TLS for its own name and puts its CA in the
	// kubeconfig; only the endpoint rules stand in the way.
	for _, exec := range []*clientcmdapi.ExecConfig{eksExecConfig(), gkeExecConfig()} {
		config := redirectedConfig(attacker.Server, "https://example.com", exec)
		if err := auth.Apply(config); err != nil {
			t.Fatalf("Apply() error = %v", err)
		}
		if config.ExecProvider == nil || config.WrapTransport != nil {
			t.Fatalf("Apply() intercepted the %q stanza for an attacker host", exec.Command)
		}
		// Send a request with whatever Apply installed, minus the exec
		// plugin (which would need the real binary).
		config.ExecProvider = nil
		_ = getStatus(t, config)
		if got := attacker.seen.Load(); got != "" {
			t.Fatalf("attacker received Authorization %q", got)
		}
	}
	if attacker.hits.Load() == 0 {
		t.Fatal("the attacker server saw no request")
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("factory calls = %d, want 0", got)
	}
}

func TestAuthenticatorAcceptsGKEEndpointForms(t *testing.T) {
	server := newAuthorizationRecorder(t, "adc-token")
	ca := redirectedConfig(server.Server, server.URL, nil).CAData
	t.Setenv("GOOGLE_CLOUD_PROJECT", "my-project")
	api := startContainerAPI(t, []servedCluster{{
		Project: "my-project", Location: "us-central1", Name: "demo",
		Cluster: clusterFor(urlHostIP(t, server.URL), ca),
	}})
	var calls atomic.Int32
	auth := NewAuthenticator(
		WithTokenSourceFactory(staticFactory("adc-token", &calls)),
		withLookPath(notOnPath),
		withContainerAPIBase(api.URL),
		withLogger(discardLogger(t)),
	)

	// An IP endpoint whose CA the Container API confirms, end to end.
	ipConfig := &rest.Config{
		Host:            server.URL,
		ExecProvider:    gkeExecConfig(),
		TLSClientConfig: rest.TLSClientConfig{CAData: ca},
	}
	if err := auth.Apply(ipConfig); err != nil {
		t.Fatalf("Apply(IP endpoint) error = %v", err)
	}
	if got := getStatus(t, ipConfig); got != http.StatusOK {
		t.Fatalf("GET status = %d, want 200", got)
	}

	// A DNS-based endpoint, end to end through the redirect.
	dnsConfig := redirectedConfig(server.Server, testGKEDNSHost, gkeExecConfig())
	if err := auth.Apply(dnsConfig); err != nil {
		t.Fatalf("Apply(DNS endpoint) error = %v", err)
	}
	if dnsConfig.ExecProvider != nil || dnsConfig.WrapTransport == nil {
		t.Fatal("Apply() did not intercept a GKE DNS-based endpoint")
	}

	// Connect Gateway.
	gateway := &rest.Config{Host: "https://connectgateway.googleapis.com/v1/projects/1/locations/global/gkeMemberships/demo", ExecProvider: gkeExecConfig()}
	if err := auth.Apply(gateway); err != nil || gateway.ExecProvider != nil {
		t.Fatalf("Apply(Connect Gateway) = %v, ExecProvider %v; want interception", err, gateway.ExecProvider)
	}
}

func TestAuthenticatorMissingCredentialsWithoutPluginFails(t *testing.T) {
	var calls atomic.Int32
	auth := NewAuthenticator(WithTokenSourceFactory(failingFactory(&calls)), withLookPath(notOnPath))

	err := auth.Apply(eksConfig(eksExecConfig()))
	if err == nil {
		t.Fatal("Apply() succeeded without credentials or plugin")
	}
	if got := errors.GetCode(err); got != ErrInProcessCloudAuthUnavailableCode {
		t.Fatalf("error code = %q, want %q", got, ErrInProcessCloudAuthUnavailableCode)
	}
	message := err.Error()
	for _, want := range []string{"kubeconfig alone is not enough", `"aws"`, "no credentials in chain"} {
		if !strings.Contains(message, want) {
			t.Fatalf("error %q does not mention %q", message, want)
		}
	}

	// Errors are not cached: once credentials appear, setup is retried.
	if err := auth.Apply(eksConfig(eksExecConfig())); err == nil {
		t.Fatal("second Apply() unexpectedly succeeded")
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("factory calls = %d, want 2 (failures must not be cached)", got)
	}
}

// bufferLogger returns a MeshKit JSON logger writing to the returned buffer.
func bufferLogger(t *testing.T) (logger.Handler, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	log, err := logger.New("cloudauth-test", logger.Options{
		Format:   logger.JsonLogFormat,
		LogLevel: int(logrus.InfoLevel),
		Output:   &buf,
	})
	if err != nil {
		t.Fatalf("logger.New() error = %v", err)
	}
	return log, &buf
}

func TestAuthenticatorMissingCredentialsFallsBackToPluginOnPath(t *testing.T) {
	var calls atomic.Int32
	log, logs := bufferLogger(t)
	auth := NewAuthenticator(
		WithTokenSourceFactory(failingFactory(&calls)),
		withLookPath(onPath),
		withLogger(log),
	)

	for i := 0; i < 2; i++ {
		exec := eksExecConfig()
		config := eksConfig(exec)
		if err := auth.Apply(config); err != nil {
			t.Fatalf("Apply() error = %v", err)
		}
		if config.ExecProvider != exec || config.WrapTransport != nil {
			t.Fatal("Apply() replaced the exec stanza although it should fall back to the plugin")
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("factory calls = %d, want 1 (fallback decision must be cached)", got)
	}

	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("logged %d entries, want 1 per fallback decision: %q", len(lines), logs.String())
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("log entry %q is not JSON: %v", lines[0], err)
	}
	if entry["level"] != "warning" || entry["code"] != ErrInProcessCloudAuthFallbackCode {
		t.Fatalf("log entry = %v, want a warning with code %q", entry, ErrInProcessCloudAuthFallbackCode)
	}
	if msg, _ := entry["msg"].(string); !strings.Contains(msg, "no credentials in chain") || !strings.Contains(msg, `"aws"`) {
		t.Fatalf("log message %q does not carry the setup failure and plugin", msg)
	}
}

func TestAuthenticatorDisabled(t *testing.T) {
	tests := []struct {
		name string
		env  string
		opts []Option
	}{
		{name: "environment variable", env: "true"},
		{name: "environment variable numeric", env: "1"},
		{name: "option", opts: []Option{withDisabled(true)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(DisableEnvVar, tt.env)
			var calls atomic.Int32
			auth := NewAuthenticator(append(tt.opts, WithTokenSourceFactory(staticFactory("unused", &calls)))...)

			exec := eksExecConfig()
			config := eksConfig(exec)
			if err := auth.Apply(config); err != nil {
				t.Fatalf("Apply() error = %v", err)
			}
			if config.ExecProvider != exec || config.WrapTransport != nil || calls.Load() != 0 {
				t.Fatal("disabled Authenticator modified the exec stanza")
			}
		})
	}

	t.Run("false value keeps minting enabled", func(t *testing.T) {
		t.Setenv(DisableEnvVar, "false")
		var calls atomic.Int32
		auth := NewAuthenticator(WithTokenSourceFactory(staticFactory("minted", &calls)))
		config := eksConfig(eksExecConfig())
		if err := auth.Apply(config); err != nil {
			t.Fatalf("Apply() error = %v", err)
		}
		if config.ExecProvider != nil {
			t.Fatal("Apply() left ExecProvider set with minting enabled")
		}
	})
}

func TestDefaultTokenSourceFactoryRejectsUnknownProvider(t *testing.T) {
	_, err := defaultTokenSourceFactory(context.Background(), Spec{Provider: "aks"})
	if got := errors.GetCode(err); got != ErrUnsupportedCloudProviderCode {
		t.Fatalf("error code = %q, want %q (error: %v)", got, ErrUnsupportedCloudProviderCode, err)
	}
}

func TestNewGKETokenSourceFromJSON(t *testing.T) {
	endpoint := useGoogleTokenEndpoint(t, http.StatusOK, validGoogleToken)

	source, err := NewGKETokenSourceFromJSON(context.Background(), fakeServiceAccountKey(t, nil))
	if err != nil {
		t.Fatalf("NewGKETokenSourceFromJSON() error = %v", err)
	}
	if got := endpoint.requests.Load(); got != 1 {
		t.Fatalf("token requests at setup = %d, want 1 (a bad key must fail fast)", got)
	}
	token, err := source.Token()
	if err != nil || token.AccessToken != "adc-token" {
		t.Fatalf("Token() = (%v, %v), want the setup token", token, err)
	}
	if got := endpoint.requests.Load(); got != 1 {
		t.Fatalf("token requests = %d, want 1 (the setup token must be reused)", got)
	}
	for _, scope := range gkeScopes {
		if !contains(endpoint.requestedScopes(), scope) {
			t.Fatalf("requested scopes = %v, want %s", endpoint.requestedScopes(), scope)
		}
	}
}

func TestNewGKETokenSourceFromJSONRejectsUnusableKeys(t *testing.T) {
	endpoint := useGoogleTokenEndpoint(t, http.StatusOK, validGoogleToken)

	for name, key := range map[string][]byte{
		"malformed JSON":                       []byte("{not json"),
		"authorized_user key":                  fakeServiceAccountKey(t, map[string]string{"type": "authorized_user"}),
		"external_account key":                 fakeServiceAccountKey(t, map[string]string{"type": "external_account"}),
		"custom token_uri":                     fakeServiceAccountKey(t, map[string]string{"token_uri": "https://attacker.example.com/token"}),
		"internal token_uri":                   fakeServiceAccountKey(t, map[string]string{"token_uri": "http://169.254.169.254/token"}),
		"legacy accounts.google.com token_uri": fakeServiceAccountKey(t, map[string]string{"token_uri": "https://accounts.google.com/o/oauth2/token"}),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := NewGKETokenSourceFromJSON(context.Background(), key)
			if got := errors.GetCode(err); got != ErrGCPServiceAccountKeyCode {
				t.Fatalf("error code = %q, want %q (error: %v)", got, ErrGCPServiceAccountKeyCode, err)
			}
		})
	}
	if got := endpoint.requests.Load(); got != 0 {
		t.Fatalf("token requests = %d, want 0: a rejected key must never be used", got)
	}

	t.Run("key rejected by Google fails at setup", func(t *testing.T) {
		useGoogleTokenEndpoint(t, http.StatusBadRequest, `{"error":"invalid_grant","error_description":"Invalid JWT Signature."}`)
		_, err := NewGKETokenSourceFromJSON(context.Background(), fakeServiceAccountKey(t, nil))
		if got := errors.GetCode(err); got != ErrGCPServiceAccountKeyCode {
			t.Fatalf("error code = %q, want %q (error: %v)", got, ErrGCPServiceAccountKeyCode, err)
		}
	})
}

func TestNewGKETokenSourceFetchesTokenAtSetup(t *testing.T) {
	endpoint := useGoogleTokenEndpoint(t, http.StatusOK, validGoogleToken)
	useADCFile(t, fakeServiceAccountKey(t, nil))

	source, err := setupGKETokenSource(context.Background())
	if err != nil {
		t.Fatalf("setupGKETokenSource() error = %v", err)
	}
	token, err := source.Token()
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	if token.AccessToken != "adc-token" {
		t.Fatalf("AccessToken = %q, want %q", token.AccessToken, "adc-token")
	}
	if got := endpoint.requests.Load(); got != 1 {
		t.Fatalf("token endpoint requests = %d, want 1 (the setup token must be reused)", got)
	}
	for _, scope := range gkeScopes {
		if !contains(endpoint.requestedScopes(), scope) {
			t.Fatalf("requested scopes = %v, want %s", endpoint.requestedScopes(), scope)
		}
	}
}

func TestNewGKETokenSourceRejectsUnsupportedADCFiles(t *testing.T) {
	endpoint := useGoogleTokenEndpoint(t, http.StatusOK, validGoogleToken)

	for name, data := range map[string][]byte{
		"service_account with a custom token_uri": fakeServiceAccountKey(t, map[string]string{"token_uri": "https://attacker.example.com/token"}),
		"authorized_user with a custom token_uri": []byte(`{"type":"authorized_user","client_id":"id","client_secret":"s","refresh_token":"r","token_uri":"https://attacker.example.com/token"}`),
		"external_account":                        []byte(`{"type":"external_account","audience":"//iam.googleapis.com/x","subject_token_type":"urn:ietf:params:oauth:token-type:jwt","token_url":"https://attacker.example.com/token","credential_source":{"file":"/tmp/token"}}`),
		"impersonated_service_account":            []byte(`{"type":"impersonated_service_account","service_account_impersonation_url":"https://attacker.example.com/","source_credentials":{"type":"authorized_user","client_id":"id","client_secret":"s","refresh_token":"r"}}`),
	} {
		t.Run(name, func(t *testing.T) {
			useADCFile(t, data)
			_, err := setupGKETokenSource(context.Background())
			if got := errors.GetCode(err); got != ErrGCPCredentialsUnavailableCode {
				t.Fatalf("error code = %q, want %q (error: %v)", got, ErrGCPCredentialsUnavailableCode, err)
			}
		})
	}
	if got := endpoint.requests.Load(); got != 0 {
		t.Fatalf("token requests = %d, want 0: rejected credentials must never be used", got)
	}

	t.Run("authorized_user without token_uri is accepted", func(t *testing.T) {
		useADCFile(t, []byte(`{"type":"authorized_user","client_id":"id","client_secret":"s","refresh_token":"r"}`))
		if _, err := setupGKETokenSource(context.Background()); err != nil {
			t.Fatalf("setupGKETokenSource() error = %v", err)
		}
	})
}

func TestNewGKETokenSourceRejectsUnusableCredentials(t *testing.T) {
	useGoogleTokenEndpoint(t, http.StatusBadRequest, `{"error":"invalid_grant","error_description":"revoked"}`)
	useADCFile(t, fakeServiceAccountKey(t, nil))

	_, err := setupGKETokenSource(context.Background())
	if got := errors.GetCode(err); got != ErrGCPCredentialsUnavailableCode {
		t.Fatalf("error code = %q, want %q (error: %v)", got, ErrGCPCredentialsUnavailableCode, err)
	}
}

// Google's JWT-bearer flow accepts a 200 with an empty access_token, so the
// setup token must be checked the same way refreshes are: an empty bearer
// must never reach the API server.
func TestGKESetupRejectsEmptyAccessToken(t *testing.T) {
	const emptyToken = `{"access_token":"","token_type":"Bearer","expires_in":3600}`

	t.Run("application default credentials", func(t *testing.T) {
		useGoogleTokenEndpoint(t, http.StatusOK, emptyToken)
		useADCFile(t, fakeServiceAccountKey(t, nil))

		_, err := setupGKETokenSource(context.Background())
		if got := errors.GetCode(err); got != ErrGCPCredentialsUnavailableCode {
			t.Fatalf("error code = %q, want %q (error: %v)", got, ErrGCPCredentialsUnavailableCode, err)
		}
		if !strings.Contains(errors.GetLDescription(err), errEmptyToken.Error()) {
			t.Fatalf("error description = %q, want the empty-token cause", errors.GetLDescription(err))
		}
	})

	t.Run("service-account key JSON", func(t *testing.T) {
		useGoogleTokenEndpoint(t, http.StatusOK, emptyToken)

		_, err := NewGKETokenSourceFromJSON(context.Background(), fakeServiceAccountKey(t, nil))
		if got := errors.GetCode(err); got != ErrGCPServiceAccountKeyCode {
			t.Fatalf("error code = %q, want %q (error: %v)", got, ErrGCPServiceAccountKeyCode, err)
		}
		if !strings.Contains(errors.GetLDescription(err), errEmptyToken.Error()) {
			t.Fatalf("error description = %q, want the empty-token cause", errors.GetLDescription(err))
		}
	})
}

// A source supplied through WithTokenSourceFactory is held to the same
// empty-token rule as the package's own sources.
func TestAuthenticatorRejectsEmptyTokenFromFactorySource(t *testing.T) {
	server := newAuthorizationRecorder(t, "never")
	auth := NewAuthenticator(WithTokenSourceFactory(func(context.Context, Spec) (oauth2.TokenSource, error) {
		return oauth2.StaticTokenSource(&oauth2.Token{TokenType: "Bearer"}), nil
	}), withLookPath(onPath))

	config := redirectedConfig(server.Server, testEKSHost, eksExecConfig())
	if err := auth.Apply(config); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got := getStatus(t, config); got != 0 {
		t.Fatalf("GET status = %d, want the request to fail before reaching the server", got)
	}
	if got := server.hits.Load(); got != 0 {
		t.Fatalf("API server requests = %d, want 0 (an empty bearer must not be sent)", got)
	}
}

func TestNewGKETokenSourceSetupTimeoutCancelsTheRequest(t *testing.T) {
	endpoint := useGoogleTokenEndpoint(t, http.StatusOK, validGoogleToken)
	cancelled := make(chan struct{})
	endpoint.respond = func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		close(cancelled)
		return nil, r.Context().Err()
	}
	useADCFile(t, fakeServiceAccountKey(t, nil))

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := setupGKETokenSource(ctx); err == nil {
		t.Fatal("setupGKETokenSource() succeeded against a stalled token endpoint")
	}
	select {
	case <-cancelled:
	case <-time.After(10 * time.Second):
		t.Fatal("the stalled token request was not cancelled when setup timed out")
	}
}

func TestAuthenticatorFallsBackToGKEPluginWhenADCCannotMint(t *testing.T) {
	useGoogleTokenEndpoint(t, http.StatusBadRequest, `{"error":"invalid_grant","error_description":"revoked"}`)
	useADCFile(t, fakeServiceAccountKey(t, nil))

	auth := NewAuthenticator(withLookPath(onPath))
	config := &rest.Config{Host: testGKEDNSHost, ExecProvider: gkeExecConfig()}
	if err := auth.Apply(config); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if config.ExecProvider == nil || config.ExecProvider.Command != "gke-gcloud-auth-plugin" {
		t.Fatalf("ExecProvider = %#v, want the plugin stanza left for client-go", config.ExecProvider)
	}
	if config.WrapTransport != nil {
		t.Fatal("Apply() wrapped the transport although ADC could not mint a token")
	}
}

// sequenceFactory returns, on its nth call, a source yielding tokens[n] (or
// the last entry once exhausted). An empty token makes the source fail.
func sequenceFactory(calls *atomic.Int32, tokens ...string) tokenSourceFactory {
	return func(context.Context, Spec) (oauth2.TokenSource, error) {
		n := int(calls.Add(1)) - 1
		if n >= len(tokens) {
			n = len(tokens) - 1
		}
		if tokens[n] == "" {
			return failingSource{}, nil
		}
		return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: tokens[n], TokenType: "Bearer"}), nil
	}
}

type failingSource struct{}

func (failingSource) Token() (*oauth2.Token, error) {
	return nil, fmt.Errorf("security token included in the request is expired")
}

func TestAuthenticatorSetsUpAgainAfterUnauthorized(t *testing.T) {
	server := newAuthorizationRecorder(t, "fresh")
	var calls atomic.Int32
	auth := NewAuthenticator(WithTokenSourceFactory(sequenceFactory(&calls, "stale", "fresh")), withLookPath(onPath))

	config := redirectedConfig(server.Server, testEKSHost, eksExecConfig())
	if err := auth.Apply(config); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got := getStatus(t, config); got != http.StatusUnauthorized {
		t.Fatalf("first GET status = %d, want 401 with the stale credentials", got)
	}
	if got := getStatus(t, config); got != http.StatusOK {
		t.Fatalf("GET after 401 status = %d, want 200 from re-loaded credentials", got)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("factory calls = %d, want 2 (a 401 must re-run setup once)", got)
	}

	// Clients built later share the refreshed source.
	second := redirectedConfig(server.Server, testEKSHost, eksExecConfig())
	if err := auth.Apply(second); err != nil {
		t.Fatalf("second Apply() error = %v", err)
	}
	if got := getStatus(t, second); got != http.StatusOK || calls.Load() != 2 {
		t.Fatalf("second client status = %d, factory calls = %d; want 200 and 2", got, calls.Load())
	}
}

func TestAuthenticatorSetsUpAgainAfterTokenFailure(t *testing.T) {
	server := newAuthorizationRecorder(t, "fresh")
	var calls atomic.Int32
	auth := NewAuthenticator(WithTokenSourceFactory(sequenceFactory(&calls, "", "fresh")), withLookPath(onPath))

	config := redirectedConfig(server.Server, testEKSHost, eksExecConfig())
	if err := auth.Apply(config); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got := getStatus(t, config); got == http.StatusOK {
		t.Fatal("first GET succeeded although the token source failed")
	}
	if got := getStatus(t, config); got != http.StatusOK {
		t.Fatalf("GET after a failed token status = %d, want 200 from re-loaded credentials", got)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("factory calls = %d, want 2", got)
	}
}

func TestAuthenticatorCachedFallbackRequiresOwnCommandOnPath(t *testing.T) {
	var calls atomic.Int32
	auth := NewAuthenticator(
		WithTokenSourceFactory(failingFactory(&calls)),
		withLookPath(func(file string) (string, error) {
			if file == "aws" {
				return "/usr/bin/aws", nil
			}
			return notOnPath(file)
		}),
	)

	// Both stanzas resolve to the same cluster identity.
	cli := eksConfig(eksExecConfig())
	if err := auth.Apply(cli); err != nil {
		t.Fatalf("Apply(aws) error = %v", err)
	}
	if cli.ExecProvider == nil {
		t.Fatal("Apply(aws) did not fall back to the installed aws CLI")
	}

	authenticator := eksConfig(&clientcmdapi.ExecConfig{Command: "aws-iam-authenticator", Args: []string{"token", "-i", "demo", "--region", "us-west-2"}})
	err := auth.Apply(authenticator)
	if got := errors.GetCode(err); got != ErrInProcessCloudAuthUnavailableCode {
		t.Fatalf("Apply(aws-iam-authenticator) error = %v, want code %q (its binary is not on PATH)", err, ErrInProcessCloudAuthUnavailableCode)
	}
	if !strings.Contains(err.Error(), "no credentials in chain") {
		t.Fatalf("error %q does not carry the setup failure", err)
	}
}

func TestAuthenticatorRetriesSetupWhenFallbackPluginDisappears(t *testing.T) {
	var calls atomic.Int32
	factory := func(context.Context, Spec) (oauth2.TokenSource, error) {
		if calls.Add(1) == 1 {
			return nil, fmt.Errorf("credentials not yet mounted")
		}
		return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "minted"}), nil
	}
	var installed atomic.Bool
	installed.Store(true)
	auth := NewAuthenticator(WithTokenSourceFactory(factory), withLookPath(func(file string) (string, error) {
		if installed.Load() {
			return onPath(file)
		}
		return notOnPath(file)
	}))
	auth.clock = newFakeClock()

	if config := eksConfig(eksExecConfig()); auth.Apply(config) != nil || config.ExecProvider == nil {
		t.Fatal("first Apply() did not fall back to the installed plugin")
	}
	// The plugin is removed and credentials appear, well inside the
	// fallback retry interval: setup must run again rather than fail.
	installed.Store(false)
	config := eksConfig(eksExecConfig())
	if err := auth.Apply(config); err != nil {
		t.Fatalf("Apply() after the plugin disappeared error = %v", err)
	}
	if config.ExecProvider != nil {
		t.Fatal("Apply() did not switch to in-process minting")
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("factory calls = %d, want 2", got)
	}
}

func TestAuthenticatorRetriesSetupAfterFallbackInterval(t *testing.T) {
	var calls atomic.Int32
	factory := func(ctx context.Context, spec Spec) (oauth2.TokenSource, error) {
		if calls.Load() == 0 {
			calls.Add(1)
			return nil, fmt.Errorf("transient STS failure")
		}
		calls.Add(1)
		return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "minted"}), nil
	}
	auth := NewAuthenticator(WithTokenSourceFactory(factory), withLookPath(onPath))
	clock := newFakeClock()
	auth.clock = clock

	apply := func() *rest.Config {
		t.Helper()
		config := eksConfig(eksExecConfig())
		if err := auth.Apply(config); err != nil {
			t.Fatalf("Apply() error = %v", err)
		}
		return config
	}

	if apply().ExecProvider == nil {
		t.Fatal("first Apply() did not fall back to the plugin")
	}
	clock.Advance(fallbackRetryInterval - time.Second)
	if apply().ExecProvider == nil || calls.Load() != 1 {
		t.Fatalf("Apply() within the retry interval re-ran setup (factory calls = %d)", calls.Load())
	}
	clock.Advance(time.Second)
	if apply().ExecProvider != nil {
		t.Fatal("Apply() after the retry interval did not switch to in-process minting")
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("factory calls = %d, want 2", got)
	}
}

// blockingFactory blocks setup for cluster "demo" until release, then
// returns result; any other cluster sets up at once. A setup whose deadline
// passes records the cancellation and returns its cause.
type blockingFactory struct {
	calls     atomic.Int32
	started   chan struct{}
	release   chan struct{}
	cancelled chan error
	result    func() (oauth2.TokenSource, error)
}

func newBlockingFactory(result func() (oauth2.TokenSource, error)) *blockingFactory {
	return &blockingFactory{
		started:   make(chan struct{}, 16),
		release:   make(chan struct{}),
		cancelled: make(chan error, 16),
		result:    result,
	}
}

func (f *blockingFactory) factory(ctx context.Context, spec Spec) (oauth2.TokenSource, error) {
	if spec.EKS.ClusterID != "demo" {
		return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "other"}), nil
	}
	f.calls.Add(1)
	f.started <- struct{}{}
	select {
	case <-f.release:
		return f.result()
	case <-ctx.Done():
		f.cancelled <- context.Cause(ctx)
		return nil, context.Cause(ctx)
	}
}

func setupWaiters(entry *authEntry) int {
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if entry.setupFlight == nil {
		return 0
	}
	return entry.setupFlight.waiters
}

func tokenWaiters(entry *authEntry) int {
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if entry.tokenFlight == nil {
		return 0
	}
	return entry.tokenFlight.waiters
}

func TestAuthenticatorSetupDoesNotHoldLocksAndSharesFailure(t *testing.T) {
	f := newBlockingFactory(func() (oauth2.TokenSource, error) {
		return nil, fmt.Errorf("STS throttled")
	})
	auth := NewAuthenticator(WithTokenSourceFactory(f.factory), withLookPath(notOnPath))
	auth.clock = newFakeClock()

	const callers = 8
	results := make(chan error, callers)
	apply := func() { results <- auth.Apply(eksConfig(eksExecConfig())) }
	go apply()
	<-f.started

	// While setup for "demo" is blocked: another cluster sets up, and the
	// 401 path for "demo" takes the entry lock and returns.
	other := eksExecConfig()
	other.Args = []string{"eks", "get-token", "--cluster-name", "other"}
	returnsPromptly(t, "Apply for another cluster", func() {
		if err := auth.Apply(eksConfig(other)); err != nil {
			t.Errorf("Apply(other) error = %v", err)
		}
	})
	spec, _ := Recognize(eksExecConfig())
	entry := auth.entry(spec)
	returnsPromptly(t, "ResetTokenOlderThan", func() { entry.ResetTokenOlderThan(time.Now()) })

	for i := 1; i < callers; i++ {
		go apply()
	}
	eventually(t, "every other Apply to join the setup", func() bool { return setupWaiters(entry) == callers-1 })

	close(f.release)
	for i := 0; i < callers; i++ {
		err := <-results
		if errors.GetCode(err) != ErrInProcessCloudAuthUnavailableCode || !strings.Contains(err.Error(), "STS throttled") {
			t.Fatalf("Apply() error = %v, want the shared setup failure", err)
		}
	}
	if got := f.calls.Load(); got != 1 {
		t.Fatalf("setup calls = %d, want 1 shared by every caller", got)
	}
}

func TestAuthenticatorSetupTimeoutCancels(t *testing.T) {
	f := newBlockingFactory(func() (oauth2.TokenSource, error) {
		t.Error("setup was released, want it cancelled by the deadline")
		return nil, nil
	})
	clock := newFakeClock()
	auth := NewAuthenticator(WithTokenSourceFactory(f.factory), withLookPath(notOnPath))
	auth.clock = clock

	result := make(chan error, 1)
	go func() { result <- auth.Apply(eksConfig(eksExecConfig())) }()
	<-f.started
	clock.Advance(defaultSetupTimeout)

	if cause := <-f.cancelled; !stderrors.Is(cause, context.DeadlineExceeded) {
		t.Fatalf("setup context cause = %v, want context.DeadlineExceeded", cause)
	}
	err := <-result
	if errors.GetCode(err) != ErrInProcessCloudAuthUnavailableCode || !strings.Contains(err.Error(), "gave up setting up credentials") {
		t.Fatalf("Apply() error = %v, want ErrInProcessCloudAuthUnavailable carrying the setup timeout", err)
	}
}

func TestAuthenticatorTokenAfterResetSharesOneBoundedSetup(t *testing.T) {
	var fail atomic.Bool
	f := newBlockingFactory(func() (oauth2.TokenSource, error) {
		if fail.Load() {
			return nil, fmt.Errorf("credentials revoked")
		}
		return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "first"}), nil
	})
	clock := newFakeClock()
	auth := NewAuthenticator(WithTokenSourceFactory(f.factory), withLookPath(notOnPath))
	auth.clock = clock

	close(f.release)
	spec, _ := Recognize(eksExecConfig())
	source, err := auth.tokenSource(spec)
	if err != nil || source == nil {
		t.Fatalf("tokenSource() = (%v, %v), want a source", source, err)
	}
	<-f.started
	entry := source.(*authEntry)

	// A 401 discards the source; the next requests re-run setup together.
	fail.Store(true)
	entry.ResetTokenOlderThan(clock.Now().Add(time.Second))
	f.release = make(chan struct{})

	const callers = 8
	results := make(chan error, callers)
	for i := 0; i < callers; i++ {
		go func() {
			_, err := entry.Token()
			results <- err
		}()
	}
	<-f.started
	eventually(t, "every other Token call to join", func() bool { return tokenWaiters(entry) == callers-1 })
	returnsPromptly(t, "ResetTokenOlderThan during setup", func() { entry.ResetTokenOlderThan(clock.Now()) })

	close(f.release)
	for i := 0; i < callers; i++ {
		if err := <-results; err == nil || !strings.Contains(err.Error(), "credentials revoked") {
			t.Fatalf("Token() error = %v, want the shared setup failure", err)
		}
	}
	if got := f.calls.Load(); got != 2 {
		t.Fatalf("setup calls = %d, want 2 (one at Apply, one shared after the reset)", got)
	}

	// The re-setup after a reset is bounded too.
	f.release = make(chan struct{})
	result := make(chan error, 1)
	go func() {
		_, err := entry.Token()
		result <- err
	}()
	<-f.started
	clock.Advance(defaultSetupTimeout)
	if cause := <-f.cancelled; !stderrors.Is(cause, context.DeadlineExceeded) {
		t.Fatalf("setup context cause = %v, want context.DeadlineExceeded", cause)
	}
	if err := <-result; errors.GetCode(err) != ErrCloudAuthTimeoutCode {
		t.Fatalf("Token() error = %v, want code %q", err, ErrCloudAuthTimeoutCode)
	}
}

func TestAuthenticatorBoundsForeignTokenSources(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	clock := newFakeClock()
	auth := NewAuthenticator(WithTokenSourceFactory(func(context.Context, Spec) (oauth2.TokenSource, error) {
		return stalledSource{block: block}, nil
	}))
	auth.clock = clock

	spec, _ := Recognize(eksExecConfig())
	source, err := auth.tokenSource(spec)
	if err != nil {
		t.Fatalf("tokenSource() error = %v", err)
	}
	timers := func() int {
		clock.mu.Lock()
		defer clock.mu.Unlock()
		return len(clock.timers)
	}
	setupTimers := timers()
	result := make(chan error, 1)
	go func() {
		_, err := source.Token()
		result <- err
	}()
	entry := source.(*authEntry)
	eventually(t, "the token call to start its deadline", func() bool { return timers() > setupTimers })
	clock.Advance(defaultMintTimeout)
	if err := <-result; errors.GetCode(err) != ErrCloudAuthTimeoutCode {
		t.Fatalf("Token() error = %v, want code %q", err, ErrCloudAuthTimeoutCode)
	}
	entry.mu.Lock()
	discarded := entry.source == nil
	entry.mu.Unlock()
	if !discarded {
		t.Fatal("a timed-out source was kept; the next call must set up again")
	}
}

// stalledSource never returns a token until block is closed.
type stalledSource struct{ block chan struct{} }

func (s stalledSource) Token() (*oauth2.Token, error) {
	<-s.block
	return nil, fmt.Errorf("released")
}
