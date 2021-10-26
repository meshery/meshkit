package walker

import "github.com/layer5io/meshkit/errors"

var (
	ErrInvalidModeCode = "replace"
	ErrCloningRepoCode = "replace"
)

func ErrInvalidMode(err error) error {
	return errors.New(ErrInvalidModeCode, errors.Alert, []string{"Empty or Invalid mode passed"}, []string{err.Error()}, []string{}, []string{})
}
func ErrCloningRepo(err error) error {
	return errors.New(ErrCloningRepoCode, errors.Alert, []string{"could not clone the repo"}, []string{err.Error()}, []string{}, []string{})
}
