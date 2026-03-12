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

type wrappedErrorV2 struct {
	v1 *Error
	v2 *ErrorV2
}

func (e wrappedErrorV2) Error() string {
	return "wrapped error"
}

func (e wrappedErrorV2) As(target any) bool {
	switch t := target.(type) {
	case **ErrorV2:
		*t = e.v2
		return true
	case **Error:
		*t = e.v1
		return true
	default:
		return false
	}
}

func TestGettersPreferErrorV2WhenBothMatch(t *testing.T) {
	err := wrappedErrorV2{
		v1: New(
			"meshkit-00001",
			Alert,
			[]string{"v1 short"},
			[]string{"v1 long"},
			[]string{"v1 cause"},
			[]string{"v1 remedy"},
		),
		v2: NewV2(
			"meshkit-00002",
			Critical,
			[]string{"v2 short"},
			[]string{"v2 long"},
			[]string{"v2 cause"},
			[]string{"v2 remedy"},
			map[string]string{"field": "value"},
		),
	}

	assert.Equal(t, "meshkit-00002", GetCode(err))
	assert.Equal(t, Severity(Critical), GetSeverity(err))
	assert.Equal(t, "v2 short", GetSDescription(err))
	assert.Equal(t, "v2 long", GetLDescription(err))
	assert.Equal(t, "v2 cause", GetCause(err))
	assert.Equal(t, "v2 remedy", GetRemedy(err))
}
