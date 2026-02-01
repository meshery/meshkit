package github

import (
	"net/url"

	"github.com/meshery/meshkit/generators/models"
)

type DownloaderScheme interface {
	GetContent() (models.Package, error)
}

func NewDownloaderForScheme(scheme string, url *url.URL, gp GitHubPackageManager) DownloaderScheme {
	switch scheme {
	case "git":
		return GitRepo{
			URL:         url,
			PackageName: gp.PackageName,
			Recursive:   gp.Recursive,
			MaxDepth:    gp.MaxDepth,
		}
	case "http":
		fallthrough
	case "https":
		return URL{
			URL:         url,
			PackageName: gp.PackageName,
		}
	}
	return nil
}
