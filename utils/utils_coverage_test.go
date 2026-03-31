package utils

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	stdjson "encoding/json"
	"encoding/pem"
	"encoding/xml"
	stderrors "errors"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/cuecontext"
	cueformat "cuelang.org/go/cue/format"
	meshkiterrors "github.com/meshery/meshkit/errors"
	mklogger "github.com/meshery/meshkit/logger"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
	kubeerror "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type staticReadCloser struct {
	reader   io.Reader
	closeErr error
}

func (s *staticReadCloser) Read(p []byte) (int, error) {
	return s.reader.Read(p)
}

func (s *staticReadCloser) Close() error {
	return s.closeErr
}

type failingReadCloser struct {
	readErr  error
	closeErr error
}

func (f *failingReadCloser) Read([]byte) (int, error) {
	return 0, f.readErr
}

func (f *failingReadCloser) Close() error {
	return f.closeErr
}

type errCloser struct {
	err error
}

func (e errCloser) Close() error {
	return e.err
}

type failAfterWriter struct {
	failAfter int
	writes    int
	err       error
}

func (f *failAfterWriter) Write(p []byte) (int, error) {
	if f.writes >= f.failAfter {
		return 0, f.err
	}
	f.writes++
	return len(p), nil
}

type failingFileWriter struct {
	writeErr       error
	writeStringErr error
	closeErr       error
}

func (f *failingFileWriter) Write(p []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return len(p), nil
}

func (f *failingFileWriter) WriteString(s string) (int, error) {
	if f.writeStringErr != nil {
		return 0, f.writeStringErr
	}
	return len(s), nil
}

func (f *failingFileWriter) Close() error {
	return f.closeErr
}

type failingXMLTokenEncoder struct {
	encodeErr error
	flushErr  error
}

func (f *failingXMLTokenEncoder) EncodeToken(xml.Token) error {
	return f.encodeErr
}

func (f *failingXMLTokenEncoder) Flush() error {
	return f.flushErr
}

type failOnNonEmptyWrite struct {
	err         error
	seenNonZero bool
}

func (f *failOnNonEmptyWrite) Write(p []byte) (int, error) {
	if len(p) > 0 {
		if !f.seenNonZero {
			f.seenNonZero = true
			return len(p), nil
		}
		return 0, f.err
	}
	return 0, nil
}

type unsupportedTypeJSON struct{}

func (*unsupportedTypeJSON) UnmarshalJSON([]byte) error {
	return &stdjson.UnsupportedTypeError{Type: reflect.TypeOf(func() {})}
}

type unsupportedValueJSON struct{}

func (*unsupportedValueJSON) UnmarshalJSON([]byte) error {
	return &stdjson.UnsupportedValueError{Value: reflect.ValueOf(math.Inf(1)), Str: "infinity"}
}

type genericJSONError struct{}

func (*genericJSONError) UnmarshalJSON([]byte) error {
	return stderrors.New("custom unmarshal failure")
}

type marshalYAMLError struct{}

func (marshalYAMLError) MarshalYAML() (interface{}, error) {
	return nil, stderrors.New("marshal yaml error")
}

type panicYAMLError struct{}

func (panicYAMLError) MarshalYAML() (interface{}, error) {
	panic(stderrors.New("panic yaml error"))
}

func withDefaultTransport(t *testing.T, rt http.RoundTripper) {
	t.Helper()
	oldTransport := http.DefaultTransport
	http.DefaultTransport = rt
	t.Cleanup(func() {
		http.DefaultTransport = oldTransport
	})
}

func httpResponse(status int, body io.ReadCloser) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       body,
	}
}

func withWorkingDir(t *testing.T, dir string) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to change working directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})
}

func createServiceAccountCredential(t *testing.T) string {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	cred := map[string]string{
		"type":         "service_account",
		"project_id":   "meshkit-test",
		"private_key":  string(privateKeyPEM),
		"client_email": "meshkit-test@example.iam.gserviceaccount.com",
		"token_uri":    "https://oauth2.googleapis.com/token",
	}

	raw, err := stdjson.Marshal(cred)
	if err != nil {
		t.Fatalf("failed to marshal credential JSON: %v", err)
	}

	return base64.StdEncoding.EncodeToString(raw)
}

type tarEntry struct {
	name     string
	body     string
	typeflag byte
}

func writeTarGz(t *testing.T, archivePath string, entries []tarEntry) {
	t.Helper()

	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("failed to create tar.gz archive: %v", err)
	}
	defer func() { _ = file.Close() }()

	gzw := gzip.NewWriter(file)
	defer func() { _ = gzw.Close() }()

	tw := tar.NewWriter(gzw)
	defer func() { _ = tw.Close() }()

	for _, entry := range entries {
		header := &tar.Header{
			Name:     entry.name,
			Mode:     0o644,
			Size:     int64(len(entry.body)),
			Typeflag: entry.typeflag,
		}
		if entry.typeflag == tar.TypeDir {
			header.Mode = 0o755
			header.Size = 0
		}
		if entry.typeflag == 0 {
			header.Typeflag = tar.TypeReg
		}

		if err := tw.WriteHeader(header); err != nil {
			t.Fatalf("failed to write tar header for %s: %v", entry.name, err)
		}
		if header.Typeflag == tar.TypeReg {
			if _, err := tw.Write([]byte(entry.body)); err != nil {
				t.Fatalf("failed to write tar body for %s: %v", entry.name, err)
			}
		}
	}
}

type zipEntry struct {
	name  string
	body  string
	isDir bool
}

func writeZip(t *testing.T, archivePath string, entries []zipEntry) {
	t.Helper()

	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("failed to create zip archive: %v", err)
	}
	defer func() { _ = file.Close() }()

	zw := zip.NewWriter(file)
	defer func() { _ = zw.Close() }()

	for _, entry := range entries {
		header := &zip.FileHeader{
			Name:   entry.name,
			Method: zip.Deflate,
		}
		if entry.isDir {
			header.Name = strings.TrimSuffix(entry.name, "/") + "/"
			header.SetMode(0o755)
		} else {
			header.SetMode(0o644)
		}

		writer, err := zw.CreateHeader(header)
		if err != nil {
			t.Fatalf("failed to create zip header for %s: %v", entry.name, err)
		}
		if !entry.isDir {
			if _, err := writer.Write([]byte(entry.body)); err != nil {
				t.Fatalf("failed to write zip body for %s: %v", entry.name, err)
			}
		}
	}
}

func TestUnmarshalCoverage(t *testing.T) {
	type typeTarget struct {
		Age int `json:"age"`
	}

	testCases := []struct {
		name   string
		input  string
		target interface{}
		code   string
	}{
		{"syntax error", "{", &map[string]interface{}{}, ErrUnmarshalSyntaxCode},
		{"type error", `{"age":"oops"}`, &typeTarget{}, ErrUnmarshalTypeCode},
		{"invalid target", `{}`, nil, ErrUnmarshalInvalidCode},
		{"unsupported type", `"value"`, &unsupportedTypeJSON{}, ErrUnmarshalUnsupportedTypeCode},
		{"unsupported value", `"value"`, &unsupportedValueJSON{}, ErrUnmarshalUnsupportedValueCode},
		{"generic error", `"value"`, &genericJSONError{}, ErrUnmarshalCode},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := Unmarshal(tc.input, tc.target)
			if err == nil {
				t.Fatal("expected Unmarshal to return an error")
			}
			if got := meshkiterrors.GetCode(err); got != tc.code {
				t.Fatalf("expected error code %q, got %q", tc.code, got)
			}
		})
	}
}

func TestMarshalAndMarshalAndUnmarshalCoverage(t *testing.T) {
	_, err := Marshal(map[string]interface{}{"bad": func() {}})
	if err == nil {
		t.Fatal("expected Marshal to fail for unsupported values")
	}
	if got := meshkiterrors.GetCode(err); got != ErrMarshalCode {
		t.Fatalf("expected error code %q, got %q", ErrMarshalCode, got)
	}

	_, err = MarshalAndUnmarshal[map[string]interface{}, map[string]interface{}](map[string]interface{}{"bad": func() {}})
	if err == nil {
		t.Fatal("expected MarshalAndUnmarshal to fail when marshaling fails")
	}
	if got := meshkiterrors.GetCode(err); got != ErrMarshalCode {
		t.Fatalf("expected error code %q, got %q", ErrMarshalCode, got)
	}

	_, err = MarshalAndUnmarshal[string, genericJSONError]("meshkit")
	if err == nil {
		t.Fatal("expected MarshalAndUnmarshal to fail when unmarshaling fails")
	}
	if got := meshkiterrors.GetCode(err); got != ErrUnmarshalCode {
		t.Fatalf("expected error code %q, got %q", ErrUnmarshalCode, got)
	}
}

func TestRuntimeAndIdentityHelpers(t *testing.T) {
	location := Filepath()
	if !strings.Contains(location, "utils.go") || !strings.Contains(location, "line:") {
		t.Fatalf("unexpected Filepath() output: %q", location)
	}

	if home := GetHome(); home == "" {
		t.Fatal("expected GetHome() to return a non-empty path")
	}

	id, err := NewUUID()
	if err != nil {
		t.Fatalf("NewUUID() returned error: %v", err)
	}
	match, err := regexp.MatchString(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`, id)
	if err != nil {
		t.Fatalf("failed to compile UUID regex: %v", err)
	}
	if !match {
		t.Fatalf("expected a UUID-like string, got %q", id)
	}
}

func TestReadFileSourceAndRemoteHelpers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "remote content")
	}))
	defer server.Close()

	result, err := ReadFileSource(server.URL)
	if err != nil {
		t.Fatalf("ReadFileSource() returned error: %v", err)
	}
	if result != "remote content" {
		t.Fatalf("expected remote content, got %q", result)
	}

	localPath := filepath.Join(t.TempDir(), "local.txt")
	if err := os.WriteFile(localPath, []byte("local through source"), 0o644); err != nil {
		t.Fatalf("failed to write local file fixture: %v", err)
	}
	result, err = ReadFileSource("file://" + localPath)
	if err != nil {
		t.Fatalf("ReadFileSource() returned error for file URI: %v", err)
	}
	if result != "local through source" {
		t.Fatalf("expected local file contents, got %q", result)
	}

	_, err = ReadRemoteFile("http://%zz")
	if err == nil {
		t.Fatal("expected ReadRemoteFile to fail for an invalid URL")
	}

	notFoundServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer notFoundServer.Close()

	_, err = ReadRemoteFile(notFoundServer.URL)
	if err == nil {
		t.Fatal("expected ReadRemoteFile to return a not-found error")
	}
	if got := meshkiterrors.GetCode(err); got != ErrRemoteFileNotFoundCode {
		t.Fatalf("expected error code %q, got %q", ErrRemoteFileNotFoundCode, got)
	}

	withDefaultTransport(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		return httpResponse(http.StatusOK, &failingReadCloser{readErr: stderrors.New("copy failed")}), nil
	}))
	_, err = ReadRemoteFile("https://example.com/file")
	if err == nil {
		t.Fatal("expected ReadRemoteFile to fail while copying the response body")
	}
	if got := meshkiterrors.GetCode(err); got != ErrReadingRemoteFileCode {
		t.Fatalf("expected error code %q, got %q", ErrReadingRemoteFileCode, got)
	}

	_, err = ReadLocalFile("file:///definitely/missing")
	if err == nil {
		t.Fatal("expected ReadLocalFile to fail for a missing file")
	}
	if got := meshkiterrors.GetCode(err); got != ErrReadingLocalFileCode {
		t.Fatalf("expected error code %q, got %q", ErrReadingLocalFileCode, got)
	}

	safeClose(errCloser{err: stderrors.New("close failed")})
}

func TestDownloadFileCoverage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "downloaded")
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "download.txt")
	if err := DownloadFile(path, server.URL); err != nil {
		t.Fatalf("DownloadFile() returned error: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}
	if string(content) != "downloaded" {
		t.Fatalf("expected downloaded content, got %q", string(content))
	}

	if err := DownloadFile(filepath.Join(t.TempDir(), "unused"), "http://%zz"); err == nil {
		t.Fatal("expected DownloadFile to fail for an invalid URL")
	}

	statusServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer statusServer.Close()
	if err := DownloadFile(filepath.Join(t.TempDir(), "bad.txt"), statusServer.URL); err == nil {
		t.Fatal("expected DownloadFile to fail on a non-200 response")
	}

	parentFile := filepath.Join(t.TempDir(), "parent")
	if err := os.WriteFile(parentFile, []byte("parent"), 0o644); err != nil {
		t.Fatalf("failed to create parent file: %v", err)
	}
	if err := DownloadFile(filepath.Join(parentFile, "child.txt"), server.URL); err == nil {
		t.Fatal("expected DownloadFile to fail when the output path cannot be created")
	}

	withDefaultTransport(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		return httpResponse(http.StatusOK, &failingReadCloser{readErr: stderrors.New("copy failed")}), nil
	}))
	if err := DownloadFile(filepath.Join(t.TempDir(), "copy-error.txt"), "https://example.com/file"); err == nil {
		t.Fatal("expected DownloadFile to fail while copying the response body")
	}
}

func TestGetLatestReleaseTagsSortedCoverage(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		withDefaultTransport(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
			body := &staticReadCloser{
				reader: strings.NewReader(`/releases/tag/v1.1.0" /releases/tag/v0.9.0" /releases/tag/v1.0.0"`),
			}
			return httpResponse(http.StatusOK, body), nil
		}))

		tags, err := GetLatestReleaseTagsSorted("meshery", "meshkit")
		if err != nil {
			t.Fatalf("GetLatestReleaseTagsSorted() returned error: %v", err)
		}
		expected := []string{"v0.9.0", "v1.0.0", "v1.1.0"}
		if !reflect.DeepEqual(tags, expected) {
			t.Fatalf("expected tags %v, got %v", expected, tags)
		}
	})

	t.Run("http error", func(t *testing.T) {
		withDefaultTransport(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, stderrors.New("request failed")
		}))

		_, err := GetLatestReleaseTagsSorted("meshery", "meshkit")
		if err == nil {
			t.Fatal("expected GetLatestReleaseTagsSorted to fail on request error")
		}
		if got := meshkiterrors.GetCode(err); got != ErrGettingLatestReleaseTagCode {
			t.Fatalf("expected error code %q, got %q", ErrGettingLatestReleaseTagCode, got)
		}
	})

	t.Run("bad status", func(t *testing.T) {
		withDefaultTransport(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
			return httpResponse(http.StatusBadGateway, &staticReadCloser{reader: strings.NewReader("nope")}), nil
		}))

		_, err := GetLatestReleaseTagsSorted("meshery", "meshkit")
		if err == nil {
			t.Fatal("expected GetLatestReleaseTagsSorted to fail on a non-OK status")
		}
		if got := meshkiterrors.GetCode(err); got != ErrGettingLatestReleaseTagCode {
			t.Fatalf("expected error code %q, got %q", ErrGettingLatestReleaseTagCode, got)
		}
	})

	t.Run("body read error", func(t *testing.T) {
		withDefaultTransport(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
			return httpResponse(http.StatusOK, &failingReadCloser{readErr: stderrors.New("body failed")}), nil
		}))

		_, err := GetLatestReleaseTagsSorted("meshery", "meshkit")
		if err == nil {
			t.Fatal("expected GetLatestReleaseTagsSorted to fail while reading the response body")
		}
		if got := meshkiterrors.GetCode(err); got != ErrGettingLatestReleaseTagCode {
			t.Fatalf("expected error code %q, got %q", ErrGettingLatestReleaseTagCode, got)
		}
	})

	t.Run("no releases", func(t *testing.T) {
		withDefaultTransport(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
			return httpResponse(http.StatusOK, &staticReadCloser{reader: strings.NewReader("<html></html>")}), nil
		}))

		_, err := GetLatestReleaseTagsSorted("meshery", "meshkit")
		if err == nil {
			t.Fatal("expected GetLatestReleaseTagsSorted to fail when no releases are present")
		}
		if got := meshkiterrors.GetCode(err); got != ErrGettingLatestReleaseTagCode {
			t.Fatalf("expected error code %q, got %q", ErrGettingLatestReleaseTagCode, got)
		}
	})
}

func TestFileAndDirectoryWriterCoverage(t *testing.T) {
	parentFile := filepath.Join(t.TempDir(), "parent")
	if err := os.WriteFile(parentFile, []byte("parent"), 0o644); err != nil {
		t.Fatalf("failed to create parent file: %v", err)
	}

	if err := CreateFile([]byte("content"), "child.txt", parentFile); err == nil {
		t.Fatal("expected CreateFile to fail when location is not a directory")
	}

	if err := WriteToFile(filepath.Join(parentFile, "child.txt"), "content"); err == nil {
		t.Fatal("expected WriteToFile to fail when parent is not a directory")
	}

	if err := WriteYamlToFile(filepath.Join(parentFile, "child.yaml"), map[string]string{"name": "meshery"}); err == nil {
		t.Fatal("expected WriteYamlToFile to fail when parent is not a directory")
	}

	if err := WriteYamlToFile(filepath.Join(t.TempDir(), "bad.yaml"), map[string]interface{}{"bad": func() {}}); err == nil {
		t.Fatal("expected WriteYamlToFile to fail for unmarshalable data")
	}
	if err := WriteYamlToFile(filepath.Join(t.TempDir(), "bad-marshaler.yaml"), marshalYAMLError{}); err == nil {
		t.Fatal("expected WriteYamlToFile to fail when MarshalYAML returns an error")
	}
	if err := WriteYamlToFile(filepath.Join(t.TempDir(), "panic-error.yaml"), panicYAMLError{}); err == nil {
		t.Fatal("expected WriteYamlToFile to recover panic(error) values")
	}

	if err := WriteJSONToFile(filepath.Join(t.TempDir(), "bad.json"), map[string]interface{}{"bad": func() {}}); err == nil {
		t.Fatal("expected WriteJSONToFile to fail while marshaling")
	}

	if err := WriteJSONToFile(filepath.Join(parentFile, "child.json"), map[string]string{"name": "meshery"}); err == nil {
		t.Fatal("expected WriteJSONToFile to fail when the output path is invalid")
	}

	if err := CreateDirectory(filepath.Join(parentFile, "child")); err == nil {
		t.Fatal("expected CreateDirectory to fail when the parent path is a file")
	}

	originalOpenWritableFile := openWritableFile
	t.Cleanup(func() {
		openWritableFile = originalOpenWritableFile
	})

	openWritableFile = func(string, int, os.FileMode) (fileWriter, error) {
		return &failingFileWriter{writeErr: stderrors.New("write failed")}, nil
	}
	if err := CreateFile([]byte("content"), "child.txt", t.TempDir()); err == nil {
		t.Fatal("expected CreateFile to fail when Write fails")
	}

	openWritableFile = func(string, int, os.FileMode) (fileWriter, error) {
		return &failingFileWriter{closeErr: stderrors.New("close failed")}, nil
	}
	if err := CreateFile([]byte("content"), "child.txt", t.TempDir()); err == nil {
		t.Fatal("expected CreateFile to fail when Close fails")
	}

	originalCreateWritableFile := createWritableFile
	t.Cleanup(func() {
		createWritableFile = originalCreateWritableFile
	})

	createWritableFile = func(string) (fileWriter, error) {
		return &failingFileWriter{writeStringErr: stderrors.New("write string failed")}, nil
	}
	if err := WriteToFile(filepath.Join(t.TempDir(), "write.txt"), "content"); err == nil {
		t.Fatal("expected WriteToFile to fail when WriteString fails")
	}

	createWritableFile = func(string) (fileWriter, error) {
		return &failingFileWriter{closeErr: stderrors.New("close failed")}, nil
	}
	if err := WriteToFile(filepath.Join(t.TempDir(), "write.txt"), "content"); err == nil {
		t.Fatal("expected WriteToFile to fail when Close fails")
	}
}

func TestStringAndMapCoverage(t *testing.T) {
	if got := ExtractDomainFromURL("%"); got != "%" {
		t.Fatalf("expected invalid URL to be returned as-is, got %q", got)
	}

	recursive := RecursiveCastMapStringInterfaceToMapStringInterface(map[string]interface{}{
		"nested": map[interface{}]interface{}{"key": "value"},
	})
	nested, ok := recursive["nested"].(map[string]interface{})
	if !ok || nested["key"] != "value" {
		t.Fatalf("unexpected recursive cast result: %#v", recursive)
	}

	sliceResult := ConvertMapInterfaceMapString([]interface{}{
		map[string]interface{}{"inner": map[interface{}]interface{}{"key": "value"}},
		"plain",
	}).([]interface{})
	inner := sliceResult[0].(map[string]interface{})
	if _, ok := inner["inner"].(map[string]interface{}); !ok {
		t.Fatalf("expected nested slice item to be converted, got %#v", sliceResult)
	}
	if got := ConvertMapInterfaceMapString("plain"); got != "plain" {
		t.Fatalf("expected default conversion to return original value, got %#v", got)
	}

	compatible := ConvertToJSONCompatible([]interface{}{
		map[interface{}]interface{}{"name": "meshery"},
		"plain",
	}).([]interface{})
	if _, ok := compatible[0].(map[string]interface{}); !ok {
		t.Fatalf("expected JSON-compatible map, got %#v", compatible)
	}
	if got := ConvertToJSONCompatible("plain"); got != "plain" {
		t.Fatalf("expected default JSON-compatible conversion to return original value, got %#v", got)
	}

	if got := RecursiveCastMapStringInterfaceToMapStringInterface(nil); got != nil {
		t.Fatalf("expected nil recursive cast result for nil input, got %#v", got)
	}

	dir := t.TempDir()
	svgPath := filepath.Join(dir, "icon.svg")
	if err := os.WriteFile(svgPath, []byte("<svg></svg>"), 0o644); err != nil {
		t.Fatalf("failed to write SVG fixture: %v", err)
	}
	data, err := ReadSVGData(dir, "/icon.svg")
	if err != nil {
		t.Fatalf("ReadSVGData() returned error: %v", err)
	}
	if data != "<svg></svg>" {
		t.Fatalf("expected SVG data, got %q", data)
	}
	if _, err := ReadSVGData(dir, "/missing.svg"); err == nil {
		t.Fatal("expected ReadSVGData to fail for a missing file")
	}
}

func TestCueCoverage(t *testing.T) {
	ctx := cuecontext.New()

	validSchema := ctx.CompileString(`
name: string
age: int
`)
	validValue := ctx.CompileString(`
name: "meshery"
age: 1
`)
	ok, errs := Validate(validSchema, validValue)
	if !ok || len(errs) != 0 {
		t.Fatalf("expected Validate to succeed, got ok=%v errs=%v", ok, errs)
	}

	jsonVal, err := JsonToCue([]byte(`{"name":"meshery"}`))
	if err != nil {
		t.Fatalf("JsonToCue() returned error: %v", err)
	}
	if !jsonVal.Exists() {
		t.Fatal("expected JsonToCue() to return a valid CUE value")
	}
	if _, err := JsonToCue([]byte(`{`)); err == nil {
		t.Fatal("expected JsonToCue to fail for invalid JSON")
	}
	if _, err := JsonToCue([]byte(`{"a":1,"a":2}`)); err == nil {
		t.Fatal("expected JsonToCue to fail when the extracted expression has conflicts")
	}

	yamlVal, err := YamlToCue("name: meshery\n")
	if err != nil {
		t.Fatalf("YamlToCue() returned error: %v", err)
	}
	if !yamlVal.Exists() {
		t.Fatal("expected YamlToCue() to return a valid CUE value")
	}
	if _, err := YamlToCue(": bad"); err == nil {
		t.Fatal("expected YamlToCue to fail for invalid YAML")
	}
	if _, err := YamlToCue("a: 1\na: 2\n"); err == nil {
		t.Fatal("expected YamlToCue to fail when the built CUE value has conflicts")
	}

	schemaVal, err := JsonSchemaToCue(`{"type":"object","properties":{"name":{"type":"string"}}}`)
	if err != nil {
		t.Fatalf("JsonSchemaToCue() returned error: %v", err)
	}
	if !schemaVal.Exists() {
		t.Fatal("expected JsonSchemaToCue() to return a valid CUE value")
	}
	if _, err := JsonSchemaToCue(`{`); err == nil {
		t.Fatal("expected JsonSchemaToCue to fail for invalid JSON")
	}
	if _, err := JsonSchemaToCue(`{"a":1,"a":2}`); err == nil {
		t.Fatal("expected JsonSchemaToCue to fail when the built schema expression has conflicts")
	}
	if _, err := JsonSchemaToCue(`{"type":"object","properties":{"name":{"type":"not-a-type"}}}`); err == nil {
		t.Fatal("expected JsonSchemaToCue to fail for invalid JSON schema definitions")
	}
	originalFormatCueNode := formatCueNode
	t.Cleanup(func() {
		formatCueNode = originalFormatCueNode
	})
	formatCueNode = func(ast.Node, ...cueformat.Option) ([]byte, error) {
		return nil, stderrors.New("format failed")
	}
	if _, err := JsonSchemaToCue(`{"type":"object","properties":{"name":{"type":"string"}}}`); err == nil {
		t.Fatal("expected JsonSchemaToCue to fail when formatting the extracted schema fails")
	}
	formatCueNode = originalFormatCueNode

	originalCompileCue := compileCue
	t.Cleanup(func() {
		compileCue = originalCompileCue
	})
	compileCue = func(ctx *cue.Context, src string) cue.Value {
		return ctx.CompileString("{]")
	}
	if _, err := JsonSchemaToCue(`{"type":"object","properties":{"name":{"type":"string"}}}`); err == nil {
		t.Fatal("expected JsonSchemaToCue to fail when compiling the generated CUE fails")
	}
	compileCue = originalCompileCue

	lookupRoot := ctx.CompileString(`name: "meshery"`)
	lookedUp, err := Lookup(lookupRoot, "name")
	if err != nil {
		t.Fatalf("Lookup() returned error: %v", err)
	}
	if value, err := lookedUp.String(); err != nil || value != "meshery" {
		t.Fatalf("expected Lookup() to resolve the string value, got value=%q err=%v", value, err)
	}

	if _, err := Lookup(lookupRoot, "missing"); err == nil {
		t.Fatal("expected Lookup to fail for a missing path")
	}
	if _, err := Lookup(cue.Value{}, "name"); err == nil {
		t.Fatal("expected Lookup to fail for an invalid root value")
	}
	originalLookupCuePath := lookupCuePath
	t.Cleanup(func() {
		lookupCuePath = originalLookupCuePath
	})
	lookupCuePath = func(cue.Value, string) (cue.Value, error, bool) {
		return cue.Value{}, nil, false
	}
	if _, err := Lookup(lookupRoot, "missing"); err == nil {
		t.Fatal("expected Lookup to fail when the resolved path does not exist")
	}
	lookupCuePath = originalLookupCuePath

	converted, err := ConvertoCue(strings.NewReader("name: meshery\n"))
	if err != nil {
		t.Fatalf("ConvertoCue() returned error: %v", err)
	}
	if !converted.Exists() {
		t.Fatal("expected ConvertoCue() to return a valid CUE value")
	}
	if _, err := ConvertoCue(strings.NewReader(": bad")); err == nil {
		t.Fatal("expected ConvertoCue to fail for invalid YAML")
	}
	if _, err := ConvertoCue(strings.NewReader("a: 1\na: 2\n")); err == nil {
		t.Fatal("expected ConvertoCue to fail when the built CUE value has conflicts")
	}
}

func TestTemplateAndSVGCoverage(t *testing.T) {
	if _, err := MergeToTemplate([]byte("{{"), nil); err == nil {
		t.Fatal("expected MergeToTemplate to fail for an invalid template")
	}
	if _, err := MergeToTemplate([]byte("{{index . 0}}"), map[string]string{"name": "meshery"}); err == nil {
		t.Fatal("expected MergeToTemplate to fail during execution")
	}

	svg := `<svg width="100" xmlns="http://www.w3.org/2000/svg"><g xmlns="http://example.com"><rect/></g></svg>`
	result, err := UpdateSVGString(svg, 240, 320, true)
	if err != nil {
		t.Fatalf("UpdateSVGString() returned error: %v", err)
	}
	if !strings.Contains(result, "<g") || !strings.Contains(result, "<rect") {
		t.Fatalf("expected nested SVG elements to be preserved, got %q", result)
	}
	if !strings.Contains(result, `height="320"`) || !strings.Contains(result, `width="240"`) {
		t.Fatalf("expected width and height to be updated, got %q", result)
	}

	noNamespaceResult, err := UpdateSVGString(`<svg width="100" height="100"><g id="child"><rect/></g></svg>`, 100, 100, true)
	if err != nil {
		t.Fatalf("UpdateSVGString() returned error: %v", err)
	}
	if !strings.Contains(noNamespaceResult, `id="child"`) {
		t.Fatalf("expected child attributes without xmlns to survive unchanged, got %q", noNamespaceResult)
	}

	originalNewXMLTokenEncoder := newXMLTokenEncoder
	t.Cleanup(func() {
		newXMLTokenEncoder = originalNewXMLTokenEncoder
	})
	newXMLTokenEncoder = func(io.Writer) xmlTokenEncoder {
		return &failingXMLTokenEncoder{encodeErr: stderrors.New("encode failed")}
	}
	if _, err := UpdateSVGString(`<svg></svg>`, 100, 100, true); err == nil {
		t.Fatal("expected UpdateSVGString to fail when token encoding fails")
	}

	newXMLTokenEncoder = func(io.Writer) xmlTokenEncoder {
		return &failingXMLTokenEncoder{flushErr: stderrors.New("flush failed")}
	}
	if _, err := UpdateSVGString("", 100, 100, true); err == nil {
		t.Fatal("expected UpdateSVGString to fail when encoder flush fails")
	}
	newXMLTokenEncoder = originalNewXMLTokenEncoder
}

func TestNetworkAndGitCoverage(t *testing.T) {
	hp := &HostPort{Address: "127.0.0.1", Port: 8080}
	if got := hp.String(); got != "127.0.0.1:8080" {
		t.Fatalf("expected HostPort.String() to return %q, got %q", "127.0.0.1:8080", got)
	}
	if !TcpCheck(hp, &MockOptions{DesiredEndpoint: "127.0.0.1:8080"}) {
		t.Fatal("expected TcpCheck to honor the mock endpoint")
	}
	if TcpCheck(hp, &MockOptions{DesiredEndpoint: "127.0.0.1:9090"}) {
		t.Fatal("expected TcpCheck mock mismatch to return false")
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start TCP listener: %v", err)
	}
	defer func() { _ = listener.Close() }()

	done := make(chan struct{})
	go func() {
		conn, _ := listener.Accept()
		if conn != nil {
			_ = conn.Close()
		}
		close(done)
	}()

	addr := listener.Addr().(*net.TCPAddr)
	liveEndpoint := &HostPort{Address: "127.0.0.1", Port: int32(addr.Port)}
	if !TcpCheck(liveEndpoint, nil) {
		t.Fatal("expected TcpCheck to succeed for a live TCP listener")
	}
	<-done
	_ = listener.Close()
	if TcpCheck(liveEndpoint, nil) {
		t.Fatal("expected TcpCheck to fail once the listener has been closed")
	}

	originalVersionPath := gitVersionFilePath
	t.Cleanup(func() {
		gitVersionFilePath = originalVersionPath
	})

	gitVersionFilePath = filepath.Join(t.TempDir(), "missing-version")
	version, head := Git()
	if version != "" || head != "" {
		t.Fatalf("expected empty git version data for a missing file, got version=%q head=%q", version, head)
	}

	gitVersionFilePath = filepath.Join(t.TempDir(), "version")
	if err := os.WriteFile(gitVersionFilePath, []byte("commit123\n\nv1.2.3\n"), 0o644); err != nil {
		t.Fatalf("failed to write git version fixture: %v", err)
	}
	version, head = Git()
	if version != "v1.2.3" || head != "commit123" {
		t.Fatalf("expected parsed git version data, got version=%q head=%q", version, head)
	}

	if err := os.WriteFile(gitVersionFilePath, []byte("commit123\n\"\"\nv1.2.3\n"), 0o644); err != nil {
		t.Fatalf("failed to write git blank-row fixture: %v", err)
	}
	version, head = Git()
	if version != "v1.2.3" || head != "commit123" {
		t.Fatalf("expected parsed git data with blank rows skipped, got version=%q head=%q", version, head)
	}
}

func TestGoogleCoverage(t *testing.T) {
	if _, err := NewSheetSRV(base64.StdEncoding.EncodeToString([]byte("not-json"))); err == nil {
		t.Fatal("expected NewSheetSRV to fail for invalid credentials")
	}

	srv, err := NewSheetSRV(createServiceAccountCredential(t))
	if err != nil {
		t.Fatalf("NewSheetSRV() returned error: %v", err)
	}
	if srv == nil {
		t.Fatal("expected NewSheetSRV() to return a non-nil service")
	}

	originalNewSheetsService := newSheetsService
	t.Cleanup(func() {
		newSheetsService = originalNewSheetsService
	})
	newSheetsService = func(context.Context, ...option.ClientOption) (*sheets.Service, error) {
		return nil, stderrors.New("service failed")
	}
	if _, err := NewSheetSRV(createServiceAccountCredential(t)); err == nil {
		t.Fatal("expected NewSheetSRV to surface service construction errors")
	}
}

func TestArchiveAndExtractionCoverage(t *testing.T) {
	t.Run("tar writer errors", func(t *testing.T) {
		tw := NewTarWriter()
		tw.Close()
		if err := tw.Compress("closed.txt", []byte("data")); err == nil {
			t.Fatal("expected TarWriter.Compress to fail after Close")
		}

		failWriter := &failAfterWriter{failAfter: 1, err: stderrors.New("write failed")}
		custom := &TarWriter{Writer: tar.NewWriter(failWriter), Buffer: bytes.NewBuffer(nil)}
		if err := custom.Compress("broken.txt", []byte("data")); err == nil {
			t.Fatal("expected TarWriter.Compress to fail when writes fail")
		}

		failOnDataWriter := &failAfterWriter{failAfter: 2, err: stderrors.New("data write failed")}
		custom = &TarWriter{Writer: tar.NewWriter(failOnDataWriter), Buffer: bytes.NewBuffer(nil)}
		if err := custom.Compress("broken-data.txt", []byte("data")); err == nil {
			t.Fatal("expected TarWriter.Compress to fail while writing tar data")
		}

		dataOnlyFailWriter := &failOnNonEmptyWrite{err: stderrors.New("data write failed")}
		custom = &TarWriter{Writer: tar.NewWriter(dataOnlyFailWriter), Buffer: bytes.NewBuffer(nil)}
		if err := custom.Compress("broken-data-only.txt", []byte("data")); err == nil {
			t.Fatal("expected TarWriter.Compress to fail when the file payload write fails")
		}
	})

	t.Run("compress", func(t *testing.T) {
		if err := Compress("/definitely/missing", io.Discard); err == nil {
			t.Fatal("expected Compress to fail for a missing source path")
		}

		dir := t.TempDir()
		successDir := filepath.Join(dir, "success")
		if err := os.MkdirAll(successDir, 0o755); err != nil {
			t.Fatalf("failed to create compression success directory: %v", err)
		}
		if err := os.WriteFile(filepath.Join(successDir, "data.txt"), []byte("content"), 0o644); err != nil {
			t.Fatalf("failed to write compression success file: %v", err)
		}
		var successBuffer bytes.Buffer
		if err := Compress(successDir, &successBuffer); err != nil {
			t.Fatalf("expected Compress to succeed for a directory with files, got %v", err)
		}

		filePath := filepath.Join(dir, "secret.txt")
		if err := os.WriteFile(filePath, []byte("secret"), 0o000); err != nil {
			t.Fatalf("failed to write unreadable file: %v", err)
		}
		if err := Compress(dir, io.Discard); err == nil {
			t.Fatal("expected Compress to fail for an unreadable file")
		}

		if err := os.Chmod(filePath, 0o644); err != nil {
			t.Fatalf("failed to restore file permissions: %v", err)
		}
		failWriter := &failAfterWriter{failAfter: 0, err: stderrors.New("writer failed")}
		if err := Compress(dir, failWriter); err == nil {
			t.Fatal("expected Compress to fail when the destination writer fails")
		}

		originalTarHeaderForFile := tarHeaderForFile
		t.Cleanup(func() {
			tarHeaderForFile = originalTarHeaderForFile
		})
		tarHeaderForFile = func(os.FileInfo, string) (*tar.Header, error) {
			return nil, stderrors.New("header failed")
		}
		if err := Compress(successDir, io.Discard); err == nil {
			t.Fatal("expected Compress to fail when tar header generation fails")
		}
		tarHeaderForFile = originalTarHeaderForFile

		originalRelativePath := relativePath
		t.Cleanup(func() {
			relativePath = originalRelativePath
		})
		relativePath = func(string, string) (string, error) {
			return "", stderrors.New("relative path failed")
		}
		if err := Compress(successDir, io.Discard); err == nil {
			t.Fatal("expected Compress to fail when relative path resolution fails")
		}
		relativePath = originalRelativePath

		originalCopyToTarWriter := copyToTarWriter
		t.Cleanup(func() {
			copyToTarWriter = originalCopyToTarWriter
		})
		copyToTarWriter = func(io.Writer, io.Reader) (int64, error) {
			return 0, stderrors.New("tar writer copy failed")
		}
		if err := Compress(successDir, io.Discard); err == nil {
			t.Fatal("expected Compress to fail when copying file data into the tar writer fails")
		}
		copyToTarWriter = originalCopyToTarWriter
	})

	t.Run("extract tar.gz and zip", func(t *testing.T) {
		dir := t.TempDir()

		tarPath := filepath.Join(dir, "archive.tar.gz")
		writeTarGz(t, tarPath, []tarEntry{
			{name: "nested", typeflag: tar.TypeDir},
			{name: "nested/file.txt", body: "hello"},
		})
		tarDest := filepath.Join(dir, "tar-out")
		if err := ExtractTarGz(tarDest, tarPath); err != nil {
			t.Fatalf("ExtractTarGz() returned error: %v", err)
		}
		tarContent, err := os.ReadFile(filepath.Join(tarDest, "nested", "file.txt"))
		if err != nil {
			t.Fatalf("failed to read extracted tar file: %v", err)
		}
		if string(tarContent) != "hello" {
			t.Fatalf("expected extracted tar content, got %q", string(tarContent))
		}

		if err := ExtractTarGz(tarDest, filepath.Join(dir, "missing.tar.gz")); err == nil {
			t.Fatal("expected ExtractTarGz to fail for a missing archive")
		}

		invalidTar := filepath.Join(dir, "invalid.tar.gz")
		if err := os.WriteFile(invalidTar, []byte("not a gzip file"), 0o644); err != nil {
			t.Fatalf("failed to write invalid tar fixture: %v", err)
		}
		if err := ExtractTarGz(tarDest, invalidTar); err == nil {
			t.Fatal("expected ExtractTarGz to fail for invalid gzip data")
		}

		unsupportedTar := filepath.Join(dir, "unsupported.tar.gz")
		writeTarGz(t, unsupportedTar, []tarEntry{
			{name: "link", typeflag: tar.TypeSymlink},
		})
		if err := ExtractTarGz(tarDest, unsupportedTar); err == nil {
			t.Fatal("expected ExtractTarGz to fail for unsupported tar entry types")
		}

		zipPath := filepath.Join(dir, "archive.zip")
		writeZip(t, zipPath, []zipEntry{
			{name: "nested/", isDir: true},
			{name: "nested/file.txt", body: "zip data"},
		})
		zipDest := filepath.Join(dir, "zip-out")
		if err := os.MkdirAll(zipDest, 0o755); err != nil {
			t.Fatalf("failed to create zip destination: %v", err)
		}
		withWorkingDir(t, zipDest)
		if err := ExtractZip(zipDest, zipPath); err != nil {
			t.Fatalf("ExtractZip() returned error: %v", err)
		}
		zipContent, err := os.ReadFile(filepath.Join(zipDest, "nested", "file.txt"))
		if err != nil {
			t.Fatalf("failed to read extracted zip file: %v", err)
		}
		if string(zipContent) != "zip data" {
			t.Fatalf("expected extracted zip content, got %q", string(zipContent))
		}

		invalidZip := filepath.Join(dir, "invalid.zip")
		if err := os.WriteFile(invalidZip, []byte("not a zip file"), 0o644); err != nil {
			t.Fatalf("failed to write invalid zip fixture: %v", err)
		}
		if err := ExtractZip(zipDest, invalidZip); err == nil {
			t.Fatal("expected ExtractZip to fail for invalid zip data")
		}

		conflictingZipDest := filepath.Join(dir, "zip-dest-file")
		if err := os.WriteFile(conflictingZipDest, []byte("file"), 0o644); err != nil {
			t.Fatalf("failed to write conflicting zip destination file: %v", err)
		}
		if err := ExtractZip(conflictingZipDest, zipPath); err == nil {
			t.Fatal("expected ExtractZip to fail when destination paths conflict with existing files")
		}

		zipWithoutDirPath := filepath.Join(dir, "archive-file-only.zip")
		writeZip(t, zipWithoutDirPath, []zipEntry{
			{name: "nested/file.txt", body: "zip data"},
		})
		if err := ExtractZip(conflictingZipDest, zipWithoutDirPath); err == nil {
			t.Fatal("expected ExtractZip to fail when parent directories cannot be created")
		}

		openFileConflictDest := filepath.Join(dir, "zip-openfile-conflict")
		if err := os.MkdirAll(filepath.Join(openFileConflictDest, "conflict"), 0o755); err != nil {
			t.Fatalf("failed to create zip open-file conflict fixture: %v", err)
		}
		zipOpenFileConflict := filepath.Join(dir, "archive-openfile-conflict.zip")
		writeZip(t, zipOpenFileConflict, []zipEntry{
			{name: "conflict", body: "zip data"},
		})
		if err := ExtractZip(openFileConflictDest, zipOpenFileConflict); err == nil {
			t.Fatal("expected ExtractZip to fail when opening the destination file fails")
		}

		conflictingTarDest := filepath.Join(dir, "tar-dest-file")
		if err := os.WriteFile(conflictingTarDest, []byte("file"), 0o644); err != nil {
			t.Fatalf("failed to write conflicting tar destination file: %v", err)
		}
		if err := ExtractTarGz(conflictingTarDest, tarPath); err == nil {
			t.Fatal("expected ExtractTarGz to fail when file extraction cannot create the destination path")
		}

		tarOpenConflictDest := filepath.Join(dir, "tar-openfile-conflict")
		if err := os.MkdirAll(filepath.Join(tarOpenConflictDest, "conflict"), 0o755); err != nil {
			t.Fatalf("failed to create tar open-file conflict fixture: %v", err)
		}
		tarOpenConflictArchive := filepath.Join(dir, "archive-openfile-conflict.tar.gz")
		writeTarGz(t, tarOpenConflictArchive, []tarEntry{
			{name: "conflict", body: "tar data"},
		})
		if err := ExtractTarGz(tarOpenConflictDest, tarOpenConflictArchive); err == nil {
			t.Fatal("expected ExtractTarGz to fail when creating an output file fails")
		}

		originalOpenZipEntry := openZipEntry
		t.Cleanup(func() {
			openZipEntry = originalOpenZipEntry
		})
		openZipEntry = func(*zip.File) (io.ReadCloser, error) {
			return nil, stderrors.New("zip entry failed")
		}
		if err := ExtractZip(zipDest, zipOpenFileConflict); err == nil {
			t.Fatal("expected ExtractZip to fail when opening a zip entry fails")
		}
		openZipEntry = originalOpenZipEntry

		originalCopyZipBuffer := copyZipBuffer
		t.Cleanup(func() {
			copyZipBuffer = originalCopyZipBuffer
		})
		copyZipBuffer = func(io.Writer, io.Reader, []byte) (int64, error) {
			return 0, stderrors.New("zip copy failed")
		}
		if err := ExtractZip(zipDest, zipOpenFileConflict); err == nil {
			t.Fatal("expected ExtractZip to fail when copying zip contents fails")
		}
		copyZipBuffer = originalCopyZipBuffer

		originalNextTarHeader := nextTarHeader
		t.Cleanup(func() {
			nextTarHeader = originalNextTarHeader
		})
		nextTarHeader = func(*tar.Reader) (*tar.Header, error) {
			return nil, stderrors.New("tar next failed")
		}
		if err := ExtractTarGz(tarDest, tarOpenConflictArchive); err == nil {
			t.Fatal("expected ExtractTarGz to fail when reading the next tar header fails")
		}
		nextTarHeader = originalNextTarHeader

		originalCopyTarContent := copyTarContent
		t.Cleanup(func() {
			copyTarContent = originalCopyTarContent
		})
		copyTarContent = func(io.Writer, io.Reader) (int64, error) {
			return 0, stderrors.New("tar copy failed")
		}
		if err := ExtractTarGz(tarDest, tarOpenConflictArchive); err == nil {
			t.Fatal("expected ExtractTarGz to fail when copying tar contents fails")
		}
		copyTarContent = originalCopyTarContent

		if err := ExtractFile(tarPath, filepath.Join(dir, "extract-tar")); err != nil {
			t.Fatalf("ExtractFile() returned error for tar.gz input: %v", err)
		}
		if err := ExtractFile(zipPath, filepath.Join(dir, "extract-zip")); err != nil {
			t.Fatalf("ExtractFile() returned error for zip input: %v", err)
		}
		if err := ExtractFile(filepath.Join(dir, "plain.txt"), filepath.Join(dir, "extract-none")); err == nil {
			t.Fatal("expected ExtractFile to fail for an unsupported archive type")
		}
	})

	t.Run("process content", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "single.txt")
		if err := os.WriteFile(filePath, []byte("single"), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		var seen []string
		if err := ProcessContent(filePath, func(path string) error {
			seen = append(seen, filepath.Base(path))
			return nil
		}); err != nil {
			t.Fatalf("ProcessContent() returned error for file input: %v", err)
		}
		if !reflect.DeepEqual(seen, []string{"single.txt"}) {
			t.Fatalf("unexpected ProcessContent file result: %v", seen)
		}

		dirWithEntries := filepath.Join(dir, "dir")
		if err := os.MkdirAll(dirWithEntries, 0o755); err != nil {
			t.Fatalf("failed to create directory fixture: %v", err)
		}
		for _, name := range []string{"a.txt", "b.txt"} {
			if err := os.WriteFile(filepath.Join(dirWithEntries, name), []byte(name), 0o644); err != nil {
				t.Fatalf("failed to write directory entry %s: %v", name, err)
			}
		}

		seen = nil
		if err := ProcessContent(dirWithEntries, func(path string) error {
			seen = append(seen, filepath.Base(path))
			return nil
		}); err != nil {
			t.Fatalf("ProcessContent() returned error for directory input: %v", err)
		}
		sort.Strings(seen)
		if !reflect.DeepEqual(seen, []string{"a.txt", "b.txt"}) {
			t.Fatalf("unexpected ProcessContent directory result: %v", seen)
		}

		if err := ProcessContent(filepath.Join(dir, "missing"), func(string) error { return nil }); err == nil {
			t.Fatal("expected ProcessContent to fail for a missing path")
		}
		if err := ProcessContent(filePath, func(string) error { return stderrors.New("callback failed") }); err == nil {
			t.Fatal("expected ProcessContent to surface callback errors")
		}
		if err := ProcessContent(dirWithEntries, func(string) error { return stderrors.New("dir callback failed") }); err == nil {
			t.Fatal("expected ProcessContent to surface directory callback errors")
		}

		unreadableDir := filepath.Join(dir, "unreadable")
		if err := os.MkdirAll(unreadableDir, 0o000); err != nil {
			t.Fatalf("failed to create unreadable directory fixture: %v", err)
		}
		defer func() { _ = os.Chmod(unreadableDir, 0o755) }()
		if err := ProcessContent(unreadableDir, func(string) error { return nil }); err == nil {
			t.Fatal("expected ProcessContent to fail when directory contents cannot be read")
		}
	})
}

func TestSortingAndVersionCoverage(t *testing.T) {
	if !isNumeric("12345") {
		t.Fatal("expected isNumeric to return true for numeric strings")
	}
	if isNumeric("12a45") {
		t.Fatal("expected isNumeric to return false for mixed strings")
	}
	if !isNumeric("") {
		t.Fatal("expected isNumeric to return true for the empty string")
	}

	if got := splitVersion("1-2.3"); !reflect.DeepEqual(got, []string{"1", "2", "3"}) {
		t.Fatalf("unexpected splitVersion result: %#v", got)
	}

	if compareVersions("1.0.0", "1.0.0") != 0 {
		t.Fatal("expected compareVersions to treat identical versions as equal")
	}
	if compareVersions("1.0.1", "1.0.0") <= 0 {
		t.Fatal("expected compareVersions to order larger numeric versions higher")
	}
	if compareVersions("1.0", "1.0.1") >= 0 {
		t.Fatal("expected compareVersions to treat longer numeric versions as greater")
	}
	if compareVersions("1.alpha", "1.1") <= 0 {
		t.Fatal("expected compareVersions to order string segments above numeric ones")
	}
	if compareVersions("1.1", "1.alpha") >= 0 {
		t.Fatal("expected compareVersions to order numeric segments below string ones")
	}
	if compareVersions("1.beta", "1.gamma") >= 0 {
		t.Fatal("expected compareVersions to compare string segments lexicographically")
	}
	if compareVersions("1.foo", "1.bar") <= 0 {
		t.Fatal("expected compareVersions to compare non-numeric string segments directly")
	}

	lesser := dottedStrings{"1.0.0.1", "1.0.1"}
	if !lesser.Less(0, 1) {
		t.Fatal("expected dottedStrings.Less to handle different-length versions")
	}
	greater := dottedStrings{"1.0.2", "1.0.1"}
	if greater.Less(0, 1) {
		t.Fatal("expected dottedStrings.Less to return false for greater versions")
	}
	equal := dottedStrings{"1.0.0", "1.0.0"}
	if equal.Less(0, 1) {
		t.Fatal("expected dottedStrings.Less to return false for equal versions")
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "not-a-dir"), []byte("data"), 0o644); err != nil {
		t.Fatalf("failed to write non-directory entry: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "v"), 0o755); err != nil {
		t.Fatalf("failed to create trimmed version directory fixture: %v", err)
	}
	if _, err := GetAllVersionDirsSortedDesc(dir); err == nil {
		t.Fatal("expected GetAllVersionDirsSortedDesc to fail when no valid version directories exist")
	}

	filePath := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(filePath, []byte("data"), 0o644); err != nil {
		t.Fatalf("failed to write file fixture: %v", err)
	}
	if IsDirectoryNonEmpty(filePath) {
		t.Fatal("expected IsDirectoryNonEmpty to return false for file paths")
	}

	unreadableDir := filepath.Join(t.TempDir(), "restricted")
	if err := os.Mkdir(unreadableDir, 0o000); err != nil {
		t.Fatalf("failed to create restricted directory: %v", err)
	}
	defer func() { _ = os.Chmod(unreadableDir, 0o755) }()
	if IsDirectoryNonEmpty(unreadableDir) {
		t.Fatal("expected IsDirectoryNonEmpty to return false when directory contents cannot be read")
	}
}

func TestKubernetesAndTimingCoverage(t *testing.T) {
	if IsErrKubeStatusErr(stderrors.New("plain error")) {
		t.Fatal("expected IsErrKubeStatusErr to return false for plain errors")
	}

	statusErr := &kubeerror.StatusError{
		ErrStatus: v1.Status{
			Status:  v1.StatusFailure,
			Message: "pod is invalid",
			Reason:  v1.StatusReasonBadRequest,
			Code:    http.StatusBadRequest,
			Details: &v1.StatusDetails{
				Kind: "Pod",
				Causes: []v1.StatusCause{
					{Type: v1.CauseTypeFieldValueRequired, Field: "spec.name", Message: "name is required"},
				},
			},
		},
	}
	if !IsErrKubeStatusErr(statusErr) {
		t.Fatal("expected IsErrKubeStatusErr to detect StatusError values")
	}

	for _, reason := range []v1.StatusReason{
		v1.StatusReasonGone,
		v1.StatusReasonServerTimeout,
		v1.StatusReasonTimeout,
		v1.StatusReasonTooManyRequests,
		v1.StatusReasonMethodNotAllowed,
		v1.StatusReasonExpired,
	} {
		if probableCause, remedy := handleStatusReason(reason); probableCause == "" || remedy == "" {
			t.Fatalf("expected handleStatusReason to describe %s", reason)
		}
	}

	short, long, probable, remedy := ParseKubeStatusErr(nil)
	if len(short) != 0 || len(long) != 0 || len(probable) != 0 || len(remedy) != 0 {
		t.Fatal("expected ParseKubeStatusErr(nil) to return empty slices")
	}

	short, long, probable, remedy = ParseKubeStatusErr(statusErr)
	if len(short) != 0 {
		t.Fatalf("expected short descriptions to remain empty, got %v", short)
	}
	if len(long) < 2 || len(probable) < 2 || len(remedy) < 2 {
		t.Fatalf("expected ParseKubeStatusErr to include reason and cause details, got long=%v probable=%v remedy=%v", long, probable, remedy)
	}

	noCauseErr := &kubeerror.StatusError{
		ErrStatus: v1.Status{
			Status:  v1.StatusFailure,
			Message: "service unavailable",
			Reason:  v1.StatusReasonServiceUnavailable,
			Code:    http.StatusServiceUnavailable,
		},
	}
	_, long, probable, remedy = ParseKubeStatusErr(noCauseErr)
	if len(long) != 1 || len(probable) != 2 || len(remedy) != 2 {
		t.Fatalf("expected ParseKubeStatusErr to repeat reason guidance when no causes exist, got long=%v probable=%v remedy=%v", long, probable, remedy)
	}

	var logBuffer bytes.Buffer
	handler, err := mklogger.New("meshkit-test", mklogger.Options{
		Format:      mklogger.TerminalLogFormat,
		LogLevel:    int(logrus.DebugLevel),
		Output:      &logBuffer,
		ErrorOutput: &logBuffer,
	})
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	TrackTime(handler, time.Now().Add(-time.Second), "coverage")
	if !strings.Contains(logBuffer.String(), "coverage took") {
		t.Fatalf("expected TrackTime to log elapsed time, got %q", logBuffer.String())
	}
}

func TestArchiveDetectionAndYAMLToJSONCoverage(t *testing.T) {
	dir := t.TempDir()

	shortTarGz := filepath.Join(dir, "short.tar.gz")
	var tarBuffer bytes.Buffer
	gzWriter := gzip.NewWriter(&tarBuffer)
	if _, err := gzWriter.Write([]byte("tiny")); err != nil {
		t.Fatalf("failed to write short gzip contents: %v", err)
	}
	if err := gzWriter.Close(); err != nil {
		t.Fatalf("failed to close short gzip writer: %v", err)
	}
	if err := os.WriteFile(shortTarGz, tarBuffer.Bytes(), 0o644); err != nil {
		t.Fatalf("failed to write short tar.gz fixture: %v", err)
	}
	if !IsTarGz(shortTarGz) {
		t.Fatal("expected IsTarGz to recognize a short gzip file")
	}

	shortYAML := filepath.Join(dir, "short.yaml")
	shortYAMLContent := strings.Repeat("name: meshery\n", 10)
	if err := os.WriteFile(shortYAML, []byte(shortYAMLContent), 0o644); err != nil {
		t.Fatalf("failed to write short YAML fixture: %v", err)
	}
	if IsYaml(shortYAML) {
		t.Fatal("expected short YAML sniffing to return false with the current zero-padded buffer behavior")
	}

	emptyFile := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(emptyFile, []byte{}, 0o644); err != nil {
		t.Fatalf("failed to write empty file fixture: %v", err)
	}

	buffer, err := readData(emptyFile)
	if err != io.EOF {
		t.Fatalf("expected readData to return io.EOF for short files, got %v", err)
	}
	if len(buffer) != 512 {
		t.Fatalf("expected readData to return a 512-byte sniffing buffer, got %d bytes", len(buffer))
	}

	originalJSONMarshal := jsonMarshal
	t.Cleanup(func() {
		jsonMarshal = originalJSONMarshal
	})
	jsonMarshal = func(interface{}) ([]byte, error) {
		return nil, stderrors.New("marshal failed")
	}
	if _, err := YAMLToJSON([]byte("name: meshery\n")); err == nil {
		t.Fatal("expected YAMLToJSON to fail when JSON marshaling fails")
	}
}
