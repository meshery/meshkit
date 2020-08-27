package errors

type (
	Error struct {
		Code        string
		Description string
	}
)

func New(code string, description string, doc ...string) *Error {
	return &Error{
		Code:        code,
		Description: description,
	}
}

func (e *Error) Error() string { return e.Description }

func GetCode(err error) string {

	if errVal := err.(*Error); errVal != nil {
		return errVal.Code
	}
	return " "
}

func Is(err error) (*Error, bool) {
	if err != nil {
		er, ok := err.(*Error)
		return er, ok
	}
	return nil, false
}
