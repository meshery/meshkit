package relvalidation

import (
	"fmt"
	"strings"

	"github.com/meshery/meshkit/errors"
)

var (
	ErrRelationshipValidationCode = "meshkit-11320"
)

// ValidationError represents a single validation finding.
type ValidationError struct {
	Field    string `json:"field"`
	Message  string `json:"message"`
	Severity string `json:"severity"` // "error" or "warning"
}

// ValidationResult aggregates all findings from relationship validation.
type ValidationResult struct {
	Errors   []ValidationError `json:"errors"`
	Warnings []ValidationError `json:"warnings"`
}

// IsValid returns true if there are no errors (warnings are allowed).
func (vr *ValidationResult) IsValid() bool {
	return len(vr.Errors) == 0
}

// Error returns a meshkit error summarizing all validation errors, or nil if valid.
func (vr *ValidationResult) Error() error {
	if vr.IsValid() {
		return nil
	}
	msgs := make([]string, 0, len(vr.Errors))
	for _, e := range vr.Errors {
		if e.Field != "" {
			msgs = append(msgs, fmt.Sprintf("[%s] %s", e.Field, e.Message))
		} else {
			msgs = append(msgs, e.Message)
		}
	}
	return errors.New(
		ErrRelationshipValidationCode,
		errors.Alert,
		[]string{"Relationship validation failed"},
		msgs,
		[]string{"The relationship definition contains invalid or missing fields"},
		[]string{"Review the errors and fix the relationship definition"},
	)
}

// Summary returns a human-readable summary of the validation result.
func (vr *ValidationResult) Summary() string {
	if vr.IsValid() && len(vr.Warnings) == 0 {
		return "PASS (0 errors, 0 warnings)"
	}
	var b strings.Builder
	if vr.IsValid() {
		fmt.Fprintf(&b, "PASS (0 errors, %d warnings)", len(vr.Warnings))
	} else {
		fmt.Fprintf(&b, "FAIL (%d errors, %d warnings)", len(vr.Errors), len(vr.Warnings))
	}
	if len(vr.Errors) > 0 {
		b.WriteString("\n  Errors:")
		for i, e := range vr.Errors {
			fmt.Fprintf(&b, "\n    %d. [%s] %s: %s", i+1, e.Severity, e.Field, e.Message)
		}
	}
	if len(vr.Warnings) > 0 {
		b.WriteString("\n  Warnings:")
		for i, w := range vr.Warnings {
			fmt.Fprintf(&b, "\n    %d. [%s] %s: %s", i+1, w.Severity, w.Field, w.Message)
		}
	}
	return b.String()
}

func (vr *ValidationResult) addError(field, message string) {
	vr.Errors = append(vr.Errors, ValidationError{
		Field:    field,
		Message:  message,
		Severity: "error",
	})
}

func (vr *ValidationResult) addWarning(field, message string) {
	vr.Warnings = append(vr.Warnings, ValidationError{
		Field:    field,
		Message:  message,
		Severity: "warning",
	})
}
