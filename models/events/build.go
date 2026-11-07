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

// Build returns a distinct *Event carrying the builder's current fields, with
// its Metadata deep-copied, and then mints a fresh ID on the builder. Every
// Build is therefore a distinct event with its own non-nil ID, so a reused
// builder never hands two events the same primary key, and mutating the
// builder after Build does not affect an event already built.
func (e *EventBuilder) Build() *Event {
	event := e.event
	event.Metadata = cloneMetadata(e.event.Metadata)
	e.event.ID = uuid.Must(uuid.NewV4())
	return &event
}

// cloneMetadata deep-copies the nested maps and slices of a metadata map, so
// in-place writes through either copy (such as WithEventError setting
// metadata.error) never reach the other.
func cloneMetadata(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = cloneMetadataValue(v)
	}
	return out
}

func cloneMetadataValue(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		return cloneMetadata(t)
	case []interface{}:
		out := make([]interface{}, len(t))
		for i, item := range t {
			out[i] = cloneMetadataValue(item)
		}
		return out
	default:
		return v
	}
}
