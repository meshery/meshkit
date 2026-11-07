package coder

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	mesherr "github.com/meshery/meshkit/cmd/errorutil/internal/error"
	"github.com/meshery/schemas/models/v1beta3/errorcode"
	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
)

// mixedErrorGo is a package's error.go that defines errors with New, NewV2 and NewV3. The NewV3 errors are the
// meshery/schemas#1334 acceptance sample: two causes that share one remediation (declared once, in another
// file) and each have one of their own, with action policies.
const mixedErrorGo = `package sample

import (
	"github.com/meshery/meshkit/errors"
)

const (
	ErrFlatCode    = "replace_me"
	ErrWithInfoCode = "replace_me"
	ErrNestedCode  = "replace_me"
	ErrLiteralPolicyCode = "replace_me"
)

func ErrFlat(err error) error {
	return errors.New(ErrFlatCode, errors.Alert, []string{"Flat short"}, []string{err.Error()}, []string{"Flat cause"}, []string{"Flat remedy"})
}

func ErrWithInfo(info any) error {
	return errors.NewV2(ErrWithInfoCode, errors.Critical, []string{"V2 short"}, []string{"V2 long"}, []string{"V2 cause one", "V2 cause two"}, []string{"V2 remedy"}, info)
}

func ErrNested(err error) error {
	e := errors.NewV3(ErrNestedCode, errors.Alert,
		[]string{"Unable to connect to the cluster"},
		[]string{"Meshery Server could not reach the cluster."},
		[]errors.Cause{
			{
				ID:          "cluster-unreachable",
				Description: "The cluster's API server is not reachable from Meshery Server.",
				Remediations: []errors.Remediation{
					{ID: "check-network", Description: "Check that Meshery Server can reach the cluster's API server address.", ActionPolicy: errors.Manual},
					remReconnect,
				},
			},
			{
				ID:          "credentials-expired",
				Description: "The connection's credentials for the cluster have expired.",
				Remediations: []errors.Remediation{
					{ID: "refresh-credentials", Description: "Refresh the connection's credentials from its credential store.", Action: "connection.refresh-credentials", ActionPolicy: errors.Automatic},
					remReconnect,
				},
			},
		})
	e.LongDescription = append(e.LongDescription, err.Error())
	return e
}

func ErrLiteralPolicy() error {
	return errors.NewV3(ErrLiteralPolicyCode, errors.None, []string{"Literal policy"}, []string{},
		[]errors.Cause{{ID: "only", Description: "The only cause.", Remediations: []errors.Remediation{{ID: "do-it", Description: "Do it.", Action: "thing.do", ActionPolicy: "requiresApproval"}}}})
}
`

// sharedGo declares the shared remediation in a file other than error.go.
const sharedGo = `package sample

import "github.com/meshery/meshkit/errors"

var remReconnect = errors.Remediation{
	ID:           "reconnect",
	Description:  "Reconnect the connection once the cluster is reachable.",
	Action:       "connection.reconnect",
	ActionPolicy: errors.RequiresApproval,
}
`

func writeSamplePackage(t *testing.T, errorGo string) (root string, flags globalFlags) {
	t.Helper()
	root = t.TempDir()
	pkg := filepath.Join(root, "sample")
	helpers := filepath.Join(root, "helpers")
	for _, dir := range []string{pkg, helpers} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		filepath.Join(pkg, "error.go"):                errorGo,
		filepath.Join(pkg, "shared.go"):               sharedGo,
		filepath.Join(helpers, "component_info.json"): `{"name": "meshery-server", "type": "component", "next_error_code": 1033}`,
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root, globalFlags{rootDir: root, outDir: helpers, infoDir: helpers}
}

// findings returns the warnings and errors errorutil logged, which a component's CI treats as failures.
func findings(hook *logtest.Hook) []string {
	var out []string
	for _, e := range hook.AllEntries() {
		if e.Level <= logrus.WarnLevel {
			out = append(out, e.Level.String()+": "+e.Message)
		}
	}
	return out
}

func readJSON(t *testing.T, path string, into any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, into); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

func TestMixedPackageUpdateAndAnalyze(t *testing.T) {
	_, flags := writeSamplePackage(t, mixedErrorGo)
	hook := logtest.NewGlobal()
	defer hook.Reset()

	// update allocates the four placeholder codes. The only findings are the placeholders seen as duplicates
	// before allocation, which every update of a fresh package reports.
	if err := walkSummarizeExport(flags, true, false); err != nil {
		t.Fatalf("update: %v", err)
	}
	for _, f := range findings(hook) {
		if !strings.Contains(f, "replace_") {
			t.Errorf("update finding: %s", f)
		}
	}
	src, err := os.ReadFile(filepath.Join(flags.rootDir, "sample", "error.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{"meshery-server-1033", "meshery-server-1034", "meshery-server-1035", "meshery-server-1036"} {
		if !strings.Contains(string(src), code) {
			t.Errorf("update did not allocate %s:\n%s", code, src)
		}
	}

	// analyze of the updated package: no findings at all.
	hook.Reset()
	if err := walkSummarizeExport(flags, false, false); err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if f := findings(hook); len(f) != 0 {
		t.Errorf("analyze of a package mixing New, NewV2 and NewV3 reported findings:\n%s", strings.Join(f, "\n"))
	}

	var analysis mesherr.InfoAll
	readJSON(t, filepath.Join(flags.outDir, "errorutil_analyze_errors.json"), &analysis)
	for name, constructor := range map[string]string{"ErrFlatCode": "New", "ErrWithInfoCode": "NewV2", "ErrNestedCode": "NewV3", "ErrLiteralPolicyCode": "NewV3"} {
		if got := analysis.Errors[name]; len(got) != 1 || got[0].Constructor != constructor {
			t.Errorf("%s: analysis %+v, want one %s definition", name, got, constructor)
		}
	}

	// The legacy export keeps its shape for every constructor and gains probableCauses for NewV3 only.
	var export struct {
		ComponentName string                    `json:"component_name"`
		ComponentType string                    `json:"component_type"`
		Errors        map[string]map[string]any `json:"errors"`
	}
	readJSON(t, filepath.Join(flags.outDir, "errorutil_errors_export.json"), &export)
	legacyKeys := []string{"name", "code", "severity", "long_description", "short_description", "probable_cause", "suggested_remediation"}
	for code, entry := range export.Errors {
		for _, k := range legacyKeys {
			if _, ok := entry[k]; !ok {
				t.Errorf("export %s lacks legacy key %s", code, k)
			}
		}
		_, nested := entry["probableCauses"]
		if wantNested := entry["name"] == "ErrNestedCode" || entry["name"] == "ErrLiteralPolicyCode"; nested != wantNested {
			t.Errorf("export %s (%s): probableCauses present = %v, want %v", code, entry["name"], nested, wantNested)
		}
	}
	if got := export.Errors["1034"]["probable_cause"]; got != "V2 cause one\nV2 cause two" {
		t.Errorf("NewV2 details not exported: probable_cause = %q", got)
	}
	nested := export.Errors["1035"]
	if nested["suggested_remediation"] != "Check that Meshery Server can reach the cluster's API server address.\nReconnect the connection once the cluster is reachable.\nRefresh the connection's credentials from its credential store." {
		t.Errorf("NewV3 flat remediation = %q", nested["suggested_remediation"])
	}

	// The index is an ErrorReferenceIndex of every definition, in code order.
	var index errorcode.ErrorReferenceIndex
	readJSON(t, filepath.Join(flags.outDir, "errorutil_errors_index.json"), &index)
	if err := index.Validate(false); err != nil {
		t.Fatalf("index invalid: %v", err)
	}
	if index.SchemaVersion != errorcode.SchemaVersion2 || len(index.Errors) != 4 {
		t.Fatalf("index = version %d with %d errors", index.SchemaVersion, len(index.Errors))
	}
	var codes []string
	for _, d := range index.Errors {
		codes = append(codes, d.Code)
	}
	if want := []string{"meshery-server-1033", "meshery-server-1034", "meshery-server-1035", "meshery-server-1036"}; !reflect.DeepEqual(codes, want) {
		t.Errorf("index codes = %v, want %v", codes, want)
	}

	flat := index.Errors[0]
	if flat.Moniker != "ErrFlatCode" || flat.Component.Type != "component" || flat.NumericCode != "1033" ||
		*flat.Severity != errorcode.SeverityAlert || flat.ProbableCauses != nil || flat.LongDescription != nil ||
		!reflect.DeepEqual(*flat.ProbableCause, []string{"Flat cause"}) {
		t.Errorf("flat definition = %+v", flat)
	}
	if v2 := index.Errors[1]; *v2.Severity != errorcode.SeverityCritical || *v2.LongDescription != "V2 long" {
		t.Errorf("NewV2 definition = %+v", v2)
	}

	d := index.Errors[2]
	if *d.ShortDescription != "Unable to connect to the cluster" || *d.LongDescription != "Meshery Server could not reach the cluster." {
		t.Errorf("descriptions = %q / %q", *d.ShortDescription, *d.LongDescription)
	}
	causes := *d.ProbableCauses
	if len(causes) != 2 || causes[0].ID != "cluster-unreachable" || causes[1].ID != "credentials-expired" {
		t.Fatalf("causes = %+v", causes)
	}
	shared0, shared1 := causes[0].Remediations[1], causes[1].Remediations[1]
	if !reflect.DeepEqual(shared0, shared1) || shared0.ID != "reconnect" || *shared0.Action != "connection.reconnect" ||
		*shared0.ActionPolicy != errorcode.ActionPolicyRequiresApproval {
		t.Errorf("shared remediation = %+v / %+v", shared0, shared1)
	}
	if own := causes[1].Remediations[0]; *own.ActionPolicy != errorcode.ActionPolicyAutomatic || *own.Action != "connection.refresh-credentials" {
		t.Errorf("own remediation = %+v", own)
	}
	// The flat lists derived from the nesting equal today's shape.
	wantCause := []string{"The cluster's API server is not reachable from Meshery Server.", "The connection's credentials for the cluster have expired."}
	wantRemedy := []string{
		"Check that Meshery Server can reach the cluster's API server address.",
		"Reconnect the connection once the cluster is reachable.",
		"Refresh the connection's credentials from its credential store.",
	}
	if !reflect.DeepEqual(*d.ProbableCause, wantCause) || !reflect.DeepEqual(*d.SuggestedRemediation, wantRemedy) {
		t.Errorf("flat lists = %q / %q", *d.ProbableCause, *d.SuggestedRemediation)
	}
	if lit := index.Errors[3]; lit.Severity != nil || *(*lit.ProbableCauses)[0].Remediations[0].ActionPolicy != errorcode.ActionPolicyRequiresApproval {
		t.Errorf("literal policy definition = %+v", lit)
	}
}

const invalidErrorGo = `package sample

import "github.com/meshery/meshkit/errors"

const (
	ErrBadPolicyCode  = "meshery-server-1001"
	ErrNoActionCode   = "meshery-server-1002"
	ErrNonLiteralCode = "meshery-server-1003"
	ErrUnkeyedCode    = "meshery-server-1004"
	ErrUnknownVarCode = "meshery-server-1005"
	ErrRuntimeDescCode = "meshery-server-1006"
	ErrConflictCode   = "meshery-server-1007"
	ErrValidCode      = "meshery-server-1008"
)

var description = "Computed at run time."

func ErrBadPolicy() error {
	return errors.NewV3(ErrBadPolicyCode, errors.Alert, []string{"s"}, []string{},
		[]errors.Cause{{ID: "a", Description: "A.", Remediations: []errors.Remediation{{ID: "r", Description: "R.", ActionPolicy: "whenever"}}}})
}

func ErrNoAction() error {
	return errors.NewV3(ErrNoActionCode, errors.Alert, []string{"s"}, []string{},
		[]errors.Cause{{ID: "a", Description: "A.", Remediations: []errors.Remediation{{ID: "r", Description: "R.", ActionPolicy: errors.Automatic}}}})
}

func ErrNonLiteral() error {
	return errors.NewV3(ErrNonLiteralCode, errors.Alert, []string{"s"}, []string{},
		[]errors.Cause{{ID: "a", Description: description, Remediations: []errors.Remediation{{ID: "r", Description: "R."}}}})
}

func ErrUnkeyed() error {
	return errors.NewV3(ErrUnkeyedCode, errors.Alert, []string{"s"}, []string{},
		[]errors.Cause{{"a", "A.", nil}})
}

func ErrRuntimeDesc(err error) error {
	return errors.NewV3(ErrRuntimeDescCode, errors.Alert, []string{"s", description}, []string{"l", err.Error()},
		[]errors.Cause{{ID: "a", Description: "A.", Remediations: []errors.Remediation{{ID: "r", Description: "R."}}}})
}

func ErrConflict() error {
	return errors.NewV3(ErrConflictCode, errors.Alert, []string{"s"}, []string{},
		[]errors.Cause{
			{ID: "a", Description: "A.", Remediations: []errors.Remediation{{ID: "shared", Description: "Text one."}}},
			{ID: "b", Description: "B.", Remediations: []errors.Remediation{{ID: "shared", Description: "Text two."}}},
		})
}

func ErrValid() error {
	return errors.NewV3(ErrValidCode, errors.Alert, []string{"s"}, []string{},
		[]errors.Cause{{ID: "a", Description: "A.", Remediations: []errors.Remediation{{ID: "r", Description: "R."}}}})
}

func ErrUnknownVar() error {
	return errors.NewV3(ErrUnknownVarCode, errors.Alert, []string{"s"}, []string{},
		[]errors.Cause{{ID: "a", Description: "A.", Remediations: []errors.Remediation{notDeclared}}})
}
`

// errorutil refuses NewV3 definitions that break the contract: it reports each as an error finding, records it
// in the summary, leaves it out of both exports, and fails analyze and update naming each one.
func TestInvalidNewV3DefinitionsAreFindings(t *testing.T) {
	for _, update := range []bool{false, true} {
		t.Run(map[bool]string{false: "analyze", true: "update"}[update], func(t *testing.T) {
			_, flags := writeSamplePackage(t, invalidErrorGo)
			if err := os.WriteFile(filepath.Join(flags.infoDir, "component_info.json"), []byte(`{"name": "meshery-server", "type": "component", "next_error_code": 1009}`), 0o600); err != nil {
				t.Fatal(err)
			}
			hook := logtest.NewGlobal()
			defer hook.Reset()
			invalidNames := []string{"ErrBadPolicyCode", "ErrConflictCode", "ErrNoActionCode", "ErrNonLiteralCode", "ErrRuntimeDescCode", "ErrUnkeyedCode", "ErrUnknownVarCode"}
			err := walkSummarizeExport(flags, update, false)
			if err == nil {
				t.Fatal("walkSummarizeExport succeeded with invalid NewV3 definitions, want an error")
			}
			for _, want := range append([]string{
				"7 invalid errors.NewV3 definition(s)",
				"ErrRuntimeDescCode: shortDescription[1]: use a string literal",
				"\"shared\"",
			}, invalidNames...) {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error lacks %q:\n%v", want, err)
				}
			}
			if strings.Contains(err.Error(), "ErrValidCode") {
				t.Errorf("error names the valid definition:\n%v", err)
			}
			all := strings.Join(findings(hook), "\n")
			for _, want := range []string{
				"'ErrBadPolicyCode' defined with NewV3 cannot be exported: causes[0].remediations[0].actionPolicy: \"whenever\"",
				"'ErrNoActionCode' defined with NewV3 cannot be exported",
				"needs an action",
				"'ErrNonLiteralCode' defined with NewV3 cannot be exported: causes[0].Description: use a string literal",
				"'ErrUnkeyedCode' defined with NewV3 cannot be exported: causes[0]: field 0 is unkeyed",
				"'ErrUnknownVarCode' defined with NewV3 cannot be exported: causes[0].remediations[0]: notDeclared is not a package-level errors.Remediation variable",
				"'ErrRuntimeDescCode' defined with NewV3 cannot be exported: shortDescription[1]: use a string literal",
				"longDescription[1]: use a string literal",
				"'ErrConflictCode' defined with NewV3 cannot be exported",
			} {
				if !strings.Contains(all, want) {
					t.Errorf("findings lack %q:\n%s", want, all)
				}
			}

			var summary struct {
				InvalidDefinitions map[string][]string `json:"invalid_definitions"`
			}
			readJSON(t, filepath.Join(flags.outDir, "errorutil_analyze_summary.json"), &summary)
			if len(summary.InvalidDefinitions) != len(invalidNames) {
				t.Errorf("summary invalid_definitions = %v, want %d entries", summary.InvalidDefinitions, len(invalidNames))
			}

			// Only the valid definition is exported, in both files.
			var export struct {
				Errors map[string]mesherr.Error `json:"errors"`
			}
			readJSON(t, filepath.Join(flags.outDir, "errorutil_errors_export.json"), &export)
			if len(export.Errors) != 1 || export.Errors["1008"].Name != "ErrValidCode" {
				t.Errorf("legacy export = %+v, want only ErrValidCode", export.Errors)
			}
			var index errorcode.ErrorReferenceIndex
			readJSON(t, filepath.Join(flags.outDir, "errorutil_errors_index.json"), &index)
			if len(index.Errors) != 1 || index.Errors[0].Moniker != "ErrValidCode" {
				t.Errorf("index = %+v, want only ErrValidCode", index.Errors)
			}
		})
	}
}
