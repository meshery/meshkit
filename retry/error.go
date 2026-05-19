package retry

import (
	"errors"

	"github.com/cenkalti/backoff/v5"
)

type PermanentError = backoff.PermanentError

// Permanent wraps err to signal no further retries should be attempted.
// Use for non-transient errors (HTTP 4xx, auth failures, validation errors).
// Do NOT use for context-cancellation; return ctx.Err() directly.
func Permanent(err error) error {
	return backoff.Permanent(err)
}

// IsPermanent reports whether err is (or wraps) a PermanentError.
func IsPermanent(err error) bool {
	var pErr *backoff.PermanentError
	return errors.As(err, &pErr)
}
