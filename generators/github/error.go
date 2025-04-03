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
	return errors.New(ErrGenerateGitHubPackageCode, errors.Alert, []string{fmt.Sprintf("Error generating package for %s", pkgName)}, []string{fmt.Sprintf("Error generating package for %s. %s", pkgName, err.Error())}, []string{"Invalid source URL provided", "Repository might be private"}, []string{"Ensure source URL follows the format: git://[owner]/[repositoryname]/[branch]/[path from the root of repository]", "Make sure the path to CRDs is correct in the URL", "Private repositories aren't supported currently, ensure that provided URL is from a public repository"})
}

func ErrInvalidGitHubSourceURL(err error) error {
	return errors.New(ErrInvalidGitHubSourceURLCode, errors.Alert, []string{"Error parsing the source URL"}, []string{fmt.Sprintf("Error parsing the source URL: %s", err.Error())}, []string{"Source URL provided might be invalid", "Provided repo/version tag does not exist"}, []string{"Ensure source url follows the format: git://[owner]/[repositoryname]/[branch]/[path from the root of repository]"})
}
