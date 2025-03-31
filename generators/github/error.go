package github

import (
	"fmt"

	"github.com/layer5io/meshkit/errors"
)

const (
	ErrGenerateGitHubPackageCode  = "meshkit-11139"
	ErrInvalidGitHubSourceURLCode = "meshkit-11140"
)

func ErrGenerateGitHubPackage(err error, pkgName string) error {
	return errors.New(ErrGenerateGitHubPackageCode, errors.Alert, 
		[]string{fmt.Sprintf("Error generating package for %s.", pkgName)}, 
		[]string{err.Error()}, 
		[]string{
			"Invalid source URL provided.",
			"Repository might be private.",
		}, 
		[]string{
			"Provide source URL according to the format.",
			"Provide appropriate credentials to clone a private repository.",
		})
}

func ErrInvalidGitHubSourceURL(err error) error {
	return errors.New(ErrInvalidGitHubSourceURLCode, errors.Alert, 
		[]string{"Invalid GitHub source URL."}, 
		[]string{err.Error()}, 
		[]string{
			"Source URL provided might be invalid.",
			"Provided repository/version tag does not exist.",
		}, 
		[]string{"Ensure source URL follows the format: git://<owner>/<repositoryname>/<branch>/<version>/<path from the root of repository>."})
}
