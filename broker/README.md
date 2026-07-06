# broker

`broker` defines the message-broker abstraction used across the Meshery
ecosystem (Meshery Server, MeshSync, adapters). Producers publish and consumers
subscribe through the `Handler` interface, decoupling callers from the concrete
transport.

Two implementations ship with MeshKit:

- **`broker/nats`** — a NATS-backed handler (`Nats`) used in cluster deployments,
  where MeshSync publishes discovery events to Meshery Broker (NATS) and Meshery
  Server consumes them.
- **`broker/channel`** — an in-process handler (`ChannelBrokerHandler`) backed by
  Go channels, used for embedded/library mode and tests, with no external broker.

## Handler interface

```go
type Handler interface {
	Publish(subject string, message *Message) error
	PublishWithChannel(subject string, msgch chan *Message) error
	Subscribe(subject, queue string, message []byte) error
	SubscribeWithChannel(subject, queue string, msgch chan *Message) error
	Unsubscribe(subject string) error
	Info() string
	DeepCopyObject() Handler
	DeepCopyInto(Handler)
	IsEmpty() bool
	CloseConnection()
	ConnectedEndpoints() []string
}
```

## Unsubscribe

`Unsubscribe(subject string) error` tears down **every** subscription previously
created for `subject` (across all queue groups) and releases the resources they
hold: for the NATS handler the underlying `nats.Subscription`(s); for the channel
handler the per-queue delivery channels (which ends the goroutines started by
`SubscribeWithChannel`).

It is:

- a **no-op** for a subject with no active subscriptions (including a
  nil/uninitialized connection), and
- **safe to call more than once**.

### When to call it

Long-lived, request-scoped subscriptions must be torn down when the request ends,
or the subscription and its delivery goroutine leak for the lifetime of the
process. The canonical case is an interactive session (exec, log stream) keyed by
a unique subject:

```go
subject := fmt.Sprintf("input.%s", sessionID)
if err := handler.SubscribeWithChannel(subject, connName, msgCh); err != nil {
	return err
}
defer handler.Unsubscribe(subject) // release the subscription on session teardown
```

Subscriptions that live for the whole process (a server's long-running consumer)
do not need explicit unsubscription; `CloseConnection` tears everything down.
