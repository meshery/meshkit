package walker

import (
	"fmt"

	"github.com/meshery/meshkit/errors"
)

var (
	ErrInvalidSizeFileCode = "meshkit-11241"
	ErrCloningRepoCode     = "meshkit-11242"
	ErrResolvingGitRefCode = "meshkit-11328"
	ErrFetchingGitTreeCode = "meshkit-11329"
	ErrFetchingGitBlobCode = "meshkit-11330"
	ErrInvalidBaseURLCode  = "meshkit-11331"

	ErrNoFileInterceptorCode = "meshkit-11332"
)

func ErrCloningRepo(err error) error {
	return errors.New(ErrCloningRepoCode, errors.Alert, []string{"could not clone the repo"}, []string{err.Error()}, []string{}, []string{})
}

func ErrInvalidSizeFile(err error) error {
	return errors.New(ErrInvalidSizeFileCode, errors.Alert, []string{err.Error()}, []string{"Could not read the file while walking the repo"}, []string{"Given file size is either 0 or exceeds the limit of 50 MB"}, []string{""})
}

// ErrResolvingGitRef is returned when a branch, tag or reference name could not
// be resolved to a commit SHA through the GitHub API.
func ErrResolvingGitRef(err error, ref string) error {
	return errors.New(
		ErrResolvingGitRefCode,
		errors.Alert,
		[]string{"Could not resolve the git reference to a commit"},
		[]string{fmt.Sprintf("%s: %s", ref, err.Error())},
		[]string{"The reference does not exist on the remote repository", "The repository is private and no access token was supplied", "The GitHub API rate limit has been exhausted"},
		[]string{"Verify the branch, tag or reference name exists on the remote", "Supply a GitHub App or OAuth token with the Token option so private repositories and higher rate limits are available"},
	)
}

// ErrFetchingGitTree is returned when the recursive Git Trees API call fails.
func ErrFetchingGitTree(err error, ref string) error {
	return errors.New(
		ErrFetchingGitTreeCode,
		errors.Alert,
		[]string{"Could not fetch the git tree of the repository"},
		[]string{fmt.Sprintf("%s: %s", ref, err.Error())},
		[]string{"The commit does not exist on the remote repository", "The repository is private and no access token was supplied", "The GitHub API rate limit has been exhausted"},
		[]string{"Verify the reference exists on the remote", "Supply a GitHub App or OAuth token with the Token option so private repositories and higher rate limits are available"},
	)
}

// ErrFetchingGitBlob is returned when a selected blob could not be downloaded.
func ErrFetchingGitBlob(err error, path string) error {
	return errors.New(
		ErrFetchingGitBlobCode,
		errors.Alert,
		[]string{"Could not fetch the contents of a file in the repository"},
		[]string{fmt.Sprintf("%s: %s", path, err.Error())},
		[]string{"The blob was removed after the tree was listed", "The repository is private and no access token was supplied", "The GitHub API rate limit has been exhausted"},
		[]string{"Retry the import so a fresh tree is listed", "Supply a GitHub App or OAuth token with the Token option so private repositories and higher rate limits are available"},
	)
}

// ErrNoFileInterceptor is returned when files are fetched with no interceptor
// registered to receive them.
func ErrNoFileInterceptor() error {
	return errors.New(
		ErrNoFileInterceptorCode,
		errors.Alert,
		[]string{"No file interceptor is registered to receive the fetched files"},
		[]string{"Fetching hands every file it downloads to a registered file interceptor, and the walker has none"},
		[]string{"RegisterFileInterceptor was not called on the walker before the files were fetched"},
		[]string{"Call RegisterFileInterceptor with the function that should receive each fetched file"},
	)
}

// ErrInvalidBaseURL is returned when the configured base URL cannot be used for
// the GitHub API, either because it does not parse or because it names a host
// other than github.com.
func ErrInvalidBaseURL(err error, baseURL string) error {
	return errors.New(
		ErrInvalidBaseURLCode,
		errors.Alert,
		[]string{"Could not use the repository base URL for the GitHub API"},
		[]string{fmt.Sprintf("%s: %s", baseURL, err.Error())},
		[]string{"The base URL passed to the walker is not a valid URL", "The base URL names a host other than github.com, which the Git Trees API cannot answer for"},
		[]string{"Pass a valid base URL such as https://github.com to the BaseURL option", "Walk a repository hosted elsewhere with Walk or WalkContext, which clone it with go-git"},
	)
}
