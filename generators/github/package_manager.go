package github

import (
	"fmt"
	"net/url"

	"github.com/layer5io/meshkit/models"
	"github.com/layer5io/meshkit/utils/walker"
)

type GitHubPackageManager struct {
	PackageName string
	SourceURL   string
	Version string
}

// Do something if raw.githubuser.content url provided
func (ghpm GitHubPackageManager) GetPackage() (models.Package, error) {
	url, err := url.Parse(ghpm.SourceURL)
	if err != nil {
		err = walker.ErrCloningRepo(err)
		return nil, err
	}
 	protocol := url.Scheme
	
	downloader := NewDownloaderForScheme(protocol, url, ghpm.PackageName)
	ghPackage, err := downloader.GetContent()
	fmt.Println(ghPackage, "GH PACKAGE ")
	if err != nil {
		return nil, ErrGenerateGitHubPackage(err)
	}
	return ghPackage, nil
}
