package kubernetes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProcessConfig(t *testing.T) {
	config := []byte(`apiVersion: v1
clusters:
- cluster:
    server: https://127.0.0.1:6443
  name: test
contexts:
- context:
    cluster: test
    user: test
  name: test
current-context: test
kind: Config
users:
- name: test
  user:
    token: test-token
`)

	got, output, err := ProcessConfig(config, "")
	if err != nil {
		t.Fatalf("ProcessConfig() error = %v", err)
	}
	if got.CurrentContext != "test" {
		t.Errorf("CurrentContext = %q, want %q", got.CurrentContext, "test")
	}
	if len(output) == 0 {
		t.Error("ProcessConfig() returned empty config data")
	}
}

func TestProcessConfigInvalidInput(t *testing.T) {
	if _, _, err := ProcessConfig([]byte("not a kubeconfig"), ""); err == nil {
		t.Error("ProcessConfig() error = nil, want error for invalid input")
	}
}

func TestDetectKubeConfigProvidedConfig(t *testing.T) {
	config, err := DetectKubeConfig([]byte(`apiVersion: v1
clusters:
- cluster:
    server: https://127.0.0.1:6443
  name: test
contexts:
- context:
    cluster: test
    user: test
  name: test
current-context: test
kind: Config
users:
- name: test
  user:
    token: test-token
`))
	if err != nil {
		t.Fatalf("DetectKubeConfig() error = %v", err)
	}
	if config.Host != "https://127.0.0.1:6443" {
		t.Errorf("DetectKubeConfig() Host = %q", config.Host)
	}
}

func TestDetectKubeConfigReturnsExplicitKubeconfigError(t *testing.T) {
	const invalidProxyURL = "://invalid-proxy"
	kubeconfig := []byte(`apiVersion: v1
kind: Config
clusters:
- name: test-cluster
  cluster:
    server: https://cluster.example.com
    proxy-url: "://invalid-proxy"
contexts:
- name: test-context
  context:
    cluster: test-cluster
    user: test-user
current-context: test-context
users:
- name: test-user
  user:
    token: test-token
`)

	kubeconfigPath := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(kubeconfigPath, kubeconfig, 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", kubeconfigPath, err)
	}
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("KUBERNETES_SERVICE_PORT", "")
	t.Setenv("KUBECONFIG", kubeconfigPath)

	_, _, err := detectKubeConfig(nil)
	if err == nil {
		t.Fatal("detectKubeConfig() error = nil, want invalid explicit kubeconfig error")
	}
	if !strings.Contains(err.Error(), invalidProxyURL) {
		t.Fatalf("detectKubeConfig() error = %q, want error for explicit proxy %q", err, invalidProxyURL)
	}
}
