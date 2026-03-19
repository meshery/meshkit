package encoding

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestUnmarshal_JSON(t *testing.T) {
	input := []byte(`{"name":"test","count":42}`)
	var result map[string]interface{}
	err := Unmarshal(input, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["name"] != "test" {
		t.Errorf("expected name=test, got %v", result["name"])
	}
	if result["count"] != float64(42) {
		t.Errorf("expected count=42, got %v", result["count"])
	}
}

func TestUnmarshal_YAML(t *testing.T) {
	input := []byte("name: test\ncount: 42\n")
	var result map[string]interface{}
	err := Unmarshal(input, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["name"] != "test" {
		t.Errorf("expected name=test, got %v", result["name"])
	}
	if result["count"] != 42 {
		t.Errorf("expected count=42, got %v", result["count"])
	}
}

func TestUnmarshal_InvalidData(t *testing.T) {
	input := []byte(`{{{invalid}}}`)
	var result map[string]interface{}
	err := Unmarshal(input, &result)
	if err == nil {
		t.Fatal("expected error for invalid data, got nil")
	}
}

func TestUnmarshal_YAMLNode(t *testing.T) {
	input := []byte("key: value\n")
	var node yaml.Node
	err := Unmarshal(input, &node)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.Kind != yaml.DocumentNode {
		t.Errorf("expected DocumentNode, got %v", node.Kind)
	}
}

func TestUnmarshal_EmptyJSON(t *testing.T) {
	input := []byte(`{}`)
	var result map[string]interface{}
	err := Unmarshal(input, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}

func TestUnmarshal_JSONArray(t *testing.T) {
	input := []byte(`[1,2,3]`)
	var result []int
	err := Unmarshal(input, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 3 || result[0] != 1 || result[2] != 3 {
		t.Errorf("expected [1,2,3], got %v", result)
	}
}

func TestMarshal_JSON(t *testing.T) {
	input := map[string]string{"hello": "world"}
	result, err := Marshal(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should produce indented JSON
	expected := "{\n  \"hello\": \"world\"\n}"
	if string(result) != expected {
		t.Errorf("expected %q, got %q", expected, string(result))
	}
}

func TestMarshal_Struct(t *testing.T) {
	type sample struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	input := sample{Name: "test", Count: 5}
	result, err := Marshal(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) == 0 {
		t.Error("expected non-empty result")
	}
}

func TestToYaml_FromJSON(t *testing.T) {
	input := []byte(`{"name":"test","items":["a","b"]}`)
	result, err := ToYaml(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) == 0 {
		t.Error("expected non-empty YAML output")
	}
	// Verify it's valid YAML
	var check map[string]interface{}
	if err := yaml.Unmarshal(result, &check); err != nil {
		t.Fatalf("output is not valid YAML: %v", err)
	}
	if check["name"] != "test" {
		t.Errorf("expected name=test, got %v", check["name"])
	}
}

func TestToYaml_FromYAML(t *testing.T) {
	input := []byte("name: test\ncount: 42\n")
	result, err := ToYaml(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var check map[string]interface{}
	if err := yaml.Unmarshal(result, &check); err != nil {
		t.Fatalf("output is not valid YAML: %v", err)
	}
	if check["name"] != "test" {
		t.Errorf("expected name=test, got %v", check["name"])
	}
}

func TestToYaml_EmptyContent(t *testing.T) {
	input := []byte(``)
	_, err := ToYaml(input)
	if err == nil {
		t.Fatal("expected error for empty content, got nil")
	}
}

func TestUnmarshal_NestedJSON(t *testing.T) {
	input := []byte(`{"outer":{"inner":"value"}}`)
	var result map[string]interface{}
	err := Unmarshal(input, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	outer, ok := result["outer"].(map[string]interface{})
	if !ok {
		t.Fatal("expected outer to be a map")
	}
	if outer["inner"] != "value" {
		t.Errorf("expected inner=value, got %v", outer["inner"])
	}
}
