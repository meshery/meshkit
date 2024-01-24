package github

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/layer5io/meshkit/models"
	"github.com/layer5io/meshkit/utils"
	"github.com/layer5io/meshkit/utils/walker"
)

type GitRepo struct {
	URL         *url.URL
	PackageName  string
	FilesToWalk []string
}

// Assumpations: 1. Always a K8s manifest
// 2. Unzipped/unarchived File type

func (gr GitRepo) GetContent() (models.Package, error) {
	gitWalker := walker.NewGit()

	owner, repo, branch, version, root, err := gr.extractRepoDetailsFromSourceURL()
	if err != nil {
		return nil, err
	}

	filePath := fmt.Sprintf("%s_%s_%s_%s", owner, repo, branch, utils.GetRandomAlphabetsOfDigit(5))
	fd, err := os.Create(filePath) // perform cleanup
	if err != nil {
		return nil, utils.ErrCreateFile(err, filePath)
	}
	br := bufio.NewWriter(fd)
	err = gitWalker.
		Owner(owner).
		Repo(repo).
		Branch(branch).
		Root(root).
		RegisterFileInterceptor(func(file walker.File) error {
			ext := filepath.Ext(file.Name)
			if ext == ".yaml" || ext == ".yml" {
				br.WriteString("\n---\n")
				br.WriteString(file.Content)
			}
			return nil
		}).
		ReferenceName(fmt.Sprintf("refs/tags/%s", version)).
		Walk()

	if err != nil {
		fmt.Println(err, "ERR WALKING ")
		return nil, walker.ErrCloningRepo(err)
	}

	fmt.Println("TEST SUCCESUGLY CLONED")
	return GitHubPackage{
		Name:       gr.PackageName,
		filePath:   filePath,
		branch:     branch,
		repository: repo,
		SourceURL:  gr.URL.String(),
		version:    version,
	}, nil
}

func (gr GitRepo) extractRepoDetailsFromSourceURL() (owner, repo, branch, versionTag, root string, err error) {
	parts := strings.SplitN(strings.TrimPrefix(gr.URL.Path, "/"), "/", 5)
	if len(parts) == 5 {
		owner = parts[0]
		repo = parts[1]
		branch = parts[2]
		versionTag = parts[3]
		root = parts[4]
	} else {
		err = ErrInvalidGitHubSourceURL(fmt.Errorf("specify owner, repo, branch and filepath in the url according to the specified source url format"))
	}
	return
}