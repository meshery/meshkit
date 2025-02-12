package generators

import "github.com/layer5io/meshkit/errors"

var (
	ErrUnsupportedRegistrantCode = "meshkit-11138"
)

func ErrUnsupportedRegistrant(err error) error {
	return errors.New(ErrUnsupportedRegistrantCode, errors.Alert, 
		[]string{"Unsupported registrant."}, 
		[]string{err.Error()}, 
		[]string{"The selected registrant is not supported by Meshery."}, 
		[]string{"Please check the documentation for the list of supported registrants."})
}
