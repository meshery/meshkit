// Package events provides primitives to interact with the openapi HTTP API.
package events

import (
	"time"

	core "github.com/meshery/schemas/models/core"
	eventv1beta3 "github.com/meshery/schemas/models/v1beta3/event"
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
	Owner     *Owner    `db:"owner" json:"owner,omitempty"`

	// The fields below are the schemas event construct's condition correlation,
	// provenance and trace context (docs/event-correlation-contract.md in
	// meshery/schemas). All are optional and omitted when unset, so events that
	// set none of them serialize and persist exactly as before. The producer
	// sets them; nothing mints them at persist time.

	// CorrelationKey names the ongoing condition the event records, qualified by
	// source and component, for example
	// `meshery-server/k8s-connection/<id>/unreachable`. Never an error code alone.
	CorrelationKey *string `db:"correlation_key" json:"correlationKey,omitempty" gorm:"column:correlation_key"`
	// OccurrenceCount is how many occurrences of the condition the event reports
	// (on a create) or counts (on a stored event). At least 1.
	OccurrenceCount *int `db:"occurrence_count" json:"occurrenceCount,omitempty" gorm:"column:occurrence_count"`
	// FirstSeenAt is when the condition's first counted occurrence was seen.
	FirstSeenAt *time.Time `db:"first_seen_at" json:"firstSeenAt,omitempty" gorm:"column:first_seen_at"`
	// LastSeenAt is when the condition's latest counted occurrence was seen.
	LastSeenAt *time.Time `db:"last_seen_at" json:"lastSeenAt,omitempty" gorm:"column:last_seen_at"`
	// ResolvedAt is when the condition cleared; nil while it is open. An event
	// carrying CorrelationKey and ResolvedAt is a resolution event.
	ResolvedAt *time.Time `db:"resolved_at" json:"resolvedAt,omitempty" gorm:"column:resolved_at"`
	// Source is which surface emitted the event, for example `meshery-server`,
	// `meshery-ui`, `mesheryctl` or `meshery-adapter-istio` (a CloudEvents source).
	Source *string `db:"source" json:"source,omitempty" gorm:"column:source"`
	// Type is the CloudEvents-compatible type, for example `io.meshery.user.signup`.
	// It sits beside Category and Action and does not replace them.
	Type *string `db:"type" json:"type,omitempty" gorm:"column:type"`
	// SpecVersion is the CloudEvents version the event is compatible with (`1.0`).
	SpecVersion *EventSpecVersion `db:"specversion" json:"specversion,omitempty" gorm:"column:specversion"`
	// TraceID is the W3C Trace Context trace-id of the operation (32 lowercase hex).
	TraceID *string `db:"trace_id" json:"traceId,omitempty" gorm:"column:trace_id"`
	// ParentID is the W3C Trace Context parent-id of the sub-operation that
	// emitted the event (16 lowercase hex).
	ParentID *string `db:"parent_id" json:"parentId,omitempty" gorm:"column:parent_id"`
}

// EventSpecVersion is the schemas event construct's CloudEvents spec version.
type EventSpecVersion = eventv1beta3.EventSpecVersion

// SpecVersion10 is CloudEvents 1.0, the only version the contract defines.
const SpecVersion10 = eventv1beta3.SpecVersion10

// EventError is the schemas event construct's `metadata.error` shape: one
// occurrence of an error definition (docs/error-definition-contract.md in
// meshery/schemas). Build one from a MeshKit error with NewEventError.
type EventError = eventv1beta3.EventError

// EventMatchedCause names one cause of an EventError as likely for this
// occurrence. Build one with MatchCause or MatchCauseWithConfidence.
type EventMatchedCause = eventv1beta3.EventMatchedCause

// MetadataErrorKey is the Event.Metadata key that carries an EventError.
const MetadataErrorKey = "error"

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

	// Owner UUIDs of owners to filter events by.
	Owner []string `json:"owner"`

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

// Owner defines model for owner uuid.
type Owner = core.Uuid
