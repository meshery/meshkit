package walker

import "github.com/layer5io/meshkit/errors"

var (
	ErrInvalidModeCode = "replace"
	ErrCloningRepoCode = "replace"
)

func ErrInvalidMode(err error) error {
	return errors.New(ErrInvalidModeCode, errors.Alert, []string{"Empty or Invalid mode passed"}, []string{err.Error()}, []string{"An invalid mode of walking the repository was passed", "The correct registeration function for the passed mode was not used"}, []string{"Pass a valid mode from the package", "The correct registeration function for the passed mode must be used. Use \"GetFileFuncName\" and \"GetDirFuncName\" functions to get name of appropriate registeration functions"})
}
func ErrCloningRepo(err error) error {
	return errors.New(ErrCloningRepoCode, errors.Alert, []string{"could not clone the repo"}, []string{err.Error()}, []string{}, []string{})
}
