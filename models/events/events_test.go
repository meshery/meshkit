package events

import (
	"testing"

	"github.com/gofrs/uuid"
)

func TestNewEvent(t *testing.T) {
	builder := NewEvent()
	if builder == nil {
		t.Fatal("expected non-nil EventBuilder")
	}
	event := builder.Build()
	if event.Status != Unread {
		t.Errorf("expected default status Unread, got %s", event.Status)
	}
	if event.OperationID == uuid.Nil {
		t.Error("expected non-nil operation ID")
	}
}

func TestEventBuilder_FullChain(t *testing.T) {
	resourceID, err := uuid.FromString("11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("failed to parse resourceID UUID: %v", err)
	}
	userID, err := uuid.FromString("22222222-2222-2222-2222-222222222222")
	if err != nil {
		t.Fatalf("failed to parse userID UUID: %v", err)
	}
	systemID, err := uuid.FromString("33333333-3333-3333-3333-333333333333")
	if err != nil {
		t.Fatalf("failed to parse systemID UUID: %v", err)
	}
	metadata := map[string]interface{}{"key": "value"}

	event := NewEvent().
		ActedUpon(resourceID).
		WithDescription("test event").
		WithCategory("test").
		WithAction("create").
		WithMetadata(metadata).
		WithSeverity(Informational).
		WithStatus(Read).
		FromUser(userID).
		FromSystem(systemID).
		Build()

	if event.ActedUpon != resourceID {
		t.Error("ActedUpon mismatch")
	}
	if event.Description != "test event" {
		t.Errorf("expected description 'test event', got %s", event.Description)
	}
	if event.Category != "test" {
		t.Errorf("expected category 'test', got %s", event.Category)
	}
	if event.Action != "create" {
		t.Errorf("expected action 'create', got %s", event.Action)
	}
	if event.Metadata["key"] != "value" {
		t.Error("metadata mismatch")
	}
	if event.Severity != Informational {
		t.Errorf("expected severity Informational, got %s", event.Severity)
	}
	if event.Status != Read {
		t.Errorf("expected status Read, got %s", event.Status)
	}
	if event.UserID == nil || *event.UserID != userID {
		t.Error("UserID mismatch")
	}
	if event.SystemID != systemID {
		t.Error("SystemID mismatch")
	}
}

func TestEventBuilder_PartialChain(t *testing.T) {
	event := NewEvent().
		WithDescription("partial").
		WithSeverity(Warning).
		Build()

	if event.Description != "partial" {
		t.Errorf("expected description 'partial', got %s", event.Description)
	}
	if event.Severity != Warning {
		t.Errorf("expected severity Warning, got %s", event.Severity)
	}
	if event.Status != Unread {
		t.Errorf("expected default status Unread, got %s", event.Status)
	}
}

func TestIsEventStatusSupported(t *testing.T) {
	tests := []struct {
		name    string
		status  EventStatus
		wantErr bool
	}{
		{"read status", Read, false},
		{"unread status", Unread, false},
		{"invalid status", EventStatus("invalid"), true},
		{"empty status", EventStatus(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &Event{Status: tt.status}
			err := isEventStatusSupported(event)
			if (err != nil) != tt.wantErr {
				t.Errorf("isEventStatusSupported() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEventSeverityConstants(t *testing.T) {
	severities := []EventSeverity{Alert, Critical, Debug, Emergency, Error, Informational, Warning, Success}
	for _, s := range severities {
		if s == "" {
			t.Error("severity constant should not be empty")
		}
	}
}
