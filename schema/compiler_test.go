package schema

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dlclark/regexp2"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
)

func TestKeywordFromLocation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		location string
		expected string
	}{
		{
			name:     "empty location",
			location: "#/",
			expected: "",
		},
		{
			name:     "plain keyword",
			location: "#/properties/name/type",
			expected: "type",
		},
		{
			name:     "escaped slash",
			location: "#/properties/foo~1bar",
			expected: "foo/bar",
		},
		{
			name:     "escaped tilde",
			location: "#/properties/foo~0bar",
			expected: "foo~bar",
		},
		{
			name:     "absolute location",
			location: "meshkit:///schemas/example.yaml#/properties/foo~1bar",
			expected: "foo/bar",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, testCase.expected, keywordFromLocation(testCase.location))
		})
	}
}

func TestDlclarkRegexpMatchStringLogsError(t *testing.T) {
	compiled := regexp2.MustCompile("^(a+)+$", regexp2.ECMAScript)
	compiled.MatchTimeout = time.Nanosecond

	re := (*dlclarkRegexp)(compiled)

	var logged string
	previous := regexpMatchStringErrorf
	regexpMatchStringErrorf = func(format string, args ...any) {
		logged = fmt.Sprintf(format, args...)
	}
	t.Cleanup(func() {
		regexpMatchStringErrorf = previous
	})

	assert.False(t, re.MatchString(strings.Repeat("a", 5_000)+"!"))
	assert.Contains(t, logged, `regexp2 MatchString failed for pattern "^(a+)+$"`)
	assert.Contains(t, logged, "match timeout")
}

func TestJSONPointer(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", jsonPointer(nil))
	assert.Equal(t, "/kind", jsonPointer([]string{"kind"}))
	assert.Equal(t, "/properties/foo~1bar", jsonPointer([]string{"properties", "foo/bar"}))
	assert.Equal(t, "/properties/foo~0bar", jsonPointer([]string{"properties", "foo~bar"}))
}

func TestViolationsFromErrorWithWrappedSchemaError(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("wrapped: %w", &openapi3.SchemaError{
		SchemaField: "enum",
		Reason:      "value is not one of the allowed values",
	})

	violations := violationsFromError(err)
	assert.Len(t, violations, 1)
	assert.Equal(t, "enum", violations[0].Keyword)
	assert.Equal(t, "/enum", violations[0].SchemaPath)
	assert.Equal(t, "value is not one of the allowed values", violations[0].Message)
}

func TestViolationsFromErrorPrefersNestedSchemaErrors(t *testing.T) {
	t.Parallel()

	child := &openapi3.SchemaError{
		SchemaField: "enum",
		Reason:      "nested reason",
	}
	parent := &openapi3.SchemaError{
		SchemaField: "oneOf",
		Reason:      "parent reason",
		Origin:      openapi3.MultiError{child},
	}

	violations := violationsFromError(parent)
	assert.Len(t, violations, 1)
	assert.Equal(t, "enum", violations[0].Keyword)
	assert.Equal(t, "nested reason", violations[0].Message)
}

func TestViolationsFromErrorSupportsMultiErrorUnwrap(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("wrapped: %w", openapi3.MultiError{
		&openapi3.SchemaError{
			SchemaField: "pattern",
			Reason:      "string doesn't match regular expression",
		},
	})

	violations := violationsFromError(err)
	assert.Len(t, violations, 1)
	assert.Equal(t, "pattern", violations[0].Keyword)
}

func TestSchemaErrorMessageFallsBackToErrorString(t *testing.T) {
	t.Parallel()

	err := &openapi3.SchemaError{
		SchemaField: "type",
	}

	assert.NotEmpty(t, schemaErrorMessage(err))
}
