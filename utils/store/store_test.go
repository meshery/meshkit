package store

import (
	"sync"
	"testing"
)

func TestNewGenericThreadSafeStore(t *testing.T) {
	s := NewGenericThreadSafeStore[string]()
	if s == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestSetAndGet(t *testing.T) {
	s := NewGenericThreadSafeStore[int]()
	s.Set("key1", 42)

	val, ok := s.Get("key1")
	if !ok {
		t.Fatal("expected key1 to exist")
	}
	if val != 42 {
		t.Errorf("expected 42, got %d", val)
	}
}

func TestGet_NonExistentKey(t *testing.T) {
	s := NewGenericThreadSafeStore[string]()

	_, ok := s.Get("missing")
	if ok {
		t.Error("expected ok=false for missing key")
	}
}

func TestDelete(t *testing.T) {
	s := NewGenericThreadSafeStore[string]()
	s.Set("key1", "value1")
	s.Delete("key1")

	_, ok := s.Get("key1")
	if ok {
		t.Error("expected key to be deleted")
	}
}

func TestDelete_NonExistentKey(t *testing.T) {
	s := NewGenericThreadSafeStore[string]()
	// Should not panic
	s.Delete("missing")
}

func TestGetAllPairs(t *testing.T) {
	s := NewGenericThreadSafeStore[int]()
	s.Set("a", 1)
	s.Set("b", 2)
	s.Set("c", 3)

	pairs := s.GetAllPairs()
	if len(pairs) != 3 {
		t.Fatalf("expected 3 pairs, got %d", len(pairs))
	}
	if pairs["a"] != 1 || pairs["b"] != 2 || pairs["c"] != 3 {
		t.Errorf("unexpected pairs: %v", pairs)
	}
}

func TestGetAllPairs_ReturnsACopy(t *testing.T) {
	s := NewGenericThreadSafeStore[string]()
	s.Set("key", "value")

	pairs := s.GetAllPairs()
	pairs["key"] = "modified"

	val, _ := s.Get("key")
	if val != "value" {
		t.Error("modifying returned map should not affect store")
	}
}

func TestOverwrite(t *testing.T) {
	s := NewGenericThreadSafeStore[string]()
	s.Set("key", "first")
	s.Set("key", "second")

	val, _ := s.Get("key")
	if val != "second" {
		t.Errorf("expected second, got %s", val)
	}
}

func TestConcurrentAccess(t *testing.T) {
	s := NewGenericThreadSafeStore[int]()
	var wg sync.WaitGroup
	n := 100

	// Concurrent writes
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s.Set("key", i)
		}(i)
	}
	wg.Wait()

	// Should have exactly one value (last write wins, but value is valid)
	_, ok := s.Get("key")
	if !ok {
		t.Error("expected key to exist after concurrent writes")
	}

	// Concurrent reads and writes
	for i := 0; i < n; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			s.Set("concurrent", i)
		}(i)
		go func() {
			defer wg.Done()
			s.Get("concurrent")
		}()
	}
	wg.Wait()
}
