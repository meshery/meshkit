package kubernetes

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetKubeConfigAndCurrentContext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	contents := []byte("apiVersion: v1\nkind: Config\ncurrent-context: local\n")
	if err := os.WriteFile(path, contents, 0600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
	t.Setenv("KUBECONFIG", path)

	client := &Client{}
	config, err := client.GetKubeConfig()
	if err != nil {
		t.Fatalf("GetKubeConfig() error = %v", err)
	}
	if config.CurrentContext != "local" {
		t.Errorf("CurrentContext = %q, want local", config.CurrentContext)
	}
	context, err := client.GetCurrentContext()
	if err != nil {
		t.Fatalf("GetCurrentContext() error = %v", err)
	}
	if context != "local" {
		t.Errorf("GetCurrentContext() = %q, want local", context)
	}
}

func TestGetKubeConfigReadError(t *testing.T) {
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "missing"))
	if _, err := (&Client{}).GetKubeConfig(); err == nil {
		t.Error("GetKubeConfig() error = nil, want missing file error")
	}
}
