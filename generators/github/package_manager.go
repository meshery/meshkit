package github

import (
	"errors"
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
		err = errors.New("unsupported protocol")
		return nil, ErrGenerateGitHubPackage(err, ghpm.PackageName)
	}
	ghPackage, err := downloader.GetContent()
	if err != nil {
		return nil, ErrGenerateGitHubPackage(err, ghpm.PackageName)
	}
	return ghPackage, nil
}
