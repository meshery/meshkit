package github

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"github.com/layer5io/meshkit/generators/models"
	"github.com/layer5io/meshkit/utils"
	"github.com/layer5io/meshkit/utils/helm"
	"github.com/layer5io/meshkit/utils/walker"
)

type GitRepo struct {
	// <git://github.com/owner/repo/branch/versiontag/root(path to the directory/file)>
	URL         *url.URL
	PackageName string
}

// Assumpations: 1. Always a K8s manifest
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
	fd, err := os.Create(filePath) // perform cleanup
	if err != nil {
		return nil, utils.ErrCreateFile(err, filePath)
	}
	br := bufio.NewWriter(fd)

	defer func() {
		_ = br.Flush()
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
	parts := strings.SplitN(strings.TrimPrefix(gr.URL.Path, "/"), "/", 4)
	size := len(parts)
	if size > 3 {
		owner = parts[0]
		repo = parts[1]
		branch = parts[2]
		root = parts[3]

	} else {
		err = ErrInvalidGitHubSourceURL(fmt.Errorf("Source URL %s is invalid, specify owner, repo, branch and filepath in the url according to the specified source url format", gr.URL.String()))
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
		err := helm.ConvertToK8sManifest(d.Path, br)
		if err != nil {
			return err
		}
		return nil
	}
}
