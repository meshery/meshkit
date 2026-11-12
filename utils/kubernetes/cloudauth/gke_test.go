package cloudauth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/meshery/meshkit/errors"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func isolateGCPEnv(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CLOUDSDK_CONFIG", t.TempDir())
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
	t.Setenv("CLOUDSDK_CORE_ACCOUNT", "")
	// Keep empty: setting GCE_METADATA_HOST non-empty causes metadata.OnGCE() to return true.
	t.Setenv("GCE_METADATA_HOST", "")
	t.Setenv(DisableEnvVar, "")
}

func buildTestServiceAccountJSON(t *testing.T, tokenURI string) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	der := x509.MarshalPKCS1PrivateKey(key)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der})

	payload := map[string]string{
		"type":                        "service_account",
		"project_id":                  "meshery-test-project",
		"private_key_id":              "key-1",
		"private_key":                 string(pemBytes),
		"client_email":                "meshery@meshery-test-project.iam.gserviceaccount.com",
		"client_id":                   "1234567890",
		"auth_uri":                    "https://accounts.google.com/o/oauth2/auth",
		"token_uri":                   tokenURI,
		"auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return b
}

func startMockGoogleOAuthServer(t *testing.T, calls *atomic.Int32, expiresInSeconds int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if r.Form.Get("grant_type") != "urn:ietf:params:oauth:grant-type:jwt-bearer" {
			http.Error(w, "unexpected grant_type: "+r.Form.Get("grant_type"), http.StatusBadRequest)
			return
		}
		n := calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"access_token":"gke-access-%d","token_type":"Bearer","expires_in":%d}`, n, expiresInSeconds)
	}))
}

func TestGKEServiceAccountJSONMintAndProactiveRefresh(t *testing.T) {
	isolateGCPEnv(t)

	var oauthCalls atomic.Int32
	// expires_in=5 ensures oauth2's internal reuseTokenSource (~10s leeway) allows
	// our RefreshingTokenSource refresh call to actually re-exchange with the server.
	srv := startMockGoogleOAuthServer(t, &oauthCalls, 5)
	defer srv.Close()

	saJSON := buildTestServiceAccountJSON(t, srv.URL)
	now := time.Now().UTC()

	authInfo := &clientcmdapi.AuthInfo{
		Exec: &clientcmdapi.ExecConfig{
			Command: "gke-gcloud-auth-plugin",
		},
	}

	ts, matched, ok, err := ResolveAuthInfo(
		context.Background(),
		authInfo,
		WithCredentials(Credentials{
			GCP: &GCPCredentials{ServiceAccountJSON: saJSON},
		}),
		withClock(func() time.Time { return now }),
		withExpiryDelta(time.Second),
	)
	if err != nil || !ok {
		t.Fatalf("ResolveAuthInfo ok=%v err=%v", ok, err)
	}
	if matched.Provider != ProviderGKE {
		t.Fatalf("Provider = %q, want gke", matched.Provider)
	}
	// ResolveAuthInfo must NOT make an eager OAuth2 network exchange at setup time.
	if oauthCalls.Load() != 0 {
		t.Fatalf("ResolveAuthInfo made %d network token calls at setup; expected 0", oauthCalls.Load())
	}

	tok1, err := ts.Token()
	if err != nil {
		t.Fatalf("Token() error: %v", err)
	}
	if tok1.AccessToken != "gke-access-1" || oauthCalls.Load() != 1 {
		t.Fatalf("got token=%q calls=%d, want gke-access-1 calls=1", tok1.AccessToken, oauthCalls.Load())
	}

	// Move fake clock safely before Expiry - 1s -> cached hit without hitting OAuth endpoint.
	now = tok1.Expiry.Add(-2 * time.Second)
	tokCached, err := ts.Token()
	if err != nil {
		t.Fatalf("cached Token() error: %v", err)
	}
	if tokCached.AccessToken != "gke-access-1" || oauthCalls.Load() != 1 {
		t.Fatalf("expected cached token, got %q calls=%d", tokCached.AccessToken, oauthCalls.Load())
	}

	// Move fake clock inside Expiry - 1s window -> refreshes via OAuth2 endpoint.
	now = tok1.Expiry.Add(-500 * time.Millisecond)
	tok2, err := ts.Token()
	if err != nil {
		t.Fatalf("refreshed Token() error: %v", err)
	}
	if tok2.AccessToken != "gke-access-2" || oauthCalls.Load() != 2 {
		t.Fatalf("expected refreshed gke-access-2, got %q calls=%d", tok2.AccessToken, oauthCalls.Load())
	}
}

func TestGKEExecEnvGoogleApplicationCredentials(t *testing.T) {
	isolateGCPEnv(t)

	var oauthCalls atomic.Int32
	srv := startMockGoogleOAuthServer(t, &oauthCalls, 300)
	defer srv.Close()

	saJSON := buildTestServiceAccountJSON(t, srv.URL)
	credPath := filepath.Join(t.TempDir(), "sa.json")
	if err := os.WriteFile(credPath, saJSON, 0o600); err != nil {
		t.Fatalf("WriteFile sa.json: %v", err)
	}

	authInfo := &clientcmdapi.AuthInfo{
		Exec: &clientcmdapi.ExecConfig{
			Command: "gcloud",
			Args:    []string{"config", "config-helper", "--format=json"},
			Env: []clientcmdapi.ExecEnvVar{
				{Name: "GOOGLE_APPLICATION_CREDENTIALS", Value: credPath},
			},
		},
	}

	ts, matched, ok, err := ResolveAuthInfo(context.Background(), authInfo)
	if err != nil || !ok {
		t.Fatalf("ResolveAuthInfo ok=%v err=%v", ok, err)
	}
	if matched.Provider != ProviderGKE {
		t.Fatalf("Provider = %q, want gke", matched.Provider)
	}
	tok, err := ts.Token()
	if err != nil {
		t.Fatalf("Token() error: %v", err)
	}
	if tok.AccessToken != "gke-access-1" {
		t.Fatalf("AccessToken = %q, want gke-access-1", tok.AccessToken)
	}
}

func TestGKEMockADCAndMissingCredentials(t *testing.T) {
	isolateGCPEnv(t)

	// Custom ADC resolver path works for legacy auth-provider: gcp too.
	cfg := defaultGKEConfig()
	cfg.findDefaultCredentials = func(_ context.Context, _ ...string) (*google.Credentials, error) {
		return &google.Credentials{
			TokenSource: oauth2.StaticTokenSource(&oauth2.Token{
				AccessToken: "adc-mock-token",
				TokenType:   "Bearer",
			}),
		}, nil
	}
	ts, err := newGKETokenSource(context.Background(), MatchedConfig{
		Provider:       ProviderGKE,
		Command:        "auth-provider:gcp",
		LegacyProvider: true,
	}, nil, cfg)
	if err != nil {
		t.Fatalf("newGKETokenSource ADC error: %v", err)
	}
	tok, err := ts.Token()
	if err != nil || tok.AccessToken != "adc-mock-token" {
		t.Fatalf("got tok=%v err=%v", tok, err)
	}

	// Missing credentials error when ADC fails.
	cfgErr := defaultGKEConfig()
	cfgErr.findDefaultCredentials = func(_ context.Context, _ ...string) (*google.Credentials, error) {
		return nil, fmt.Errorf("could not find default credentials")
	}
	_, err = newGKETokenSource(context.Background(), MatchedConfig{
		Provider: ProviderGKE,
		Command:  "gke-gcloud-auth-plugin",
	}, nil, cfgErr)
	if err == nil {
		t.Fatal("expected error when ADC is unavailable")
	}
	if got := errors.GetCode(err); got != ErrGCPCredentialsCode {
		t.Fatalf("code = %q, want %q (err=%v)", got, ErrGCPCredentialsCode, err)
	}
}

func TestGKE401ResetEvictsInnerOAuthCache(t *testing.T) {
	isolateGCPEnv(t)

	var oauthCalls atomic.Int32
	// Realistic 300s expiry so oauth2.ReuseTokenSource caches internally unless onReset clears it.
	oauthSrv := startMockGoogleOAuthServer(t, &oauthCalls, 300)
	defer oauthSrv.Close()

	saJSON := buildTestServiceAccountJSON(t, oauthSrv.URL)
	ts, _, ok, err := ResolveAuthInfo(
		context.Background(),
		&clientcmdapi.AuthInfo{
			Exec: &clientcmdapi.ExecConfig{Command: "gke-gcloud-auth-plugin"},
		},
		WithCredentials(Credentials{GCP: &GCPCredentials{ServiceAccountJSON: saJSON}}),
	)
	if err != nil || !ok {
		t.Fatalf("ResolveAuthInfo ok=%v err=%v", ok, err)
	}

	var seenAuth []string
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		seenAuth = append(seenAuth, auth)
		if auth == "Bearer gke-access-1" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer apiSrv.Close()

	hc := &http.Client{Transport: ts.WrapTransport(http.DefaultTransport)}

	// First request mints gke-access-1, receives 401.
	r1, err := hc.Get(apiSrv.URL)
	if err != nil {
		t.Fatalf("r1 error: %v", err)
	}
	_ = r1.Body.Close()

	// Sleep briefly so request start > mintedAt; second request sends cached gke-access-1, gets 401, triggers reset.
	time.Sleep(5 * time.Millisecond)
	r2, err := hc.Get(apiSrv.URL)
	if err != nil {
		t.Fatalf("r2 error: %v", err)
	}
	_ = r2.Body.Close()

	// Third request must mint fresh gke-access-2 despite 299s left on inner oauth2 cache.
	r3, err := hc.Get(apiSrv.URL)
	if err != nil {
		t.Fatalf("r3 error: %v", err)
	}
	_ = r3.Body.Close()
	if r3.StatusCode != http.StatusOK {
		t.Fatalf("r3 status = %d, want 200", r3.StatusCode)
	}
	if oauthCalls.Load() != 2 {
		t.Fatalf("oauthCalls = %d, want 2 after 401 cache eviction", oauthCalls.Load())
	}
	if len(seenAuth) != 3 || seenAuth[2] != "Bearer gke-access-2" {
		t.Fatalf("seenAuth = %v", seenAuth)
	}
}

func TestGKEExecEnvAuthorizedUserAndUnsupportedTypes(t *testing.T) {
	isolateGCPEnv(t)

	noBinLookPath := func(cmd string) (string, error) {
		return "", &exec.Error{Name: cmd, Err: exec.ErrNotFound}
	}

	// authorized_user JSON parses cleanly at ResolveAuthInfo time without network exchange.
	authUserPath := filepath.Join(t.TempDir(), "adc.json")
	if err := os.WriteFile(authUserPath, []byte(`{
  "type": "authorized_user",
  "client_id": "12345.apps.googleusercontent.com",
  "client_secret": "test-secret",
  "refresh_token": "1//test-refresh"
}`), 0o600); err != nil {
		t.Fatalf("WriteFile adc.json: %v", err)
	}
	_, matched, ok, err := ResolveAuthInfo(
		context.Background(),
		&clientcmdapi.AuthInfo{
			Exec: &clientcmdapi.ExecConfig{
				Command: "gcloud",
				Args:    []string{"auth", "print-access-token"},
				Env:     []clientcmdapi.ExecEnvVar{{Name: "GOOGLE_APPLICATION_CREDENTIALS", Value: authUserPath}},
			},
		},
		withLookPath(noBinLookPath),
	)
	if err != nil || !ok || matched.Provider != ProviderGKE {
		t.Fatalf("expected authorized_user acceptance offline, got ok=%v matched=%+v err=%v", ok, matched, err)
	}

	// external_account JSON in exec.env is rejected as unsupported/unsafe credential type.
	extPath := filepath.Join(t.TempDir(), "external.json")
	if err := os.WriteFile(extPath, []byte(`{"type":"external_account"}`), 0o600); err != nil {
		t.Fatalf("WriteFile external.json: %v", err)
	}
	_, _, ok, err = ResolveAuthInfo(
		context.Background(),
		&clientcmdapi.AuthInfo{
			Exec: &clientcmdapi.ExecConfig{
				Command: "gke-gcloud-auth-plugin",
				Env:     []clientcmdapi.ExecEnvVar{{Name: "GOOGLE_APPLICATION_CREDENTIALS", Value: extPath}},
			},
		},
		withLookPath(noBinLookPath),
	)
	if !ok || errors.GetCode(err) != ErrGCPCredentialsCode {
		t.Fatalf("expected ErrGCPCredentials for external_account, got ok=%v err=%v", ok, err)
	}
}

func TestGKEFallbackOnPathAndDisableEnvVar(t *testing.T) {
	isolateGCPEnv(t)

	foundOnPath := func(cmd string) (string, error) {
		return "/usr/bin/" + cmd, nil
	}
	missingADC := func(context.Context, ...string) (*google.Credentials, error) {
		return nil, fmt.Errorf("could not find default credentials")
	}

	// Missing ADC with gke-gcloud-auth-plugin on PATH falls back to #986 exec loader.
	_, _, ok, err := ResolveAuthInfo(
		context.Background(),
		&clientcmdapi.AuthInfo{
			Exec: &clientcmdapi.ExecConfig{Command: "gke-gcloud-auth-plugin"},
		},
		withLookPath(foundOnPath),
		withGKEFindDefaultCredentials(missingADC),
	)
	if ok || err != nil {
		t.Fatalf("expected PATH fallback for GKE, got ok=%v err=%v", ok, err)
	}

	// --impersonate_service_account with binary on PATH falls back to #986.
	_, _, ok, err = ResolveAuthInfo(
		context.Background(),
		&clientcmdapi.AuthInfo{
			Exec: &clientcmdapi.ExecConfig{
				Command: "gke-gcloud-auth-plugin",
				Args:    []string{"--impersonate_service_account=sa@proj.iam.gserviceaccount.com"},
			},
		},
		withLookPath(foundOnPath),
	)
	if ok || err != nil {
		t.Fatalf("expected PATH fallback for GKE impersonation, got ok=%v err=%v", ok, err)
	}

	// Explicit bad ServiceAccountJSON must NOT silently fall back even if binary is on PATH.
	_, _, ok, err = ResolveAuthInfo(
		context.Background(),
		&clientcmdapi.AuthInfo{
			Exec: &clientcmdapi.ExecConfig{Command: "gke-gcloud-auth-plugin"},
		},
		WithCredentials(Credentials{GCP: &GCPCredentials{ServiceAccountJSON: []byte(`not-json`)}}),
		withLookPath(foundOnPath),
	)
	if !ok || errors.GetCode(err) != ErrGCPCredentialsCode {
		t.Fatalf("expected ErrGCPCredentials for invalid explicit JSON, got ok=%v err=%v", ok, err)
	}
}

func TestGKEDefaultScopesIncludeUserinfoEmail(t *testing.T) {
	isolateGCPEnv(t)

	var seenScopes []string
	cfg := defaultGKEConfig()
	cfg.findDefaultCredentials = func(_ context.Context, scopes ...string) (*google.Credentials, error) {
		seenScopes = append([]string(nil), scopes...)
		return &google.Credentials{
			TokenSource: oauth2.StaticTokenSource(&oauth2.Token{
				AccessToken: "scope-check-token",
				TokenType:   "Bearer",
			}),
		}, nil
	}

	if _, err := newGKETokenSource(context.Background(), MatchedConfig{
		Provider: ProviderGKE,
		Command:  "gke-gcloud-auth-plugin",
	}, nil, cfg); err != nil {
		t.Fatalf("newGKETokenSource error: %v", err)
	}

	want := []string{
		"https://www.googleapis.com/auth/cloud-platform",
		"https://www.googleapis.com/auth/userinfo.email",
	}
	if !reflect.DeepEqual(seenScopes, want) {
		t.Fatalf("default GKE scopes = %v, want %v", seenScopes, want)
	}
}

func TestGKEMintTimeoutAndCancellation(t *testing.T) {
	isolateGCPEnv(t)

	unblock := make(chan struct{})
	defer close(unblock)
	var failSlow atomic.Bool
	failSlow.Store(true)

	ts, _, ok, err := ResolveAuthInfo(
		context.Background(),
		&clientcmdapi.AuthInfo{
			Exec: &clientcmdapi.ExecConfig{Command: "gke-gcloud-auth-plugin"},
		},
		withMintTimeout(25*time.Millisecond),
		withGKEFindDefaultCredentials(func(_ context.Context, _ ...string) (*google.Credentials, error) {
			return &google.Credentials{
				TokenSource: oauth2.TokenSource(&funcTokenSource{fn: func() (*oauth2.Token, error) {
					if failSlow.Load() {
						<-unblock
						return nil, fmt.Errorf("should have timed out")
					}
					return &oauth2.Token{
						AccessToken: "gke-recovered-token",
						TokenType:   "Bearer",
						Expiry:      time.Now().Add(5 * time.Minute),
					}, nil
				}}),
			}, nil
		}),
	)
	if err != nil || !ok {
		t.Fatalf("ResolveAuthInfo ok=%v err=%v", ok, err)
	}

	_, err = ts.Token()
	if err == nil {
		t.Fatal("expected timeout error from hanging GKE TokenSource")
	}
	if got := errors.GetCode(err); got != ErrGKETokenMintCode {
		t.Fatalf("timeout error code = %q, want %q (err=%v)", got, ErrGKETokenMintCode, err)
	}

	// Canceled caller context aborts immediately as well.
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = ts.TokenWithContext(cancelCtx)
	if err == nil || errors.GetCode(err) != ErrGKETokenMintCode {
		t.Fatalf("canceled context code = %q, want %q (err=%v)", errors.GetCode(err), ErrGKETokenMintCode, err)
	}

	// Next call recovers cleanly without getting wedged.
	failSlow.Store(false)
	tok, err := ts.Token()
	if err != nil || tok.AccessToken != "gke-recovered-token" {
		t.Fatalf("expected recovered token, got tok=%v err=%v", tok, err)
	}
}

type funcTokenSource struct {
	fn func() (*oauth2.Token, error)
}

func (f *funcTokenSource) Token() (*oauth2.Token, error) {
	return f.fn()
}
