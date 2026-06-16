package provider

import (
	"testing"
)

func TestNewInMem(t *testing.T) {
	h, err := NewInMem(Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestInMem_SetAndGetKey(t *testing.T) {
	h, err := NewInMem(Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	h.SetKey("name", "meshery")
	got := h.GetKey("name")
	if got != "meshery" {
		t.Errorf("expected meshery, got %s", got)
	}
}

func TestInMem_GetKey_Missing(t *testing.T) {
	h, err := NewInMem(Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := h.GetKey("missing")
	if got != "" {
		t.Errorf("expected empty string for missing key, got %s", got)
	}
}

func TestInMem_OverwriteKey(t *testing.T) {
	h, err := NewInMem(Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	h.SetKey("key", "first")
	h.SetKey("key", "second")
	got := h.GetKey("key")
	if got != "second" {
		t.Errorf("expected second, got %s", got)
	}
}

func TestInMem_SetAndGetObject(t *testing.T) {
	h, err := NewInMem(Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	input := map[string]string{"hello": "world"}
	err = h.SetObject("obj", input)
	if err != nil {
		t.Fatalf("unexpected error setting object: %v", err)
	}

	var result map[string]interface{}
	err = h.GetObject("obj", &result)
	if err != nil {
		t.Fatalf("unexpected error getting object: %v", err)
	}
	if result["hello"] != "world" {
		t.Errorf("expected hello=world, got %v", result["hello"])
	}
}

func TestInMem_MultipleKeys(t *testing.T) {
	h, err := NewInMem(Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	h.SetKey("a", "1")
	h.SetKey("b", "2")
	h.SetKey("c", "3")

	if h.GetKey("a") != "1" || h.GetKey("b") != "2" || h.GetKey("c") != "3" {
		t.Error("failed to store/retrieve multiple keys")
	}
}
