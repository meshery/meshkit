package github

import (
	"errors"
	"net/url"

	"github.com/meshery/meshkit/generators/models"
	"github.com/meshery/meshkit/utils/walker"
)

type GitHubPackageManager struct {
	PackageName string
	SourceURL   string
	Recursive   bool
	MaxDepth    int
}

func (ghpm GitHubPackageManager) GetPackage() (models.Package, error) {
	url, err := url.Parse(ghpm.SourceURL)
	if err != nil {
		err = walker.ErrCloningRepo(err)
		return nil, err
	}
	protocol := url.Scheme

	downloader := NewDownloaderForScheme(protocol, url, ghpm)
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
