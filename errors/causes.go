package errors

import (
	stderrors "errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/meshery/schemas/models/v1beta3/errorcode"
)

// ActionPolicy says whether a client may apply a Remediation's Action on its own.
// It is the schemas error construct's ActionPolicy (meshery/schemas#1334), reused
// so MeshKit and every consumer share one set of values.
type ActionPolicy = errorcode.ActionPolicy

const (
	// Automatic lets a client run the remediation's Action without asking, then tell the person it did.
	Automatic = errorcode.ActionPolicyAutomatic
	// RequiresApproval lets a client run the remediation's Action only after a person approves it.
	RequiresApproval = errorcode.ActionPolicyRequiresApproval
	// Manual means a person applies the remediation; a client shows it and runs nothing.
	// An empty ActionPolicy is Manual.
	Manual = errorcode.ActionPolicyManual
)

// Remediation is one way to answer a Cause, written in source for errors.NewV3.
//
// Its JSON form is the schemas construct's Remediation (`id`, `description`,
// `action`, `actionPolicy`). It is a separate Go type from errorcode.Remediation
// only because the generated type uses pointers for its optional fields, and a
// pointer cannot be written as the string literal errorutil reads from source.
// ToSchema converts it.
//
// A remediation that answers more than one cause is repeated under each cause
// with the same ID and identical fields; declare it once as a package-level
// variable and reference it from each cause.
type Remediation struct {
	// ID is stable once published: lowercase words joined by hyphens, for example `start-service`.
	ID string `json:"id"`
	// Description says what to do, written for a person.
	Description string `json:"description"`
	// Action optionally names what a client can run to apply the remediation,
	// as dot-separated lowercase words, for example `service.start`.
	Action string `json:"action,omitempty"`
	// ActionPolicy says whether a client may run Action on its own. Empty is Manual.
	// Automatic and RequiresApproval need an Action.
	ActionPolicy ActionPolicy `json:"actionPolicy,omitempty"`
}

// Cause is one probable cause of an error with the remediations that answer it,
// written in source for errors.NewV3. Its JSON form is the schemas construct's
// ProbableCause (`id`, `description`, `remediations`).
type Cause struct {
	// ID is stable once published and unique within the error: lowercase words
	// joined by hyphens, for example `service-not-running`. Events name the
	// causes that applied to an occurrence by this ID.
	ID string `json:"id"`
	// Description says what may have caused the error.
	Description string `json:"description"`
	// Remediations answer this cause, the most likely fix first. Never empty.
	Remediations []Remediation `json:"remediations"`
}

// EffectivePolicy is the policy a client must act on: ActionPolicy when it is
// one of Automatic, RequiresApproval or Manual, and Manual otherwise, so an
// empty or unknown policy never runs anything.
func (r Remediation) EffectivePolicy() ActionPolicy {
	switch r.ActionPolicy {
	case Automatic, RequiresApproval:
		return r.ActionPolicy
	default:
		return Manual
	}
}

// ToSchema returns the remediation as the schemas construct's Remediation.
// An empty ActionPolicy is written as Manual, and an empty Action is omitted.
func (r Remediation) ToSchema() errorcode.Remediation {
	out := errorcode.Remediation{ID: r.ID, Description: r.Description}
	if r.Action != "" {
		action := r.Action
		out.Action = &action
	}
	policy := r.ActionPolicy
	if policy == "" {
		policy = Manual
	}
	out.ActionPolicy = &policy
	return out
}

// ToSchema returns the cause as the schemas construct's ProbableCause.
func (c Cause) ToSchema() errorcode.ProbableCause {
	out := errorcode.ProbableCause{ID: c.ID, Description: c.Description, Remediations: make([]errorcode.Remediation, 0, len(c.Remediations))}
	for _, r := range c.Remediations {
		out.Remediations = append(out.Remediations, r.ToSchema())
	}
	return out
}

// CausesToSchema returns causes as the schemas construct's ProbableCause list,
// or nil when there are none.
func CausesToSchema(causes []Cause) []errorcode.ProbableCause {
	if len(causes) == 0 {
		return nil
	}
	out := make([]errorcode.ProbableCause, 0, len(causes))
	for _, c := range causes {
		out = append(out, c.ToSchema())
	}
	return out
}

// FlattenCauses derives the flat lists earlier consumers read from nested causes:
// probableCause is each cause's Description in order, and suggestedRemediation
// is each remediation's Description walking the causes in order, a remediation
// ID kept the first time it appears. Remediations without an ID are never
// merged. This is errorcode.FlattenProbableCauses for MeshKit's source types.
func FlattenCauses(causes []Cause) (probableCause, suggestedRemediation []string) {
	probableCause = make([]string, 0, len(causes))
	suggestedRemediation = []string{}
	seen := map[string]bool{}
	for _, cause := range causes {
		probableCause = append(probableCause, cause.Description)
		for _, r := range cause.Remediations {
			if r.ID != "" {
				if seen[r.ID] {
					continue
				}
				seen[r.ID] = true
			}
			suggestedRemediation = append(suggestedRemediation, r.Description)
		}
	}
	return probableCause, suggestedRemediation
}

var (
	idPattern     = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
	actionPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*(\.[a-z][a-z0-9-]*)+$`)
)

// IsID reports whether s is a well-formed cause or remediation id: lowercase
// words joined by hyphens, for example `cluster-unreachable`.
func IsID(s string) bool {
	return idPattern.MatchString(s)
}

// ValidateCauses checks causes against the schemas error construct
// (docs/error-definition-contract.md in meshery/schemas): ids present and well
// formed, cause ids unique, a repeated remediation id identical wherever it
// appears, every cause with at least one remediation, a known ActionPolicy,
// and an Action behind every Automatic or RequiresApproval policy.
// errorutil runs the same checks on NewV3 call sites, so a definition that
// fails here fails the component's error analysis.
func ValidateCauses(causes []Cause) error {
	var errs []error
	for i, cause := range causes {
		path := fmt.Sprintf("causes[%d]", i)
		if !IsID(cause.ID) {
			errs = append(errs, fmt.Errorf("%s.id: %q is not lowercase words joined by hyphens", path, cause.ID))
		}
		if strings.TrimSpace(cause.Description) == "" {
			errs = append(errs, fmt.Errorf("%s.description: is empty", path))
		}
		if len(cause.Remediations) == 0 {
			errs = append(errs, fmt.Errorf("%s.remediations: a cause names at least one remediation", path))
		}
		for j, r := range cause.Remediations {
			rpath := fmt.Sprintf("%s.remediations[%d]", path, j)
			if !IsID(r.ID) {
				errs = append(errs, fmt.Errorf("%s.id: %q is not lowercase words joined by hyphens", rpath, r.ID))
			}
			if strings.TrimSpace(r.Description) == "" {
				errs = append(errs, fmt.Errorf("%s.description: is empty", rpath))
			}
			switch r.ActionPolicy {
			case "", Manual, Automatic, RequiresApproval:
			default:
				errs = append(errs, fmt.Errorf("%s.actionPolicy: %q is not automatic, requiresApproval or manual", rpath, r.ActionPolicy))
			}
			if r.Action != "" && !actionPattern.MatchString(r.Action) {
				errs = append(errs, fmt.Errorf("%s.action: %q is not dot-separated lowercase words", rpath, r.Action))
			}
		}
	}
	if len(errs) > 0 {
		return stderrors.Join(errs...)
	}
	// Uniqueness, shared-remediation identity and action-behind-policy are the
	// schemas helper's rules; run them on the converted form so both agree.
	return errorcode.ValidateProbableCauses(CausesToSchema(causes), "causes")
}

// NewV3 returns a MeshKit error whose probable causes each carry the remediations
// that answer them (meshery/meshkit#1138, meshery/schemas#1334).
//
// The flat ProbableCause and SuggestedRemediation fields, and so GetCause,
// GetRemedy, Error() and every existing reader, are derived from causes with
// FlattenCauses. An empty ActionPolicy is stored as Manual. An unknown policy is
// kept as written, never coerced: ValidateCauses and errorutil reject it, and
// EffectivePolicy treats it as Manual so nothing runs by mistake.
//
// As with New and NewV2, pass the code constant, and write every description,
// id, action and policy as a string literal (or a package-level Remediation
// variable for a shared remediation) so errorutil can export the definition.
//
// Example:
//
//	var remReconnect = errors.Remediation{ID: "reconnect", Description: "Reconnect once the cluster is reachable.",
//	    Action: "connection.reconnect", ActionPolicy: errors.RequiresApproval}
//
//	errors.NewV3(ErrConnectCode, errors.Alert,
//	    []string{"Unable to connect to the cluster"},
//	    []string{"Meshery Server could not reach the cluster's API server"},
//	    []errors.Cause{
//	        {ID: "cluster-unreachable", Description: "The cluster's API server is not reachable.",
//	            Remediations: []errors.Remediation{
//	                {ID: "check-network", Description: "Check the network path to the API server."},
//	                remReconnect,
//	            }},
//	        {ID: "credentials-expired", Description: "The connection's credentials have expired.",
//	            Remediations: []errors.Remediation{
//	                {ID: "refresh-credentials", Description: "Refresh the credentials.",
//	                    Action: "connection.refresh-credentials", ActionPolicy: errors.Automatic},
//	                remReconnect,
//	            }},
//	    })
//
// errorutil rejects description lists or causes not written as []string and
// []errors.Cause literals (a variable or nil, for example), and any description
// element that is not a string literal. To
// carry instance text such as err.Error(), append it to the returned error's
// LongDescription after NewV3 returns.
func NewV3(code string, severity Severity, sdescription []string, ldescription []string, causes []Cause) *ErrorV2 {
	stored := make([]Cause, 0, len(causes))
	for _, c := range causes {
		rems := make([]Remediation, 0, len(c.Remediations))
		for _, r := range c.Remediations {
			if r.ActionPolicy == "" {
				r.ActionPolicy = Manual
			}
			rems = append(rems, r)
		}
		c.Remediations = rems
		stored = append(stored, c)
	}
	if len(stored) == 0 {
		stored = nil
	}
	probableCause, suggestedRemediation := FlattenCauses(stored)
	return &ErrorV2{
		Code:                 code,
		Severity:             severity,
		ShortDescription:     sdescription,
		LongDescription:      ldescription,
		ProbableCause:        probableCause,
		SuggestedRemediation: suggestedRemediation,
		Causes:               stored,
	}
}

// GetCauses returns the nested causes of a MeshKit error created with NewV3,
// or nil for any other error (including New and NewV2 errors, whose flat lists
// are unpaired).
func GetCauses(err error) []Cause {
	var errV2 *ErrorV2
	if stderrors.As(err, &errV2) && errV2 != nil {
		return errV2.Causes
	}
	return nil
}

// SeverityNamed returns the MeshKit Severity whose constant is named name, as
// errorutil reads it from source (`Alert` for errors.Alert).
func SeverityNamed(name string) (Severity, bool) {
	switch name {
	case "Emergency":
		return Emergency, true
	case "None":
		return None, true
	case "Alert":
		return Alert, true
	case "Critical":
		return Critical, true
	case "Fatal":
		return Fatal, true
	default:
		return 0, false
	}
}

// IsSchemaSeverity reports whether s is one of the eight levels of the schemas
// ErrorSeverity ladder.
func IsSchemaSeverity(s errorcode.ErrorSeverity) bool {
	switch s {
	case errorcode.SeverityEmergency, errorcode.SeverityAlert, errorcode.SeverityCritical, errorcode.SeverityFatal,
		errorcode.SeverityError, errorcode.SeverityWarning, errorcode.SeverityInformational, errorcode.SeverityDebug:
		return true
	default:
		return false
	}
}

// NonEmptyLines returns lines trimmed, without empty lines and MeshKit's
// "None" placeholder, which the schemas flat lists do not allow.
func NonEmptyLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if line = strings.TrimSpace(line); line != "" && line != noneText {
			out = append(out, line)
		}
	}
	return out
}

// SchemaSeverity maps a MeshKit severity to the schemas construct's
// ErrorSeverity. None (and any unknown value) has no schema severity: ok is false.
func (s Severity) SchemaSeverity() (severity errorcode.ErrorSeverity, ok bool) {
	switch s {
	case Emergency:
		return errorcode.SeverityEmergency, true
	case Alert:
		return errorcode.SeverityAlert, true
	case Critical:
		return errorcode.SeverityCritical, true
	case Fatal:
		return errorcode.SeverityFatal, true
	default:
		return "", false
	}
}

// JoinDescription joins MeshKit description lines into the single string the
// schemas construct carries: lines joined with ". ", each line's own trailing
// period dropped first so no line ends in "..". Empty lines and the "None"
// placeholder are skipped; the result is empty when nothing remains.
func JoinDescription(lines []string) string {
	parts := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == NoneString[0] {
			continue
		}
		parts = append(parts, line)
	}
	for i := 0; i < len(parts)-1; i++ {
		parts[i] = strings.TrimRight(parts[i], ".")
	}
	return strings.Join(parts, ". ")
}

// AsErrorV2 returns the MeshKit error in err's chain as an ErrorV2 value: the
// *ErrorV2 itself, or an *Error converted with no AdditionalInfo. ok is false
// when err carries no MeshKit error. It gives readers the raw description,
// cause and remediation lines (and Causes for NewV3) without the joining the
// Get* accessors apply.
func AsErrorV2(err error) (ErrorV2, bool) {
	var errV2 *ErrorV2
	if stderrors.As(err, &errV2) && errV2 != nil {
		return *errV2, true
	}
	var errV1 *Error
	if stderrors.As(err, &errV1) && errV1 != nil {
		return errV1.ErrorV2(nil), true
	}
	return ErrorV2{}, false
}
