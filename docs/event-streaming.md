# Meshery Event Streaming Framework

This document describes how events flow end-to-end through the Meshery
ecosystem — from the producers that emit them, through the in-process
broadcast primitives that live in MeshKit, into Meshery Server's transport
layers, and out to UI clients. It is the reference for contributors who need
to add new producers, consumers, or transports without breaking the existing
guarantees.

The event-streaming framework is split across two repositories:

- **MeshKit** owns the reusable primitives (the in-process broadcasters,
  the canonical `Event` type, the `EventBuilder`).
- **Meshery Server** wires those primitives into HTTP, SSE, and GraphQL
  transports and into the adapter event pipeline.

If you are reading this from inside MeshKit, you are looking at half the
system; the corresponding Meshery code is at
[`server/handlers/events_streamer.go`](https://github.com/meshery/meshery/blob/master/server/handlers/events_streamer.go)
and
[`server/models/event_broadcast.go`](https://github.com/meshery/meshery/blob/master/server/models/event_broadcast.go).

---

## Contents

- [Big picture](#big-picture)
- [The event model](#the-event-model)
- [MeshKit primitives](#meshkit-primitives)
  - [`utils/events.EventStreamer` — system-wide fan-out](#utilseventseventstreamer--system-wide-fan-out)
  - [`utils/broadcast.Broadcaster` — typed multi-listener channel](#utilsbroadcastbroadcaster--typed-multi-listener-channel)
- [How Meshery composes them](#how-meshery-composes-them)
  - [`Handler.EventsBuffer` — server-wide events](#handlereventsbuffer--server-wide-events)
  - [`Handler.EventBroadcaster` — per-user fan-out](#handlereventbroadcaster--per-user-fan-out)
  - [Adapter event ingress](#adapter-event-ingress)
- [Wire transports](#wire-transports)
  - [REST + Server-Sent Events](#rest--server-sent-events)
  - [GraphQL subscription](#graphql-subscription)
  - [Persistence](#persistence)
- [Lifecycle and invariants](#lifecycle-and-invariants)
- [Failure modes](#failure-modes)
- [Testing patterns](#testing-patterns)
- [Adding a new producer](#adding-a-new-producer)
- [Adding a new consumer](#adding-a-new-consumer)
- [Source map](#source-map)

---

## Big picture

```
                                     ┌──────────────────────────────┐
  ┌───────────────────┐  gRPC stream │ Meshery Adapters (Istio,     │
  │ MeshSync          │ ───────────▶ │ Linkerd, Consul, ...)        │
  └───────────────────┘              └──────────────┬───────────────┘
                                                    │ EventsResponse
                                                    ▼
  ┌──────────────────────────────────────────────────────────────────┐
  │                     Meshery Server (Go)                          │
  │                                                                  │
  │  Handler routes / business logic ──┐                             │
  │       │                            │                             │
  │       │ NewEvent().Build()         │ EventsBuffer.Publish(*res)  │
  │       │ persist via Provider       │   (system-wide fan-out)     │
  │       │                            │                             │
  │       ▼                            ▼                             │
  │  ┌──────────────────┐       ┌──────────────────────────────┐     │
  │  │ EventBroadcaster │       │ EventsBuffer                 │     │
  │  │ (per-user)       │       │ (meshkit utils/events        │     │
  │  │ models.Broadcast │       │  EventStreamer, system-wide) │     │
  │  └────────┬─────────┘       └──────────────┬───────────────┘     │
  │           │ ch per user              ch per SSE connection ▼     │
  │           ▼                          listenForCoreEvents          │
  │  ┌───────────────────┐               EventStreamHandler           │
  │  │ GraphQL resolver  │               (server/handlers/            │
  │  │ eventsResolver    │                events_streamer.go)         │
  │  └────────┬──────────┘                          │                 │
  │           │ subscription                        │ text/event-stream │
  └───────────┼─────────────────────────────────────┼─────────────────┘
              ▼                                     ▼
        Meshery UI client                   Meshery UI client
        (Relay subscription)                (EventSource / SSE)
```

Two parallel fan-out paths exist deliberately:

1. **System-wide** events (model registration, publish flows, anything
   that should surface to every connected client of the server) ride on
   MeshKit's `EventStreamer` — wrapped as `Handler.EventsBuffer` — and are
   read by the SSE handler in `EventStreamHandler`.
2. **User-scoped** events (the bulk of UI notifications: deploy succeeded,
   pattern saved, prometheus configured for *this user*) ride on
   Meshery's own `models.Broadcast` — wrapped as `Handler.EventBroadcaster`
   — and are read by the GraphQL `events` subscription via `eventsResolver`.

A single producer can fan out to both: the adapter listener, for
example, persists to the database, publishes to `EventBroadcaster` for
the originating user, and pushes the raw `EventsResponse` onto the SSE
write channel for any system-wide subscribers.

---

## The event model

The canonical event shape is defined in
[`models/events/events.go`](../models/events/events.go) and persisted by
Meshery Server. It is generated from the OpenAPI spec in
[`meshery/schemas`](https://github.com/meshery/schemas) and shared across
the MeshKit, Meshery Server, and UI codebases — never redeclare it
locally.

```go
// models/events/events.go (excerpt)
type Event struct {
    ID          ID                     `db:"id" json:"id"`
    UserID      *UserID                `db:"user_id"      json:"user_id,omitempty"`
    SystemID    SystemID               `db:"system_id"    json:"system_id"`
    OperationID OperationID            `db:"operation_id" json:"operation_id"`

    Category    string                 `db:"category"     json:"category"`
    Action      string                 `db:"action"       json:"action"`
    Description string                 `db:"description"  json:"description"`
    Severity    EventSeverity          `db:"severity"     json:"severity"`
    Status      EventStatus            `db:"status"       json:"status"`

    ActedUpon   core.Uuid              `db:"acted_upon"   json:"acted_upon"`
    Metadata    map[string]interface{} `db:"metadata"     json:"metadata" gorm:"type:bytes;serializer:json"`

    CreatedAt   CreatedAt              `db:"created_at" json:"created_at"`
    UpdatedAt   UpdatedAt              `db:"updated_at" json:"updated_at"`
    DeletedAt   *DeletedAt             `db:"deleted_at" json:"deleted_at,omitempty"`
}

type EventSeverity string // alert | critical | debug | emergency | error | informational | warning | success
type EventStatus   string // read | unread
```

Producers build events through a fluent builder defined in
[`models/events/build.go`](../models/events/build.go):

```go
event := events.
    NewEvent().                                 // assigns OperationID, CreatedAt, Status=Unread
    FromUser(userID).
    FromSystem(systemID).
    WithCategory("pattern").
    WithAction("deploy").
    WithSeverity(events.Informational).
    WithDescription("Pattern deployed successfully.").
    ActedUpon(patternID).
    WithMetadata(map[string]interface{}{"summary": "..."}).
    Build()
```

`Event.ID` is generated automatically in the GORM `BeforeCreate` hook
(see [`models/events/database.go`](../models/events/database.go)) so
producers do not assign it manually.

The same struct flows over the wire (camelCase JSON), into the database
(snake_case columns via `db:` tags), and into the GraphQL `Event` type
exposed by Meshery Server. The casing contract is documented
authoritatively in
[`meshery/schemas/AGENTS.md § Casing rules at a glance`](https://github.com/meshery/schemas/blob/master/AGENTS.md);
do not mix forms within a single resource.

---

## MeshKit primitives

### `utils/events.EventStreamer` — system-wide fan-out

```go
// utils/events/event.go
type EventStreamer struct { /* ... */ }

func NewEventStreamer() *EventStreamer
func (e *EventStreamer) Publish(i interface{})
func (e *EventStreamer) Subscribe(ch chan interface{})
func (e *EventStreamer) Unsubscribe(ch chan interface{})
```

`EventStreamer` is a thin, lock-protected fan-out. `Subscribe` appends a
caller-owned channel to a slice. `Publish` snapshots the slice under a
mutex and spawns one short-lived goroutine per subscriber that performs
a blocking send (`ch <- i`). `Unsubscribe` removes every occurrence of
the channel from the slice.

The semantics that matter for callers:

- **Subscribe does not deduplicate.** A channel subscribed N times is
  fully detached after a single `Unsubscribe`. Callers that want
  "unsubscribe exactly one logical subscription" must track counts
  themselves.
- **Subscribe must complete before Publish to be guaranteed delivery.**
  The fan-out is best-effort — if a publish runs while you are still
  inside `go Subscribe(ch)`, your channel is not in the snapshot and
  loses the message. Subscribe synchronously to close the race; this is
  why `listenForCoreEvents` no longer wraps the call in a goroutine.
- **Publish does not block the caller.** It returns as soon as it has
  spawned the per-subscriber sender goroutines. Those goroutines may
  still be mid-send when `Publish` returns; that is what makes
  `Unsubscribe` insufficient on its own to release a channel — see
  [Lifecycle and invariants](#lifecycle-and-invariants).
- **Subscribers must drain.** Each subscriber channel is buffered to
  whatever the subscriber chose at allocation. If the buffer fills, the
  per-publish sender goroutines block until the buffer drains. A
  subscriber that stops reading without unsubscribing pins those
  goroutines indefinitely.
- **Unsubscribe is safe to call multiple times.** A channel that is not
  subscribed is a no-op.
- **Closing a subscriber channel after Unsubscribe is unsafe today.** A
  per-publish sender goroutine spawned just before `Unsubscribe` ran is
  already committed to `ch <- i`; if you close `ch` it panics. Drain
  with a timeout instead, or wait for the upcoming
  hardening (`recover()` / non-blocking send) in `Publish` itself.

### `utils/broadcast.Broadcaster` — typed multi-listener channel

```go
// utils/broadcast/broadcaster.go (excerpt)
type Broadcaster interface {
    Register(chan<- BroadcastMessage)
    Unregister(chan<- BroadcastMessage)
    Submit(BroadcastMessage)
    Close() error
}

type BroadcastMessage struct {
    Id     uuid.UUID
    Source BroadcastSource // e.g. urn:meshery:operator:sync
    Type   string
    Data   interface{}
    Time   time.Time
}
```

This is a heavier-weight pub/sub used for control-plane signals like
operator status (`OperatorSyncChannel`). It is *not* the path for
user-facing events; new event producers should target one of the
`Event`-typed channels above.

It is documented here for completeness because both packages are
sometimes referred to as "the broadcaster" in conversation.

---

## How Meshery composes them

### `Handler.EventsBuffer` — server-wide events

[`server/handlers/handler_instance.go`](https://github.com/meshery/meshery/blob/master/server/handlers/handler_instance.go)
holds:

```go
type Handler struct {
    // ...
    EventsBuffer *events.EventStreamer
    // ...
}
```

`EventsBuffer` is the singleton MeshKit `EventStreamer`. Producers call
`go h.EventsBuffer.Publish(&res)` to fan out a message to every active
SSE subscriber. Examples:

- [`meshery_pattern_handler.go`](https://github.com/meshery/meshery/blob/master/server/handlers/meshery_pattern_handler.go)
  publishes pattern deploy results.
- [`meshery_filter_handler.go`](https://github.com/meshery/meshery/blob/master/server/handlers/meshery_filter_handler.go)
  publishes filter lifecycle events.

The single subscriber today is `listenForCoreEvents`, started for each
HTTP request that hits `EventStreamHandler`.

### `Handler.EventBroadcaster` — per-user fan-out

[`server/models/event_broadcast.go`](https://github.com/meshery/meshery/blob/master/server/models/event_broadcast.go)
defines a custom multi-tenant fan-out:

```go
type Broadcast struct {
    clients sync.Map      // userUUID -> clients{listeners []chan interface{}, mu *sync.Mutex}
    Name    string
}

func (c *Broadcast) Subscribe(id core.Uuid) (chan interface{}, func())
func (c *Broadcast) Publish(id core.Uuid, data interface{})
```

This is a *per-user* broadcaster: `Subscribe(userID)` returns a buffered
channel and an `unsubscribe()` closure that removes and closes it. The
clients map is keyed by user, so `Publish(userID, evt)` only notifies
subscribers for that user.

Producers publish into it after persisting the event:

```go
event := events.NewEvent(). /* ... */ .Build()
_ = provider.PersistEvent(*event, token)
go h.config.EventBroadcaster.Publish(userID, event)
```

The single consumer today is the GraphQL `events` subscription.

### Adapter event ingress

[`server/handlers/events_streamer.go`](https://github.com/meshery/meshery/blob/master/server/handlers/events_streamer.go)
also runs `listenForAdapterEvents` per connected adapter:

```go
streamClient, _ := mClient.MClient.StreamEvents(ctx, &meshes.EventsRequest{})
for {
    event, err := streamClient.Recv() // gRPC server-stream from the adapter
    if err == io.EOF { return }
    // Translate gRPC EventsResponse -> models/events.Event:
    event := events.NewEvent().FromSystem(...).WithDescription(event.Summary).WithCategory(event.ComponentName)...
    _ = provider.PersistEvent(*event, token)
    ec.Publish(userUUID, event)        // per-user fan-out
    sendStreamEvent(ctx, respChan, raw) // raw EventsResponse onto the SSE write channel
}
```

Adapters therefore feed both fan-out paths plus the database, in that
order. This is the only place in the server where gRPC-sourced events
become Meshery events.

---

## Wire transports

### REST + Server-Sent Events

The HTTP routes touching events are mounted in
[`server/router/server.go`](https://github.com/meshery/meshery/blob/master/server/router/server.go):

| Path                                | Verb     | Handler                    | Purpose |
|-------------------------------------|----------|----------------------------|---------|
| `/api/system/events`                | `POST`   | `ClientEventHandler`       | Accept a user-emitted event, persist it, fan out via `EventBroadcaster`. |
| `/api/system/events`                | `GET`    | `GetAllEvents`             | Paginated history from the database. |
| `/api/system/events/types`          | `GET`    | `GetEventTypes`            | Categories/actions taxonomy. |
| `/api/system/events/status/{id}`    | `PUT`    | `UpdateEventStatus`        | Mark read / unread. |
| `/api/system/events/status/bulk`    | `PUT`    | `BulkUpdateEventStatus`    | Bulk version. |
| `/api/system/events/{id}`           | `DELETE` | `DeleteEvent`              | Soft-delete. |
| `/api/system/events/bulk`           | `DELETE` | `BulkDeleteEvent`          | Bulk version. |
| `/api/system/events/config`         | `GET/PUT`| `ServerEventConfiguration` | Per-server filtering preferences. |

`EventStreamHandler` is the SSE handler that owns the long-lived
`text/event-stream` connection. Per request it:

1. Allocates a per-request `respChan := make(chan []byte, 100)` (the SSE
   write queue) and `newAdaptersChan := make(chan *meshes.MeshClient)`.
2. Starts three goroutines, each scoped to `req.Context()`:
   - one that ranges over `newAdaptersChan` and spawns
     `listenForAdapterEvents` for every newly connected adapter,
   - `listenForCoreEvents(... h.EventsBuffer ...)` which subscribes a
     local `datach chan interface{}` to the system-wide streamer and
     forwards `*meshes.EventsResponse` values onto `respChan`,
   - `writeEventStream(...)` which reads from `respChan` and writes
     `data: <json>\n\n` frames to the response, flushing after each.
3. Polls `prefObj.MeshAdapters` every 5 seconds to refresh adapter
   connections, using a context-aware `select { case <-notify.Done(): case <-time.After(5s) }`
   so the loop exits promptly on disconnect rather than being pinned by
   a bare `time.Sleep`.
4. On exit (client disconnect or error) defers
   `closeAdapterConnections(...)` so every adapter channel is drained
   before the handler returns.

`listenForCoreEvents` itself uses the MeshKit primitives:

```go
datach := make(chan interface{}, 10)
subscribe(eb, datach)                  // synchronous; closes the early-publish race
defer func() {
    eb.Unsubscribe(datach)             // release from broadcaster's fan-out slice
    drainTimer := time.NewTimer(eventStreamDrainTimeout)
    defer drainTimer.Stop()
    for {
        select {
        case <-datach:                 // absorb in-flight sends
        case <-drainTimer.C:
            return
        }
    }
}()
```

The drain phase exists because `Publish` snapshots the subscriber slice
under a mutex and *then* fans out sends in fresh goroutines: any
goroutine that won the scheduler race before `Unsubscribe` ran will
still try to `ch <- i`. Draining with a bounded timeout absorbs that
in-flight traffic without waiting on an idle publisher. Once the
upcoming `Publish` hardening (`recover()` / non-blocking send) lands in
MeshKit, the drain becomes redundant.

`subscribe` is injected as a `subscribeFunc` parameter rather than a
package-level var so tests can swap it without racing under
`t.Parallel()`.

### GraphQL subscription

[`server/internal/graphql/resolver/events.go`](https://github.com/meshery/meshery/blob/master/server/internal/graphql/resolver/events.go)
exposes `eventsResolver`. Its job is to:

1. `Subscribe(userID)` against `EventBroadcaster`, getting back a
   `chan interface{}` and an `unsubscribe()` closure.
2. Spawn a goroutine that translates `*models/events.Event` payloads
   into the GraphQL `model.Event` shape and forwards them on a typed
   `chan *model.Event` exposed to the GraphQL runtime.
3. On `ctx.Done()`, call `unsubscribe()` (which closes the underlying
   listener channel) and close the GraphQL output channel.

This is the path the UI uses for the live notification feed; the SSE
handler covers system-wide events that are not user-scoped.

### Persistence

Both `ClientEventHandler` and `listenForAdapterEvents` call
`provider.PersistEvent(event, token)` *before* publishing. This means:

- Database history is the source of truth; live streams are best-effort.
- A reconnecting UI replays from `GET /api/system/events` and resumes
  the live subscription — there is no replay over SSE / GraphQL.
- A producer that crashes mid-publish has the event durably stored even
  if no live client received it.

Always persist before publishing. Reversing the order risks "ghost"
events that subscribers see but the database never recorded.

---

## Lifecycle and invariants

A correctly behaved subscriber/producer pair satisfies:

1. **Subscribe synchronously, before any publish that must be observed.**
   Both `EventStreamer.Subscribe` and `Broadcast.Subscribe` register a
   channel; both are best-effort against a Publish that runs first.
2. **Buffer subscriber channels.** Every channel handed to `Subscribe`
   is buffered (10 for `listenForCoreEvents`, 1 for
   `Broadcast.Subscribe`). A blocking unbuffered channel will pin
   per-publish sender goroutines.
3. **Drain or unsubscribe before the subscriber goroutine returns.**
   For `EventStreamer`: call `Unsubscribe(ch)` then briefly drain to
   absorb any in-flight `Publish` senders. For `Broadcast`: call the
   `unsubscribe()` closure returned by `Subscribe`; it both removes the
   listener and closes the channel.
4. **Persist before publishing.** Live streams are not durable.
5. **Never close a channel you handed to `EventStreamer`** until the
   `Publish`-hardening change lands; today an in-flight send will
   panic. `Broadcast.Subscribe`'s `unsubscribe()` closure does close
   the channel internally, but only because `Broadcast.Publish` checks
   `IsClosed` first — that pattern is local to that broadcaster.
6. **Treat `Publish` as fire-and-forget.** Both broadcasters deliver
   asynchronously through goroutines; do not assume the message has
   been consumed by the time `Publish` returns.
7. **Use `events.NewEvent().Build()` to construct events** — never
   build the struct directly. The builder fills in `OperationID`,
   `CreatedAt`, and `Status` with sensible defaults.

---

## Failure modes

| Symptom | Likely cause | Fix |
|---|---|---|
| First publish missed by a subscriber | Subscribe ran in a goroutine, lost the race against Publish | Subscribe synchronously |
| Goroutines leaking on every reconnect | Subscriber returned without unsubscribing (or before MeshKit had `Unsubscribe`) | Defer `Unsubscribe(ch)` and drain |
| Panic: send on closed channel | Channel closed while a `Publish` sender goroutine was mid-flight | Don't close subscriber channels handed to `EventStreamer`; wait for the upcoming `Publish` `recover()` hardening |
| SSE clients hung after `Ctrl-C` on the publisher | A subscriber buffer filled and pinned the per-publish goroutines | Increase buffer or actively drain inside the subscriber loop |
| GraphQL subscription stops delivering after one event | `eventsChan` send blocked because the consumer stopped reading; surrounding goroutine wedged on the unbuffered output | Make the GraphQL output channel buffered or read promptly |
| `EventsBuffer.Publish` blocking the producer | Should not happen — `Publish` is non-blocking. If it does, you're calling `Subscribe` on an unbuffered channel | Always buffer the channel passed to `Subscribe` |
| Events visible in DB but never on the live stream | Producer published *before* persist, then process exited | Always persist first, publish second |

---

## Testing patterns

The patterns used in
[`utils/events/event_test.go`](../utils/events/event_test.go) and
[`server/handlers/events_streamer_test.go`](https://github.com/meshery/meshery/blob/master/server/handlers/events_streamer_test.go)
are worth replicating in any new event-touching test:

- **Buffer for the worst case.** When stress-testing
  Subscribe/Publish/Unsubscribe concurrently, size the shared channel
  buffer to `rounds * rounds` so per-publish sender goroutines never
  block past the test (`-race` would flag the leak).
- **Use a readiness channel instead of `time.Sleep`.** Wrap the
  `subscribeFunc` injected into `listenForCoreEvents` so it sends on a
  `subscribed chan struct{}` after the real Subscribe returns; the test
  waits on that signal before publishing.
- **Pre-fill the response buffer to force a blocked send,** then
  cancel the context and assert the goroutine returns. This is how
  `TestListenForCoreEvents_StopsBlockedSendOnCancellation` proves the
  send is interruptible.
- **Use a `safeBuffer`** (mutex-wrapped `bytes.Buffer`) and an atomic
  flush counter when asserting on output written by goroutines.
- **Run race-sensitive tests with `-count=N -race`.** N=50 catches
  most schedule-dependent regressions on developer machines.
- **Inject the subscribe function** rather than mutating a package-level
  var; otherwise concurrent tests in the package race over the var when
  any of them runs with `t.Parallel()`.

---

## Adding a new producer

1. Construct the event via `events.NewEvent()...Build()`.
2. Call `provider.PersistEvent(*event, token)` and check the error —
   wrap it with a MeshKit structured error if you need to surface it to
   the caller.
3. Decide the audience:
   - *This user only* → `go h.config.EventBroadcaster.Publish(userID, event)`.
   - *Every connected client of this server* →
     `go h.EventsBuffer.Publish(&res)`.
   - *Both* → call both, in that order, after persistence.
4. Keep `Publish` calls in `go func()` if the producer is on a hot path
   — `Publish` is fast but spawns goroutines, and detaching keeps the
   request handler trivially cancellable.

## Adding a new consumer

1. Pick the source (`EventsBuffer` for system-wide, `EventBroadcaster`
   for per-user).
2. Allocate a buffered channel (`make(chan interface{}, N)` for
   `EventsBuffer`; `EventBroadcaster.Subscribe` buffers internally).
3. Subscribe synchronously.
4. Defer cleanup:
   - For `EventsBuffer`: `defer eb.Unsubscribe(ch)` and drain on a
     bounded timer, as `listenForCoreEvents` does.
   - For `EventBroadcaster`: defer the `unsubscribe()` closure returned
     by `Subscribe`; it both removes the listener and closes the
     channel.
5. Loop over the channel with a `select { case <-ctx.Done(): return }`
   so the consumer exits when its caller cancels.
6. If you forward to another channel (GraphQL, SSE, websocket), make
   that downstream channel buffered or guard the send with `ctx.Done()`.

---

## Source map

MeshKit:

- [`utils/events/event.go`](../utils/events/event.go) — `EventStreamer`
  (system-wide fan-out used by `EventsBuffer`).
- [`utils/events/event_test.go`](../utils/events/event_test.go) —
  reference patterns for testing concurrent Subscribe/Publish/
  Unsubscribe.
- [`utils/broadcast/broadcaster.go`](../utils/broadcast/broadcaster.go) —
  typed multi-listener channel (operator sync, etc.).
- [`models/events/events.go`](../models/events/events.go) — generated
  `Event` struct and severity/status enums.
- [`models/events/build.go`](../models/events/build.go) — `EventBuilder`.
- [`models/events/database.go`](../models/events/database.go) — GORM
  hooks; assigns `Event.ID`.

Meshery Server (linked):

- [`server/handlers/handler_instance.go`](https://github.com/meshery/meshery/blob/master/server/handlers/handler_instance.go)
  — wires `EventsBuffer` (`*events.EventStreamer`) and
  `EventBroadcaster` (`*models.Broadcast`) onto `Handler`.
- [`server/handlers/events_streamer.go`](https://github.com/meshery/meshery/blob/master/server/handlers/events_streamer.go)
  — `EventStreamHandler`, `listenForCoreEvents`,
  `listenForAdapterEvents`, `writeEventStream`,
  `closeAdapterConnections`.
- [`server/models/event_broadcast.go`](https://github.com/meshery/meshery/blob/master/server/models/event_broadcast.go)
  — per-user `Broadcast` type.
- [`server/internal/graphql/resolver/events.go`](https://github.com/meshery/meshery/blob/master/server/internal/graphql/resolver/events.go)
  — GraphQL `events` subscription, the canonical `EventBroadcaster`
  consumer.
- [`server/router/server.go`](https://github.com/meshery/meshery/blob/master/server/router/server.go)
  — REST routes for events.

Schemas:

- [`meshery/schemas/AGENTS.md § Casing rules at a glance`](https://github.com/meshery/schemas/blob/master/AGENTS.md)
  — authoritative casing contract for the wire shape of `Event`.
