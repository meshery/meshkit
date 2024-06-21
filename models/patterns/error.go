package patterns

import "github.com/layer5io/meshkit/errors"


const (
	ErrInvalidVersionCode = ""
)

func ErrInvalidVersion(err error) error {
	return errors.New(ErrInvalidVersionCode, errors.Alert, []string{}, []string{err.Error()}, []string{}, []string{})
}
