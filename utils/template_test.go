package utils

import (
	"bytes"
	"testing"
)

func TestSimplestTemplate(t *testing.T) {
	template := []byte("{{.namespace}}")
	result, err := MergeToTemplate(template, map[string]string{"namespace": "meshery"})
	if err != nil {
		t.Errorf("err = %v; want 'nil'", err)
	}
	if !bytes.Equal(result, []byte("meshery")) {
		t.Errorf("result = %s; want 'meshery'", result)
	}
}

func TestEmptyTemplate(t *testing.T) {
	template := []byte("")
	result, err := MergeToTemplate(template, map[string]string{"namespace": "meshery"})
	if err != nil {
		t.Errorf("err = %v; want 'nil'", err)
	}
	if !bytes.Equal(result, []byte("")) {
		t.Errorf("result = %s; want '' (empty string)", result)
	}
}

func TestEmptyDataMap(t *testing.T) {
	template := []byte("{{.namespace}}")
	result, err := MergeToTemplate(template, map[string]string{})
	if err != nil {
		t.Errorf("err = %v; want 'nil'", err)
	}
	if !bytes.Equal(result, []byte("")) {
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
	result, err := MergeToTemplate([]byte(template), map[string]string{"project": "Meshery"})
	if err != nil {
		t.Errorf("err = %v; want 'nil'", err)
	}
	if !bytes.Equal(result, []byte(expected)) {
		t.Errorf("result = %s; want '%s'", result, expected)
	}
}
