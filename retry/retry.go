package retry

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/cenkalti/backoff/v5"
)

type Operation func(ctx context.Context) error

// Do executes op with exponential backoff until success, permanent error,
// context cancellation, or budget exhaustion. Config via opts (default:
// 500ms initial, 1.5x growth, 30% jitter, 2min cap).
//
// When a ErrorClassifier is configured via WithErrorClassifier, every non-nil
// error from op (except those explicitly wrapped with Permanent) is passed to
// the classifier before the retry decision is made.
func Do(ctx context.Context, op Operation, opts ...Option) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	cfg := defaultConfig()
	for _, o := range opts {
		o(&cfg)
	}

	if err := validateConfig(cfg); err != nil {
		return err
	}

	apply := op
	if cfg.ErrorClassifier != nil {
		apply = func(ctx context.Context) error {
			err := op(ctx)
			if err == nil {
				return nil
			}
			var pErr *backoff.PermanentError
			if errors.As(err, &pErr) {
				return err
			}
			if cfg.ErrorClassifier(err) == DecisionStop {
				return backoff.Permanent(err)
			}
			return err
		}
	}

	retryOpts := []backoff.RetryOption{
		backoff.WithBackOff(buildBackOff(cfg)),
		backoff.WithMaxElapsedTime(cfg.MaxElapsedTime),
		backoff.WithNotify(cfg.Notifier),
	}
	if cfg.MaxAttempts > 0 {
		retryOpts = append(retryOpts, backoff.WithMaxTries(cfg.MaxAttempts))
	}

	_, err := backoff.Retry(ctx, func() (struct{}, error) {
		return struct{}{}, apply(ctx)
	}, retryOpts...)
	return err
}

func validateConfig(cfg Config) error {
	if cfg.InitialInterval <= 0 {
		return fmt.Errorf("retry: InitialInterval must be > 0, got %v", cfg.InitialInterval)
	}
	if cfg.MaxInterval <= 0 {
		return fmt.Errorf("retry: MaxInterval must be > 0, got %v", cfg.MaxInterval)
	}
	if cfg.MaxInterval < cfg.InitialInterval {
		return fmt.Errorf("retry: MaxInterval (%v) must be >= InitialInterval (%v)", cfg.MaxInterval, cfg.InitialInterval)
	}
	if cfg.MaxElapsedTime < 0 {
		return fmt.Errorf("retry: MaxElapsedTime must be >= 0, got %v", cfg.MaxElapsedTime)
	}
	if math.IsNaN(cfg.Multiplier) || math.IsInf(cfg.Multiplier, 0) || cfg.Multiplier < 1 {
		return fmt.Errorf("retry: Multiplier must be finite and >= 1, got %v", cfg.Multiplier)
	}
	if math.IsNaN(cfg.RandomizationFactor) || cfg.RandomizationFactor < 0 || cfg.RandomizationFactor > 1 {
		return fmt.Errorf("retry: RandomizationFactor must be finite and in [0,1], got %v", cfg.RandomizationFactor)
	}
	return nil
}

// buildBackOff constructs a backoff policy from Config.
func buildBackOff(cfg Config) backoff.BackOff {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = cfg.InitialInterval
	b.MaxInterval = cfg.MaxInterval
	b.Multiplier = cfg.Multiplier
	b.RandomizationFactor = cfg.RandomizationFactor

	return b
}
