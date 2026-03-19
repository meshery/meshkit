package utils

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetBool(t *testing.T) {
	tests := []struct {
		input   string
		want    bool
		wantErr bool
	}{
		{"true", true, false},
		{"false", false, false},
		{"1", true, false},
		{"0", false, false},
		{"TRUE", true, false},
		{"invalid", false, true},
		{"", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := GetBool(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetBool(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("GetBool(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestStrConcat(t *testing.T) {
	tests := []struct {
		input []string
		want  string
	}{
		{[]string{"a", "b", "c"}, "abc"},
		{[]string{}, ""},
		{[]string{"hello"}, "hello"},
		{[]string{"hello", " ", "world"}, "hello world"},
	}
	for _, tt := range tests {
		got := StrConcat(tt.input...)
		if got != tt.want {
			t.Errorf("StrConcat(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestContains(t *testing.T) {
	if !Contains([]string{"a", "b", "c"}, "b") {
		t.Error("expected Contains to find 'b'")
	}
	if Contains([]string{"a", "b", "c"}, "d") {
		t.Error("expected Contains not to find 'd'")
	}
	if Contains([]string{}, "a") {
		t.Error("expected Contains to return false for empty slice")
	}
	if !Contains([]int{1, 2, 3}, 2) {
		t.Error("expected Contains to find 2")
	}
}

func TestFindIndexInSlice(t *testing.T) {
	tests := []struct {
		key  string
		col  []string
		want int
	}{
		{"b", []string{"a", "b", "c"}, 1},
		{"a", []string{"a", "b", "c"}, 0},
		{"c", []string{"a", "b", "c"}, 2},
		{"d", []string{"a", "b", "c"}, -1},
		{"a", []string{}, -1},
	}
	for _, tt := range tests {
		got := FindIndexInSlice(tt.key, tt.col)
		if got != tt.want {
			t.Errorf("FindIndexInSlice(%q, %v) = %d, want %d", tt.key, tt.col, got, tt.want)
		}
	}
}

func TestFormatName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello-world"},
		{"already-formatted", "already-formatted"},
		{"UPPER CASE", "upper-case"},
		{"no spaces", "no-spaces"},
		{"", ""},
	}
	for _, tt := range tests {
		got := FormatName(tt.input)
		if got != tt.want {
			t.Errorf("FormatName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestReplaceSpacesAndConvertToLowercase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "helloworld"},
		{"ABC DEF", "abcdef"},
		{"no spaces", "nospaces"},
		{"", ""},
	}
	for _, tt := range tests {
		got := ReplaceSpacesAndConvertToLowercase(tt.input)
		if got != tt.want {
			t.Errorf("ReplaceSpacesAndConvertToLowercase(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestReplaceSpacesWithHyphenAndConvertToLowercase(t *testing.T) {
	got := ReplaceSpacesWithHyphenAndConvertToLowercase("Hello World")
	if got != "hello-world" {
		t.Errorf("expected hello-world, got %s", got)
	}
}

func TestExtractDomainFromURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://www.example.com/path", "example.com"},
		{"https://sub.domain.example.com", "example.com"},
		{"https://github.com/user/repo", "github.com"},
	}
	for _, tt := range tests {
		got := ExtractDomainFromURL(tt.input)
		if got != tt.want {
			t.Errorf("ExtractDomainFromURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIsInterfaceNil(t *testing.T) {
	if !IsInterfaceNil(nil) {
		t.Error("expected true for nil")
	}
	if IsInterfaceNil("hello") {
		t.Error("expected false for non-nil string")
	}
	if IsInterfaceNil(42) {
		t.Error("expected false for non-nil int")
	}
}

func TestCast(t *testing.T) {
	val, err := Cast[string]("hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "hello" {
		t.Errorf("expected hello, got %s", val)
	}

	_, err = Cast[string](42)
	if err == nil {
		t.Error("expected error for invalid cast")
	}

	_, err = Cast[string](nil)
	if err == nil {
		t.Error("expected error for nil cast")
	}
}

func TestMarshalAndUnmarshal(t *testing.T) {
	type testStruct struct {
		Name string `json:"name"`
	}
	input := testStruct{Name: "test"}
	result, err := MarshalAndUnmarshal[testStruct, testStruct](input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Name != "test" {
		t.Errorf("expected name=test, got %s", result.Name)
	}
}

func TestCombineErrors(t *testing.T) {
	if CombineErrors(nil, ", ") != nil {
		t.Error("expected nil for empty slice")
	}
	if CombineErrors([]error{}, ", ") != nil {
		t.Error("expected nil for empty slice")
	}

	errs := []error{errors.New("first"), errors.New("second")}
	combined := CombineErrors(errs, ", ")
	if combined == nil {
		t.Fatal("expected non-nil error")
	}
	if combined.Error() != "first, second" {
		t.Errorf("expected 'first, second', got %q", combined.Error())
	}
}

func TestMergeMaps(t *testing.T) {
	m1 := map[string]interface{}{"a": 1}
	m2 := map[string]interface{}{"b": 2}
	result := MergeMaps(m1, m2)
	if result["a"] != 1 || result["b"] != 2 {
		t.Errorf("unexpected result: %v", result)
	}

	// Nil mergeInto
	result = MergeMaps(nil, m2)
	if result["b"] != 2 {
		t.Errorf("expected b=2, got %v", result)
	}

	// Overwrite
	m3 := map[string]interface{}{"a": 1}
	m4 := map[string]interface{}{"a": 99}
	result = MergeMaps(m3, m4)
	if result["a"] != 99 {
		t.Errorf("expected a=99, got %v", result["a"])
	}
}

func TestIsSchemaEmpty(t *testing.T) {
	if IsSchemaEmpty("") {
		t.Error("expected false for empty string")
	}
	if IsSchemaEmpty(`{}`) {
		t.Error("expected false for schema without properties")
	}
	if !IsSchemaEmpty(`{"properties":{"name":{"type":"string"}}}`) {
		t.Error("expected true for schema with properties")
	}
}

func TestMarshal(t *testing.T) {
	result, err := Marshal(map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var check map[string]string
	if err := json.Unmarshal([]byte(result), &check); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if check["key"] != "value" {
		t.Errorf("expected key=value, got %v", check["key"])
	}
}

func TestYAMLToJSON(t *testing.T) {
	yaml := []byte("name: test\ncount: 42\n")
	result, err := YAMLToJSON(yaml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var check map[string]interface{}
	if err := json.Unmarshal(result, &check); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if check["name"] != "test" {
		t.Errorf("expected name=test, got %v", check["name"])
	}
}

func TestYAMLToJSON_Invalid(t *testing.T) {
	_, err := YAMLToJSON([]byte(`{{{invalid`))
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestConvertMapInterfaceMapString(t *testing.T) {
	input := map[interface{}]interface{}{
		"key":   "value",
		123:     "number-key",
		"nested": map[interface{}]interface{}{
			"inner": "data",
		},
	}
	result := ConvertMapInterfaceMapString(input)
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("expected map[string]interface{}")
	}
	if m["key"] != "value" {
		t.Errorf("expected key=value, got %v", m["key"])
	}
	if m["123"] != "number-key" {
		t.Errorf("expected 123=number-key, got %v", m["123"])
	}
}

func TestConvertToJSONCompatible(t *testing.T) {
	input := map[interface{}]interface{}{
		"name": "test",
	}
	result := ConvertToJSONCompatible(input)
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("expected map[string]interface{}")
	}
	if m["name"] != "test" {
		t.Errorf("expected name=test, got %v", m["name"])
	}
}

func TestIsClosed(t *testing.T) {
	ch := make(chan int, 1)
	if IsClosed(ch) {
		t.Error("expected false for open channel")
	}
	close(ch)
	if !IsClosed(ch) {
		t.Error("expected true for closed channel")
	}
}

func TestWriteToFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	err := WriteToFile(path, "hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("expected 'hello world', got %q", string(data))
	}
}

func TestCreateFile(t *testing.T) {
	dir := t.TempDir()
	err := CreateFile([]byte("content"), "test.txt", dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "test.txt"))
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != "content" {
		t.Errorf("expected 'content', got %q", string(data))
	}
}

func TestCreateDirectory(t *testing.T) {
	dir := t.TempDir()
	newDir := filepath.Join(dir, "sub", "dir")
	err := CreateDirectory(newDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	info, err := os.Stat(newDir)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected a directory")
	}
}

func TestReadLocalFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte("local content"), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := ReadLocalFile("file://" + path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "local content" {
		t.Errorf("expected 'local content', got %q", result)
	}
}

func TestReadFileSource_InvalidProtocol(t *testing.T) {
	_, err := ReadFileSource("ftp://example.com/file")
	if err == nil {
		t.Error("expected error for invalid protocol")
	}
}

func TestWriteYamlToFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "output.yaml")
	data := map[string]string{"name": "test"}
	err := WriteYamlToFile(path, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if len(content) == 0 {
		t.Error("expected non-empty file")
	}
}

func TestWriteJSONToFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "output.json")
	data := map[string]string{"key": "value"}
	err := WriteJSONToFile(path, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	var check map[string]string
	if err := json.Unmarshal(content, &check); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if check["key"] != "value" {
		t.Errorf("expected key=value, got %v", check["key"])
	}
}

func TestIsDirectoryNonEmpty(t *testing.T) {
	dir := t.TempDir()
	// Empty dir
	if IsDirectoryNonEmpty(dir) {
		t.Error("expected false for empty directory")
	}
	// Non-existent dir
	if IsDirectoryNonEmpty(filepath.Join(dir, "nonexistent")) {
		t.Error("expected false for non-existent directory")
	}
	// Non-empty dir
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	if !IsDirectoryNonEmpty(dir) {
		t.Error("expected true for non-empty directory")
	}
}

func TestGetRandomAlphabetsOfDigit(t *testing.T) {
	result := GetRandomAlphabetsOfDigit(10)
	if len(result) != 10 {
		t.Errorf("expected length 10, got %d", len(result))
	}
	for _, c := range result {
		if c < 'a' || c > 'z' {
			t.Errorf("expected lowercase letter, got %c", c)
		}
	}
	// Zero length
	result = GetRandomAlphabetsOfDigit(0)
	if result != "" {
		t.Errorf("expected empty string for length 0, got %q", result)
	}
}

func TestTruncateErrorMessage(t *testing.T) {
	if TruncateErrorMessage(nil, 5) != nil {
		t.Error("expected nil for nil error")
	}

	short := errors.New("short error")
	result := TruncateErrorMessage(short, 10)
	if result.Error() != "short error" {
		t.Errorf("expected unchanged error, got %q", result.Error())
	}

	long := errors.New("this is a very long error message with many words")
	result = TruncateErrorMessage(long, 3)
	if result.Error() != "this is a..." {
		t.Errorf("expected truncated error, got %q", result.Error())
	}
}

func TestHandleStatusReason(t *testing.T) {
	reasons := []v1.StatusReason{
		v1.StatusReasonUnauthorized,
		v1.StatusReasonForbidden,
		v1.StatusReasonNotFound,
		v1.StatusReasonAlreadyExists,
		v1.StatusReasonConflict,
		v1.StatusReasonInvalid,
		v1.StatusReasonBadRequest,
		v1.StatusReasonInternalError,
		v1.StatusReasonServiceUnavailable,
		v1.StatusReason("unknown"),
	}

	for _, reason := range reasons {
		pc, rem := handleStatusReason(reason)
		if pc == "" {
			t.Errorf("expected non-empty probable cause for %s", reason)
		}
		if rem == "" {
			t.Errorf("expected non-empty remedy for %s", reason)
		}
	}
}

func TestHandleStatusCause(t *testing.T) {
	causes := []v1.StatusCause{
		{Type: v1.CauseTypeFieldValueNotFound, Field: "name"},
		{Type: v1.CauseTypeFieldValueRequired, Field: "spec"},
		{Type: v1.CauseTypeFieldValueDuplicate, Field: "id"},
		{Type: v1.CauseTypeFieldValueInvalid, Field: "port", Message: "invalid port"},
		{Type: v1.CauseTypeUnexpectedServerResponse},
		{Type: v1.CauseType("unknown"), Field: "field", Message: "msg"},
	}

	for _, cause := range causes {
		pc, rem := handleStatusCause(cause, "Pod")
		if pc == "" {
			t.Errorf("expected non-empty probable cause for %s", cause.Type)
		}
		if rem == "" {
			t.Errorf("expected non-empty remedy for %s", cause.Type)
		}
	}
}

func TestIsTarGz(t *testing.T) {
	dir := t.TempDir()
	// Create a real gzip file
	tgzPath := filepath.Join(dir, "test.tar.gz")
	var buf bytes.Buffer
	if err := Compress(dir, &buf); err != nil {
		t.Fatalf("failed to compress directory for testing: %v", err)
	}
	if err := os.WriteFile(tgzPath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("failed to write test tar.gz file: %v", err)
	}
	if !IsTarGz(tgzPath) {
		t.Error("expected true for actual tar.gz file")
	}

	// Non-gzip file should return false
	txtPath := filepath.Join(dir, "plain.txt")
	os.WriteFile(txtPath, []byte("not a tarball"), 0644)
	if IsTarGz(txtPath) {
		t.Error("expected false for plain text file")
	}

	// Non-existent file
	if IsTarGz("/nonexistent/file") {
		t.Error("expected false for non-existent file")
	}
}

func TestIsZip(t *testing.T) {
	dir := t.TempDir()
	// Non-zip file should return false
	txtPath := filepath.Join(dir, "plain.txt")
	os.WriteFile(txtPath, []byte("not a zip"), 0644)
	if IsZip(txtPath) {
		t.Error("expected false for plain text file")
	}

	// Non-existent file
	if IsZip("/nonexistent/file") {
		t.Error("expected false for non-existent file")
	}
}

func TestIsYaml(t *testing.T) {
	dir := t.TempDir()
	// YAML content is detected as text/plain. The readData function reads 512 bytes,
	// so the file needs enough text content to be detected as text/plain.
	yamlContent := strings.Repeat("name: test\ncount: 42\ndescription: some description value here\n", 20)
	yamlPath := filepath.Join(dir, "test.yaml")
	os.WriteFile(yamlPath, []byte(yamlContent), 0644)
	if !IsYaml(yamlPath) {
		t.Error("expected true for YAML file")
	}

	// Non-existent file
	if IsYaml("/nonexistent/file") {
		t.Error("expected false for non-existent file")
	}
}

func TestGetAllVersionDirsSortedDesc(t *testing.T) {
	dir := t.TempDir()
	// Create version directories
	versions := []string{"v1.0.0", "v2.1.0", "v1.5.0", "v3.0.0"}
	for _, v := range versions {
		if err := os.Mkdir(filepath.Join(dir, v), 0755); err != nil {
			t.Fatal(err)
		}
	}

	result, err := GetAllVersionDirsSortedDesc(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 4 {
		t.Fatalf("expected 4 versions, got %d", len(result))
	}
	// First should be highest version
	if filepath.Base(result[0]) != "v3.0.0" {
		t.Errorf("expected v3.0.0 first, got %s", filepath.Base(result[0]))
	}
}

func TestGetAllVersionDirsSortedDesc_Empty(t *testing.T) {
	dir := t.TempDir()
	_, err := GetAllVersionDirsSortedDesc(dir)
	if err == nil {
		t.Error("expected error for empty directory")
	}
}

func TestGetAllVersionDirsSortedDesc_NonExistent(t *testing.T) {
	_, err := GetAllVersionDirsSortedDesc("/nonexistent/path")
	if err == nil {
		t.Error("expected error for non-existent directory")
	}
}

func TestFindEntityType(t *testing.T) {
	tests := []struct {
		name      string
		schema    string
		wantErr   bool
	}{
		{"relationship", `{"schemaVersion":"relationships.meshery.io/v1"}`, false},
		{"component", `{"schemaVersion":"components.meshery.io/v1"}`, false},
		{"model", `{"schemaVersion":"models.meshery.io/v1"}`, false},
		{"policy", `{"schemaVersion":"policies.meshery.io/v1"}`, false},
		{"invalid schema", `{"schemaVersion":"unknown.io/v1"}`, true},
		{"missing schema", `{"name":"test"}`, true},
		{"invalid JSON", `{invalid}`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := FindEntityType([]byte(tt.schema))
			if (err != nil) != tt.wantErr {
				t.Errorf("FindEntityType() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewTarWriter(t *testing.T) {
	tw := NewTarWriter()
	if tw == nil {
		t.Fatal("expected non-nil TarWriter")
	}
	if tw.Writer == nil {
		t.Error("expected non-nil Writer")
	}
	if tw.Buffer == nil {
		t.Error("expected non-nil Buffer")
	}
}

func TestTarWriter_Compress(t *testing.T) {
	tw := NewTarWriter()
	err := tw.Compress("test.txt", []byte("hello world"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tw.Close()
	if tw.Buffer.Len() == 0 {
		t.Error("expected non-empty buffer after compression")
	}
}

func TestTarWriter_CompressMultiple(t *testing.T) {
	tw := NewTarWriter()
	for i := 0; i < 3; i++ {
		err := tw.Compress(fmt.Sprintf("file%d.txt", i), []byte(fmt.Sprintf("content %d", i)))
		if err != nil {
			t.Fatalf("unexpected error on file %d: %v", i, err)
		}
	}
	tw.Close()
	if tw.Buffer.Len() == 0 {
		t.Error("expected non-empty buffer")
	}
}
