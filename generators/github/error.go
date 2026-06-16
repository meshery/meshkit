package github

import (
	"fmt"

	"github.com/meshery/meshkit/errors"
)

const (
	ErrGenerateGitHubPackageCode  = "meshkit-11139"
	ErrInvalidGitHubSourceURLCode = "meshkit-11140"
)

func ErrGenerateGitHubPackage(err error, pkgName string) error {
	return errors.New(ErrGenerateGitHubPackageCode, errors.Alert, []string{fmt.Sprintf("error generating package for %s", pkgName)}, []string{err.Error()}, []string{"invalid sourceURL provided", "repository might be private"}, []string{"provide sourceURL according to the format", "provide appropriate credentials to clone a private repository"})
}

func ErrInvalidGitHubSourceURL(err error) error {
	return errors.New(ErrInvalidGitHubSourceURLCode, errors.Alert, []string{}, []string{err.Error()}, []string{"sourceURL provided might be invalid", "provided repo/version tag does not exist"}, []string{"ensure sourceURL follows the format: git://<owner>/<repositoryname>/<branch>/<version>/<path from the root of repository>"})
}
