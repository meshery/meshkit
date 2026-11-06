package events

import (
	"fmt"

	"github.com/meshery/meshkit/errors"
)

const (
	ErrNotMeshkitErrorCode   = "meshkit-11337"
	ErrInvalidEventErrorCode = "meshkit-11338"
	ErrInvalidEventCode      = "meshkit-11339"
	ErrMintTraceContextCode  = "meshkit-11340"
)

// remFixProducer is shared by the causes it answers: declared once, repeated by
// reference, so errorutil exports one remediation under both causes.
var remFixProducer = errors.Remediation{
	ID:          "fix-event-producer",
	Description: "Correct the code that builds the event so it sets the field as the schemas event construct defines it.",
}

// ErrNotMeshkitError is returned when an event's metadata.error is built from
// an error that carries no MeshKit error code.
func ErrNotMeshkitError(err error) error {
	return errors.NewV3(ErrNotMeshkitErrorCode, errors.Alert,
		[]string{"The error is not a MeshKit error"},
		[]string{errorText(err)},
		[]errors.Cause{
			{
				ID:          "plain-error",
				Description: "The error was created without errors.New, errors.NewV2 or errors.NewV3, so it has no error code.",
				Remediations: []errors.Remediation{
					{ID: "use-meshkit-error", Description: "Wrap the failure in a MeshKit error defined in the package's error.go, or report it in the event description instead of metadata.error."},
				},
			},
		})
}

// ErrInvalidEventError is returned when an event's metadata.error would break
// the schemas event construct's EventError contract.
func ErrInvalidEventError(code string, err error) error {
	return errors.NewV3(ErrInvalidEventErrorCode, errors.Alert,
		[]string{"The error cannot be attached to an event"},
		[]string{fmt.Sprintf("Error %s: %s", code, errorText(err))},
		[]errors.Cause{
			{
				ID:          "invalid-definition",
				Description: "The error's causes break the error definition contract, for example a missing id, an unknown action policy, or an automatic remediation without an action.",
				Remediations: []errors.Remediation{
					{ID: "run-errorutil", Description: "Run errorutil analyze on the component and fix the definitions it reports."},
				},
			},
			{
				ID:          "unknown-matched-cause",
				Description: "A matched cause names a cause id the error does not define, or its confidence is outside 0 to 1.",
				Remediations: []errors.Remediation{
					remFixProducer,
				},
			},
		})
}

// ErrInvalidEvent is returned by Event.Validate when an event's correlation,
// provenance or trace fields break the schemas event construct.
func ErrInvalidEvent(err error) error {
	return errors.NewV3(ErrInvalidEventCode, errors.Alert,
		[]string{"The event is not valid"},
		[]string{errorText(err)},
		[]errors.Cause{
			{
				ID:          "invalid-field",
				Description: "A correlation, provenance or trace field of the event has a value the schemas event construct does not allow.",
				Remediations: []errors.Remediation{
					remFixProducer,
				},
			},
		})
}

// ErrMintTraceContext is returned when a trace or span id cannot be minted.
func ErrMintTraceContext(err error) error {
	return errors.NewV3(ErrMintTraceContextCode, errors.Alert,
		[]string{"Unable to mint a trace context id"},
		[]string{errorText(err)},
		[]errors.Cause{
			{
				ID:          "no-entropy",
				Description: "The system's secure random number generator is unavailable.",
				Remediations: []errors.Remediation{
					{ID: "check-random-source", Description: "Check that the operating system's random source is readable by the process."},
				},
			},
		})
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
