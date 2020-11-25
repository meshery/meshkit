package utils

import (
	"bytes"
	"testing"
)

func TestSimplestTemplate(t *testing.T) {
	template := []byte("{{.namespace}}")
	result, err := Merge(template, map[string]string{"namespace": "meshery"})
	if err != nil {
		t.Errorf("err = %v; want 'nil'", err)
	}
	if bytes.Compare(result, []byte("meshery")) != 0 {
		t.Errorf("result = %s; want 'meshery'", result)
	}
}

func TestEmptyTemplate(t *testing.T) {
	template := []byte("")
	result, err := Merge(template, map[string]string{"namespace": "meshery"})
	if err != nil {
		t.Errorf("err = %v; want 'nil'", err)
	}
	if bytes.Compare(result, []byte("")) != 0 {
		t.Errorf("result = %s; want '' (empty string)", result)
	}
}

func TestEmptyDataMap(t *testing.T) {
	template := []byte("{{.namespace}}")
	result, err := Merge(template, map[string]string{})
	if err != nil {
		t.Errorf("err = %v; want 'nil'", err)
	}
	if bytes.Compare(result, []byte("")) != 0 {
		t.Errorf("result = %s; want '' (empty string)", result)
	}
}

func TestMultilineTemplate(t *testing.T) {
	template := `Layer5
{{.project}} is
best`
	expected := `Layer5
Meshery is
best`
	result, err := Merge([]byte(template), map[string]string{"project": "Meshery"})
	if err != nil {
		t.Errorf("err = %v; want 'nil'", err)
	}
	if bytes.Compare(result, []byte(expected)) != 0 {
		t.Errorf("result = %s; want '%s'", result, expected)
	}
}
