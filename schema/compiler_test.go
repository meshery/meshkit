package schema

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dlclark/regexp2"
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
