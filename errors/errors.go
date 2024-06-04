// Package errors provides types and function used to implement MeshKit compatible errors across the Meshery code base.
//
// Uniform definition of errors using these types and functions facilitates extracting error information directly from the code
// and generating and publishing error code reference documentation automatically.
// The error utility tool in this module, /cmd/errorutil, is part of this toolchain.
//
// It depends on a few conventions in order to work:
//
// 1) An error code defined as a constant or variable (preferably constant), of type string.
// The naming convention for these variables is the regex "^Err[A-Z].+Code$", e.g. ErrApplyManifestCode.
// The initial value of the code is a placeholder string, e.g. "replace_me", set by the developer.
// The final value of the code is an integer, set by the errorutil tool, as part of a CI workflow.
//
// 2) Error details defined using the function New(...) in this package, see below for details.
//
// Additionally, the following conventions apply:
//
// Errors are defined in each package, in a file named error.go.
// Errors are namespaced to Meshery components, i.e. they need to be unique within a component.
// (Often, a specific component corresponds to one git repository.)
// There are no predefined error code ranges for components. Every component is free to use its own range.
// Codes carry no meaning, as e.g. HTTP status codes do.
//
// See also the doc command of errorutil, and https://docs.meshery.io/project/contributing/contributing-error.
//
// Example:
//
//	 const  ErrConnectCode        = "11000"
//	 func ErrConnect(err error) error {
//		  return errors.New(ErrConnectCode,
//		                    errors.Alert,
//		                    []string{"Connection to broker failed"},
//		                    []string{err.Error()},
//		                    []string{"Endpoint might not be reachable"},
//		                    []string{"Make sure the NATS endpoint is reachable"})
//	 }
package errors

import (
	"fmt"
	"strings"
)

// Deprecated: NewDefault is deprecated, use New(...) instead.
func NewDefault(code string, ldescription ...string) *Error {
	return &Error{
		Code:                 code,
		Severity:             None,
		ShortDescription:     NoneString,
		LongDescription:      ldescription,
		ProbableCause:        NoneString,
		SuggestedRemediation: NoneString,
	}
}

// New returns a MeshKit error using the provided parameters.
//
// In order to create MeshKit compatible errors that can be handled by the errorutil tool, consider the following points:
//
// The first parameter, 'code', has to be passed as the error code constant (or variable), not a string literal.
//
// The second parameter, 'severity', has its own type; consult its Go-doc for further details.
//
// The remaining parameters are string arrays for short and long description, probable cause, and suggested remediation.
// Use string literals in these string arrays, not constants or variables, for any static texts or format strings.
// Capitalize the first letter of each statement.
// Call expressions can be used but will be ignored by the tool when exporting error details for the documentation.
// Do not concatenate strings using the '+' operator, just add multiple elements to the string array.
//
// Example:
//
//	errors.New(ErrConnectCode,
//	           errors.Alert,
//	           []string{"Connection to broker failed"},
//	           []string{err.Error()},
//	           []string{"Endpoint might not be reachable"},
//	           []string{"Make sure the NATS endpoint is reachable"})
func New(code string, severity Severity, sdescription []string, ldescription []string, probablecause []string, remedy []string) *Error {
	return &Error{
		Code:                 code,
		Severity:             severity,
		ShortDescription:     sdescription,
		LongDescription:      ldescription,
		ProbableCause:        probablecause,
		SuggestedRemediation: remedy,
	}
}

func NewV2(code string, severity Severity, sdescription []string, ldescription []string, probablecause []string, remedy []string, additionalInfo interface{}) *ErrorV2 {
	return &ErrorV2{
		Code:                 code,
		Severity:             severity,
		ShortDescription:     sdescription,
		LongDescription:      ldescription,
		ProbableCause:        probablecause,
		SuggestedRemediation: remedy,
		AdditionalInfo:       additionalInfo,
	}
}

func (e *Error) Error() string { return strings.Join(e.LongDescription[:], ".") }

func (e *Error) ErrorV2(additionalInfo interface{}) ErrorV2 {
	return ErrorV2{Code: e.Code, Severity: e.Severity, ShortDescription: e.ShortDescription, LongDescription: e.LongDescription, ProbableCause: e.ProbableCause, SuggestedRemediation: e.SuggestedRemediation, AdditionalInfo: additionalInfo}
}

func GetCode(err error) string {
	var code string
	defer func() {
		if r := recover(); r != nil {
			code = strings.Join(NoneString[:], "")
		}
	}()
	if obj := err.(*Error); obj != nil && obj.Code != " " {
		code = obj.Code
	} else {
		code = strings.Join(NoneString[:], "")
	}
	return code
}

func GetSeverity(err error) Severity {
	var severity Severity
	defer func() {
		if r := recover(); r != nil {
			severity = None
		}
	}()
	if obj := err.(*Error); obj != nil {
		severity = obj.Severity
	} else {
		severity = None
	}
	return severity
}

func GetSDescription(err error) string {
	var description string
	defer func() {
		if r := recover(); r != nil {
			description = ""
		}
	}()
	if obj := err.(*Error); obj != nil {
		description = strings.Join(obj.ShortDescription[:], ".")
	} else {
		description = ""
	}
	return description
}

func GetCause(err error) string {
	var cause string
	defer func() {
		if r := recover(); r != nil {
			cause = ""
		}
	}()
	if obj := err.(*Error); obj != nil {
		cause = strings.Join(obj.ProbableCause[:], ".")
	} else {
		cause = fmt.Sprintf("%v", err)
	}
	return cause
}

func GetRemedy(err error) string {
	var remedy string
	defer func() {
		if r := recover(); r != nil {
			remedy = ""
		}
	}()
	if obj := err.(*Error); obj != nil {
		remedy = strings.Join(obj.SuggestedRemediation[:], ".")
	} else if err != nil {
		remedy = ""
	}
	return remedy
}

func Is(err error) (*Error, bool) {
	if err != nil {
		er, ok := err.(*Error)
		return er, ok
	}
	return nil, false
}
