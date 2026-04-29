// Package events provides primitives to interact with the openapi HTTP API.
package events

import (
	"time"

	core "github.com/meshery/schemas/models/core"
)

// Defines values for EventSeverity.
const (
	Alert         EventSeverity = "alert"
	Critical      EventSeverity = "critical"
	Debug         EventSeverity = "debug"
	Emergency     EventSeverity = "emergency"
	Error         EventSeverity = "error"
	Informational EventSeverity = "informational"
	Warning       EventSeverity = "warning"
	Success       EventSeverity = "success"
)

// Defines values for EventStatus.
const (
	Read   EventStatus = "read"
	Unread EventStatus = "unread"
)

// CreatedAt Timestamp when the resource was created.
type CreatedAt = time.Time

// DeletedAt Timestamp when the resource was deleted.
type DeletedAt = time.Time

// Event Defines model for event_trackers
type Event struct {
	// ActedUpon UUID of the entity on which the event was performed.
	ActedUpon core.Uuid `db:"acted_upon" json:"actedUpon"`

	// Action Action taken on the resource.
	Action string `db:"action" json:"action"`

	// Category Resource name on which the operation is invoked.
	Category string `db:"category" json:"category"`

	// CreatedAt Timestamp when the resource was created.
	CreatedAt CreatedAt `db:"created_at" json:"createdAt"`

	// DeletedAt Timestamp when the resource was deleted.
	DeletedAt *DeletedAt `db:"deleted_at" json:"deletedAt,omitempty"`

	// Description A summary/receipt of event that occurred.
	Description string `db:"description" json:"description"`
	ID          ID     `db:"id" json:"id"`

	// Metadata Contains meaningful information, specific to the type of event.
	// Structure of metadata can be different for different events.
	Metadata    map[string]interface{} `db:"metadata" json:"metadata" gorm:"type:bytes;serializer:json"`
	OperationID OperationID            `db:"operation_id" json:"operationId"`

	// Severity A set of seven standard event levels.
	Severity EventSeverity `db:"severity" json:"severity"`

	// Status Status for the event.
	Status   EventStatus `db:"status" json:"status"`
	SystemID SystemID    `db:"system_id" json:"systemId"`

	// UpdatedAt Timestamp when the resource was updated.
	UpdatedAt UpdatedAt `db:"updated_at" json:"updatedAt"`
	UserID    *UserID   `db:"user_id" json:"userId,omitempty"`
}

// EventSeverity A set of seven standard event levels.
type EventSeverity string

// EventStatus Status for the event.
type EventStatus string

// EventsFilter defines model for events_filter.
type EventsFilter struct {
	Action   []string `json:"action"`
	Category []string `json:"category"`
	Limit    int      `json:"limit"`
	Offset   int      `json:"offset"`

	// Order order of sort asc/desc, default is asc
	Order    string      `json:"order"`
	Provider []string    `json:"provider"`
	Search   string      `json:"search"`
	Status   EventStatus `json:"status"`
	Severity []string    `json:"severity"`
	// SortOn Field on which records are sorted
	SortOn string `json:"sortOn"`

	// ActedUpon UUID of the entity on which the event was performed.
	ActedUpon []string `json:"actedUpon"`

	// UserID UUIDs of users to filter events by.
	UserID []string `json:"userId"`

	// SystemID UUIDs of systems to filter events by.
	SystemID []string `json:"systemId"`
}

// ID defines model for id.
type ID = core.Uuid

// OperationID defines model for operation_id.
type OperationID = core.Uuid

// SystemID defines model for system_id.
type SystemID = core.Uuid

// Time defines model for time.
type Time = time.Time

// UpdatedAt Timestamp when the resource was updated.
type UpdatedAt = time.Time

// UserID defines model for user_uuid.
type UserID = core.Uuid
