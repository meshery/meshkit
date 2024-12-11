package events

import (
	"time"

	"github.com/gofrs/uuid"
)

const (
	// Categories
	CategoryPattern = "pattern"
	CategorySystem  = "system"

	// Actions
	ActionCreate = "create"
	ActionDelete = "delete"
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
			Status:      Unread,
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
