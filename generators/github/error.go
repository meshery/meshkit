package github

import "github.com/layer5io/meshkit/errors"

const (
	ErrGenerateGitHubPackageCode  = "replace_me"
	ErrInvalidGitHubSourceURLCode = "replace_me"
)

func ErrGenerateGitHubPackage(err error) error {
	return errors.New(ErrGenerateGitHubPackageCode, errors.Alert, []string{}, []string{}, []string{}, []string{})
}

func ErrInvalidGitHubSourceURL(err error) error {
	return errors.New(ErrInvalidGitHubSourceURLCode, errors.Alert, []string{}, []string{}, []string{}, []string{})
}
