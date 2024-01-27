package generators

import "github.com/layer5io/meshkit/errors"

var (
	ErrUnsupportedRegistrantCode = "11107"
)

func ErrUnsupportedRegistrant(err error) error {
	return errors.New(ErrUnsupportedRegistrantCode, errors.Alert, []string{"unsupported registrant"}, []string{err.Error()}, []string{"Select from one of the supported registrants"}, []string{"Check docs for the list of supported registrants"})
}
