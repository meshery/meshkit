package error

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/meshery/meshkit/cmd/errorutil/internal/component"
	"github.com/meshery/meshkit/cmd/errorutil/internal/config"
	mesherrors "github.com/meshery/meshkit/errors"
	"github.com/meshery/meshkit/schema"
	"github.com/meshery/schemas/models/v1beta3/errorcode"
	log "github.com/sirupsen/logrus"
)

// IndexFileSuffix names the nested export: <app>_errors_index.json, the component's error definitions as a
// schemas ErrorReferenceIndex (schemaVersion 2). The legacy <app>_errors_export.json keeps its shape, because the
// Meshery docs build reads it as is.
const IndexFileSuffix = "_errors_index.json"

// severityNames maps the identifiers of MeshKit's Severity constants to the schemas ErrorSeverity. None, and any
// identifier errorutil does not know, has no schema severity.
var severityNames = map[string]errorcode.ErrorSeverity{
	"Emergency": errorcode.SeverityEmergency,
	"Alert":     errorcode.SeverityAlert,
	"Critical":  errorcode.SeverityCritical,
	"Fatal":     errorcode.SeverityFatal,
}

// Definition returns the schemas ErrorDefinition of one exported error. details is the error's analysis entry,
// nil when errorutil found no (or duplicate) details for it.
func Definition(componentInfo *component.Info, numericCode string, name string, details *Error) errorcode.ErrorDefinition {
	d := errorcode.ErrorDefinition{
		Code:        componentInfo.Name + "-" + numericCode,
		Moniker:     name,
		Component:   errorcode.ErrorComponent{Type: componentInfo.Type, Name: componentInfo.Name},
		NumericCode: numericCode,
	}
	if details == nil {
		return d
	}
	if sev, ok := severityNames[details.Severity]; ok {
		d.Severity = &sev
	}
	if s := mesherrors.JoinDescription(details.Lines.ShortDescription); s != "" {
		d.ShortDescription = &s
	}
	if s := mesherrors.JoinDescription(details.Lines.LongDescription); s != "" {
		d.LongDescription = &s
	}
	var cause, remedy []string
	if details.ProbableCauses != nil {
		causes := append([]errorcode.ProbableCause(nil), details.ProbableCauses...)
		d.ProbableCauses = &causes
		cause, remedy = errorcode.FlattenProbableCauses(causes)
	} else {
		cause, remedy = nonEmpty(details.Lines.ProbableCause), nonEmpty(details.Lines.SuggestedRemediation)
	}
	if len(cause) > 0 {
		d.ProbableCause = &cause
	}
	if len(remedy) > 0 {
		d.SuggestedRemediation = &remedy
	}
	return d
}

func nonEmpty(lines []string) []string {
	out := []string{}
	for _, l := range lines {
		if l = strings.TrimSpace(l); l != "" && l != mesherrors.NoneString[0] {
			out = append(out, l)
		}
	}
	return out
}

// ExportIndex writes the component's exported errors as a schemas ErrorReferenceIndex to <app>_errors_index.json.
// exported is the legacy export's entries, keyed by numeric code. A definition that breaks the schemas contract
// (for example a code name the moniker pattern rejects) is reported as an error and left out, so the file always
// validates; referenceUrl is left for the docs build to assign.
func ExportIndex(componentInfo *component.Info, exported map[string]Error, infoAll *InfoAll, outputDir string) error {
	codes := make([]string, 0, len(exported))
	for k := range exported {
		codes = append(codes, k)
	}
	sort.Slice(codes, func(i, j int) bool {
		a, _ := strconv.Atoi(codes[i])
		b, _ := strconv.Atoi(codes[j])
		return a < b
	})
	index := errorcode.ErrorReferenceIndex{SchemaVersion: errorcode.SchemaVersion2, Errors: []errorcode.ErrorDefinition{}}
	for _, k := range codes {
		e := exported[k]
		var details *Error
		if found := infoAll.Errors[e.Name]; len(found) == 1 {
			details = &found[0]
		}
		d := Definition(componentInfo, k, e.Name, details)
		if err := validateDefinition(d); err != nil {
			log.Errorf("error '%s' (code %s) left out of %s: %v", e.Name, d.Code, config.App+IndexFileSuffix, err)
			continue
		}
		index.Errors = append(index.Errors, d)
	}
	if err := validateIndex(index); err != nil {
		return fmt.Errorf("error reference index: %w", err)
	}
	jsn, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}
	fname := filepath.Join(outputDir, config.App+IndexFileSuffix)
	log.Infof("exporting error reference index to %s", fname)
	return os.WriteFile(fname, jsn, 0600)
}

// Locations of the schemas error construct's definitions in the embedded schemas module.
const (
	errorConstructSpec      = "schemas/constructs/v1beta3/error/api.yml"
	errorDefinitionLocation = errorConstructSpec + "#/components/schemas/ErrorDefinition"
	errorIndexLocation      = errorConstructSpec + "#/components/schemas/ErrorReferenceIndex"
)

var (
	definitionRef = schema.Ref{Type: "errordefinition"}
	indexRef      = schema.Ref{Type: "errorreferenceindex"}
)

var (
	errorValidator     *schema.Validator
	errorValidatorErr  error
	errorValidatorOnce sync.Once
)

// errorConstructValidator validates against the schemas error construct embedded in github.com/meshery/schemas,
// so errorutil applies the construct's own JSON Schema (patterns, lengths, enums) rather than a copy of it.
func errorConstructValidator() (*schema.Validator, error) {
	errorValidatorOnce.Do(func() {
		v, err := schema.New()
		if err != nil {
			errorValidatorErr = err
			return
		}
		for _, r := range []schema.Registration{
			{Ref: definitionRef, Location: errorDefinitionLocation},
			{Ref: indexRef, Location: errorIndexLocation},
		} {
			if err := v.Register(r); err != nil {
				errorValidatorErr = err
				return
			}
		}
		errorValidator = v
	})
	return errorValidator, errorValidatorErr
}

// validateDefinition checks one definition against the construct's JSON Schema and its Go helper rules.
func validateDefinition(d errorcode.ErrorDefinition) error {
	v, err := errorConstructValidator()
	if err != nil {
		return err
	}
	if err := v.ValidateAny(definitionRef, d); err != nil {
		return err
	}
	return d.Validate()
}

// validateIndex checks a whole index against the construct's JSON Schema and its Go helper rules.
func validateIndex(index errorcode.ErrorReferenceIndex) error {
	v, err := errorConstructValidator()
	if err != nil {
		return err
	}
	if err := v.ValidateAny(indexRef, index); err != nil {
		return err
	}
	return index.Validate(false)
}
