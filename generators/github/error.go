package github

import (
	"fmt"

	"github.com/layer5io/meshkit/errors"
)

const (
	ErrGenerateGitHubPackageCode  = "11108"
	ErrInvalidGitHubSourceURLCode = "11109"
)

func ErrGenerateGitHubPackage(err error, pkgName string) error {
	return errors.New(ErrGenerateGitHubPackageCode, errors.Alert, []string{fmt.Sprintf("error generate package for %s", pkgName)}, []string{err.Error()}, []string{"invalid sourceurl provided", "repository might be private"}, []string{"provided sourceURL according to the format", "provide approparite credentials to clone a private repository"})
}

func ErrInvalidGitHubSourceURL(err error) error {
	return errors.New(ErrInvalidGitHubSourceURLCode, errors.Alert, []string{}, []string{err.Error()}, []string{"sourceURL provided might be invalid", "provided repo/version tag does not exist"}, []string{"ensure source url follows the format: git://<owner>/<repositoryname>/<branch>/<version>/<path from the root of repository>"})
}
