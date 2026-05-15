package retry

import (
	"time"

	"github.com/meshery/meshkit/logger"
)

const (
	DefaultInitialInterval     = 500 * time.Millisecond
	DefaultMaxInterval         = 30 * time.Second
	DefaultMaxElapsedTime      = 2 * time.Minute
	DefaultMultiplier          = 1.5
	DefaultRandomizationFactor = 0.3 // Never set to 0 in production
)

type Config struct {
	MaxAttempts         uint64
	InitialInterval     time.Duration
	MaxInterval         time.Duration
	MaxElapsedTime      time.Duration
	Multiplier          float64
	RandomizationFactor float64
	Notifier            func(err error, wait time.Duration)
}

func defaultConfig() Config {
	return Config{
		InitialInterval:     DefaultInitialInterval,
		MaxInterval:         DefaultMaxInterval,
		MaxElapsedTime:      DefaultMaxElapsedTime,
		Multiplier:          DefaultMultiplier,
		RandomizationFactor: DefaultRandomizationFactor,
	}
}

type Option func(*Config)

// WithMaxAttempts sets a hard cap on total calls (includes first attempt).
func WithMaxAttempts(n uint64) Option {
	return func(c *Config) { c.MaxAttempts = n }
}

func WithInitialInterval(d time.Duration) Option {
	return func(c *Config) { c.InitialInterval = d }
}

func WithMaxInterval(d time.Duration) Option {
	return func(c *Config) { c.MaxInterval = d }
}

// WithMaxElapsedTime sets wall-clock deadline. Pass 0 to disable.
func WithMaxElapsedTime(d time.Duration) Option {
	return func(c *Config) { c.MaxElapsedTime = d }
}

func WithMultiplier(m float64) Option {
	return func(c *Config) { c.Multiplier = m }
}

// WithJitter overrides randomization factor (range: 0.0-1.0). Do not set to 0.0 in production.
func WithJitter(f float64) Option {
	return func(c *Config) { c.RandomizationFactor = f }
}

func WithNotifier(n func(err error, wait time.Duration)) Option {
	return func(c *Config) { c.Notifier = n }
}

// WithLogNotifier emits a Warn log entry on each retry via MeshKit's logger.Handler.
func WithLogNotifier(log logger.Handler) Option {
	return WithNotifier(func(err error, wait time.Duration) {
		log.Infof("retry: transient error; retrying in %s", wait.Round(time.Millisecond))
		log.Warn(err)
	})
}
