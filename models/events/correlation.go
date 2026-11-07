package events

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	core "github.com/meshery/schemas/models/core"
	"go.opentelemetry.io/otel/trace"
)

// WithOperationID sets the id that groups the events of one operation,
// replacing the random one NewEvent mints. Every event of an operation carries
// the same OperationID.
func (e *EventBuilder) WithOperationID(id core.Uuid) *EventBuilder {
	e.event.OperationID = id
	return e
}

// WithIdempotencyKey sets the client-chosen key that makes recording this
// event safe to retry: a create carrying a key its caller already used records
// nothing and is answered with the event the first create recorded. Name the
// occurrence, for example a run id, never the condition (that is
// WithCorrelationKey). Build copies the key into every event it builds, so set
// a new key before building another occurrence from the same builder.
func (e *EventBuilder) WithIdempotencyKey(key string) *EventBuilder {
	e.event.IdempotencyKey = &key
	return e
}

// WithCorrelationKey sets the key of the ongoing condition this event records
// an occurrence of. Qualify it by source and component, then subject and kind
// of condition, for example `meshery-server/k8s-connection/<id>/unreachable`;
// never use an error code alone.
func (e *EventBuilder) WithCorrelationKey(key string) *EventBuilder {
	e.event.CorrelationKey = &key
	return e
}

// WithOccurrenceCount sets how many occurrences of the condition this event
// reports (default 1 when unset). It must be at least 1; Validate checks it.
func (e *EventBuilder) WithOccurrenceCount(count int) *EventBuilder {
	e.event.OccurrenceCount = &count
	return e
}

// WithFirstSeenAt sets when the condition's first reported occurrence was seen.
func (e *EventBuilder) WithFirstSeenAt(at time.Time) *EventBuilder {
	e.event.FirstSeenAt = &at
	return e
}

// WithLastSeenAt sets when the condition's latest reported occurrence was seen.
func (e *EventBuilder) WithLastSeenAt(at time.Time) *EventBuilder {
	e.event.LastSeenAt = &at
	return e
}

// WithResolvedAt sets when the condition cleared. Together with
// WithCorrelationKey it makes the event a resolution event; see Resolves.
func (e *EventBuilder) WithResolvedAt(at time.Time) *EventBuilder {
	e.event.ResolvedAt = &at
	return e
}

// Resolves makes the event the resolution of the condition named by
// correlationKey, cleared at the given time. Following the contract's
// resolution convention, also set the condition's category and a severity and
// description that say what cleared, for example Success and
// "Connection to cluster prod-1 restored". Send it only when the condition
// changes from present to cleared, never on every healthy check.
func (e *EventBuilder) Resolves(correlationKey string, at time.Time) *EventBuilder {
	return e.WithCorrelationKey(correlationKey).WithResolvedAt(at)
}

// WithSource sets which surface emitted the event, for example
// `meshery-server`, `meshery-ui`, `mesheryctl` or `meshery-adapter-istio`.
func (e *EventBuilder) WithSource(source string) *EventBuilder {
	e.event.Source = &source
	return e
}

// WithType sets the CloudEvents-compatible type, a reverse-DNS name such as
// `io.meshery.user.signup`. Category and Action stay as they are.
func (e *EventBuilder) WithType(eventType string) *EventBuilder {
	e.event.Type = &eventType
	return e
}

// WithSpecVersion sets the CloudEvents version the event is compatible with.
// Set SpecVersion10 when the producer sets Source, Type and ID as CloudEvents
// attributes.
func (e *EventBuilder) WithSpecVersion(version EventSpecVersion) *EventBuilder {
	e.event.SpecVersion = &version
	return e
}

// WithTraceContext sets the W3C Trace Context of the event: traceID (32
// lowercase hex digits) names the user-level operation and is the same on
// every event of it, and parentID (16 lowercase hex digits) names the
// sub-operation that emitted this event. An empty parentID leaves ParentID
// unset. Mint them with NewTraceID and NewSpanID, or take them from an
// OpenTelemetry span with WithTraceContextFrom.
func (e *EventBuilder) WithTraceContext(traceID, parentID string) *EventBuilder {
	e.event.TraceID = &traceID
	if parentID != "" {
		e.event.ParentID = &parentID
	} else {
		e.event.ParentID = nil
	}
	return e
}

// WithTraceContextFrom sets TraceID and ParentID from the OpenTelemetry span
// in ctx, so the event joins the trace the operation records. It leaves them
// unchanged when ctx carries no valid span context.
func (e *EventBuilder) WithTraceContextFrom(ctx context.Context) *EventBuilder {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return e
	}
	return e.WithTraceContext(sc.TraceID().String(), sc.SpanID().String())
}

// NewTraceID mints a random W3C trace-id: 32 lowercase hex digits, never all
// zeros. The client that starts an operation mints it once and sets it on
// every event of the operation.
func NewTraceID() (string, error) {
	return randomHex(16)
}

// NewSpanID mints a random W3C parent-id: 16 lowercase hex digits, never all
// zeros, naming one sub-operation.
func NewSpanID() (string, error) {
	return randomHex(8)
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	for {
		if _, err := rand.Read(b); err != nil {
			return "", ErrMintTraceContext(err)
		}
		for _, v := range b {
			if v != 0 {
				return hex.EncodeToString(b), nil
			}
		}
	}
}
