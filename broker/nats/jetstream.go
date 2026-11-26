package nats

import (
	"context"
	"errors"
	"time"

	nats "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	defaultMaxAge     = 24 * time.Hour
	defaultMaxBytes   = 256 << 20 // 256 MiB
	defaultReplicas   = 1
	defaultDuplicates = 2 * time.Minute
)

// JetStreamOptions configures the stream that EnsureStream creates
// Zero values use the defaults above
type JetStreamOptions struct {
	StreamName string
	Subjects   []string
	MaxAge     time.Duration
	MaxBytes   int64
	Replicas   int
	Duplicates time.Duration // window in which a repeated idempotency key is dropped
}

func (options JetStreamOptions) streamConfig() jetstream.StreamConfig {
	config := jetstream.StreamConfig{
		Name:       options.StreamName,
		Subjects:   options.Subjects,
		Storage:    jetstream.FileStorage,
		Retention:  jetstream.LimitsPolicy,
		Discard:    jetstream.DiscardOld,
		MaxAge:     options.MaxAge,
		MaxBytes:   options.MaxBytes,
		Replicas:   options.Replicas,
		Duplicates: options.Duplicates,
	}
	if config.MaxAge == 0 {
		config.MaxAge = defaultMaxAge
	}
	if config.MaxBytes == 0 {
		config.MaxBytes = defaultMaxBytes
	}
	if config.Replicas == 0 {
		config.Replicas = defaultReplicas
	}
	if config.Duplicates == 0 {
		config.Duplicates = defaultDuplicates
	}
	return config
}

// EnsureStream creates the stream, or updates it if it already exists.
// safe to call from several services at startup
func (n *Nats) EnsureStream(ctx context.Context, options JetStreamOptions) error {
	jetStreamClient, err := jetstream.New(n.nc)
	if err != nil {
		return ErrEnsureStream(err)
	}
	if _, err := jetStreamClient.CreateOrUpdateStream(ctx, options.streamConfig()); err != nil {
		if jetStreamMissing(err) {
			return ErrJetStreamUnavailable(err)
		}
		return ErrEnsureStream(err)
	}
	return nil
}

// jetStreamMissing reports whether err means the server has no JetStream.
func jetStreamMissing(err error) bool {
	return errors.Is(err, nats.ErrNoResponders) ||
		errors.Is(err, jetstream.ErrJetStreamNotEnabled) ||
		errors.Is(err, jetstream.ErrJetStreamNotEnabledForAccount)
}
