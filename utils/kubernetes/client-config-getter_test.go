package kubernetes

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const execCredentialHelperEnv = "MESHKIT_EXEC_CREDENTIAL_HELPER"

func testExecKubeConfig() []byte {
	return []byte(`apiVersion: v1
kind: Config
clusters:
- name: test-cluster
  cluster:
    server: https://cluster.example.com
    insecure-skip-tls-verify: true
contexts:
- name: test-context
  context:
    cluster: test-cluster
    user: test-user
current-context: test-context
users:
- name: test-user
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1
      command: credential-plugin
      interactiveMode: Never
`)
}

func renewableExecKubeConfig(t *testing.T, serverURL, stateFile string) []byte {
	t.Helper()

	const (
		clusterName = "test-cluster"
		contextName = "test-context"
		userName    = "test-user"
	)
	config := clientcmdapi.NewConfig()
	config.Clusters[clusterName] = &clientcmdapi.Cluster{
		Server:                serverURL,
		InsecureSkipTLSVerify: true,
	}
	config.AuthInfos[userName] = &clientcmdapi.AuthInfo{
		Exec: &clientcmdapi.ExecConfig{
			APIVersion: "client.authentication.k8s.io/v1",
			Command:    os.Args[0],
			Args:       []string{"-test.run=TestExecCredentialHelperProcess"},
			Env: []clientcmdapi.ExecEnvVar{
				{Name: execCredentialHelperEnv, Value: "1"},
				{Name: "MESHKIT_EXEC_CREDENTIAL_STATE_FILE", Value: stateFile},
			},
			InteractiveMode: clientcmdapi.NeverExecInteractiveMode,
		},
	}
	config.Contexts[contextName] = &clientcmdapi.Context{
		Cluster:  clusterName,
		AuthInfo: userName,
	}
	config.CurrentContext = contextName

	data, err := clientcmd.Write(*config)
	if err != nil {
		t.Fatalf("clientcmd.Write() error = %v", err)
	}
	return data
}

func TestExecCredentialHelperProcess(t *testing.T) {
	if os.Getenv(execCredentialHelperEnv) != "1" {
		return
	}

	stateFile := os.Getenv("MESHKIT_EXEC_CREDENTIAL_STATE_FILE")
	token := "expired-token"
	state := "1"
	if _, err := os.Stat(stateFile); err == nil {
		token = "fresh-token"
		state = "2"
	}
	if err := os.WriteFile(stateFile, []byte(state), 0o600); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "write exec credential state: %v", err)
		os.Exit(1)
	}

	_, _ = fmt.Fprintf(os.Stdout, `{"apiVersion":"client.authentication.k8s.io/v1","kind":"ExecCredential","status":{"token":%q}}`, token)
	os.Exit(0)
}

func TestNewRetainsKubeConfigLoader(t *testing.T) {
	client, err := New(testExecKubeConfig())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	loader := client.getRESTClientGetter().ToRawKubeConfigLoader()
	if loader == nil {
		t.Fatal("New() did not retain the kubeconfig loader")
	}

	rawConfig, err := loader.RawConfig()
	if err != nil {
		t.Fatalf("RawConfig() error = %v", err)
	}
	authInfo := rawConfig.AuthInfos["test-user"]
	if authInfo == nil || authInfo.Exec == nil {
		t.Fatal("retained kubeconfig loader lost exec authentication")
	}
	if authInfo.Exec.Command != "credential-plugin" {
		t.Fatalf("exec command = %q, want %q", authInfo.Exec.Command, "credential-plugin")
	}
}

func TestClientConfigRESTClientGetterPreservesLoader(t *testing.T) {
	_, loader, err := detectKubeConfig(testExecKubeConfig())
	if err != nil {
		t.Fatalf("detectKubeConfig() error = %v", err)
	}

	getter := newClientConfigRESTClientGetter(loader)
	if getter.ToRawKubeConfigLoader() != loader {
		t.Fatal("RESTClientGetter did not return the retained kubeconfig loader")
	}

	config, err := getter.ToRESTConfig()
	if err != nil {
		t.Fatalf("ToRESTConfig() error = %v", err)
	}
	if config.ExecProvider == nil || config.ExecProvider.Command != "credential-plugin" {
		t.Fatal("RESTClientGetter did not preserve exec authentication")
	}
	if config.QPS != 50 || config.Burst != 100 {
		t.Fatalf("REST config rate limits = (%v, %d), want (50, 100)", config.QPS, config.Burst)
	}
}

func TestNewInitializesRESTClientGetter(t *testing.T) {
	client, err := New(testExecKubeConfig())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	getter := client.getRESTClientGetter()
	if getter == nil {
		t.Fatal("New() did not initialize a RESTClientGetter")
	}

	config, err := getter.ToRESTConfig()
	if err != nil {
		t.Fatalf("ToRESTConfig() error = %v", err)
	}
	if config.ExecProvider == nil || config.ExecProvider.Command != "credential-plugin" {
		t.Fatal("initialized RESTClientGetter lost exec authentication")
	}
	if client.RestConfig.QPS != 50 || client.RestConfig.Burst != 100 {
		t.Fatalf("client rate limits = (%v, %d), want (50, 100)", client.RestConfig.QPS, client.RestConfig.Burst)
	}
}

func TestRESTClientGetterFallsBackForDirectClient(t *testing.T) {
	client := &Client{RestConfig: rest.Config{
		Host:        "https://cluster.example.com",
		BearerToken: "test-token",
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
		QPS:   1,
		Burst: 2,
	}}

	getter := client.getRESTClientGetter()
	config, err := getter.ToRESTConfig()
	if err != nil {
		t.Fatalf("ToRESTConfig() error = %v", err)
	}
	if config.Host != client.RestConfig.Host || config.BearerToken != client.RestConfig.BearerToken {
		t.Fatalf("REST config = (%q, %q), want (%q, %q)", config.Host, config.BearerToken, client.RestConfig.Host, client.RestConfig.BearerToken)
	}
	if config.QPS != 50 || config.Burst != 100 {
		t.Fatalf("REST config rate limits = (%v, %d), want (50, 100)", config.QPS, config.Burst)
	}

	namespace, explicit, err := getter.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		t.Fatalf("Namespace() error = %v", err)
	}
	if namespace != "default" || explicit {
		t.Fatalf("Namespace() = (%q, %v), want (%q, false)", namespace, explicit, "default")
	}

	rawConfig, err := getter.ToRawKubeConfigLoader().RawConfig()
	if err != nil {
		t.Fatalf("RawConfig() error = %v", err)
	}
	cluster := rawConfig.Clusters["meshkit-connection"]
	if cluster == nil || cluster.Server != client.RestConfig.Host {
		t.Fatalf("raw cluster = %#v, want server %q", cluster, client.RestConfig.Host)
	}
}

func TestRESTConfigGetterReturnsIndependentCopies(t *testing.T) {
	getter := newRESTConfigRESTClientGetter(&rest.Config{
		Host:        "https://cluster.example.com",
		BearerToken: "test-token",
	})

	first, err := getter.ToRESTConfig()
	if err != nil {
		t.Fatalf("first ToRESTConfig() error = %v", err)
	}
	first.Host = "https://different.example.com"
	first.BearerToken = "different-token"

	second, err := getter.ToRESTConfig()
	if err != nil {
		t.Fatalf("second ToRESTConfig() error = %v", err)
	}
	if second.Host != "https://cluster.example.com" || second.BearerToken != "test-token" {
		t.Fatalf("second REST config = (%q, %q), want original values", second.Host, second.BearerToken)
	}
}

func TestRESTClientGetterExecutesAndRenewsCredentials(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fresh-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"gitVersion":"v1.34.0"}`))
	}))
	defer server.Close()

	stateFile := filepath.Join(t.TempDir(), "exec-credential-state")
	client, err := New(renewableExecKubeConfig(t, server.URL, stateFile))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	discoveryClient, err := client.getRESTClientGetter().ToDiscoveryClient()
	if err != nil {
		t.Fatalf("ToDiscoveryClient() error = %v", err)
	}
	if _, err := discoveryClient.ServerVersion(); err == nil {
		t.Fatal("first ServerVersion() unexpectedly succeeded with the rejected credential")
	}
	if _, err := discoveryClient.ServerVersion(); err != nil {
		t.Fatalf("second ServerVersion() error after credential renewal = %v", err)
	}

	state, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", stateFile, err)
	}
	if string(state) != "2" {
		t.Fatalf("exec credential helper state = %q, want %q", state, "2")
	}
}
