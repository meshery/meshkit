package github

import (
	"net/url"

	"github.com/layer5io/meshkit/models"
)

type Scheme interface {
	GetContent() (models.Package, error)
}

func NewDownloaderForScheme(scheme string, url *url.URL) Scheme {
	switch scheme {
	case "git":
		return GitRepo{
			URL: url,
		}
	}
	return nil
}
