package errors

import "strings"

// Deprecated: NewDefault is deprecated, use New
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

func (e *Error) Error() string { return strings.Join(e.LongDescription[:], ".") }

func GetCode(err error) string {

	if obj := err.(*Error); obj != nil && obj.Code != " " {
		return obj.Code
	}
	return strings.Join(NoneString[:], "")
}

func GetSeverity(err error) Severity {

	if obj := err.(*Error); obj != nil {
		return obj.Severity
	}
	return None
}

func GetSDescription(err error) string {

	if obj := err.(*Error); obj != nil {
		return strings.Join(err.(*Error).ShortDescription[:], ".")
	}
	return strings.Join(NoneString[:], "")
}

func GetCause(err error) string {

	if obj := err.(*Error); obj != nil {
		return strings.Join(err.(*Error).ProbableCause[:], ".")
	}
	return strings.Join(NoneString[:], "")
}

func GetRemedy(err error) string {

	if obj := err.(*Error); obj != nil {
		return strings.Join(err.(*Error).SuggestedRemediation[:], ".")
	}
	return strings.Join(NoneString[:], "")
}

func Is(err error) (*Error, bool) {
	if err != nil {
		er, ok := err.(*Error)
		return er, ok
	}
	return nil, false
}
