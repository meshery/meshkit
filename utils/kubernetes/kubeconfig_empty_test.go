package kubernetes

import (
	"os"
	"path/filepath"
	"testing"
)

func writeKubeconfig(t *testing.T, content string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
	t.Setenv("KUBECONFIG", path)
}

// An empty kubeconfig is a common state (a file that was created and never filled
// in, or truncated). yaml.Unmarshal reports no error for it and leaves a pointer
// target nil, so GetKubeConfig returned (nil, nil) and GetCurrentContext then
// dereferenced it.
func TestGetCurrentContext_EmptyKubeconfigDoesNotPanic(t *testing.T) {
	cases := map[string]string{
		"zero bytes":         "",
		"only whitespace":    "\n  \n",
		"only a comment":     "# nothing here yet\n",
		"yaml null":          "null\n",
		"document separator": "---\n",
	}

	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			writeKubeconfig(t, content)

			ctx, err := (&Client{}).GetCurrentContext()
			if err != nil {
				t.Fatalf("an empty kubeconfig should read as having no current context, got error: %v", err)
			}
			if ctx != "" {
				t.Errorf("current context = %q, want empty", ctx)
			}
		})
	}
}

func TestGetKubeConfig_EmptyKubeconfigIsNotNil(t *testing.T) {
	writeKubeconfig(t, "")

	config, err := (&Client{}).GetKubeConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config == nil {
		t.Fatal("GetKubeConfig returned (nil, nil); callers cannot tell a nil config from a usable one")
	}
}

func TestGetCurrentContext_ReadsTheCurrentContext(t *testing.T) {
	writeKubeconfig(t, "apiVersion: v1\nkind: Config\ncurrent-context: dev\n")

	ctx, err := (&Client{}).GetCurrentContext()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx != "dev" {
		t.Errorf("current context = %q, want %q", ctx, "dev")
	}
}

func TestGetKubeConfig_InvalidYAMLStillReportsAnError(t *testing.T) {
	writeKubeconfig(t, "current-context: [unterminated\n")

	if _, err := (&Client{}).GetKubeConfig(); err == nil {
		t.Fatal("malformed YAML must still be reported as an error")
	}
}
