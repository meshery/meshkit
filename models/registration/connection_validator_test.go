package registration

import (
	"testing"
)

func TestValidateConnection(t *testing.T) {
	// Test valid connection JSON
	validJSON := []byte(`{
		"schemaVersion": "v1beta1/connection",
		"id": "00000000-00000000-00000000-00000000",
		"name": "test-kubernetes-cluster",
		"type": "kubernetes",
		"sub_type": "cluster",
		"kind": "kubernetes",
		"status": "connected"
	}`)

	conn, err := ValidateConnection(validJSON)
	if err != nil {
		t.Fatalf("Expected no error for valid connection, got: %v", err)
	}

	if conn.Name != "test-kubernetes-cluster" {
		t.Errorf("Expected name 'test-kubernetes-cluster', got: %s", conn.Name)
	}

	if conn.Type != "kubernetes" {
		t.Errorf("Expected type 'kubernetes', got: %s", conn.Type)
	}

	if conn.Kind != "kubernetes" {
		t.Errorf("Expected kind 'kubernetes', got: %s", conn.Kind)
	}

	// Test invalid connection (missing required fields)
	invalidJSON := []byte(`{
		"schemaVersion": "v1beta1/connection",
		"id": "00000000-00000000-00000000-00000000"
	}`)

	_, err = ValidateConnection(invalidJSON)
	if err == nil {
		t.Fatal("Expected error for invalid connection, got none")
	}

	// Test malformed JSON
	malformedJSON := []byte(`{ invalid json }`)
	_, err = ValidateConnection(malformedJSON)
	if err == nil {
		t.Fatal("Expected error for malformed JSON, got none")
	}
}
