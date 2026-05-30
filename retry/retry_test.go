package retry_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestRetryClassifierStopsOnDecisionStop(t *testing.T) {
	t.Parallel()

	classifyErr := errors.New("not a chance")
	var calls atomic.Int64

	err := retry.Do(context.Background(),
		func(ctx context.Context) error {
			calls.Add(1)
			return classifyErr
		},
		retry.WithErrorClassifier(func(err error) retry.ErrorDecision {
			if errors.Is(err, classifyErr) {
				return retry.DecisionStop
			}
			return retry.DecisionRetry
		}),
		retry.WithMaxAttempts(10),
		retry.WithInitialInterval(1*time.Millisecond),
	)

	if err == nil {
		t.Fatal("expected non-nil error when classifier stops")
	}
	if !errors.Is(err, classifyErr) {
		t.Fatalf("expected classifier error unwrapped, got %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected exactly 1 call when classifier stops, got %d", calls.Load())
	}
}

func TestRetryClassifierRetriesOnDecisionRetry(t *testing.T) {
	t.Parallel()

	classifyErr := errors.New("transient per classifier")
	var calls atomic.Int64

	err := retry.Do(context.Background(),
		func(ctx context.Context) error {
			n := calls.Add(1)
			if n < 3 {
				return classifyErr
			}
			return nil
		},
		retry.WithErrorClassifier(func(err error) retry.ErrorDecision {
			return retry.DecisionRetry
		}),
		retry.WithMaxAttempts(10),
		retry.WithInitialInterval(1*time.Millisecond),
		retry.WithMaxInterval(5*time.Millisecond),
	)

	if err != nil {
		t.Fatalf("expected success after classifier retries, got %v", err)
	}
	if calls.Load() != 3 {
		t.Fatalf("expected 3 calls (2 classified retries + success), got %d", calls.Load())
	}
}

func TestRetryClassifierDoesNotOverrideExplicitPermanent(t *testing.T) {
	t.Parallel()

	permErr := errors.New("explicitly permanent")
	var calls atomic.Int64

	err := retry.Do(context.Background(),
		func(ctx context.Context) error {
			calls.Add(1)
			return retry.Permanent(permErr)
		},
		// Classifier says retry everything — but Permanent should still win.
		retry.WithErrorClassifier(func(err error) retry.ErrorDecision {
			return retry.DecisionRetry
		}),
		retry.WithMaxAttempts(10),
		retry.WithInitialInterval(1*time.Millisecond),
	)

	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if !errors.Is(err, permErr) {
		t.Fatalf("expected permanent error unwrapped, got %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected exactly 1 call for explicit Permanent, got %d", calls.Load())
	}
}

func TestRetryClassifierCanMixWithPermanent(t *testing.T) {
	t.Parallel()

	permErr := errors.New("permanent")
	transientErr := errors.New("transient")
	var calls atomic.Int64

	err := retry.Do(context.Background(),
		func(ctx context.Context) error {
			n := calls.Add(1)
			if n == 1 {
				return transientErr
			}
			return retry.Permanent(permErr)
		},
		retry.WithErrorClassifier(func(err error) retry.ErrorDecision {
			if errors.Is(err, transientErr) {
				return retry.DecisionRetry
			}
			return retry.DecisionStop
		}),
		retry.WithMaxAttempts(5),
		retry.WithInitialInterval(1*time.Millisecond),
		retry.WithMaxInterval(2*time.Millisecond),
	)

	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if !errors.Is(err, permErr) {
		t.Fatalf("expected permanent error unwrapped, got %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("expected 2 calls (transient + permanent), got %d", calls.Load())
	}
}

func TestRetryConfigValidationInitialIntervalZero(t *testing.T) {
	t.Parallel()

	err := retry.Do(context.Background(),
		func(ctx context.Context) error { return errors.New("err") },
		retry.WithInitialInterval(0),
	)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "InitialInterval") {
		t.Fatalf("expected InitialInterval validation error, got %v", err)
	}
}

func TestRetryConfigValidationMaxIntervalZero(t *testing.T) {
	t.Parallel()

	err := retry.Do(context.Background(),
		func(ctx context.Context) error { return errors.New("err") },
		retry.WithMaxInterval(0),
	)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "MaxInterval") {
		t.Fatalf("expected MaxInterval validation error, got %v", err)
	}
}

func TestRetryConfigValidationMaxIntervalLessThanInitial(t *testing.T) {
	t.Parallel()

	err := retry.Do(context.Background(),
		func(ctx context.Context) error { return errors.New("err") },
		retry.WithInitialInterval(5*time.Second),
		retry.WithMaxInterval(1*time.Second),
	)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "MaxInterval") || !strings.Contains(err.Error(), "InitialInterval") {
		t.Fatalf("expected MaxInterval/InitialInterval mismatch error, got %v", err)
	}
}

func TestRetryConfigValidationMultiplierNaN(t *testing.T) {
	t.Parallel()

	err := retry.Do(context.Background(),
		func(ctx context.Context) error { return errors.New("err") },
		retry.WithMultiplier(float64(math.NaN())),
	)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "Multiplier") {
		t.Fatalf("expected Multiplier validation error, got %v", err)
	}
}

func TestRetryConfigValidationMultiplierInf(t *testing.T) {
	t.Parallel()

	err := retry.Do(context.Background(),
		func(ctx context.Context) error { return errors.New("err") },
		retry.WithMultiplier(float64(math.Inf(1))),
	)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "Multiplier") {
		t.Fatalf("expected Multiplier validation error, got %v", err)
	}
}

func TestRetryConfigValidationMultiplierLessThanOne(t *testing.T) {
	t.Parallel()

	err := retry.Do(context.Background(),
		func(ctx context.Context) error { return errors.New("err") },
		retry.WithMultiplier(0.5),
	)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "Multiplier") {
		t.Fatalf("expected Multiplier validation error, got %v", err)
	}
}

func TestRetryConfigValidationJitterNaN(t *testing.T) {
	t.Parallel()

	err := retry.Do(context.Background(),
		func(ctx context.Context) error { return errors.New("err") },
		retry.WithJitter(float64(math.NaN())),
	)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "RandomizationFactor") {
		t.Fatalf("expected RandomizationFactor validation error, got %v", err)
	}
}

func TestRetryConfigValidationJitterOutOfRange(t *testing.T) {
	t.Parallel()

	err := retry.Do(context.Background(),
		func(ctx context.Context) error { return errors.New("err") },
		retry.WithJitter(1.5),
	)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "RandomizationFactor") {
		t.Fatalf("expected RandomizationFactor validation error, got %v", err)
	}
}

func TestRetryConfigValidationJitterNegative(t *testing.T) {
	t.Parallel()

	err := retry.Do(context.Background(),
		func(ctx context.Context) error { return errors.New("err") },
		retry.WithJitter(-0.1),
	)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "RandomizationFactor") {
		t.Fatalf("expected RandomizationFactor validation error, got %v", err)
	}
}

func TestRetryConfigValidationZeroMaxElapsedTimeIsValid(t *testing.T) {
	t.Parallel()

	// 0 for MaxElapsedTime means "no wall-clock limit". Should be valid.
	err := retry.Do(context.Background(),
		func(ctx context.Context) error { return nil },
		retry.WithMaxElapsedTime(0),
	)
	if err != nil {
		t.Fatalf("expected success (0 MaxElapsedTime is valid), got %v", err)
	}
}

func TestRetryConfigValidationNegativeMaxElapsedTime(t *testing.T) {
	t.Parallel()

	err := retry.Do(context.Background(),
		func(ctx context.Context) error { return errors.New("err") },
		retry.WithMaxElapsedTime(-1),
	)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "MaxElapsedTime") {
		t.Fatalf("expected MaxElapsedTime validation error, got %v", err)
	}
}

// ExampleDo demonstrates idiomatic HTTP usage with retry budget and per-attempt timeout.
//
// MaxElapsedTime limits the retry loop but does NOT interrupt an in-flight HTTP
// request. Always pair it with http.Client.Timeout (or NewRequestWithContext) so
// each attempt has its own deadline.
func ExampleDo() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 3 * time.Second}

	err := retry.Do(context.Background(), func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
		if err != nil {
			return retry.Permanent(err)
		}

		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		switch {
		case resp.StatusCode == http.StatusOK:
			return nil
		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
			return fmt.Errorf("transient response: %s", resp.Status)
		default:
			return retry.Permanent(fmt.Errorf("non-retryable response: %s", resp.Status))
		}
	},
		retry.WithMaxAttempts(3),
		retry.WithInitialInterval(time.Second),
		retry.WithMaxElapsedTime(10*time.Second),
	)

	if err != nil {
		fmt.Printf("request failed: %v\n", err)
	}
	// Output:
	// request failed: non-retryable response: 404 Not Found
}
