package provider

import (
	"os"
	"path/filepath"
	"testing"
)

// newTestViper builds a Viper provider over a YAML file with the given content.
func newTestViper(t *testing.T, content string) *Viper {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	h, err := NewViper(Options{FilePath: dir, FileType: "yaml", FileName: "config"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	v, ok := h.(*Viper)
	if !ok {
		t.Fatalf("expected *Viper, got %T", h)
	}
	return v
}

func TestViper_SetAndGetKey(t *testing.T) {
	v := newTestViper(t, "name: meshery\n")
	if got := v.GetKey("name"); got != "meshery" {
		t.Errorf("expected meshery, got %q", got)
	}
	v.SetKey("name", "kanvas")
	if got := v.GetKey("name"); got != "kanvas" {
		t.Errorf("expected kanvas after SetKey, got %q", got)
	}
}

// GetKey is documented as retrieving a string value for a key, and the in-memory
// provider returns "" for a key that is not set. The Viper provider must do the
// same instead of panicking on the missing (nil) value.
func TestViper_GetKey_Missing(t *testing.T) {
	v := newTestViper(t, "name: meshery\n")

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("GetKey panicked on a missing key: %v", r)
		}
	}()
	if got := v.GetKey("missing"); got != "" {
		t.Errorf("expected empty string for missing key, got %q", got)
	}
}

// A config file can hold numbers and booleans. Asking for one as a string must
// give its text, not panic on the type assertion.
func TestViper_GetKey_NonStringValue(t *testing.T) {
	v := newTestViper(t, "port: 8080\ndebug: true\n")

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("GetKey panicked on a non-string value: %v", r)
		}
	}()
	if got := v.GetKey("port"); got != "8080" {
		t.Errorf("expected 8080 for an integer value, got %q", got)
	}
	if got := v.GetKey("debug"); got != "true" {
		t.Errorf("expected true for a boolean value, got %q", got)
	}
}
