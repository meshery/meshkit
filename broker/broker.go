package broker

import (
	"context"
	"time"
)

var (
	NotConnected = "not-connected"
)

type PublishInterface interface {
	Publish(string, *Message) error
	PublishWithChannel(string, chan *Message) error
}

type SubscribeInterface interface {
	Subscribe(string, string, []byte) error
	SubscribeWithChannel(string, string, chan *Message) error
	// Unsubscribe tears down every subscription previously created for the given
	// subject (across all queue groups) and releases the resources associated
	// with them - for the NATS handler the underlying nats.Subscription(s), for
	// the channel handler the per-queue delivery channels. It is safe to call for
	// a subject with no active subscriptions (a no-op) and safe to call more than
	// once. Callers that create a subscription for the lifetime of a session
	// (e.g. an interactive exec/log stream keyed by a unique subject) should call
	// Unsubscribe on teardown so the subscription and its delivery goroutine do
	// not leak.
	Unsubscribe(subject string) error
}

type Handler interface {
	PublishInterface
	SubscribeInterface
	Info() string
	DeepCopyObject() Handler
	DeepCopyInto(Handler)
	IsEmpty() bool
	CloseConnection()
	ConnectedEndpoints() []string //To get the IP addresses of connected endpoints
	IsConnected() bool            //Whether the underlying connection is currently live
}

// DurablePublisher is an optional Handler capability, found by type assertion.
// PublishDurable returns once the broker has stored the message.
// idempotencyKey drops repeated publishes, not redeliveries.
type DurablePublisher interface {
	PublishDurable(ctx context.Context, subject string, message *Message, idempotencyKey string) error
}

// DurableSubscriber is an optional Handler capability, found by type assertion.
// Reusing durableName resumes after the last acked message. ctx covers setup
// only; stop the subscription through the returned Subscription.
type DurableSubscriber interface {
	SubscribeDurable(ctx context.Context, subject, durableName string, handler func(*DurableMessage)) (Subscription, error)
}

// DurableMessage is a Message plus its outcome calls. Call exactly one.
type DurableMessage struct {
	*Message
	Ack  func() error                    // processed successfully, do not redeliver
	Nak  func(delay time.Duration) error // transient failure, redeliver after delay
	Term func(reason string) error       // permanent failure, do not redeliver
}

// Subscription stops a durable subscription; the consumer itself is kept.
type Subscription interface {
	Unsubscribe() error // stop now
	Drain() error       // stop after in-flight handlers finish
}
