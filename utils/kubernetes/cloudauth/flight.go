package cloudauth

import (
	"context"
	"time"
)

// clock is the time source shared by the Authenticator and its token
// sources. Deadlines are derived from it too, so tests can drive refresh
// timing and timeouts with a fake clock instead of sleeping.
type clock interface {
	Now() time.Time
	// WithTimeout returns a copy of ctx that is cancelled d after Now, with
	// context.DeadlineExceeded as its cause.
	WithTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc)
}

// realClock is the wall clock.
type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

func (realClock) WithTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, d)
}

// flight is one in-progress operation whose result is shared by every caller
// that arrives while it runs: the caller that starts it (the leader) runs
// the operation without holding any lock, and the others wait for done. A
// failed result is shared exactly like a successful one, so N concurrent
// callers cost one attempt, not N serial attempts.
type flight[T any] struct {
	done chan struct{}
	val  T
	err  error
	// waiters counts the callers that joined instead of leading. It is
	// guarded by the lock of the flight's owner, which increments it before
	// releasing that lock, so tests can tell when every caller has joined.
	waiters int
}

func newFlight[T any]() *flight[T] {
	return &flight[T]{done: make(chan struct{})}
}

// finish publishes the result and releases every waiter. It must be called
// exactly once, by the leader.
func (f *flight[T]) finish(val T, err error) {
	f.val, f.err = val, err
	close(f.done)
}

// wait blocks until the leader finishes. The leader's operation is itself
// bounded by a deadline, so waiting is bounded too.
func (f *flight[T]) wait() (T, error) {
	<-f.done
	return f.val, f.err
}

// runBounded calls fn and returns its result, or gives up with the cause of
// ctx's cancellation once ctx ends. fn receives ctx and should honor it;
// when it does not (oauth2.TokenSource takes no context), fn keeps running
// in its goroutine after runBounded returns and its late result is
// discarded, so every such fn must have its own transport-level timeout.
func runBounded[T any](ctx context.Context, fn func(context.Context) (T, error)) (T, error) {
	type result struct {
		val T
		err error
	}
	done := make(chan result, 1)
	go func() {
		val, err := fn(ctx)
		done <- result{val: val, err: err}
	}()

	select {
	case r := <-done:
		return r.val, r.err
	case <-ctx.Done():
		var zero T
		return zero, context.Cause(ctx)
	}
}
