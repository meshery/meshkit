package cloudauth

import (
	"context"
	"time"

	"golang.org/x/oauth2/google"
)

func withDisableIMDS() Option {
	return func(o *options) {
		o.eks.disableIMDS = true
	}
}

func withClock(now func() time.Time) Option {
	return func(o *options) {
		o.eks.now = now
		o.gke.now = now
	}
}

func withExpiryDelta(d time.Duration) Option {
	return func(o *options) {
		o.eks.expiryDelta = d
		o.gke.expiryDelta = d
	}
}

func withLookPath(fn func(string) (string, error)) Option {
	return func(o *options) {
		o.lookPath = fn
	}
}

func withGetenv(fn func(string) string) Option {
	return func(o *options) {
		o.getenv = fn
	}
}

func withGKEFindDefaultCredentials(fn func(context.Context, ...string) (*google.Credentials, error)) Option {
	return func(o *options) {
		o.gke.findDefaultCredentials = fn
	}
}
