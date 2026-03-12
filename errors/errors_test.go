package errors

import (
	stderrors "errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGettersSupportErrorV2(t *testing.T) {
	err := NewV2(
		"meshkit-99999",
		Alert,
		[]string{"short"},
		[]string{"long"},
		[]string{"cause"},
		[]string{"remedy"},
		map[string]string{"field": "value"},
	)

	assert.Equal(t, "meshkit-99999", GetCode(err))
	assert.Equal(t, Severity(Alert), GetSeverity(err))
	assert.Equal(t, "short", GetSDescription(err))
	assert.Equal(t, "long", GetLDescription(err))
	assert.Equal(t, "cause", GetCause(err))
	assert.Equal(t, "remedy", GetRemedy(err))
}

func TestGettersSupportWrappedErrorV2(t *testing.T) {
	err := fmt.Errorf("wrapped: %w", NewV2(
		"meshkit-99998",
		Critical,
		[]string{"wrapped short"},
		[]string{"wrapped long"},
		[]string{"wrapped cause"},
		[]string{"wrapped remedy"},
		nil,
	))

	var errV2 *ErrorV2
	assert.True(t, stderrors.As(err, &errV2))
	assert.Equal(t, "meshkit-99998", GetCode(err))
	assert.Equal(t, Severity(Critical), GetSeverity(err))
	assert.Equal(t, "wrapped short", GetSDescription(err))
	assert.Equal(t, "wrapped long", GetLDescription(err))
	assert.Equal(t, "wrapped cause", GetCause(err))
	assert.Equal(t, "wrapped remedy", GetRemedy(err))
}
