package walker

import "github.com/meshery/meshkit/errors"

var (
	ErrInvalidSizeFileCode = "meshkit-11241"
	ErrCloningRepoCode     = "meshkit-11242"
)

func ErrCloningRepo(err error) error {
	return errors.New(ErrCloningRepoCode, errors.Alert, []string{"could not clone the repo"}, []string{err.Error()}, []string{}, []string{})
}

func ErrInvalidSizeFile(err error) error {
	return errors.New(ErrInvalidSizeFileCode, errors.Alert, []string{err.Error()}, []string{"Could not read the file while walking the repo"}, []string{"Given file size is either 0 or exceeds the limit of 50 MB"}, []string{""})
}
