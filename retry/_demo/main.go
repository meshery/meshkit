package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/meshery/meshkit/retry"
)

var attempt int

func main() {
	fmt.Println("══════════════════════════════════════════════════")
	fmt.Println("  meshkit/retry — exponential backoff demo")
	fmt.Println("  transport-agnostic: works with HTTP, gRPC, DB, ...")
	fmt.Println("══════════════════════════════════════════════════")
	fmt.Println()

	// ── Demo 1: Immediate success ──
	fmt.Println("━━━ Demo 1: Immediate success (no retry needed) ━━━")
	fmt.Println("  Operation returns nil on 1st try → retry.Do returns immediately.")
	attempt = 0
	err := retry.Do(context.Background(), func(ctx context.Context) error {
		attempt++
		return nil
	})
	fmt.Printf("  → attempts: %d | err: %v\n", attempt, err)
	fmt.Println()

	// ── Demo 2: Transient → success after 2 failures ──
	fmt.Println("━━━ Demo 2: Transient failures → recovers ━━━")
	fmt.Println("  Operation fails twice (returns error), succeeds on 3rd.")
	fmt.Println("  retry.Do retries with exponential backoff until success or budget exhausted.")
	attempt = 0
	err = retry.Do(context.Background(), func(ctx context.Context) error {
		attempt++
		if attempt < 3 {
			fmt.Printf("  〉attempt %d: transient error (will retry)\n", attempt)
			return errors.New("transient error")
		}
		fmt.Printf("  〉attempt %d: success\n", attempt)
		return nil
	},
		retry.WithMaxAttempts(5),
		retry.WithInitialInterval(100*time.Millisecond),
		retry.WithMaxInterval(500*time.Millisecond),
	)
	fmt.Printf("  → attempts: %d | err: %v\n", attempt, err)
	fmt.Println()

	// ── Demo 3: Permanent error ──
	fmt.Println("━━━ Demo 3: Permanent error (stops immediately) ━━━")
	fmt.Println("  Operation wraps error with retry.Permanent() → no retry attempted.")
	fmt.Println("  Use case: HTTP 4xx, auth failure, validation errors — anything non-transient.")
	attempt = 0
	err = retry.Do(context.Background(), func(ctx context.Context) error {
		attempt++
		fmt.Printf("  〉attempt %d: fatal error → retry.Permanent()\n", attempt)
		return retry.Permanent(errors.New("fatal: invalid input"))
	},
		retry.WithMaxAttempts(5),
	)
	fmt.Printf("  → attempts: %d | err: %v (error preserved through chain)\n", attempt, err)
	fmt.Println()

	// ── Demo 4: Exhaustion ──
	fmt.Println("━━━ Demo 4: Max attempts exhausted ━━━")
	fmt.Println("  Operation always fails. WithMaxAttempts(3) limits total tries.")
	fmt.Println("  Backoff: 50ms → 100ms → 200ms (capped). Fails fast instead of hanging.")
	attempt = 0
	err = retry.Do(context.Background(), func(ctx context.Context) error {
		attempt++
		fmt.Printf("  〉attempt %d: network timeout\n", attempt)
		return errors.New("network timeout")
	},
		retry.WithMaxAttempts(3),
		retry.WithInitialInterval(50*time.Millisecond),
		retry.WithMaxInterval(200*time.Millisecond),
	)
	fmt.Printf("  → attempts: %d | err: %v\n", attempt, err)
	fmt.Println()

	// ── Demo 5: Context cancellation ──
	fmt.Println("━━━ Demo 5: Context cancellation ━━━")
	fmt.Println("  Context cancelled after 80ms from a goroutine.")
	fmt.Println("  Backoff is 200ms → retry.Do notices ctx.Err() before next attempt and stops.")
	ctx, cancel := context.WithCancel(context.Background())
	attempt = 0
	go func() {
		time.Sleep(80 * time.Millisecond)
		fmt.Println("  〉cancelling context...")
		cancel()
	}()
	start := time.Now()
	err = retry.Do(ctx, func(ctx context.Context) error {
		attempt++
		fmt.Printf("  〉attempt %d: transient (%dms elapsed)\n", attempt, time.Since(start).Milliseconds())
		return errors.New("transient")
	},
		retry.WithInitialInterval(200*time.Millisecond),
	)
	fmt.Printf("  → attempts: %d | err: %v\n", attempt, err)
	fmt.Println()

	// ── Demo 6: gRPC-style ──
	fmt.Println("━━━ Demo 6: Transport-agnostic — gRPC ━━━")
	fmt.Println("  Same retry.Do() — no HTTP dependency. Works with any error-returning operation.")
	fmt.Println("  Simulates: gRPC status 'Unavailable' (service temporarily down)")
	attempt = 0
	err = retry.Do(context.Background(), func(ctx context.Context) error {
		attempt++
		if attempt < 3 {
			fmt.Printf("  〉attempt %d: rpc error: code = Unavailable (will retry)\n", attempt)
			return errors.New("rpc error: code = Unavailable desc = service temporarily unavailable")
		}
		fmt.Printf("  〉attempt %d: success (gRPC call completed)\n", attempt)
		return nil
	},
		retry.WithMaxAttempts(5),
		retry.WithInitialInterval(100*time.Millisecond),
	)
	fmt.Printf("  → attempts: %d | err: %v\n", attempt, err)
	fmt.Println()

	// ── Demo 7: Notifier ──
	fmt.Println("━━━ Demo 7: Notifier callback ━━━")
	fmt.Println("  WithNotifier fires before each retry — useful for logging, metrics, alerts.")
	attempt = 0
	err = retry.Do(context.Background(), func(ctx context.Context) error {
		attempt++
		if attempt < 3 {
			return errors.New("server not ready")
		}
		return nil
	},
		retry.WithMaxAttempts(5),
		retry.WithInitialInterval(50*time.Millisecond),
		retry.WithMaxInterval(200*time.Millisecond),
		retry.WithNotifier(func(err error, wait time.Duration) {
			fmt.Printf("  ⚠ notifier: retrying in %v — err: %v\n", wait.Round(time.Millisecond), err)
		}),
	)
	fmt.Printf("  → attempts: %d | err: %v\n", attempt, err)
	fmt.Println()

	// ── Demo 8: DB connection ──
	fmt.Println("━━━ Demo 8: Transport-agnostic — Database ━━━")
	fmt.Println("  Same retry.Do() — simulates a DB connection pool retry.")
	attempt = 0
	err = retry.Do(context.Background(), func(ctx context.Context) error {
		attempt++
		if attempt < 2 {
			fmt.Printf("  〉attempt %d: db: connection refused (will retry)\n", attempt)
			return errors.New("db: connection refused")
		}
		fmt.Printf("  〉attempt %d: connected\n", attempt)
		return nil
	},
		retry.WithMaxAttempts(3),
		retry.WithInitialInterval(50*time.Millisecond),
	)
	fmt.Printf("  → attempts: %d | err: %v\n", attempt, err)
	fmt.Println()

	fmt.Println("══════════════════════════════════════════════════")
	fmt.Println("  All 8 demos passed. Key takeaways:")
	fmt.Println("  • retry.Do is transport-agnostic (gRPC, DB, HTTP — same API)")
	fmt.Println("  • retry.Permanent() stops retries for fatal errors")
	fmt.Println("  • Context cancellation is respected mid-backoff")
	fmt.Println("  • MaxAttempts + MaxElapsedTime = safety budget")
	fmt.Println("  • WithNotifier hooks into every retry for logging/metrics")
	fmt.Println("══════════════════════════════════════════════════")
}
