package cloudauth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/meshery/meshkit/errors"
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

func staticFactory(token string, calls *atomic.Int32) TokenSourceFactory {
	return func(context.Context, Spec) (oauth2.TokenSource, error) {
		calls.Add(1)
		return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token, TokenType: "Bearer"}), nil
	}
}

func failingFactory(calls *atomic.Int32) TokenSourceFactory {
	return func(context.Context, Spec) (oauth2.TokenSource, error) {
		calls.Add(1)
		return nil, ErrAWSCredentialsUnavailable(fmt.Errorf("no credentials in chain"), "demo")
	}
}

func notOnPath(string) (string, error) { return "", fmt.Errorf("executable file not found in $PATH") }

func onPath(file string) (string, error) { return "/usr/local/bin/" + file, nil }

func TestAuthenticatorInstallsBearerTransport(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer in-process-token" {
			http.Error(w, "unexpected Authorization "+got, http.StatusUnauthorized)
			return
		}
		_, _ = io.WriteString(w, "ok")
	}))
	defer server.Close()

	var calls atomic.Int32
	auth := NewAuthenticator(WithTokenSourceFactory(staticFactory("in-process-token", &calls)), WithLookPath(notOnPath))

	config := &rest.Config{Host: server.URL, ExecProvider: eksExecConfig(), TLSClientConfig: rest.TLSClientConfig{Insecure: true}}
	if err := auth.Apply(config); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if config.ExecProvider != nil {
		t.Fatal("Apply() left ExecProvider set; client-go would still fork the plugin")
	}
	if config.BearerToken != "" || config.BearerTokenFile != "" {
		t.Fatalf("Apply() set a static bearer token (%q, %q)", config.BearerToken, config.BearerTokenFile)
	}

	client, err := rest.HTTPClientFor(config)
	if err != nil {
		t.Fatalf("HTTPClientFor() error = %v", err)
	}
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("GET error = %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d (%s), want 200", resp.StatusCode, body)
	}

	// A second rest.Config for the same cluster reuses the cached source.
	second := &rest.Config{Host: server.URL, ExecProvider: eksExecConfig()}
	if err := auth.Apply(second); err != nil {
		t.Fatalf("second Apply() error = %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("factory calls = %d, want 1 (token source must be cached per cluster)", got)
	}

	// A different cluster gets its own source.
	other := eksExecConfig()
	other.Args = []string{"eks", "get-token", "--cluster-name", "other", "--region", "us-west-2"}
	if err := auth.Apply(&rest.Config{Host: server.URL, ExecProvider: other}); err != nil {
		t.Fatalf("Apply() for another cluster error = %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("factory calls = %d, want 2 after a second cluster", got)
	}
}

func TestAuthenticatorLeavesUnrecognizedExecUntouched(t *testing.T) {
	var calls atomic.Int32
	auth := NewAuthenticator(WithTokenSourceFactory(staticFactory("unused", &calls)))

	for _, exec := range []*clientcmdapi.ExecConfig{
		{Command: "kubelogin", Args: []string{"get-token"}},
		{Command: "/usr/local/bin/custom"},
	} {
		config := &rest.Config{Host: "https://cluster.example.com", ExecProvider: exec}
		if err := auth.Apply(config); err != nil {
			t.Fatalf("Apply(%q) error = %v", exec.Command, err)
		}
		if config.ExecProvider != exec || config.WrapTransport != nil {
			t.Fatalf("Apply(%q) modified an unrecognized exec stanza", exec.Command)
		}
	}
	if err := auth.Apply(&rest.Config{Host: "https://cluster.example.com"}); err != nil {
		t.Fatalf("Apply() without exec error = %v", err)
	}
	if err := auth.Apply(nil); err != nil {
		t.Fatalf("Apply(nil) error = %v", err)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("factory calls = %d, want 0", got)
	}
}

func TestAuthenticatorMissingCredentialsWithoutPluginFails(t *testing.T) {
	var calls atomic.Int32
	auth := NewAuthenticator(WithTokenSourceFactory(failingFactory(&calls)), WithLookPath(notOnPath))

	config := &rest.Config{Host: "https://cluster.example.com", ExecProvider: eksExecConfig()}
	err := auth.Apply(config)
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
	if err := auth.Apply(&rest.Config{ExecProvider: eksExecConfig()}); err == nil {
		t.Fatal("second Apply() unexpectedly succeeded")
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("factory calls = %d, want 2 (failures must not be cached)", got)
	}
}

func TestAuthenticatorMissingCredentialsFallsBackToPluginOnPath(t *testing.T) {
	var calls, observed atomic.Int32
	auth := NewAuthenticator(
		WithTokenSourceFactory(failingFactory(&calls)),
		WithLookPath(onPath),
		WithFallbackObserver(func(spec Spec, err error) {
			observed.Add(1)
			if spec.Provider != ProviderEKS || err == nil {
				t.Errorf("fallback observer got (%v, %v)", spec.Provider, err)
			}
		}),
	)

	for i := 0; i < 2; i++ {
		exec := eksExecConfig()
		config := &rest.Config{Host: "https://cluster.example.com", ExecProvider: exec}
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
	if got := observed.Load(); got != 1 {
		t.Fatalf("fallback observer calls = %d, want 1", got)
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
		{name: "option", opts: []Option{WithDisabled(true)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(DisableEnvVar, tt.env)
			var calls atomic.Int32
			auth := NewAuthenticator(append(tt.opts, WithTokenSourceFactory(staticFactory("unused", &calls)))...)

			exec := eksExecConfig()
			config := &rest.Config{ExecProvider: exec}
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
		config := &rest.Config{ExecProvider: eksExecConfig()}
		if err := auth.Apply(config); err != nil {
			t.Fatalf("Apply() error = %v", err)
		}
		if config.ExecProvider != nil {
			t.Fatal("Apply() left ExecProvider set with minting enabled")
		}
	})
}

func TestDefaultTokenSourceFactoryRejectsUnknownProvider(t *testing.T) {
	_, err := DefaultTokenSourceFactory(context.Background(), Spec{Provider: "aks"})
	if got := errors.GetCode(err); got != ErrUnsupportedCloudProviderCode {
		t.Fatalf("error code = %q, want %q (error: %v)", got, ErrUnsupportedCloudProviderCode, err)
	}
}

// fakeServiceAccountKey returns a syntactically valid service-account key
// with a freshly generated RSA key. It is not registered anywhere.
func fakeServiceAccountKey(t *testing.T, credentialType string) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey() error = %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey() error = %v", err)
	}
	data, err := json.Marshal(map[string]string{
		"type":           credentialType,
		"project_id":     "meshkit-test",
		"private_key_id": "0123456789abcdef",
		"private_key":    string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})),
		"client_email":   "meshkit-test@meshkit-test.iam.gserviceaccount.com",
		"client_id":      "123456789",
		"token_uri":      "https://oauth2.googleapis.com/token",
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return data
}

func TestNewGKETokenSourceFromJSON(t *testing.T) {
	// Building the source must not need the network; Token is never called.
	source, err := NewGKETokenSourceFromJSON(context.Background(), fakeServiceAccountKey(t, "service_account"))
	if err != nil {
		t.Fatalf("NewGKETokenSourceFromJSON() error = %v", err)
	}
	if source == nil {
		t.Fatal("NewGKETokenSourceFromJSON() returned a nil token source")
	}

	for name, key := range map[string][]byte{
		"malformed JSON":          []byte("{not json"),
		"non service-account key": fakeServiceAccountKey(t, "authorized_user"),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := NewGKETokenSourceFromJSON(context.Background(), key)
			if got := errors.GetCode(err); got != ErrGCPServiceAccountKeyCode {
				t.Fatalf("error code = %q, want %q (error: %v)", got, ErrGCPServiceAccountKeyCode, err)
			}
		})
	}
}

func TestInstallTokenSourceWithGKESource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer gke-token" {
			http.Error(w, "unexpected Authorization "+got, http.StatusUnauthorized)
			return
		}
		_, _ = io.WriteString(w, "ok")
	}))
	defer server.Close()

	config := &rest.Config{Host: server.URL, ExecProvider: &clientcmdapi.ExecConfig{Command: "gke-gcloud-auth-plugin"}}
	InstallTokenSource(config, oauth2.ReuseTokenSource(nil, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "gke-token"})))
	if config.ExecProvider != nil {
		t.Fatal("InstallTokenSource() left ExecProvider set")
	}

	client, err := rest.HTTPClientFor(config)
	if err != nil {
		t.Fatalf("HTTPClientFor() error = %v", err)
	}
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("GET error = %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d, want 200", resp.StatusCode)
	}
}

// useADCServiceAccount points Application Default Credentials at a
// service-account key whose token endpoint is a local test server answering
// with status and body, and returns how many token requests it received.
func useADCServiceAccount(t *testing.T, status int, body string) *atomic.Int32 {
	t.Helper()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)

	var key map[string]string
	if err := json.Unmarshal(fakeServiceAccountKey(t, "service_account"), &key); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	key["token_uri"] = server.URL
	data, err := json.Marshal(key)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	path := filepath.Join(t.TempDir(), "adc.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", path)
	return &requests
}

func TestNewGKETokenSourceFetchesTokenAtSetup(t *testing.T) {
	requests := useADCServiceAccount(t, http.StatusOK,
		`{"access_token":"adc-token","token_type":"Bearer","expires_in":3600}`)

	source, err := NewGKETokenSource(context.Background())
	if err != nil {
		t.Fatalf("NewGKETokenSource() error = %v", err)
	}
	token, err := source.Token()
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	if token.AccessToken != "adc-token" {
		t.Fatalf("AccessToken = %q, want %q", token.AccessToken, "adc-token")
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("token endpoint requests = %d, want 1 (the setup token must be reused)", got)
	}
}

func TestNewGKETokenSourceRejectsUnusableCredentials(t *testing.T) {
	useADCServiceAccount(t, http.StatusBadRequest, `{"error":"invalid_grant","error_description":"revoked"}`)

	_, err := NewGKETokenSource(context.Background())
	if got := errors.GetCode(err); got != ErrGCPCredentialsUnavailableCode {
		t.Fatalf("error code = %q, want %q (error: %v)", got, ErrGCPCredentialsUnavailableCode, err)
	}
}

func TestAuthenticatorFallsBackToGKEPluginWhenADCCannotMint(t *testing.T) {
	useADCServiceAccount(t, http.StatusBadRequest, `{"error":"invalid_grant","error_description":"revoked"}`)

	auth := NewAuthenticator(WithLookPath(func(file string) (string, error) {
		return "/usr/local/bin/" + file, nil
	}))
	config := &rest.Config{Host: "https://gke.example.com", ExecProvider: &clientcmdapi.ExecConfig{
		APIVersion: "client.authentication.k8s.io/v1beta1",
		Command:    "gke-gcloud-auth-plugin",
	}}
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
