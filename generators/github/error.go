package github

import (
	"fmt"

	"github.com/layer5io/meshkit/errors"
)

const (
	ErrGenerateGitHubPackageCode  = "meshkit-11139"
	ErrInvalidGitHubSourceURLCode = "meshkit-11140"
	ErrInvalidProtocolCode        = "meshkit-11311"
	ErrFetchingContentfromURLCode = "meshkit-11312"
)

func ErrGenerateGitHubPackage(err error, pkgName string) error {
	return errors.New(ErrGenerateGitHubPackageCode, errors.Alert, []string{fmt.Sprintf("error generate package for %s", pkgName)}, []string{err.Error()}, []string{"invalid sourceurl provided", "repository might be private"}, []string{"provided sourceURL according to the format", "provide approparite credentials to clone a private repository"})
}

func ErrInvalidGitHubSourceURL(err error) error {
	return errors.New(ErrInvalidGitHubSourceURLCode, errors.Alert, []string{}, []string{err.Error()}, []string{"sourceURL provided might be invalid", "provided repo/version tag does not exist"}, []string{"ensure source url follows the format: git://<owner>/<repositoryname>/<branch>/<version>/<path from the root of repository>"})
}

func ErrInvalidProtocol(err error, pkgName string, protocol string) error {
	return errors.New(ErrInvalidProtocolCode, errors.Alert, []string{fmt.Sprintf("Unsupported protocol used in source URL for %s", pkgName)}, []string{fmt.Sprintf("Unsupported protocol used in source URL for %s. Error details: %s", pkgName, err.Error())}, []string{fmt.Sprintf("The URL scheme '%s' used in %s is not supported", protocol, pkgName)}, []string{"The 'git' protocol is supported for GitHub repositories.", "Refer to documentation for more details: https://docs.meshery.io/project/contributing/contributing-models#instructions-for-creating-a-new-model"})
}

func ErrFetchingContentfromURL(err error) error {
	return errors.New(ErrFetchingContentfromURLCode, errors.Alert, []string{"Error fetching contents from provided source URL"}, []string{fmt.Sprintf("Error fetching contents from provided link to repository. Error details: %s", err.Error())}, []string{"Provided source URL might not follow a valid Git protocol URL format.", "It might have components like `/blob/` or `/tree/`, which are used for viewing files or folders in the browser, not for accessing the repository itself.", "The manifests are in compressed formats like .tgz or .zip"}, []string{"Ensure source URL follows the correct Git protocol format: git://[owner]/[repositoryname]/[branch]/[version]/[path-from-the-root-of-repository]", "Ensure that manifests are in an uncompressed form", "Refer to documentation for more details: https://docs.meshery.io/project/contributing/contributing-models#instructions-for-creating-a-new-model"})
}
