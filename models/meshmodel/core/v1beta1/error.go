package v1beta1

import "github.com/layer5io/meshkit/errors"

const (
	ErrUnknownKindCode = "meshkit-11254"
)

func ErrUnknownKind(err error) error {
	return errors.New(ErrUnknownKindCode, errors.Alert, []string{"unsupported connection kind detected"}, []string{err.Error()}, []string{"The component's registrant is not supported by the version of server you are running"}, []string{"Try upgrading to latest available version"})
}
