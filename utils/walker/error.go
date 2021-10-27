package walker

import "github.com/layer5io/meshkit/errors"

var (
	ErrInvalidModeCode = "replace"
	ErrCloningRepoCode = "replace"
)

func ErrCloningRepo(err error) error {
	return errors.New(ErrCloningRepoCode, errors.Alert, []string{"could not clone the repo"}, []string{err.Error()}, []string{}, []string{})
}
