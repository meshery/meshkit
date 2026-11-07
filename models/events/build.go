package events

import (
	"time"

	"github.com/gofrs/uuid"
	core "github.com/meshery/schemas/models/core"
)

type EventBuilder struct {
	event Event
}

// NewEvent starts an event with its ID and OperationID minted (uuid v4),
// CreatedAt set to now and Status Unread. The producer mints the ID here, at
// construction, so a copy broadcast before or instead of the stored row
// carries the same non-nil ID the row is persisted with.
func NewEvent() *EventBuilder {
	return &EventBuilder{
		event: Event{
			ID:          uuid.Must(uuid.NewV4()),
			CreatedAt:   time.Now(),
			OperationID: uuid.Must(uuid.NewV4()),
			Status:      Unread,
		},
	}
}

// WithID sets the ID of the next event Build returns, replacing the one the
// builder minted. Use it only when the producer already minted the ID
// upstream, for example to retry persisting an event it has broadcast.
func (e *EventBuilder) WithID(id core.Uuid) *EventBuilder {
	e.event.ID = id
	return e
}

func (e *EventBuilder) ActedUpon(resource core.Uuid) *EventBuilder {
	e.event.ActedUpon = resource
	return e
}

func (e *EventBuilder) WithDescription(description string) *EventBuilder {
	e.event.Description = description
	return e
}

func (e *EventBuilder) WithCategory(eventCategory string) *EventBuilder {
	e.event.Category = eventCategory
	return e
}

func (e *EventBuilder) WithAction(eventAction string) *EventBuilder {
	e.event.Action = eventAction
	return e
}

func (e *EventBuilder) WithMetadata(metadata map[string]interface{}) *EventBuilder {
	e.event.Metadata = metadata
	return e
}

func (e *EventBuilder) WithSeverity(severity EventSeverity) *EventBuilder {
	e.event.Severity = severity
	return e
}

func (e *EventBuilder) WithStatus(status EventStatus) *EventBuilder {
	e.event.Status = status
	return e
}

func (e *EventBuilder) FromOwner(id core.Uuid) *EventBuilder {
	e.event.Owner = &id
	return e
}

func (e *EventBuilder) FromSystem(id core.Uuid) *EventBuilder {
	e.event.SystemID = id
	return e
}

// Build returns a copy of the event and mints a fresh ID for the next one, so
// every event built from a reused builder is a distinct row with its own ID.
func (e *EventBuilder) Build() *Event {
	event := e.event
	e.event.ID = uuid.Must(uuid.NewV4())
	return &event
}
