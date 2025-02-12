package patterns

import "github.com/layer5io/meshkit/errors"

const (
	ErrInvalidVersionCode = "meshkit-11266"
)

func ErrInvalidVersion(err error) error {
	return errors.New(ErrInvalidVersionCode, errors.Alert, 
		[]string{"Invalid/incompatible semver version."}, 
		[]string{err.Error()}, 
		[]string{"Version history for the content has been tampered with outside Meshery."}, 
		[]string{"Roll back to one of the previous versions."})
}
