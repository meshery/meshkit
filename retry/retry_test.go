package retry_test

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/meshery/meshkit/retry"
)

func alwaysFail(err error) retry.Operation {
	return func(ctx context.Context) error { return err }
}

func countingOp(count *atomic.Int64, err error) retry.Operation {
	return func(ctx context.Context) error {
		count.Add(1)
		return err
	}
}

func TestRetrySucceedsFirstAttempt(t *testing.T) {
	t.Parallel()

	calls := 0
	err := retry.Do(context.Background(), func(ctx context.Context) error {
		calls++
		return nil
	}, retry.WithMaxAttempts(5))

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected op called once, got %d", calls)
	}
}

func TestRetrySucceedsAfterTransientErrors(t *testing.T) {
	t.Parallel()

	transient := errors.New("transient")
	var calls atomic.Int64

	err := retry.Do(context.Background(),
		func(ctx context.Context) error {
			n := calls.Add(1)
			if n < 4 {
				return transient
			}
			return nil
		},
		retry.WithMaxAttempts(10),
		retry.WithInitialInterval(1*time.Millisecond),
		retry.WithMaxInterval(5*time.Millisecond),
		retry.WithMaxElapsedTime(5*time.Second),
	)

	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if calls.Load() != 4 {
		t.Fatalf("expected 4 calls (3 failures + 1 success), got %d", calls.Load())
	}
}

func TestRetryPermanentErrorStopsImmediately(t *testing.T) {
	t.Parallel()

	permanent := errors.New("permanent failure")
	calls := 0

	err := retry.Do(context.Background(),
		func(ctx context.Context) error {
			calls++
			return retry.Permanent(permanent)
		},
		retry.WithMaxAttempts(10),
		retry.WithInitialInterval(1*time.Millisecond),
	)

	if err == nil {
		t.Fatal("expected non-nil error for permanent failure")
	}
	if !errors.Is(err, permanent) {
		t.Fatalf("expected permanent sentinel unwrapped, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected exactly 1 call, got %d", calls)
	}
}

func TestIsPermanentReturnsFalseForTransient(t *testing.T) {
	t.Parallel()

	err := errors.New("transient")
	if retry.IsPermanent(err) {
		t.Fatal("plain error should not be permanent")
	}
}

func TestIsPermanentReturnsTrueForPermanentWrapped(t *testing.T) {
	t.Parallel()

	inner := errors.New("the cause")
	wrapped := retry.Permanent(inner)
	if !retry.IsPermanent(wrapped) {
		t.Fatal("Permanent(err) should satisfy IsPermanent")
	}
}

func TestIsPermanentHandlesDoublyWrappedErrors(t *testing.T) {
	t.Parallel()

	inner := errors.New("the cause")
	wrapped := fmt.Errorf("outer layer: %w", retry.Permanent(inner))
	if !retry.IsPermanent(wrapped) {
		t.Fatal("IsPermanent should unwrap error chains successfully")
	}
}

func TestRetryContextCancellationStopsLoop(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())

	var calls atomic.Int64
	transient := errors.New("transient")

	go func() {
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()

	err := retry.Do(ctx,
		func(ctx context.Context) error {
			calls.Add(1)
			return transient
		},
		retry.WithInitialInterval(50*time.Millisecond), // longer than the cancel delay
		retry.WithMaxElapsedTime(10*time.Second),
	)

	if err == nil {
		t.Fatal("expected error after context cancellation")
	}
	if calls.Load() == 0 {
		t.Fatal("expected at least one call before cancellation")
	}
}

func TestRetryContextAlreadyCancelledBeforeFirstAttempt(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	var calls atomic.Int64
	err := retry.Do(ctx,
		func(ctx context.Context) error {
			calls.Add(1)
			return errors.New("should not reach")
		},
		retry.WithMaxAttempts(5),
		retry.WithInitialInterval(1*time.Millisecond),
	)

	if err == nil {
		t.Fatal("expected error for pre-cancelled context")
	}
	if calls.Load() > 1 {
		t.Fatalf("expected at most 1 call for pre-cancelled context, got %d", calls.Load())
	}
}

func TestRetryMaxAttemptsEnforced(t *testing.T) {
	t.Parallel()

	const maxAttempts = 4
	var count atomic.Int64

	err := retry.Do(context.Background(),
		countingOp(&count, errors.New("always fails")),
		retry.WithMaxAttempts(maxAttempts),
		retry.WithInitialInterval(1*time.Millisecond),
		retry.WithMaxInterval(2*time.Millisecond),
		retry.WithMaxElapsedTime(0), // disable elapsed-time cap
	)

	if err == nil {
		t.Fatal("expected error when max attempts exhausted")
	}
	if count.Load() != maxAttempts {
		t.Fatalf("expected exactly %d calls, got %d", maxAttempts, count.Load())
	}
}

func TestRetryMaxElapsedTimeEnforced(t *testing.T) {
	t.Parallel()

	start := time.Now()
	const budget = 80 * time.Millisecond

	err := retry.Do(context.Background(),
		alwaysFail(errors.New("always fails")),
		retry.WithMaxElapsedTime(budget),
		retry.WithInitialInterval(5*time.Millisecond),
		retry.WithMaxInterval(10*time.Millisecond),
		retry.WithJitter(0), // deterministic for timing assertions
	)

	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected error when elapsed time exceeded")
	}
	if elapsed > 3*budget {
		t.Fatalf("loop ran for %s, expected <= %s", elapsed, 3*budget)
	}
}

func TestRetryNotifierCalledOnEachRetry(t *testing.T) {
	t.Parallel()

	const failures = 3
	transient := errors.New("transient")
	var notifyCount atomic.Int64

	notifier := func(err error, wait time.Duration) {
		notifyCount.Add(1)
		if !errors.Is(err, transient) {
			t.Errorf("notifier: unexpected error %v", err)
		}
	}

	var calls atomic.Int64
	_ = retry.Do(context.Background(),
		func(ctx context.Context) error {
			if calls.Add(1) <= failures {
				return transient
			}
			return nil
		},
		retry.WithMaxAttempts(10),
		retry.WithInitialInterval(1*time.Millisecond),
		retry.WithMaxInterval(2*time.Millisecond),
		retry.WithNotifier(notifier),
	)

	if notifyCount.Load() != failures {
		t.Fatalf("expected notifier called %d times, got %d", failures, notifyCount.Load())
	}
}

func TestRetryNotifierNotCalledOnImmediateSuccess(t *testing.T) {
	t.Parallel()

	var notifyCount atomic.Int64
	_ = retry.Do(context.Background(),
		func(ctx context.Context) error { return nil },
		retry.WithNotifier(func(err error, wait time.Duration) {
			notifyCount.Add(1)
		}),
	)
	if notifyCount.Load() != 0 {
		t.Fatalf("notifier should not be called on immediate success, called %d time(s)", notifyCount.Load())
	}
}

func TestRetryNotifierNotCalledOnPermanentError(t *testing.T) {
	t.Parallel()

	var notifyCount atomic.Int64
	_ = retry.Do(context.Background(),
		func(ctx context.Context) error { return retry.Permanent(errors.New("perm")) },
		retry.WithMaxAttempts(5),
		retry.WithInitialInterval(1*time.Millisecond),
		retry.WithNotifier(func(err error, wait time.Duration) {
			notifyCount.Add(1)
		}),
	)
	if notifyCount.Load() != 0 {
		t.Fatalf("notifier called %d times for permanent error, expected 0", notifyCount.Load())
	}
}

func TestRetryZeroMaxAttemptsMeansUnlimited(t *testing.T) {
	t.Parallel()

	err := retry.Do(context.Background(),
		alwaysFail(errors.New("always fails")),
		retry.WithMaxAttempts(0),
		retry.WithMaxElapsedTime(50*time.Millisecond),
		retry.WithInitialInterval(5*time.Millisecond),
		retry.WithMaxInterval(10*time.Millisecond),
	)
	if err == nil {
		t.Fatal("expected error when elapsed time runs out with unlimited attempts")
	}
}

func TestRetryWithMaxAttemptsOneNoRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	err := retry.Do(context.Background(),
		countingOp(&calls, errors.New("fail")),
		retry.WithMaxAttempts(1),
		retry.WithInitialInterval(1*time.Millisecond),
		retry.WithMaxElapsedTime(0),
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if calls.Load() != 1 {
		t.Fatalf("WithMaxAttempts(1) should allow exactly 1 call, got %d", calls.Load())
	}
}

func TestRetryDefaultsAreApplied(t *testing.T) {
	t.Parallel()

	transient := errors.New("transient")
	var calls atomic.Int64

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_ = retry.Do(ctx,
		func(ctx context.Context) error {
			if calls.Add(1) >= 2 {
				return nil
			}
			return transient
		},
	)

	if calls.Load() < 2 {
		t.Fatalf("expected at least 2 calls with default config, got %d", calls.Load())
	}
}
