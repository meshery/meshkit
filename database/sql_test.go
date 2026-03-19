package database

import (
	"encoding/json"
	"testing"
)

func TestMap_Interface(t *testing.T) {
	m := Map{"key": "value"}
	iface := m.Interface()
	result, ok := iface.(map[string]interface{})
	if !ok {
		t.Fatal("expected map[string]interface{}")
	}
	if result["key"] != "value" {
		t.Errorf("expected key=value, got %v", result["key"])
	}
}

func TestMap_Scan_Bytes(t *testing.T) {
	m := &Map{}
	err := m.Scan([]byte(`{"name":"test","count":42}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if (*m)["name"] != "test" {
		t.Errorf("expected name=test, got %v", (*m)["name"])
	}
}

func TestMap_Scan_String(t *testing.T) {
	m := &Map{}
	err := m.Scan(`{"name":"test"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if (*m)["name"] != "test" {
		t.Errorf("expected name=test, got %v", (*m)["name"])
	}
}

func TestMap_Scan_InvalidType(t *testing.T) {
	m := &Map{}
	err := m.Scan(12345)
	if err == nil {
		t.Fatal("expected error for invalid scan type")
	}
}

func TestMap_Scan_InvalidJSON(t *testing.T) {
	m := &Map{}
	err := m.Scan([]byte(`{invalid json}`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestMap_Value(t *testing.T) {
	m := Map{"hello": "world"}
	val, err := m.Value()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	str, ok := val.(string)
	if !ok {
		t.Fatal("expected string value")
	}
	var check map[string]interface{}
	if err := json.Unmarshal([]byte(str), &check); err != nil {
		t.Fatalf("value is not valid JSON: %v", err)
	}
	if check["hello"] != "world" {
		t.Errorf("expected hello=world, got %v", check["hello"])
	}
}

func TestMap_UnmarshalJSON(t *testing.T) {
	var m Map
	err := m.UnmarshalJSON([]byte(`{"key":"value","num":123}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m["key"] != "value" {
		t.Errorf("expected key=value, got %v", m["key"])
	}
	if m["num"] != float64(123) {
		t.Errorf("expected num=123, got %v", m["num"])
	}
}

func TestMap_UnmarshalJSON_Invalid(t *testing.T) {
	var m Map
	err := m.UnmarshalJSON([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestMap_UnmarshalJSON_IntoExisting(t *testing.T) {
	m := Map{"existing": "data"}
	err := m.UnmarshalJSON([]byte(`{"new":"data"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m["existing"] != "data" {
		t.Error("expected existing data to be preserved")
	}
	if m["new"] != "data" {
		t.Error("expected new data to be added")
	}
}

func TestMap_UnmarshalText(t *testing.T) {
	m := Map{}
	err := m.UnmarshalText([]byte(`{"key":"value"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m["key"] != "value" {
		t.Errorf("expected key=value, got %v", m["key"])
	}
}

func TestMap_UnmarshalText_Invalid(t *testing.T) {
	m := Map{}
	err := m.UnmarshalText([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid text")
	}
}

func TestMap_Value_Empty(t *testing.T) {
	m := Map{}
	val, err := m.Value()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "{}" {
		t.Errorf("expected {}, got %v", val)
	}
}

func TestMap_Scan_EmptyJSON(t *testing.T) {
	m := &Map{}
	err := m.Scan([]byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(*m) != 0 {
		t.Errorf("expected empty map, got %v", *m)
	}
}
