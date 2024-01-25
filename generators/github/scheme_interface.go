package github

import (
	"net/url"

	"github.com/layer5io/meshkit/models"
)

type DownloaderScheme interface {
	GetContent() (models.Package, error)
}

func NewDownloaderForScheme(scheme string, url *url.URL, packageName string) DownloaderScheme {
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
