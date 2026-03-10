package relvalidation

import (
	"encoding/json"
	"testing"

	"github.com/meshery/schemas/models/v1alpha3/relationship"
	"github.com/meshery/schemas/models/v1beta1/component"
	"github.com/meshery/schemas/models/v1beta1/model"
)

func strPtr(s string) *string { return &s }

func statusPtr(s relationship.RelationshipDefinitionStatus) *relationship.RelationshipDefinitionStatus {
	return &s
}

// relFromJSON constructs a RelationshipDefinition by unmarshaling JSON.
// This decouples tests from generated struct names that change across schema versions.
func relFromJSON(t *testing.T, js string) *relationship.RelationshipDefinition {
	t.Helper()
	var rel relationship.RelationshipDefinition
	if err := json.Unmarshal([]byte(js), &rel); err != nil {
		t.Fatalf("failed to unmarshal relationship JSON: %v", err)
	}
	return &rel
}

// testModelRef is reused across JSON fixtures to satisfy the schema's required model fields.
const testModelRef = `"name": "kubernetes", "displayName": "Kubernetes", "version": "v1.0.0", "model": {"version": "v1.0.0"}, "registrant": {"kind": "artifacthub"}`
const testID = `"00000000-0000-0000-0000-000000000000"`

// baseFields are common required fields for a valid relationship JSON.
const baseFields = `"id": ` + testID + `,
	"kind": "hierarchical",
	"type": "parent",
	"subType": "inventory",
	"schemaVersion": "relationships.meshery.io/v1alpha3",
	"version": "v1.0.0",
	"status": "enabled",
	"model": {` + testModelRef + `}`

const validRelJSON = `{
	` + baseFields + `,
	"selectors": [{
		"allow": {
			"from": [{"kind": "Namespace"}],
			"to":   [{"kind": "Pod"}]
		}
	}]
}`

func validRelationship() *relationship.RelationshipDefinition {
	var rel relationship.RelationshipDefinition
	if err := json.Unmarshal([]byte(validRelJSON), &rel); err != nil {
		panic("validRelJSON is invalid: " + err.Error())
	}
	return &rel
}

func TestValidate_ValidRelationship(t *testing.T) {
	result := Validate(validRelationship())
	if !result.IsValid() {
		t.Errorf("expected valid, got: %s", result.Summary())
	}
	if len(result.Warnings) > 0 {
		t.Errorf("expected no warnings, got: %s", result.Summary())
	}
}

func TestValidate_RequiredFields(t *testing.T) {
	tests := []struct {
		name     string
		modify   func(r *relationship.RelationshipDefinition)
		errField string
	}{
		// kind has an enum constraint; empty string is rejected.
		{"missing kind", func(r *relationship.RelationshipDefinition) { r.Kind = "" }, "kind"},
		// model.name has minLength: 1 in the ModelReference schema.
		{"missing model.name", func(r *relationship.RelationshipDefinition) { r.Model.Name = "" }, "model.name"},
		// Note: type, subType, schemaVersion, and version are required but have no
		// minLength constraint — an empty string satisfies the schema.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rel := validRelationship()
			tt.modify(rel)
			result := Validate(rel)
			if result.IsValid() {
				t.Fatal("expected validation to fail")
			}
			found := false
			for _, e := range result.Errors {
				if e.Field == tt.errField {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected error on field %q, got: %v", tt.errField, result.Errors)
			}
		})
	}
}

func TestValidate_InvalidKind(t *testing.T) {
	rel := validRelationship()
	rel.Kind = "banana"
	result := Validate(rel)
	if result.IsValid() {
		t.Fatal("expected validation to fail for invalid kind")
	}
}

func TestValidate_InvalidStatus(t *testing.T) {
	rel := validRelationship()
	bad := relationship.RelationshipDefinitionStatus("active")
	rel.Status = &bad
	result := Validate(rel)
	if result.IsValid() {
		t.Fatal("expected validation to fail for invalid status")
	}
}

func TestValidate_NilSelectors(t *testing.T) {
	rel := validRelationship()
	rel.Selectors = nil
	result := Validate(rel)
	if result.IsValid() {
		t.Fatal("expected validation to fail for nil selectors")
	}
}

func TestValidate_EmptySelectors(t *testing.T) {
	rel := relFromJSON(t, `{`+baseFields+`, "selectors": []}`)
	result := Validate(rel)
	if result.IsValid() {
		t.Fatal("expected validation to fail for empty selectors")
	}
}

func TestValidate_EmptyFromTo(t *testing.T) {
	rel := relFromJSON(t, `{`+baseFields+`, "selectors": [{"allow": {"from": [], "to": []}}]}`)
	result := Validate(rel)
	if result.IsValid() {
		t.Fatal("expected validation to fail for empty from/to")
	}
	if len(result.Errors) < 2 {
		t.Errorf("expected at least 2 errors (from + to), got %d", len(result.Errors))
	}
}

func TestValidate_MutatorRefMutatedRefParity(t *testing.T) {
	rel := relFromJSON(t, `{`+baseFields+`,
		"selectors": [{
			"allow": {
				"from": [{"kind": "Namespace", "patch": {"mutatorRef": [["displayName"],["metadata","name"]], "mutatedRef": [["configuration","metadata","namespace"]]}}],
				"to": [{"kind": "Pod"}]
			}
		}]
	}`)
	result := Validate(rel)
	if result.IsValid() {
		t.Fatal("expected validation to fail for mismatched mutatorRef/mutatedRef lengths")
	}
}

func TestValidate_MutatorRefMutatedRefParityOK(t *testing.T) {
	rel := relFromJSON(t, `{`+baseFields+`,
		"selectors": [{
			"allow": {
				"from": [{"kind": "Namespace", "patch": {"mutatorRef": [["displayName"]], "mutatedRef": [["configuration","metadata","namespace"]]}}],
				"to": [{"kind": "Pod"}]
			}
		}]
	}`)
	result := Validate(rel)
	if !result.IsValid() {
		t.Errorf("expected valid, got: %s", result.Summary())
	}
}

func TestValidate_TaxonomyWarning(t *testing.T) {
	rel := validRelationship()
	rel.RelationshipType = "unknown_type"
	result := Validate(rel)
	if !result.IsValid() {
		t.Fatal("taxonomy mismatch should be a warning, not an error")
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected a taxonomy warning for unknown type")
	}
}

func TestValidate_SubTypeWarning(t *testing.T) {
	rel := validRelationship()
	rel.Kind = relationship.Edge
	rel.RelationshipType = "binding"
	rel.SubType = "unknown_sub"
	result := Validate(rel)
	if !result.IsValid() {
		t.Fatal("subType mismatch should be a warning, not an error")
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected a subType taxonomy warning")
	}
}

func TestValidate_NilStatusIsOK(t *testing.T) {
	rel := validRelationship()
	rel.Status = nil
	result := Validate(rel)
	if !result.IsValid() {
		t.Errorf("nil status should be valid, got: %s", result.Summary())
	}
}

// --- Tier 2 tests ---

const testSelectorModelK8s = `"model": {` + testModelRef + `}`
const testSelectorModelIstio = `"model": {"name": "istio", "displayName": "Istio", "version": "v1.0.0", "model": {"version": "v1.0.0"}, "registrant": {"kind": "artifacthub"}}`

func TestValidateWithModel_ValidKinds(t *testing.T) {
	rel := relFromJSON(t, `{`+baseFields+`,
		"selectors": [{
			"allow": {
				"from": [{"kind": "Namespace", `+testSelectorModelK8s+`}],
				"to":   [{"kind": "Pod", `+testSelectorModelK8s+`}]
			}
		}]
	}`)

	mdl := &model.ModelDefinition{Name: "kubernetes"}
	comps := []component.ComponentDefinition{
		{Component: component.Component{Kind: "Namespace"}},
		{Component: component.Component{Kind: "Pod"}},
	}
	result := ValidateWithModel(rel, mdl, comps)
	if !result.IsValid() {
		t.Errorf("expected valid, got: %s", result.Summary())
	}
}

func TestValidateWithModel_TypoDetection(t *testing.T) {
	rel := relFromJSON(t, `{`+baseFields+`,
		"selectors": [{
			"allow": {
				"from": [{"kind": "Namespace", `+testSelectorModelK8s+`}],
				"to":   [{"kind": "Deploymnet", `+testSelectorModelK8s+`}]
			}
		}]
	}`)

	mdl := &model.ModelDefinition{Name: "kubernetes"}
	comps := []component.ComponentDefinition{
		{Component: component.Component{Kind: "Namespace"}},
		{Component: component.Component{Kind: "Pod"}},
		{Component: component.Component{Kind: "Deployment"}},
	}
	result := ValidateWithModel(rel, mdl, comps)
	if result.IsValid() {
		t.Fatal("expected validation to fail for typo in component kind")
	}
}

func TestValidateWithModel_WildcardSkipped(t *testing.T) {
	rel := relFromJSON(t, `{`+baseFields+`,
		"selectors": [{
			"allow": {
				"from": [{"kind": "*", `+testSelectorModelK8s+`}],
				"to":   [{"kind": "Pod", `+testSelectorModelK8s+`}]
			}
		}]
	}`)

	mdl := &model.ModelDefinition{Name: "kubernetes"}
	comps := []component.ComponentDefinition{{Component: component.Component{Kind: "Pod"}}}
	result := ValidateWithModel(rel, mdl, comps)
	if !result.IsValid() {
		t.Errorf("wildcard kind should be skipped, got: %s", result.Summary())
	}
}

func TestValidateWithModel_CrossModelSkipped(t *testing.T) {
	rel := relFromJSON(t, `{`+baseFields+`,
		"selectors": [{
			"allow": {
				"from": [{"kind": "SomeOtherKind", `+testSelectorModelIstio+`}],
				"to":   [{"kind": "Pod", `+testSelectorModelK8s+`}]
			}
		}]
	}`)

	mdl := &model.ModelDefinition{Name: "kubernetes"}
	comps := []component.ComponentDefinition{{Component: component.Component{Kind: "Pod"}}}
	result := ValidateWithModel(rel, mdl, comps)
	if !result.IsValid() {
		t.Errorf("cross-model reference should be skipped, got: %s", result.Summary())
	}
}

// --- Result type tests ---

func TestValidationResult_Summary(t *testing.T) {
	result := &ValidationResult{}
	if result.Summary() != "PASS (0 errors, 0 warnings)" {
		t.Errorf("unexpected summary: %s", result.Summary())
	}
}

func TestValidationResult_Error(t *testing.T) {
	result := &ValidationResult{}
	if result.Error() != nil {
		t.Error("expected nil error for valid result")
	}
	result.addError("kind", "missing")
	if result.Error() == nil {
		t.Error("expected non-nil error for invalid result")
	}
}
