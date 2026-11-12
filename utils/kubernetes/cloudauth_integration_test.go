package kubernetes

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	meshkiterrors "github.com/meshery/meshkit/errors"
	"github.com/meshery/meshkit/utils/kubernetes/cloudauth"
)

func isolateIntegrationEnv(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CLOUDSDK_CONFIG", t.TempDir())
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	// Keep empty: setting GCE_METADATA_HOST non-empty causes metadata.OnGCE() to return true.
	t.Setenv("GCE_METADATA_HOST", "")
	t.Setenv(cloudauth.DisableEnvVar, "")
	for _, k := range []string{
		"AWS_ACCESS_KEY_ID",
		"AWS_SECRET_ACCESS_KEY",
		"AWS_SESSION_TOKEN",
		"AWS_PROFILE",
		"AWS_DEFAULT_PROFILE",
		"AWS_REGION",
		"AWS_DEFAULT_REGION",
		"AWS_CONFIG_FILE",
		"AWS_SHARED_CREDENTIALS_FILE",
		"AWS_WEB_IDENTITY_TOKEN_FILE",
		"AWS_ROLE_ARN",
		"GOOGLE_APPLICATION_CREDENTIALS",
	} {
		t.Setenv(k, "")
	}
}

func startMockKubeAPIServer(t *testing.T) (*httptest.Server, *[]string, *sync.Mutex) {
	t.Helper()
	var mu sync.Mutex
	var authHeaders []string

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		authHeaders = append(authHeaders, r.Header.Get("Authorization"))
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"major":"1","minor":"31","gitVersion":"v1.31.0"}`)
	}))
	t.Cleanup(srv.Close)
	return srv, &authHeaders, &mu
}

func buildTestGCPServiceAccountJSON(t *testing.T, tokenURI string) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	der := x509.MarshalPKCS1PrivateKey(key)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der})

	b, err := json.Marshal(map[string]string{
		"type":                        "service_account",
		"project_id":                  "meshery-it-project",
		"private_key_id":              "it-key-1",
		"private_key":                 string(pemBytes),
		"client_email":                "meshery-it@meshery-it-project.iam.gserviceaccount.com",
		"client_id":                   "111222333444",
		"auth_uri":                    "https://accounts.google.com/o/oauth2/auth",
		"token_uri":                   tokenURI,
		"auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
	})
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return b
}

func TestNewWithOptionsEKSIntegration(t *testing.T) {
	isolateIntegrationEnv(t)

	apiSrv, authHeaders, mu := startMockKubeAPIServer(t)
	kubeconfig := []byte(fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- name: eks-cluster
  cluster:
    server: %s
    insecure-skip-tls-verify: true
contexts:
- name: eks-context
  context:
    cluster: eks-cluster
    user: eks-user
    namespace: kube-system
current-context: eks-context
users:
- name: eks-user
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: aws
      args:
      - eks
      - get-token
      - --cluster-name
      - prod-eks
      - --region
      - us-west-2
`, apiSrv.URL))

	client, err := NewWithOptions(kubeconfig, WithCloudCredentials(cloudauth.Credentials{
		AWS: &cloudauth.AWSCredentials{
			AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
			SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		},
	}))
	if err != nil {
		t.Fatalf("NewWithOptions EKS error: %v", err)
	}

	getter := client.getRESTClientGetter()
	rawCfg, err := getter.ToRawKubeConfigLoader().RawConfig()
	if err != nil {
		t.Fatalf("RawConfig() error: %v", err)
	}
	if rawCfg.AuthInfos["eks-user"] == nil || rawCfg.AuthInfos["eks-user"].Exec == nil {
		t.Fatal("expected RawConfig().AuthInfos[eks-user].Exec to be preserved")
	}
	if client.RestConfig.ExecProvider != nil || client.RestConfig.AuthProvider != nil || client.RestConfig.BearerToken != "" || client.RestConfig.WrapTransport == nil {
		t.Fatalf("unexpected client.RestConfig cloudauth wiring: ExecProvider=%v AuthProvider=%v BearerToken=%q WrapTransport=%v",
			client.RestConfig.ExecProvider, client.RestConfig.AuthProvider, client.RestConfig.BearerToken, client.RestConfig.WrapTransport != nil)
	}
	getterCfg, err := getter.ToRESTConfig()
	if err != nil {
		t.Fatalf("ToRESTConfig() error: %v", err)
	}
	if getterCfg.ExecProvider != nil || getterCfg.AuthProvider != nil || getterCfg.BearerToken != "" || getterCfg.WrapTransport == nil {
		t.Fatalf("unexpected ToRESTConfig() cloudauth wiring: ExecProvider=%v AuthProvider=%v BearerToken=%q WrapTransport=%v",
			getterCfg.ExecProvider, getterCfg.AuthProvider, getterCfg.BearerToken, getterCfg.WrapTransport != nil)
	}
	loaderCfg, err := getter.ToRawKubeConfigLoader().ClientConfig()
	if err != nil {
		t.Fatalf("ClientConfig() error: %v", err)
	}
	if loaderCfg.ExecProvider != nil || loaderCfg.AuthProvider != nil || loaderCfg.BearerToken != "" || loaderCfg.WrapTransport == nil {
		t.Fatalf("unexpected ClientConfig() cloudauth wiring: ExecProvider=%v AuthProvider=%v BearerToken=%q WrapTransport=%v",
			loaderCfg.ExecProvider, loaderCfg.AuthProvider, loaderCfg.BearerToken, loaderCfg.WrapTransport != nil)
	}

	ns, explicit, err := getter.ToRawKubeConfigLoader().Namespace()
	if err != nil || explicit || ns != "kube-system" {
		t.Fatalf("Namespace() = (%q, %v, %v), want (kube-system, false, nil)", ns, explicit, err)
	}

	ver, err := client.KubeClient.Discovery().ServerVersion()
	if err != nil {
		t.Fatalf("ServerVersion() error: %v", err)
	}
	if ver.GitVersion != "v1.31.0" {
		t.Fatalf("GitVersion = %q, want v1.31.0", ver.GitVersion)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(*authHeaders) == 0 {
		t.Fatal("expected Kubernetes API server to receive Authorization header")
	}
	header := (*authHeaders)[0]
	if !strings.HasPrefix(header, "Bearer k8s-aws-v1.") {
		t.Fatalf("Authorization = %q, want Bearer k8s-aws-v1.*", header)
	}
	rawURL, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(header, "Bearer k8s-aws-v1."))
	if err != nil {
		t.Fatalf("DecodeString EKS token: %v", err)
	}
	parsed, err := url.Parse(string(rawURL))
	if err != nil {
		t.Fatalf("url.Parse EKS token: %v", err)
	}
	if parsed.Host != "sts.us-west-2.amazonaws.com" || parsed.Query().Get("Action") != "GetCallerIdentity" {
		t.Fatalf("unexpected decoded EKS URL: %s", string(rawURL))
	}
}

func TestNewWithOptionsGKEExecAndLegacyIntegration(t *testing.T) {
	isolateIntegrationEnv(t)

	var oauthCalls atomic.Int32
	oauthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := oauthCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"access_token":"gke-integration-token-%d","token_type":"Bearer","expires_in":300}`, n)
	}))
	defer oauthSrv.Close()

	saJSON := buildTestGCPServiceAccountJSON(t, oauthSrv.URL)

	cases := []struct {
		name           string
		userYAML       string
		wantExec       bool
		wantLegacyAuth bool
		expectCall     int32
	}{
		{
			name: "gke-gcloud-auth-plugin exec",
			userYAML: `users:
- name: gke-user
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: gke-gcloud-auth-plugin`,
			wantExec:   true,
			expectCall: 1,
		},
		{
			name: "legacy auth-provider gcp",
			userYAML: `users:
- name: gke-user
  user:
    auth-provider:
      name: gcp`,
			wantLegacyAuth: true,
			expectCall:     2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			apiSrv, authHeaders, mu := startMockKubeAPIServer(t)
			kubeconfig := []byte(fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- name: gke-cluster
  cluster:
    server: %s
    insecure-skip-tls-verify: true
contexts:
- name: gke-context
  context:
    cluster: gke-cluster
    user: gke-user
current-context: gke-context
%s
`, apiSrv.URL, tc.userYAML))

			callsBefore := oauthCalls.Load()
			client, err := NewWithOptions(kubeconfig, WithCloudCredentials(cloudauth.Credentials{
				GCP: &cloudauth.GCPCredentials{ServiceAccountJSON: saJSON},
			}))
			if err != nil {
				t.Fatalf("NewWithOptions GKE error: %v", err)
			}
			if oauthCalls.Load() != callsBefore {
				t.Fatalf("expected zero OAuth calls during setup, got delta %d", oauthCalls.Load()-callsBefore)
			}

			getter := client.getRESTClientGetter()
			rawCfg, err := getter.ToRawKubeConfigLoader().RawConfig()
			if err != nil {
				t.Fatalf("RawConfig() error: %v", err)
			}
			authInfo := rawCfg.AuthInfos["gke-user"]
			if authInfo == nil {
				t.Fatal("expected RawConfig().AuthInfos[gke-user] to exist")
			}
			if tc.wantExec && authInfo.Exec == nil {
				t.Fatal("expected RawConfig().AuthInfos[gke-user].Exec to be preserved")
			}
			if tc.wantLegacyAuth && authInfo.AuthProvider == nil {
				t.Fatal("expected RawConfig().AuthInfos[gke-user].AuthProvider to be preserved")
			}
			if client.RestConfig.ExecProvider != nil || client.RestConfig.AuthProvider != nil || client.RestConfig.BearerToken != "" || client.RestConfig.WrapTransport == nil {
				t.Fatalf("unexpected client.RestConfig cloudauth wiring: ExecProvider=%v AuthProvider=%v BearerToken=%q WrapTransport=%v",
					client.RestConfig.ExecProvider, client.RestConfig.AuthProvider, client.RestConfig.BearerToken, client.RestConfig.WrapTransport != nil)
			}
			getterCfg, err := getter.ToRESTConfig()
			if err != nil {
				t.Fatalf("ToRESTConfig() error: %v", err)
			}
			if getterCfg.ExecProvider != nil || getterCfg.AuthProvider != nil || getterCfg.BearerToken != "" || getterCfg.WrapTransport == nil {
				t.Fatalf("unexpected ToRESTConfig() cloudauth wiring: ExecProvider=%v AuthProvider=%v BearerToken=%q WrapTransport=%v",
					getterCfg.ExecProvider, getterCfg.AuthProvider, getterCfg.BearerToken, getterCfg.WrapTransport != nil)
			}

			ver, err := client.KubeClient.Discovery().ServerVersion()
			if err != nil {
				t.Fatalf("ServerVersion() error: %v", err)
			}
			if ver.GitVersion != "v1.31.0" {
				t.Fatalf("GitVersion = %q, want v1.31.0", ver.GitVersion)
			}

			mu.Lock()
			gotHeaders := append([]string(nil), (*authHeaders)...)
			mu.Unlock()
			wantAuth := fmt.Sprintf("Bearer gke-integration-token-%d", tc.expectCall)
			if len(gotHeaders) == 0 || gotHeaders[0] != wantAuth {
				t.Fatalf("Authorization headers = %v, want first == %q", gotHeaders, wantAuth)
			}
		})
	}
}

func TestDetectKubeConfigPreservesUnknownExecPlugin(t *testing.T) {
	isolateIntegrationEnv(t)

	_, loader, err := detectKubeConfig(testExecKubeConfig())
	if err != nil {
		t.Fatalf("detectKubeConfig error: %v", err)
	}
	if loader == nil {
		t.Fatal("expected non-nil kubeconfig loader")
	}
	raw, err := loader.RawConfig()
	if err != nil {
		t.Fatalf("RawConfig error: %v", err)
	}
	authInfo := raw.AuthInfos["test-user"]
	if authInfo == nil || authInfo.Exec == nil || authInfo.Exec.Command != "credential-plugin" {
		t.Fatalf("expected unknown exec plugin to remain intact for #986 fallback, got %+v", authInfo)
	}
}

func TestNewWithOptionsGKE401EvictsCachedTokenIntegration(t *testing.T) {
	isolateIntegrationEnv(t)

	var oauthCalls atomic.Int32
	oauthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := oauthCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"access_token":"gke-401-token-%d","token_type":"Bearer","expires_in":300}`, n)
	}))
	defer oauthSrv.Close()

	var mu sync.Mutex
	var authHeaders []string
	var apiCalls atomic.Int32
	apiSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		authHeaders = append(authHeaders, r.Header.Get("Authorization"))
		mu.Unlock()
		n := apiCalls.Add(1)
		if n == 2 {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, `{"kind":"Status","apiVersion":"v1","status":"Failure","code":401}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"major":"1","minor":"31","gitVersion":"v1.31.0"}`)
	}))
	defer apiSrv.Close()

	kubeconfig := []byte(fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- name: gke-cluster
  cluster:
    server: %s
    insecure-skip-tls-verify: true
contexts:
- name: gke-context
  context:
    cluster: gke-cluster
    user: gke-user
current-context: gke-context
users:
- name: gke-user
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: gke-gcloud-auth-plugin
`, apiSrv.URL))

	saJSON := buildTestGCPServiceAccountJSON(t, oauthSrv.URL)
	client, err := NewWithOptions(kubeconfig, WithCloudCredentials(cloudauth.Credentials{
		GCP: &cloudauth.GCPCredentials{ServiceAccountJSON: saJSON},
	}))
	if err != nil {
		t.Fatalf("NewWithOptions GKE error: %v", err)
	}

	// Call 1 mints gke-401-token-1 and succeeds.
	if _, err := client.KubeClient.Discovery().ServerVersion(); err != nil {
		t.Fatalf("first ServerVersion() error: %v", err)
	}
	// Call 2 uses cached gke-401-token-1, hits 401, and triggers ResetTokenOlderThan eviction.
	if _, err := client.KubeClient.Discovery().ServerVersion(); err == nil {
		t.Fatal("expected second ServerVersion() call to fail with HTTP 401")
	}
	// Call 3 must mint fresh gke-401-token-2 and succeed.
	ver, err := client.KubeClient.Discovery().ServerVersion()
	if err != nil {
		t.Fatalf("third ServerVersion() error after 401 eviction: %v", err)
	}
	if ver.GitVersion != "v1.31.0" {
		t.Fatalf("GitVersion = %q, want v1.31.0", ver.GitVersion)
	}
	if got := oauthCalls.Load(); got != 2 {
		t.Fatalf("oauthCalls = %d, want 2 after 401 eviction", got)
	}

	mu.Lock()
	gotHeaders := append([]string(nil), authHeaders...)
	mu.Unlock()
	if len(gotHeaders) != 3 ||
		gotHeaders[0] != "Bearer gke-401-token-1" ||
		gotHeaders[1] != "Bearer gke-401-token-1" ||
		gotHeaders[2] != "Bearer gke-401-token-2" {
		t.Fatalf("unexpected Authorization header progression across 401 eviction: %v", gotHeaders)
	}
}

func TestDetectKubeConfigAndNewRespectDisableEnvVar(t *testing.T) {
	isolateIntegrationEnv(t)
	t.Setenv(cloudauth.DisableEnvVar, "true")

	kubeconfig := []byte(`apiVersion: v1
kind: Config
clusters:
- name: eks-cluster
  cluster:
    server: https://127.0.0.1:6443
    insecure-skip-tls-verify: true
contexts:
- name: eks-context
  context:
    cluster: eks-cluster
    user: eks-user
current-context: eks-context
users:
- name: eks-user
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: aws
      args:
      - eks
      - get-token
      - --cluster-name
      - prod-eks
      - --region
      - us-west-2
`)

	_, loader, err := detectKubeConfig(kubeconfig)
	if err != nil {
		t.Fatalf("detectKubeConfig with %s=true error: %v", cloudauth.DisableEnvVar, err)
	}
	clientCfg, err := loader.ClientConfig()
	if err != nil {
		t.Fatalf("loader.ClientConfig() error: %v", err)
	}
	if clientCfg.ExecProvider == nil || clientCfg.WrapTransport != nil {
		t.Fatalf("expected cloudauth disabled in ClientConfig: ExecProvider=%v WrapTransport=%v",
			clientCfg.ExecProvider != nil, clientCfg.WrapTransport != nil)
	}

	client, err := New(kubeconfig)
	if err != nil {
		t.Fatalf("New() with %s=true error: %v", cloudauth.DisableEnvVar, err)
	}
	if client.RestConfig.ExecProvider == nil || client.RestConfig.WrapTransport != nil {
		t.Fatalf("expected cloudauth disabled in client.RestConfig: ExecProvider=%v WrapTransport=%v",
			client.RestConfig.ExecProvider != nil, client.RestConfig.WrapTransport != nil)
	}
}

func TestDetectKubeConfigEKSMissingCredentialsAndRegionErrors(t *testing.T) {
	t.Run("missing AWS credentials with unavailable exec binary", func(t *testing.T) {
		isolateIntegrationEnv(t)
		kubeconfig := []byte(`apiVersion: v1
kind: Config
clusters:
- name: eks-cluster
  cluster:
    server: https://127.0.0.1:6443
contexts:
- name: eks-context
  context:
    cluster: eks-cluster
    user: eks-user
current-context: eks-context
users:
- name: eks-user
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: /nonexistent/bin/aws
      args:
      - eks
      - get-token
      - --cluster-name
      - prod-eks
      - --region
      - us-west-2
`)
		_, _, err := detectKubeConfig(kubeconfig)
		if err == nil {
			t.Fatal("expected error for missing AWS credentials without exec fallback")
		}
		if code := meshkiterrors.GetCode(err); code != cloudauth.ErrAWSCredentialsCode {
			t.Fatalf("error code = %q, want %q (err=%v)", code, cloudauth.ErrAWSCredentialsCode, err)
		}
	})

	t.Run("missing EKS region returns ErrEKSRegionUnresolvedCode", func(t *testing.T) {
		isolateIntegrationEnv(t)
		kubeconfig := []byte(`apiVersion: v1
kind: Config
clusters:
- name: eks-cluster
  cluster:
    server: https://127.0.0.1:6443
contexts:
- name: eks-context
  context:
    cluster: eks-cluster
    user: eks-user
current-context: eks-context
users:
- name: eks-user
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: /nonexistent/bin/aws
      args:
      - eks
      - get-token
      - --cluster-name
      - prod-eks
`)
		_, err := NewWithOptions(kubeconfig, WithCloudCredentials(cloudauth.Credentials{
			AWS: &cloudauth.AWSCredentials{
				AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
				SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			},
		}))
		if err == nil {
			t.Fatal("expected error for unresolved EKS region")
		}
		if code := meshkiterrors.GetCode(err); code != cloudauth.ErrEKSRegionUnresolvedCode {
			t.Fatalf("error code = %q, want %q (err=%v)", code, cloudauth.ErrEKSRegionUnresolvedCode, err)
		}
	})
}
