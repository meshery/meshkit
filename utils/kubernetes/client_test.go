package kubernetes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
