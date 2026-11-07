package events

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"math"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	meshkiterrors "github.com/meshery/meshkit/errors"
	eventv1beta3 "github.com/meshery/schemas/models/v1beta3/event"
	"go.opentelemetry.io/otel/trace"
	sqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var (
	opID     = uuid.Must(uuid.FromString("44444444-4444-4444-4444-444444444444"))
	userID   = uuid.Must(uuid.FromString("22222222-2222-2222-2222-222222222222"))
	systemID = uuid.Must(uuid.FromString("33333333-3333-3333-3333-333333333333"))
	t0       = time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	t1       = t0.Add(5 * time.Minute)
	traceHex = "4bf92f3577b34da6a3ce929d0e0e4736"
	spanHex  = "00f067aa0ba902b7"
	condKey  = "meshery-server/k8s-connection/5b0b1c2d-3e4f-4a5b-8c6d-7e8f9a0b1c2d/unreachable"
)

// newFields lists the JSON keys this change adds; none may appear on an event
// that sets none of them.
var newFields = []string{"correlationKey", "occurrenceCount", "firstSeenAt", "lastSeenAt", "resolvedAt", "source", "type", "specversion", "traceId", "parentId"}

func legacyEvent() *Event {
	return NewEvent().WithOperationID(opID).FromOwner(userID).FromSystem(systemID).
		WithCategory("connection").WithAction("update").WithSeverity(Informational).
		WithDescription("legacy").Build()
}

func correlatedEvent() *Event {
	return NewEvent().WithOperationID(opID).FromOwner(userID).FromSystem(systemID).
		WithCategory("connection").WithAction("update").WithSeverity(Error).WithDescription("Cluster prod-1 unreachable").
		WithCorrelationKey(condKey).WithOccurrenceCount(3).WithFirstSeenAt(t0).WithLastSeenAt(t1).
		WithSource("meshery-server").WithType("io.meshery.connection.unreachable").WithSpecVersion(SpecVersion10).
		WithTraceContext(traceHex, spanHex).Build()
}

func TestWithOperationIDReplacesMintedID(t *testing.T) {
	minted := NewEvent().Build().OperationID
	if minted == uuid.Nil {
		t.Fatal("NewEvent minted a nil OperationID")
	}
	if got := NewEvent().WithOperationID(opID).Build().OperationID; got != opID {
		t.Errorf("OperationID = %s, want %s", got, opID)
	}
}

// An event built without the new fields serializes with none of their keys,
// and a payload from a producer that predates them decodes and validates.
func TestLegacyEventUnchanged(t *testing.T) {
	ev := legacyEvent()
	raw, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range newFields {
		if strings.Contains(string(raw), `"`+key+`"`) {
			t.Errorf("legacy event JSON carries %q: %s", key, raw)
		}
	}
	if err := ev.Validate(); err != nil {
		t.Errorf("legacy event invalid: %v", err)
	}

	stored := `{"id":"11111111-1111-1111-1111-111111111111","actedUpon":"00000000-0000-0000-0000-000000000000","action":"update","category":"connection","createdAt":"2026-11-06T12:00:00Z","description":"legacy","metadata":{"error":{"Code":"meshkit-11159","Severity":2,"ProbableCause":["a"],"SuggestedRemediation":["b"]}},"operationId":"44444444-4444-4444-4444-444444444444","severity":"error","status":"unread","systemId":"33333333-3333-3333-3333-333333333333","updatedAt":"2026-11-06T12:00:00Z"}`
	var decoded Event
	if err := json.Unmarshal([]byte(stored), &decoded); err != nil {
		t.Fatalf("legacy payload does not decode: %v", err)
	}
	if decoded.CorrelationKey != nil || decoded.TraceID != nil || decoded.OccurrenceCount != nil {
		t.Error("legacy payload decoded with new fields set")
	}
	if err := decoded.Validate(); err != nil {
		t.Errorf("legacy payload with a Go-field-name metadata.error is invalid: %v", err)
	}
}

// A stored metadata.error in the contract's camelCase shape is decoded and
// validated, so a violation read back from storage is reported.
func TestStoredEventErrorValidated(t *testing.T) {
	for name, metaErr := range map[string]string{
		"unknown matched cause": `{"code":"meshkit-1","probableCauses":[{"id":"a","description":"A","remediations":[{"id":"r","description":"R"}]}],"probableCause":["A"],"suggestedRemediation":["R"],"matchedCauses":[{"causeId":"b"}]}`,
		"severity not a string": `{"code":"meshkit-1","severity":2}`,
		"unknown severity":      `{"code":"meshkit-1","severity":"bogus"}`,
		"confidence above 1":    `{"code":"meshkit-1","matchedCauses":[{"causeId":"a","confidence":5}]}`,
		"bad cause id":          `{"code":"meshkit-1","matchedCauses":[{"causeId":"Not An Id"}]}`,
		"unknown key":           `{"code":"meshkit-1","severity":"error","Bogus":1}`,
		"empty code":            `{"code":"","severity":"error"}`,
		"miscased key":          `{"code":"meshkit-1","SHORTDESCRIPTION":"x"}`,
		"miscased list key":     `{"code":"meshkit-1","MatchedCauses":[{"causeId":"a"}]}`,
		"miscased nested key":   `{"code":"meshkit-1","matchedCauses":[{"CauseId":"a"}]}`,
		"miscased cause key":    `{"code":"meshkit-1","probableCauses":[{"id":"a","Description":"A","remediations":[{"id":"r","description":"R"}]}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			var ev Event
			if err := json.Unmarshal([]byte(`{"metadata":{"error":`+metaErr+`}}`), &ev); err != nil {
				t.Fatal(err)
			}
			if err := ev.Validate(); meshkiterrors.GetCode(err) != ErrInvalidEventCode {
				t.Errorf("stored metadata.error %s: %v", metaErr, err)
			}
		})
	}

	built, err := NewEvent().WithError(connectError(), MatchCauseWithConfidence("cluster-unreachable", 0.8))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(built.Build())
	if err != nil {
		t.Fatal(err)
	}
	var back Event
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if err := back.Validate(); err != nil {
		t.Errorf("stored valid metadata.error rejected: %v", err)
	}
}

func TestCorrelatedEventJSONRoundTrip(t *testing.T) {
	ev := correlatedEvent()
	if err := ev.Validate(); err != nil {
		t.Fatalf("correlated event invalid: %v", err)
	}
	raw, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range newFields[:4] {
		if !strings.Contains(string(raw), `"`+key+`"`) {
			t.Errorf("JSON lacks %q: %s", key, raw)
		}
	}
	var back Event
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back.CorrelationKey, ev.CorrelationKey) || *back.OccurrenceCount != 3 ||
		!back.FirstSeenAt.Equal(t0) || !back.LastSeenAt.Equal(t1) || *back.Source != "meshery-server" ||
		*back.Type != "io.meshery.connection.unreachable" || *back.SpecVersion != SpecVersion10 ||
		*back.TraceID != traceHex || *back.ParentID != spanHex || back.OperationID != opID {
		t.Errorf("round trip changed the event:\n%+v", back)
	}

	// The schemas event construct reads the same JSON with the same values.
	var schemaEvent eventv1beta3.Event
	if err := json.Unmarshal(raw, &schemaEvent); err != nil {
		t.Fatalf("schemas Event cannot decode MeshKit's event: %v", err)
	}
	if *schemaEvent.CorrelationKey != condKey || *schemaEvent.OccurrenceCount != 3 || *schemaEvent.TraceID != traceHex ||
		*schemaEvent.ParentID != spanHex || *schemaEvent.SpecVersion != eventv1beta3.SpecVersion10 ||
		*schemaEvent.Source != "meshery-server" || !schemaEvent.LastSeenAt.Equal(t1) {
		t.Errorf("schemas Event decoded different values: %+v", schemaEvent)
	}
}

func TestResolves(t *testing.T) {
	ev := NewEvent().WithCategory("connection").WithSeverity(Success).
		WithDescription("Connection to cluster prod-1 restored").Resolves(condKey, t1).Build()
	if *ev.CorrelationKey != condKey || !ev.ResolvedAt.Equal(t1) {
		t.Errorf("resolution event = %+v", ev)
	}
}

func TestEventValidateRejects(t *testing.T) {
	zeroTrace := strings.Repeat("0", 32)
	tests := []struct {
		name string
		ev   *Event
		want string
	}{
		{"zero occurrence", NewEvent().WithOccurrenceCount(0).Build(), "occurrenceCount"},
		{"uppercase trace", NewEvent().WithTraceContext(strings.ToUpper(traceHex), spanHex).Build(), "traceId"},
		{"all-zero trace", NewEvent().WithTraceContext(zeroTrace, spanHex).Build(), "traceId"},
		{"short parent", NewEvent().WithTraceContext(traceHex, "abc").Build(), "parentId"},
		{"spec version", NewEvent().WithSpecVersion("0.3").Build(), "specversion"},
		{"empty key", NewEvent().WithCorrelationKey("").Build(), "correlationKey"},
		{"last before first", NewEvent().WithFirstSeenAt(t1).WithLastSeenAt(t0).Build(), "lastSeenAt"},
		{"empty source", NewEvent().WithSource("").Build(), "source"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ev.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Validate = %v, want an error about %s", err, tt.want)
			}
			if meshkiterrors.GetCode(err) != ErrInvalidEventCode {
				t.Errorf("Validate error code = %q, want %q", meshkiterrors.GetCode(err), ErrInvalidEventCode)
			}
		})
	}
}

func TestWithTraceContextEmptyParent(t *testing.T) {
	ev := NewEvent().WithTraceContext(traceHex, spanHex).WithTraceContext(traceHex, "").Build()
	if ev.ParentID != nil {
		t.Errorf("ParentID = %q, want unset", *ev.ParentID)
	}
}

func TestWithTraceContextFrom(t *testing.T) {
	tid, _ := trace.TraceIDFromHex(traceHex)
	sid, _ := trace.SpanIDFromHex(spanHex)
	ctx := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{TraceID: tid, SpanID: sid}))
	ev := NewEvent().WithTraceContextFrom(ctx).Build()
	if ev.TraceID == nil || *ev.TraceID != traceHex || ev.ParentID == nil || *ev.ParentID != spanHex {
		t.Errorf("trace context = %v / %v", ev.TraceID, ev.ParentID)
	}
	if ev := NewEvent().WithTraceContextFrom(context.Background()).Build(); ev.TraceID != nil {
		t.Error("a context without a span set a TraceID")
	}
}

func TestNewTraceAndSpanID(t *testing.T) {
	traceRe := regexp.MustCompile(`^[0-9a-f]{32}$`)
	spanRe := regexp.MustCompile(`^[0-9a-f]{16}$`)
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		tid, err := NewTraceID()
		if err != nil || !traceRe.MatchString(tid) {
			t.Fatalf("NewTraceID = %q, %v", tid, err)
		}
		sid, err := NewSpanID()
		if err != nil || !spanRe.MatchString(sid) {
			t.Fatalf("NewSpanID = %q, %v", sid, err)
		}
		if seen[tid] {
			t.Fatalf("NewTraceID repeated %q", tid)
		}
		seen[tid] = true
		if err := NewEvent().WithTraceContext(tid, sid).Build().Validate(); err != nil {
			t.Fatalf("minted ids invalid: %v", err)
		}
	}
}

// Every new column persists and reads back, and a row written without them
// stores NULLs and reads back unchanged.
func TestEventPersistsNewColumns(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Event{}); err != nil {
		t.Fatal(err)
	}
	for _, col := range []string{"correlation_key", "occurrence_count", "first_seen_at", "last_seen_at", "resolved_at", "source", "type", "specversion", "trace_id", "parent_id"} {
		if !db.Migrator().HasColumn(&Event{}, col) {
			t.Errorf("column %s not created", col)
		}
	}
	legacy, full := legacyEvent(), correlatedEvent()
	if err := db.Create(legacy).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(full).Error; err != nil {
		t.Fatal(err)
	}
	var gotLegacy, gotFull Event
	if err := db.First(&gotLegacy, "id = ?", legacy.ID).Error; err != nil {
		t.Fatal(err)
	}
	if gotLegacy.CorrelationKey != nil || gotLegacy.SpecVersion != nil || gotLegacy.FirstSeenAt != nil || gotLegacy.Description != "legacy" {
		t.Errorf("legacy row read back as %+v", gotLegacy)
	}
	if err := db.First(&gotFull, "id = ?", full.ID).Error; err != nil {
		t.Fatal(err)
	}
	if *gotFull.CorrelationKey != condKey || *gotFull.OccurrenceCount != 3 || !gotFull.FirstSeenAt.Equal(t0) ||
		!gotFull.LastSeenAt.Equal(t1) || *gotFull.SpecVersion != SpecVersion10 || *gotFull.TraceID != traceHex ||
		*gotFull.ParentID != spanHex || *gotFull.Type != "io.meshery.connection.unreachable" {
		t.Errorf("correlated row read back as %+v", gotFull)
	}
	var keyed int64
	if err := db.Model(&Event{}).Where("correlation_key = ? AND resolved_at IS NULL", condKey).Count(&keyed).Error; err != nil || keyed != 1 {
		t.Errorf("open condition lookup = %d, %v", keyed, err)
	}
}

// --- metadata.error ---

var remReconnect = meshkiterrors.Remediation{
	ID: "reconnect", Description: "Reconnect the connection once the cluster is reachable.",
	Action: "connection.reconnect", ActionPolicy: meshkiterrors.RequiresApproval,
}

func connectError() error {
	return meshkiterrors.NewV3("meshery-server-1033", meshkiterrors.Alert,
		[]string{"Unable to connect to the cluster"},
		[]string{"dial tcp 10.0.0.1:6443: i/o timeout"},
		[]meshkiterrors.Cause{
			{ID: "cluster-unreachable", Description: "The cluster's API server is not reachable from Meshery Server.",
				Remediations: []meshkiterrors.Remediation{
					{ID: "check-network", Description: "Check that Meshery Server can reach the cluster's API server address."},
					remReconnect,
				}},
			{ID: "credentials-expired", Description: "The connection's credentials for the cluster have expired.",
				Remediations: []meshkiterrors.Remediation{
					{ID: "refresh-credentials", Description: "Refresh the connection's credentials from its credential store.",
						Action: "connection.refresh-credentials", ActionPolicy: meshkiterrors.Automatic},
					remReconnect,
				}},
		})
}

// The acceptance case of meshery/schemas#1334: an error event naming one
// matched cause validates, and its flat lists equal today's shape.
func TestNewEventErrorPairedWithMatchedCause(t *testing.T) {
	err := connectError()
	ee, eeErr := NewEventError(err, MatchCauseWithConfidence("cluster-unreachable", 0.8))
	if eeErr != nil {
		t.Fatalf("NewEventError: %v", eeErr)
	}
	if !ee.Paired() || len(*ee.ProbableCauses) != 2 {
		t.Fatalf("not paired: %+v", ee)
	}
	if ee.Code != "meshery-server-1033" || string(*ee.Severity) != "alert" || *ee.ShortDescription != "Unable to connect to the cluster" ||
		*ee.LongDescription != "dial tcp 10.0.0.1:6443: i/o timeout" {
		t.Errorf("fields = %+v", ee)
	}
	mk, _ := meshkiterrors.AsErrorV2(err)
	if !reflect.DeepEqual(*ee.ProbableCause, mk.ProbableCause) || !reflect.DeepEqual(*ee.SuggestedRemediation, mk.SuggestedRemediation) {
		t.Errorf("flat lists %q / %q differ from the MeshKit error's %q / %q", *ee.ProbableCause, *ee.SuggestedRemediation, mk.ProbableCause, mk.SuggestedRemediation)
	}
	if len(*ee.SuggestedRemediation) != 3 {
		t.Errorf("shared remediation not deduplicated: %q", *ee.SuggestedRemediation)
	}

	ev, attachErr := NewEvent().WithMetadata(map[string]interface{}{"connectionId": "abc"}).WithError(err, MatchCause("credentials-expired"))
	if attachErr != nil {
		t.Fatal(attachErr)
	}
	built := ev.WithSeverity(Error).Build()
	if built.Metadata["connectionId"] != "abc" {
		t.Error("WithError dropped other metadata keys")
	}
	if verr := built.Validate(); verr != nil {
		t.Errorf("error event invalid: %v", verr)
	}

	// The wire form decodes into the schemas EventError and passes its rules.
	raw, mErr := json.Marshal(built)
	if mErr != nil {
		t.Fatal(mErr)
	}
	var wire struct {
		Metadata struct {
			Error eventv1beta3.EventError `json:"error"`
		} `json:"metadata"`
	}
	if uErr := json.Unmarshal(raw, &wire); uErr != nil {
		t.Fatal(uErr)
	}
	if vErr := wire.Metadata.Error.Validate(); vErr != nil {
		t.Errorf("decoded metadata.error fails the schemas rules: %v", vErr)
	}
	if (*wire.Metadata.Error.MatchedCauses)[0].CauseID != "credentials-expired" {
		t.Errorf("matched cause lost: %+v", wire.Metadata.Error.MatchedCauses)
	}
	if !strings.Contains(string(raw), `"actionPolicy":"automatic"`) {
		t.Errorf("action policy missing from the wire: %s", raw)
	}
}

func legacyMeshkitError() error {
	return meshkiterrors.New("meshkit-11159", meshkiterrors.Critical,
		[]string{"Unable to unmarshal"}, []string{"None"}, []string{"Bad JSON", ""}, []string{"Fix the JSON"})
}

func TestNewEventErrorUnpairedFromLegacyError(t *testing.T) {
	err := legacyMeshkitError()
	ee, eeErr := NewEventError(err, MatchCause("bad-json"))
	if eeErr != nil {
		t.Fatal(eeErr)
	}
	if ee.Paired() || ee.LongDescription != nil {
		t.Errorf("legacy error produced %+v", ee)
	}
	if !reflect.DeepEqual(*ee.ProbableCause, []string{"Bad JSON"}) || !reflect.DeepEqual(*ee.SuggestedRemediation, []string{"Fix the JSON"}) {
		t.Errorf("flat lists = %q / %q", *ee.ProbableCause, *ee.SuggestedRemediation)
	}
	if string(*ee.Severity) != "critical" {
		t.Errorf("severity = %q", *ee.Severity)
	}
	none, _ := NewEventError(meshkiterrors.New("meshkit-1", meshkiterrors.None, nil, nil, nil, nil))
	if none.Severity != nil {
		t.Errorf("None severity mapped to %q", *none.Severity)
	}
}

func TestNewEventErrorRejects(t *testing.T) {
	if _, err := NewEventError(stderrors.New("plain")); meshkiterrors.GetCode(err) != ErrNotMeshkitErrorCode {
		t.Errorf("plain error: %v", err)
	}
	if _, err := NewEventError(connectError(), MatchCause("not-a-cause")); meshkiterrors.GetCode(err) != ErrInvalidEventErrorCode {
		t.Errorf("unknown matched cause: %v", err)
	}
	if _, err := NewEventError(connectError(), MatchCauseWithConfidence("cluster-unreachable", 1.5)); err == nil {
		t.Error("confidence 1.5 accepted")
	}
	if _, err := NewEventError(connectError(), MatchCauseWithConfidence("cluster-unreachable", float32(math.NaN()))); meshkiterrors.GetCode(err) != ErrInvalidEventErrorCode {
		t.Errorf("NaN confidence: %v", err)
	}
	bad := meshkiterrors.NewV3("meshkit-1", meshkiterrors.Alert, nil, nil, []meshkiterrors.Cause{
		{ID: "a", Description: "A", Remediations: []meshkiterrors.Remediation{{ID: "r", Description: "R", ActionPolicy: meshkiterrors.Automatic}}},
	})
	if _, err := NewEventError(bad); err == nil || !strings.Contains(err.Error(), "needs an action") {
		t.Errorf("automatic without action: %v", err)
	}
	b := NewEvent().WithMetadata(map[string]interface{}{"k": "v"})
	if _, err := b.WithError(stderrors.New("plain")); err == nil {
		t.Error("WithError of a plain error returned no error")
	}
	if _, ok := b.Build().Metadata[MetadataErrorKey]; ok {
		t.Error("a failed WithError still set metadata.error")
	}
}
