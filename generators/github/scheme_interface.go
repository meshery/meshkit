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

// isGitHubURL checks if the URL is a GitHub repository URL
// This enables automatic CRD discovery for standard GitHub URLs
func isGitHubURL(scheme string, url *url.URL) bool {
	host := strings.ToLower(url.Host)
	// Check for github.com domain
	if host == "github.com" || strings.HasSuffix(host, ".github.com") {
		// Check if it looks like a repository URL (has at least owner/repo in path)
		path := strings.TrimPrefix(url.Path, "/")
		parts := strings.Split(path, "/")
		// Valid GitHub repo URL should have at least owner and repo
		if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
			// Exclude certain paths that aren't repositories
			excluded := []string{"settings", "explore", "marketplace", "pulls", "issues", "new", "organizations", "login", "join"}
			for _, exclude := range excluded {
				if parts[0] == exclude {
					return false
				}
			}
			return true
		}
	}
	return false
}
