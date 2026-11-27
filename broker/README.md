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
	IsConnected() bool
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

## Durable delivery (JetStream)

`Publish` on core NATS is fire-and-forget: a message sent while no subscriber is
running is lost. The NATS handler can also deliver through a NATS JetStream
**stream**, which stores messages on disk until they expire by age or size,
whether or not a consumer has read them.

Durable delivery is an **optional capability**, not part of `Handler`. Callers
discover it with a type assertion and fall back to `Publish` when it is missing
(the channel handler does not implement it):

```go
type DurablePublisher interface {
	PublishDurable(ctx context.Context, subject string, message *Message, idempotencyKey string) error
}
```

### Setup: create the stream first

A stream must exist before anything is published to its subjects. Messages
published earlier are not stored. Call `EnsureStream` once at startup. It
creates the stream or updates its configuration, so several services can call
it safely. Use the comma-ok type assertion: the channel handler is not a
`*nats.Nats` and a broker without JetStream is a supported setup.

```go
streamReady := false
if natsHandler, ok := handler.(*nats.Nats); ok {
	err := natsHandler.EnsureStream(ctx, nats.JetStreamOptions{
		StreamName: "MESHSYNC_EVENTS",
		Subjects:   []string{"meshery.meshsync.core"},
	})
	switch {
	case err == nil:
		streamReady = true
	case nats.IsJetStreamUnavailable(err):
		// no JetStream on this broker: keep using Publish
	default:
		return err // a real misconfiguration, report it
	}
}
```

Zero-valued options use these defaults: file storage, limits retention,
discard-old, `MaxAge` 24h, `MaxBytes` 256 MiB, `Replicas` 1, `Duplicates` 2m
(capped at `MaxAge`, since the server rejects a dedup window longer than the
retention).

### Publishing

```go
publisher, ok := handler.(broker.DurablePublisher)
if !ok || !streamReady {
	return handler.Publish(subject, message) // core NATS fallback
}
key := uid + "|" + resourceVersion + "|" + string(eventType)
err := publisher.PublishDurable(ctx, subject, message, key)
if nats.IsJetStreamUnavailable(err) {
	return handler.Publish(subject, message) // log the fallback once
}
return err
```

- `PublishDurable` returns once the stream has stored the message.
- `idempotencyKey` is sent as `Nats-Msg-Id`. A repeated key within the stream's
  `Duplicates` window is stored once and still returns `nil`. An empty key turns
  this off. The key covers publisher retries only, not consumer redelivery, so
  consumers must still be idempotent.
- Only call it after `EnsureStream` succeeded. Without a stream, a core
  subscriber on the same subject receives the message but never acknowledges
  it and the call times out with `ErrPublishDurable`.

### Errors

| Error | Severity | Meaning | Caller action |
|---|---|---|---|
| `ErrJetStreamUnavailable` | None | The server has no JetStream, or no stream captures the subject | Fall back to `Publish`. Check with `IsJetStreamUnavailable(err)` |
| `ErrEnsureStream` | Alert | JetStream is present but the stream could not be created or updated (for example a conflicting config, or more `Replicas` than servers) | Report it, do not fall back silently |
| `ErrPublishDurable` | Alert | JetStream is present but the publish failed or timed out | Retry with the same key, or report it |
