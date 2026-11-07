package events

import (
	"bytes"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"regexp"

	meshkiterrors "github.com/meshery/meshkit/errors"
	"github.com/meshery/schemas/models/v1beta3/errorcode"
)

// Limits of EventError's lists in the schemas event construct.
const (
	maxEventErrorCauses       = 20
	maxEventErrorRemediations = 50
)

// MatchCause names the cause with the given id as likely for this occurrence.
func MatchCause(causeID string) EventMatchedCause {
	return EventMatchedCause{CauseID: causeID}
}

// MatchCauseWithConfidence names the cause with the given id as likely for this
// occurrence, with how sure the producer is, from 0 (a guess) to 1 (certain).
func MatchCauseWithConfidence(causeID string, confidence float32) EventMatchedCause {
	return EventMatchedCause{CauseID: causeID, Confidence: &confidence}
}

// NewEventError builds the `metadata.error` of an event that reports err, a
// MeshKit error: its code, its severity, its short and long descriptions as
// rendered for this occurrence, and its causes. matched names the causes most
// likely this time, most likely first.
//
// For an error created with errors.NewV3 it carries the nested probableCauses
// and the flat lists derived from them, and each matched cause must be one of
// them. For an errors.New or errors.NewV2 error it carries the flat lists
// alone (an unpaired error), and matched cause ids are kept as given.
//
// The caller may then set Component, ReferenceURL and Details (instance values,
// never secrets, credentials, design contents or request bodies) before
// attaching it with EventBuilder.WithEventError.
//
// It returns ErrNotMeshkitError when err carries no MeshKit error code, and
// ErrInvalidEventError when the result breaks the schemas contract.
func NewEventError(err error, matched ...EventMatchedCause) (EventError, error) {
	mk, ok := meshkiterrors.AsErrorV2(err)
	if !ok || mk.Code == "" || mk.Code == meshkiterrors.NoneString[0] {
		return EventError{}, ErrNotMeshkitError(err)
	}
	ee := EventError{Code: mk.Code}
	if sev, ok := mk.Severity.SchemaSeverity(); ok {
		ee.Severity = &sev
	}
	if short := meshkiterrors.JoinDescription(mk.ShortDescription); short != "" {
		ee.ShortDescription = &short
	}
	if long := meshkiterrors.JoinDescription(mk.LongDescription); long != "" {
		ee.LongDescription = &long
	}

	var probableCause, suggestedRemediation []string
	if len(mk.Causes) > 0 {
		if len(mk.Causes) > maxEventErrorCauses {
			return EventError{}, ErrInvalidEventError(mk.Code, fmt.Errorf("%d probable causes, at most %d", len(mk.Causes), maxEventErrorCauses))
		}
		if verr := meshkiterrors.ValidateCauses(mk.Causes); verr != nil {
			return EventError{}, ErrInvalidEventError(mk.Code, verr)
		}
		causes := meshkiterrors.CausesToSchema(mk.Causes)
		ee.ProbableCauses = &causes
		probableCause, suggestedRemediation = errorcode.FlattenProbableCauses(causes)
	} else {
		probableCause = nonEmptyLines(mk.ProbableCause)
		suggestedRemediation = nonEmptyLines(mk.SuggestedRemediation)
	}
	if len(probableCause) > maxEventErrorCauses || len(suggestedRemediation) > maxEventErrorRemediations {
		return EventError{}, ErrInvalidEventError(mk.Code, fmt.Errorf("%d probable causes and %d remediations, at most %d and %d",
			len(probableCause), len(suggestedRemediation), maxEventErrorCauses, maxEventErrorRemediations))
	}
	if len(probableCause) > 0 {
		ee.ProbableCause = &probableCause
	}
	if len(suggestedRemediation) > 0 {
		ee.SuggestedRemediation = &suggestedRemediation
	}

	if len(matched) > 0 {
		if len(matched) > maxEventErrorCauses {
			return EventError{}, ErrInvalidEventError(mk.Code, fmt.Errorf("%d matched causes, at most %d", len(matched), maxEventErrorCauses))
		}
		for i, m := range matched {
			if !causeIDPattern.MatchString(m.CauseID) {
				return EventError{}, ErrInvalidEventError(mk.Code, fmt.Errorf("matchedCauses[%d].causeId %q is not lowercase words joined by hyphens", i, m.CauseID))
			}
			if m.Confidence != nil && !(*m.Confidence >= 0 && *m.Confidence <= 1) {
				return EventError{}, ErrInvalidEventError(mk.Code, fmt.Errorf("matchedCauses[%d].confidence %v is not between 0 and 1", i, *m.Confidence))
			}
		}
		list := append([]EventMatchedCause(nil), matched...)
		ee.MatchedCauses = &list
	}
	if verr := ee.Validate(); verr != nil {
		return EventError{}, ErrInvalidEventError(mk.Code, verr)
	}
	return ee, nil
}

var causeIDPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// nonEmptyLines drops empty lines and MeshKit's "None" placeholder, which the
// schema's flat lists do not allow.
func nonEmptyLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if l == "" || l == meshkiterrors.NoneString[0] {
			continue
		}
		out = append(out, l)
	}
	return out
}

// WithEventError sets the event's `metadata.error`, keeping every other
// metadata key. Call it after WithMetadata, which replaces the whole map.
func (e *EventBuilder) WithEventError(ee EventError) *EventBuilder {
	if e.event.Metadata == nil {
		e.event.Metadata = map[string]interface{}{}
	}
	e.event.Metadata[MetadataErrorKey] = ee
	return e
}

// WithError sets the event's `metadata.error` from err with NewEventError,
// naming matched as the likely causes. It returns the error NewEventError
// returns, leaving the event unchanged, so a producer decides what to do with
// an error it cannot attach (for example, keep err.Error() in the description).
func (e *EventBuilder) WithError(err error, matched ...EventMatchedCause) (*EventBuilder, error) {
	ee, eeErr := NewEventError(err, matched...)
	if eeErr != nil {
		return e, eeErr
	}
	return e.WithEventError(ee), nil
}

var (
	traceIDPattern  = regexp.MustCompile(`^[0-9a-f]{32}$`)
	parentIDPattern = regexp.MustCompile(`^[0-9a-f]{16}$`)
)

// Validate checks the event's optional correlation, provenance and trace
// fields, and its `metadata.error` when that is an EventError or a decoded
// object in the EventError shape, against the schemas event construct. An event that sets none of them is valid.
// It returns ErrInvalidEvent listing every violation.
func (ev *Event) Validate() error {
	var errs []error
	if ev.CorrelationKey != nil && (len(*ev.CorrelationKey) < 1 || len(*ev.CorrelationKey) > 255) {
		errs = append(errs, fmt.Errorf("correlationKey: length %d is not 1 to 255", len(*ev.CorrelationKey)))
	}
	if ev.OccurrenceCount != nil && *ev.OccurrenceCount < 1 {
		errs = append(errs, fmt.Errorf("occurrenceCount: %d is less than 1", *ev.OccurrenceCount))
	}
	if ev.FirstSeenAt != nil && ev.LastSeenAt != nil && ev.LastSeenAt.Before(*ev.FirstSeenAt) {
		errs = append(errs, fmt.Errorf("lastSeenAt: %s is before firstSeenAt %s", ev.LastSeenAt, ev.FirstSeenAt))
	}
	if ev.Source != nil && (len(*ev.Source) < 1 || len(*ev.Source) > 255) {
		errs = append(errs, fmt.Errorf("source: length %d is not 1 to 255", len(*ev.Source)))
	}
	if ev.Type != nil && (len(*ev.Type) < 1 || len(*ev.Type) > 255) {
		errs = append(errs, fmt.Errorf("type: length %d is not 1 to 255", len(*ev.Type)))
	}
	if ev.SpecVersion != nil && *ev.SpecVersion != SpecVersion10 {
		errs = append(errs, fmt.Errorf("specversion: %q is not %q", *ev.SpecVersion, SpecVersion10))
	}
	if ev.TraceID != nil && (!traceIDPattern.MatchString(*ev.TraceID) || *ev.TraceID == "00000000000000000000000000000000") {
		errs = append(errs, fmt.Errorf("traceId: %q is not 32 lowercase hex digits, not all zeros", *ev.TraceID))
	}
	if ev.ParentID != nil && (!parentIDPattern.MatchString(*ev.ParentID) || *ev.ParentID == "0000000000000000") {
		errs = append(errs, fmt.Errorf("parentId: %q is not 16 lowercase hex digits, not all zeros", *ev.ParentID))
	}
	if ee, ok, derr := metadataEventError(ev.Metadata[MetadataErrorKey]); derr != nil {
		errs = append(errs, fmt.Errorf("metadata.error: %w", derr))
	} else if ok {
		if ee.Code == "" {
			errs = append(errs, fmt.Errorf("metadata.error.code: is empty"))
		}
		if verr := ee.Validate(); verr != nil {
			errs = append(errs, fmt.Errorf("metadata.error: %w", verr))
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return ErrInvalidEvent(stderrors.Join(errs...))
}

// metadataEventError returns the EventError a `metadata.error` value carries:
// the value itself when an EventError, or the strict decoding of a JSON object
// that has the contract's "code" key, as an event read back from storage holds.
// Any other value (a legacy MeshKit error serialized with Go field names) is
// not an EventError and is reported as absent.
func metadataEventError(v interface{}) (EventError, bool, error) {
	switch val := v.(type) {
	case EventError:
		return val, true, nil
	case map[string]interface{}:
		if _, ok := val["code"]; !ok {
			return EventError{}, false, nil
		}
		raw, err := json.Marshal(val)
		if err != nil {
			return EventError{}, false, err
		}
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		var ee EventError
		if err := dec.Decode(&ee); err != nil {
			return EventError{}, false, err
		}
		return ee, true, nil
	}
	return EventError{}, false, nil
}
