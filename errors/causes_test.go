package errors

import (
	"encoding/json"
	stderrors "errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/meshery/schemas/models/v1beta3/errorcode"
)

const errSampleCode = "meshkit-11999"

// remReconnect is the shared remediation of the meshery/schemas#1334 acceptance
// sample: declared once, referenced from both causes.
var remReconnect = Remediation{
	ID:           "reconnect",
	Description:  "Reconnect the connection once the cluster is reachable.",
	Action:       "connection.reconnect",
	ActionPolicy: RequiresApproval,
}

func sampleCauses() []Cause {
	return []Cause{
		{
			ID:          "cluster-unreachable",
			Description: "The cluster's API server is not reachable from Meshery Server.",
			Remediations: []Remediation{
				{ID: "check-network", Description: "Check that Meshery Server can reach the cluster's API server address."},
				remReconnect,
			},
		},
		{
			ID:          "credentials-expired",
			Description: "The connection's credentials for the cluster have expired.",
			Remediations: []Remediation{
				{ID: "refresh-credentials", Description: "Refresh the connection's credentials from its credential store.", Action: "connection.refresh-credentials", ActionPolicy: Automatic},
				remReconnect,
			},
		},
	}
}

func sampleV3() *ErrorV2 {
	return NewV3(errSampleCode, Alert,
		[]string{"Unable to connect to the cluster"},
		[]string{"dial tcp 10.0.0.1:6443: i/o timeout"},
		sampleCauses())
}

var (
	wantProbableCause = []string{
		"The cluster's API server is not reachable from Meshery Server.",
		"The connection's credentials for the cluster have expired.",
	}
	wantSuggestedRemediation = []string{
		"Check that Meshery Server can reach the cluster's API server address.",
		"Reconnect the connection once the cluster is reachable.",
		"Refresh the connection's credentials from its credential store.",
	}
)

func TestNewV3DerivesFlatLists(t *testing.T) {
	e := sampleV3()
	if !reflect.DeepEqual(e.ProbableCause, wantProbableCause) {
		t.Errorf("ProbableCause = %q, want %q", e.ProbableCause, wantProbableCause)
	}
	if !reflect.DeepEqual(e.SuggestedRemediation, wantSuggestedRemediation) {
		t.Errorf("SuggestedRemediation = %q, want %q (shared remediation once, first position)", e.SuggestedRemediation, wantSuggestedRemediation)
	}
	if got, want := GetCause(e), strings.Join(wantProbableCause, "."); got != want {
		t.Errorf("GetCause = %q, want %q", got, want)
	}
	if got, want := GetRemedy(e), strings.Join(wantSuggestedRemediation, "."); got != want {
		t.Errorf("GetRemedy = %q, want %q", got, want)
	}
	if GetCode(e) != errSampleCode || GetSeverity(e) != Alert {
		t.Errorf("code/severity = %q/%v", GetCode(e), GetSeverity(e))
	}
}

// The flat lists MeshKit derives are the lists the schemas helper derives from
// the same causes: the two implementations of the contract agree.
func TestFlattenCausesMatchesSchemasHelper(t *testing.T) {
	causes := sampleCauses()
	gotCauses, gotRems := FlattenCauses(causes)
	wantCauses, wantRems := errorcode.FlattenProbableCauses(CausesToSchema(causes))
	if !reflect.DeepEqual(gotCauses, wantCauses) || !reflect.DeepEqual(gotRems, wantRems) {
		t.Errorf("FlattenCauses = %q / %q, schemas helper = %q / %q", gotCauses, gotRems, wantCauses, wantRems)
	}
}

func TestFlattenCausesNeverMergesRemediationsWithoutID(t *testing.T) {
	causes := []Cause{
		{ID: "a", Description: "A", Remediations: []Remediation{{Description: "Same text"}}},
		{ID: "b", Description: "B", Remediations: []Remediation{{Description: "Same text"}}},
	}
	_, rems := FlattenCauses(causes)
	if len(rems) != 2 {
		t.Errorf("remediations without an id were merged: %q", rems)
	}
}

func TestNewV3DefaultsEmptyPolicyToManualAndKeepsUnknown(t *testing.T) {
	e := NewV3(errSampleCode, Alert, nil, nil, []Cause{
		{ID: "a", Description: "A", Remediations: []Remediation{
			{ID: "empty", Description: "Empty policy"},
			{ID: "unknown", Description: "Unknown policy", Action: "x.y", ActionPolicy: ActionPolicy("whenever")},
		}},
	})
	rems := e.Causes[0].Remediations
	if rems[0].ActionPolicy != Manual {
		t.Errorf("empty policy stored as %q, want manual", rems[0].ActionPolicy)
	}
	if rems[1].ActionPolicy != "whenever" {
		t.Errorf("unknown policy was coerced to %q; it must be kept so validation rejects it", rems[1].ActionPolicy)
	}
	if rems[1].EffectivePolicy() != Manual {
		t.Errorf("EffectivePolicy of an unknown policy = %q, want manual", rems[1].EffectivePolicy())
	}
	if err := ValidateCauses(e.Causes); err == nil || !strings.Contains(err.Error(), "whenever") {
		t.Errorf("ValidateCauses did not reject the unknown policy: %v", err)
	}
}

func TestNewV3DoesNotAliasCallerSlices(t *testing.T) {
	causes := []Cause{{ID: "a", Description: "A", Remediations: []Remediation{{ID: "r", Description: "R"}}}}
	e := NewV3(errSampleCode, Alert, nil, nil, causes)
	causes[0].Remediations[0].Description = "changed"
	if e.Causes[0].Remediations[0].Description != "R" {
		t.Error("NewV3 kept a reference to the caller's remediation slice")
	}
	if causes[0].Remediations[0].ActionPolicy != "" {
		t.Error("NewV3 wrote the defaulted policy into the caller's slice")
	}
}

func TestValidateCauses(t *testing.T) {
	if err := ValidateCauses(sampleCauses()); err != nil {
		t.Fatalf("acceptance sample rejected: %v", err)
	}
	tests := []struct {
		name   string
		causes []Cause
		want   string
	}{
		{"bad cause id", []Cause{{ID: "Bad Id", Description: "A", Remediations: []Remediation{{ID: "r", Description: "R"}}}}, "causes[0].id"},
		{"no remediations", []Cause{{ID: "a", Description: "A"}}, "at least one remediation"},
		{"duplicate cause id", []Cause{
			{ID: "a", Description: "A", Remediations: []Remediation{{ID: "r", Description: "R"}}},
			{ID: "a", Description: "B", Remediations: []Remediation{{ID: "s", Description: "S"}}},
		}, "not unique"},
		{"shared id with different text", []Cause{
			{ID: "a", Description: "A", Remediations: []Remediation{{ID: "r", Description: "R"}}},
			{ID: "b", Description: "B", Remediations: []Remediation{{ID: "r", Description: "Different"}}},
		}, "two different remediations"},
		{"automatic without action", []Cause{{ID: "a", Description: "A", Remediations: []Remediation{{ID: "r", Description: "R", ActionPolicy: Automatic}}}}, "needs an action"},
		{"malformed action", []Cause{{ID: "a", Description: "A", Remediations: []Remediation{{ID: "r", Description: "R", Action: "Start Service"}}}}, "dot-separated"},
		{"empty remediation description", []Cause{{ID: "a", Description: "A", Remediations: []Remediation{{ID: "r"}}}}, "description: is empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCauses(tt.causes)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("ValidateCauses = %v, want an error containing %q", err, tt.want)
			}
		})
	}
}

// MeshKit's Cause marshals to exactly the schemas construct's ProbableCause
// JSON, and decodes back without loss.
func TestCauseJSONMatchesSchemaShape(t *testing.T) {
	e := sampleV3()
	mk, err := json.Marshal(e.Causes)
	if err != nil {
		t.Fatal(err)
	}
	schemaJSON, err := json.Marshal(CausesToSchema(e.Causes))
	if err != nil {
		t.Fatal(err)
	}
	var mkAny, schemaAny any
	if err := json.Unmarshal(mk, &mkAny); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(schemaJSON, &schemaAny); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(mkAny, schemaAny) {
		t.Errorf("MeshKit causes JSON\n%s\ndiffers from the schemas ProbableCause JSON\n%s", mk, schemaJSON)
	}
	var decoded []errorcode.ProbableCause
	if err := json.Unmarshal(mk, &decoded); err != nil {
		t.Fatalf("schemas type cannot decode MeshKit causes: %v", err)
	}
	if err := errorcode.ValidateProbableCauses(decoded, "probableCauses"); err != nil {
		t.Errorf("decoded causes fail the schemas rules: %v", err)
	}
	var back []Cause
	if err := json.Unmarshal(mk, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back, e.Causes) {
		t.Errorf("round trip changed the causes:\n%+v\n%+v", back, e.Causes)
	}
	if !strings.Contains(string(mk), `"actionPolicy":"requiresApproval"`) || !strings.Contains(string(mk), `"action":"connection.reconnect"`) {
		t.Errorf("unexpected JSON field names: %s", mk)
	}
}

// A NewV2 error serializes exactly as before: no Causes key.
func TestNewV2JSONUnchanged(t *testing.T) {
	e := NewV2("meshkit-1", Alert, []string{"s"}, []string{"l"}, []string{"c"}, []string{"r"}, nil)
	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"Code":"meshkit-1","Severity":2,"ShortDescription":["s"],"LongDescription":["l"],"ProbableCause":["c"],"SuggestedRemediation":["r"],"AdditionalInfo":null}`
	if string(raw) != want {
		t.Errorf("NewV2 JSON = %s, want %s", raw, want)
	}
	if GetCauses(e) != nil {
		t.Error("GetCauses of a NewV2 error is not nil")
	}
	v3, err := json.Marshal(sampleV3())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(v3), `"Causes":[{"id":"cluster-unreachable"`) {
		t.Errorf("NewV3 JSON does not carry the nested causes: %s", v3)
	}
}

func TestGetCausesThroughWrapping(t *testing.T) {
	wrapped := fmt.Errorf("context: %w", sampleV3())
	if got := GetCauses(wrapped); len(got) != 2 {
		t.Errorf("GetCauses through %%w = %d causes, want 2", len(got))
	}
	if GetCauses(stderrors.New("plain")) != nil {
		t.Error("GetCauses of a plain error is not nil")
	}
	if GetCauses(New("meshkit-1", Alert, nil, nil, nil, nil)) != nil {
		t.Error("GetCauses of an errors.New error is not nil")
	}
}

func TestNewV3WithoutCausesMatchesEmptyNewV2(t *testing.T) {
	e := NewV3("meshkit-1", Critical, []string{"s"}, []string{"l"}, nil)
	if e.Causes != nil {
		t.Error("Causes of a NewV3 error with no causes is not nil")
	}
	if len(e.ProbableCause) != 0 || len(e.SuggestedRemediation) != 0 {
		t.Errorf("flat lists = %q / %q, want empty", e.ProbableCause, e.SuggestedRemediation)
	}
}

func TestSchemaSeverity(t *testing.T) {
	tests := []struct {
		in   Severity
		want errorcode.ErrorSeverity
		ok   bool
	}{
		{Emergency, errorcode.SeverityEmergency, true},
		{Alert, errorcode.SeverityAlert, true},
		{Critical, errorcode.SeverityCritical, true},
		{Fatal, errorcode.SeverityFatal, true},
		{None, "", false},
		{Severity(42), "", false},
	}
	for _, tt := range tests {
		got, ok := tt.in.SchemaSeverity()
		if got != tt.want || ok != tt.ok {
			t.Errorf("Severity(%d).SchemaSeverity() = %q, %v; want %q, %v", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestJoinDescription(t *testing.T) {
	tests := []struct {
		in   []string
		want string
	}{
		{nil, ""},
		{[]string{"None"}, ""},
		{[]string{"Unable to create UUID instance"}, "Unable to create UUID instance"},
		{[]string{"First.", "Second"}, "First. Second"},
		{[]string{"First", " ", "Second."}, "First. Second."},
	}
	for _, tt := range tests {
		if got := JoinDescription(tt.in); got != tt.want {
			t.Errorf("JoinDescription(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestAsErrorV2(t *testing.T) {
	v1 := New("meshkit-1", Fatal, []string{"s"}, []string{"l"}, []string{"c"}, []string{"r"})
	got, ok := AsErrorV2(fmt.Errorf("wrap: %w", v1))
	if !ok || got.Code != "meshkit-1" || got.Severity != Fatal || got.ProbableCause[0] != "c" || got.Causes != nil {
		t.Errorf("AsErrorV2(v1) = %+v, %v", got, ok)
	}
	got, ok = AsErrorV2(sampleV3())
	if !ok || len(got.Causes) != 2 {
		t.Errorf("AsErrorV2(v3) = %+v, %v", got, ok)
	}
	if _, ok := AsErrorV2(stderrors.New("plain")); ok {
		t.Error("AsErrorV2 of a plain error reported ok")
	}
}
