package github

import (
	"net/url"
	"strings"

	"github.com/meshery/meshkit/generators/models"
)

type DownloaderScheme interface {
	GetContent() (models.Package, error)
}

func NewDownloaderForScheme(scheme string, url *url.URL, packageName string) DownloaderScheme {
	// Check if this is a GitHub URL - route to GitRepo for automatic CRD discovery
	if isGitHubURL(scheme, url) {
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

func isGitHubURL(scheme string, u *url.URL) bool {
	host := strings.ToLower(u.Host)
	if host != "github.com" && !strings.HasSuffix(host, ".github.com") {
		return false
	}
	if strings.HasPrefix(host, "gist.") {
		return false
	}
	
	path := strings.Trim(u.Path, "/")
	path = strings.TrimSuffix(path, ".git")
	
	if path == "" {
		return false
	}
	
	parts := strings.Split(path, "/")
	
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return false
	}
	

	excluded := []string{"settings", "explore", "marketplace", "pulls", "issues", "new", "organizations", "login", "join", "logout", "pricing", "blog"}
	for _, exclude := range excluded {
		if parts[0] == exclude {
			return false
		}
	}
	return true
}
