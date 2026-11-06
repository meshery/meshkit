package events

import (
	"testing"

	"github.com/meshery/meshkit/schema"
)

// The metadata.error MeshKit builds validates against the schemas event
// construct's EventError JSON Schema itself (patterns, enums, closed shape),
// not only the Go helper's cross-field rules.
func TestEventErrorValidatesAgainstSchema(t *testing.T) {
	v, err := schema.New()
	if err != nil {
		t.Fatal(err)
	}
	ref := schema.Ref{Type: "eventerror"}
	if err := v.Register(schema.Registration{Ref: ref, Location: "schemas/constructs/v1beta3/event/api.yml#/components/schemas/EventError"}); err != nil {
		t.Fatal(err)
	}

	paired, err := NewEventError(connectError(), MatchCauseWithConfidence("cluster-unreachable", 0.5))
	if err != nil {
		t.Fatal(err)
	}
	paired.Details = map[string]any{"connectionName": "prod-1"}
	if err := v.ValidateAny(ref, paired); err != nil {
		t.Errorf("paired metadata.error fails the schema: %v", err)
	}

	unpairedErr := legacyMeshkitError()
	unpaired, err := NewEventError(unpairedErr)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.ValidateAny(ref, unpaired); err != nil {
		t.Errorf("unpaired metadata.error fails the schema: %v", err)
	}

	// The schema rejects what NewEventError must never produce: an unknown key.
	if err := v.ValidateAny(ref, map[string]any{"code": "meshkit-1", "Code": "meshkit-1"}); err == nil {
		t.Error("schema accepted an unknown key; the check above proves nothing")
	}
}
