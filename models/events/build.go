package events

import (
	"encoding/json"
	"time"

	"github.com/gofrs/uuid"
	"gorm.io/gorm"
)

func (e *Event) BeforeCreate(tx *gorm.DB) (err error) {
    e.ID, _ = uuid.NewV4()
    return
}

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

func (e *EventBuilder) WithCategory(eventCategory string) *EventBuilder {
	e.event.Category = eventCategory
	return e
}

func (e *EventBuilder) WithAction(eventAction string) *EventBuilder {
	e.event.Action = eventAction
	return e
}

func (e *EventBuilder) WithMetadata(metadata map[string]interface{}) *EventBuilder {
	b, _ := json.Marshal(metadata)
	e.event.Metadata = b
	return e
}

func (e *EventBuilder) WithSeverity(severity EventSeverity) *EventBuilder {
	e.event.Severity = severity
	return e
}

func (e *EventBuilder) WithStatus(status string) *EventBuilder {
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
