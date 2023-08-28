package events

import (
	"time"

	"github.com/gofrs/uuid"
)

type EventBuilder struct {
	event Event
}

func NewEvent() *EventBuilder {
	operationId, _ := uuid.NewV4()
	return &EventBuilder{
		event: Event{
			CreatedAt:   time.Now(),
			OperationID: operationId,
		},
	}
}

func (e *EventBuilder) ActedUpon(resource uuid.UUID) *EventBuilder {
	e.event.ActedUpon = resource
	return e
}

func (e *EventBuilder) WithDescription(description string) *EventBuilder {
	e.event.Description = description
	return e
}

func (e *EventBuilder) WithEventType(eventType string) *EventBuilder {
	e.event.EventType = eventType
	return e
}

func (e *EventBuilder) WithMetadata(metadata interface{}) *EventBuilder {
	e.event.Metadata = metadata
	return e
}

func (e *EventBuilder) WithSeverity(severity EventSeverity) *EventBuilder {
	e.event.Severity = severity
	return e
}

func (e *EventBuilder) FromUser(id uuid.UUID) *EventBuilder {
	e.event.UserID = &id
	return e
}

func (e *EventBuilder) FromSystem(id uuid.UUID) *EventBuilder {
	e.event.SystemID = id
	return e
}

func (e *EventBuilder) Build() *Event {
	return &e.event
}
