package github

import (
	"errors"
	"net/url"
	"strings"

	"github.com/meshery/meshkit/generators/models"
	"github.com/meshery/meshkit/utils/walker"
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
	// not all https links are URL downloaders, some are git repos
	//  Check if it's a GitHub Browser/Web UI link (contains /tree/ or /blob/ or /releases/)
	isGitHubUI := url.Host == "github.com" && (strings.Contains(url.Path, "/tree/") || strings.Contains(url.Path, "/blob/") || strings.Contains(url.Path, "/releases/"))
	//  Check if it's a "Short" link (just owner/repo)
	pathParts := strings.Split(strings.Trim(url.Path, "/"), "/")
	isShortRepoLink := url.Host == "github.com" && len(pathParts) == 2

	// If it's either of those, we MUST use 'git' protocol (GitRepo walker)
	if (protocol == "https" || protocol == "http") && (isGitHubUI || isShortRepoLink) {
		protocol = "git"
	}

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
