package retry

import (
	"context"

	"github.com/cenkalti/backoff/v5"
)

type Operation func(ctx context.Context) error

// Do executes op with exponential backoff until success, permanent error,
// context cancellation, or budget exhaustion. Config via opts (default:
// 500ms initial, 1.5x growth, 30% jitter, 2min cap).
func Do(ctx context.Context, op Operation, opts ...Option) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	cfg := defaultConfig()
	for _, o := range opts {
		o(&cfg)
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
		return struct{}{}, op(ctx)
	}, retryOpts...)
	return err
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
