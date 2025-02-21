package walker

import "github.com/layer5io/meshkit/errors"

var (
	ErrInvalidSizeFileCode = "meshkit-11241"
	ErrCloningRepoCode     = "meshkit-11242"
)

func ErrCloningRepo(err error) error {
	return errors.New(ErrCloningRepoCode, errors.Alert, 
		[]string{"Could not clone the repository."}, 
		[]string{err.Error()}, 
		[]string{"Repository might be inaccessible or invalid."}, 
		[]string{"Make sure the repository URL is correct and accessible."})
}

func ErrInvalidSizeFile(err error) error {
	return errors.New(ErrInvalidSizeFileCode, errors.Alert, 
		[]string{err.Error()}, 
		[]string{"Could not read the file while walking the repository."}, 
		[]string{"Given file size is either 0 or exceeds the limit of 50 MB."}, 
		[]string{"Please ensure the file size is within the acceptable range (0-50 MB)."})
}
