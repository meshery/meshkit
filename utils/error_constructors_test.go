package utils

import (
	stderrors "errors"
	"math"
	"reflect"
	"testing"

	meshkiterrors "github.com/meshery/meshkit/errors"
)

func TestErrorConstructorsExposeExpectedCodes(t *testing.T) {
	testErr := stderrors.New("boom")

	testCases := []struct {
		name string
		err  error
		code string
	}{
		{"invalid construct schema version", ErrInvalidConstructSchemaVersion("design", "v0", "v1"), ErrInvalidSchemaVersionCode},
		{"cue lookup", ErrCueLookup(testErr), ErrCueLookupCode},
		{"json schema to cue", ErrJsonSchemaToCue(testErr), ErrJsonSchemaToCueCode},
		{"yaml to cue", ErrYamlToCue(testErr), ErrYamlToCueCode},
		{"json to cue", ErrJsonToCue(testErr), ErrJsonToCueCode},
		{"expected type mismatch", ErrExpectedTypeMismatch(testErr, "string"), ErrExpectedTypeMismatchCode},
		{"missing field", ErrMissingField(testErr, "name"), ErrMissingFieldCode},
		{"unmarshal", ErrUnmarshal(testErr), ErrUnmarshalCode},
		{"unmarshal invalid", ErrUnmarshalInvalid(testErr, reflect.TypeOf("")), ErrUnmarshalInvalidCode},
		{"unmarshal syntax", ErrUnmarshalSyntax(testErr, 42), ErrUnmarshalSyntaxCode},
		{"unmarshal type", ErrUnmarshalType(testErr, "field"), ErrUnmarshalTypeCode},
		{"unmarshal unsupported type", ErrUnmarshalUnsupportedType(testErr, reflect.TypeOf(func() {})), ErrUnmarshalUnsupportedTypeCode},
		{"unmarshal unsupported value", ErrUnmarshalUnsupportedValue(testErr, reflect.ValueOf(math.Inf(1))), ErrUnmarshalUnsupportedValueCode},
		{"marshal", ErrMarshal(testErr), ErrMarshalCode},
		{"get bool", ErrGetBool("enabled", testErr), ErrGetBoolCode},
		{"remote file not found", ErrRemoteFileNotFound("https://example.com/file"), ErrRemoteFileNotFoundCode},
		{"reading remote file", ErrReadingRemoteFile(testErr), ErrReadingRemoteFileCode},
		{"reading local file", ErrReadingLocalFile(testErr), ErrReadingLocalFileCode},
		{"read file", ErrReadFile(testErr, "/tmp/file"), ErrReadFileCode},
		{"write file", ErrWriteFile(testErr, "/tmp/file"), ErrWriteFileCode},
		{"create file", ErrCreateFile(testErr, "/tmp/file"), ErrCreateFileCode},
		{"create dir", ErrCreateDir(testErr, "/tmp/dir"), ErrCreateDirCode},
		{"convert to byte", ErrConvertToByte(testErr), ErrConvertToByteCode},
		{"getting latest release tag", ErrGettingLatestReleaseTag(testErr), ErrGettingLatestReleaseTagCode},
		{"type cast", ErrTypeCast(testErr), ErrTypeCastCode},
		{"decode yaml", ErrDecodeYaml(testErr), ErrDecodeYamlCode},
		{"compress to tar gz", ErrCompressToTarGZ(testErr, "/tmp/file"), ErrCompressToTarGZCode},
		{"extract tar xz", ErrExtractTarXZ(testErr, "/tmp/archive"), ErrExtractTarXZCode},
		{"extract zip", ErrExtractZip(testErr, "/tmp/archive"), ErrExtractZipCode},
		{"read dir", ErrReadDir(testErr, "/tmp/dir"), ErrReadDirCode},
		{"file walk dir", ErrFileWalkDir(testErr, "/tmp/dir"), ErrFileWalkDirCode},
		{"relative path", ErrRelPath(testErr, "/tmp/dir"), ErrRelPathCode},
		{"copy file", ErrCopyFile(testErr), ErrCopyFileCode},
		{"close file", ErrCloseFile(testErr), ErrCloseFileCode},
		{"open file", ErrOpenFile("/tmp/file"), ErrOpenFileCode},
		{"google jwt invalid", ErrGoogleJwtInvalid(testErr), ErrGoogleJwtInvalidCode},
		{"google sheet service", ErrGoogleSheetSRV(testErr), ErrGoogleSheetSRVCode},
		{"writing into file", ErrWritingIntoFile(testErr, "artifact"), ErrWritingIntoFileCode},
		{"invalid protocol", ErrInvalidProtocol, ErrInvalidProtocolCode},
		{"extract type", ErrExtractType, ErrUnmarshalTypeCode},
		{"invalid schema version", ErrInvalidSchemaVersion, ErrInvalidSchemaVersionCode},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := meshkiterrors.GetCode(tc.err); got != tc.code {
				t.Fatalf("expected error code %q, got %q", tc.code, got)
			}
		})
	}
}
