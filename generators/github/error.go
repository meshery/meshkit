package github

import (
	"fmt"

	"github.com/meshery/meshkit/errors"
)

const (
	ErrGenerateGitHubPackageCode  = "meshkit-11139"
	ErrInvalidGitHubSourceURLCode = "meshkit-11140"
	ErrGetGithubDefaultBranchCode = "meshkit-11510"
)

func ErrGenerateGitHubPackage(err error, pkgName string) error {
	return errors.New(ErrGenerateGitHubPackageCode, errors.Alert, []string{fmt.Sprintf("error generate package for %s", pkgName)}, []string{err.Error()}, []string{"invalid sourceurl provided", "repository might be private"}, []string{"provided sourceURL according to the format", "provide approparite credentials to clone a private repository"})
}

func ErrInvalidGitHubSourceURL(err error) error {
	return errors.New(ErrInvalidGitHubSourceURLCode, errors.Alert, []string{}, []string{err.Error()}, []string{"sourceURL provided might be invalid", "provided repo/version tag does not exist"}, []string{"ensure source url follows the format: git://<owner>/<repositoryname>/<branch>/<version>/<path from the root of repository>"})
}

// ErrGetDefaultBranch is the error for getting the default branch
func ErrGetDefaultBranch(err error, owner, repo string) error {
	return errors.New(ErrGetGithubDefaultBranchCode, errors.Alert, []string{fmt.Sprintf("Error while getting default branch for %s/%s", owner, repo)}, []string{err.Error()}, []string{"The repository might not exist"}, []string{})
}
