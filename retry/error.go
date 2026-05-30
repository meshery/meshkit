package retry

import (
	"errors"

	"github.com/cenkalti/backoff/v5"
)

// ErrorDecision controls retry behaviour for a single error.
type ErrorDecision int

const (
	DecisionRetry ErrorDecision = iota
	DecisionStop
)

// ErrorClassifier returns the retry decision for a given error.
// Return DecisionStop for errors that should not be retried (e.g. HTTP 4xx,
// validation failures, auth errors). Return DecisionRetry for transient
// errors (timeouts, 5xx, rate limits).
//
// Ignored when the operation explicitly returns Permanent(err).
type ErrorClassifier func(err error) ErrorDecision

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
