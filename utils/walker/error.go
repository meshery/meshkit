package walker

import "github.com/layer5io/meshkit/errors"

var (
	ErrInvalidSizeFileCode = "11072"
	ErrCloningRepoCode     = "11073"
)

func ErrCloningRepo(err error) error {
	return errors.New(ErrCloningRepoCode, errors.Alert, []string{"could not clone the repo"}, []string{err.Error()}, []string{}, []string{})
}

func ErrInvalidSizeFile(err error) error {
	return errors.New(ErrInvalidSizeFileCode, errors.Alert, []string{err.Error()}, []string{"Could not read the file while walking the repo"}, []string{"Given file size is either 0 or exceeds the limit of 50 MB"}, []string{""})
}
