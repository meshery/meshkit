package errors

import "strings"

func NewDefault(code string, sdescription ...string) *Error {

	return &Error{
		Code:                 code,
		Severity:             None,
		ShortDescription:     sdescription,
		LongDescription:      []string{"None"},
		ProbableCause:        []string{"None"},
		SuggestedRemediation: []string{"None"},
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

func GetCode(err error) string {

	if obj := err.(*Error); obj != nil && obj.Code != " " {
		return obj.Code
	}
	return " "
}

func GetSeverity(err error) Severity {

	if obj := err.(*Error); obj != nil {
		return obj.Severity
	}
	return None
}

func (e *Error) Error() string { return strings.Join(e.LongDescription[:], ",") }

func GetSDescription(err error) string { return strings.Join(err.(*Error).ShortDescription[:], ",") }

func GetCause(err error) string { return strings.Join(err.(*Error).ProbableCause[:], ",") }

func GetRemedy(err error) string { return strings.Join(err.(*Error).SuggestedRemediation[:], ",") }

func Is(err error) (*Error, bool) {
	if err != nil {
		er, ok := err.(*Error)
		return er, ok
	}
	return nil, false
}
