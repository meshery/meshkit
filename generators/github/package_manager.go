package github

import (
	"net/url"

	"github.com/layer5io/meshkit/generators/models"
	"github.com/layer5io/meshkit/utils/walker"
)

type GitHubPackageManager struct {
	PackageName string
	SourceURL   string
}

func (ghpm GitHubPackageManager) GetPackage() (models.Package, error) {
	url, err := url.Parse(ghpm.SourceURL)
	if err != nil {
		err = walker.ErrCloningRepo(err)
		return nil, err
	}
	protocol := url.Scheme

	downloader := NewDownloaderForScheme(protocol, url, ghpm.PackageName)
	if downloader == nil {
		return nil, ErrInvalidProtocol(err, ghpm.PackageName, protocol)
	}
	ghPackage, err := downloader.GetContent()
	if err != nil {
		return nil, ErrFetchingContentfromURL(err)
	}
	return ghPackage, nil
}
