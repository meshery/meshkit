package github

import (
	"net/url"

	giturlparse "github.com/git-download-manager/git-url-parse"
	"github.com/meshery/meshkit/generators/models"
)

type DownloaderScheme interface {
	GetContent() (models.Package, error)
}

func NewDownloaderForScheme(scheme string, url *url.URL, packageName string) DownloaderScheme {
	// Check if this is a GitHub URL - route to GitRepo for automatic CRD discovery
	if isGitHubURL(url) {
		return GitRepo{
			URL:         url,
			PackageName: packageName,
		}
	}
	
	switch scheme {
	case "git":
		return GitRepo{
			URL:         url,
			PackageName: packageName,
		}
	case "http":
		fallthrough
	case "https":
		return URL{
			URL:         url,
			PackageName: packageName,
		}
	}
	return nil
}

func isGitHubURL(u *url.URL) bool {
	gitRepository := giturlparse.NewGitRepository("", "", u.String(), "")
	if err := gitRepository.Parse("", 0, ""); err != nil {
		return false
	}

	if gitRepository.Hostname != "github.com" {
		return false
	}

	return gitRepository.Owner != "" && gitRepository.Name != ""
}
