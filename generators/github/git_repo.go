package github

import (
	"bufio"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/meshery/meshkit/generators/models"
	"github.com/meshery/meshkit/utils"
	"github.com/meshery/meshkit/utils/helm"
	"github.com/meshery/meshkit/utils/walker"
)

type GitRepo struct {
	// <git://github.com/owner/repo/branch/versiontag/root(path to the directory/file)>
	URL         *url.URL
	PackageName string
}

// Assumptions:
// 1. Always a K8s manifest
// 2. Unzipped/unarchived File type

func (gr GitRepo) GetContent() (models.Package, error) {
	gitWalker := walker.NewGit()

	owner, repo, branch, root, err := gr.extractRepoDetailsFromSourceURL()
	if err != nil {
		return nil, err
	}

	versions, err := utils.GetLatestReleaseTagsSorted(owner, repo)
	if err != nil {
		return nil, ErrInvalidGitHubSourceURL(err)
	}
	version := versions[len(versions)-1]
	dirPath := filepath.Join(os.TempDir(), owner, repo, branch)
	_ = os.MkdirAll(dirPath, 0755)
	filePath := filepath.Join(dirPath, utils.GetRandomAlphabetsOfDigit(5))
	fd, err := os.Create(filePath)
	if err != nil {
		os.RemoveAll(dirPath)
		return nil, utils.ErrCreateFile(err, filePath)
	}
	br := bufio.NewWriter(fd)

	defer func() {
		_ = br.Flush()
		_ = fd.Close()
	}()
	gw := gitWalker.
		Owner(owner).
		Repo(repo).
		Branch(branch).
		Root(root).
		RegisterFileInterceptor(fileInterceptor(br)).
		RegisterDirInterceptor(dirInterceptor(br))

	if version != "" {
		gw = gw.ReferenceName(fmt.Sprintf("refs/tags/%s", version))
	}
	err = gw.Walk()

	if err != nil {
		return nil, walker.ErrCloningRepo(err)
	}

	return GitHubPackage{
		Name:       gr.PackageName,
		filePath:   filePath,
		branch:     branch,
		repository: repo,
		SourceURL:  gr.URL.String(),
		version:    version,
	}, nil
}

func (gr GitRepo) extractRepoDetailsFromSourceURL() (owner, repo, branch, root string, err error) {
	// Trim slashes from both ends to handle "owner/repo/" correctly
	path := strings.Trim(gr.URL.Path, "/")

	// If path is empty after trimming, it's definitely invalid
	if path == "" {
		return "", "", "", "", ErrInvalidGitHubSourceURL(fmt.Errorf("Source URL %s is invalid", gr.URL.String()))
	}

	// Split the path into parts
	raw := strings.Split(path, "/")
	parts := make([]string, 0)
	for _, p := range raw {
		if p != "" {
			parts = append(parts, p)
		}
	}
	// Handle GitHub Web UI URLS:
	// Pattern: owner/repo/tree/branch/... or owner/repo/blob/branch/...
	if len(parts) >= 3 && (parts[2] == "tree" || parts[2] == "blob") {
		// Remove 'tree' or 'blob' so the rest of the logic sees [owner, repo, branch, root]
		parts = append(parts[:2], parts[3:]...)
	}

	// Handle Release URLs:
	// Pattern: owner/repo/releases/tag/v1.0
	if len(parts) >= 5 && parts[2] == "releases" && parts[3] == "tag" {
		owner, repo = parts[0], parts[1]
		branch = parts[4] // The tag is treated as the 'branch' for cloning purposes
		root = "/**"
		return owner, repo, branch, root, nil
	}
	size := len(parts)

	switch {
	case size >= 4:
		// Use the first three for owner, repo, branch, and join the rest for root
		owner, repo, branch = parts[0], parts[1], parts[2]
		root = strings.Join(parts[3:], "/")
	case size == 3:
		owner, repo, branch = parts[0], parts[1], parts[2]
		root = "/**"
	case size == 2:
		owner, repo = parts[0], parts[1]
		// we use the default http client here

		b, err := GetDefaultBranchFromGitHub("https://api.github.com", owner, repo, http.DefaultClient)
		if err != nil {
			return "", "", "", "", err
		}
		branch = b
		root = "/**"
	default:
		return "", "", "", "", ErrInvalidGitHubSourceURL(fmt.Errorf("Source URL %s is invalid; expected owner/repo[/branch[/path]]", gr.URL.String()))
	}

	return
}

func (gr GitRepo) ExtractRepoDetailsFromSourceURL() (owner, repo, branch, root string, err error) {
	return gr.extractRepoDetailsFromSourceURL()
}

func fileInterceptor(br *bufio.Writer) walker.FileInterceptor {
	return func(file walker.File) error {
		tempPath := filepath.Join(os.TempDir(), utils.GetRandomAlphabetsOfDigit(5))
		return ProcessContent(br, tempPath, file.Path)
	}
}

// When passing a directory to extract charts and the format introspector is provided as file/dir interceptor i.e. ConvertToK8sManifest ensure the Recurese is off. It is required othweise we will process the dir as well as process the file in that dir separately.
// Add more calrifying commment and entry inside docs.
func dirInterceptor(br *bufio.Writer) walker.DirInterceptor {
	return func(d walker.Directory) error {
		err := helm.ConvertToK8sManifest(d.Path, "", br)
		if err != nil {
			return err
		}
		return nil
	}
}
