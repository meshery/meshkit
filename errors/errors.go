package errors

import "strings"

func NewDefault(code string, description ...string) *Error {

	return &Error{
		Code:        code,
		Severity:    None,
		Description: description,
		Remedy:      []string{"None"},
	}
}

func New(code string, severity Severity, description []string, remedy []string) *Error {

	return &Error{
		Code:        code,
		Severity:    severity,
		Description: description,
		Remedy:      remedy,
	}
}

func (e *Error) Error() string { return strings.Join(e.Description[:], ",") }

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

func GetRemedy(err error) []string {

	if obj := err.(*Error); obj != nil && obj.Remedy != nil {
		return obj.Remedy
	}
	return []string{"None"}
}

func Is(err error) (*Error, bool) {
	if err != nil {
		er, ok := err.(*Error)
		return er, ok
	}
	return nil, false
}
