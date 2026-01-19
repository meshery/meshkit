package github

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	giturlparse "github.com/git-download-manager/git-url-parse"
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
	fd, err := os.Create(filePath)
	if err != nil {
		_ = os.RemoveAll(dirPath)
		return nil, utils.ErrCreateFile(err, filePath)
	}
	br := bufio.NewWriter(fd)

	defer func() {
		_ = br.Flush()
		_ = fd.Close()
	}()
	
	// If root is not specified, enable recursive traversal from root to discover CRDs automatically
	// This makes the generator robust to repository structure changes
	rootPath := root
	isAutoDiscovery := rootPath == ""
	if isAutoDiscovery {
		// Use "/**" to enable recursive traversal from repository root
		rootPath = "/**"
	}
	
	gw := gitWalker.
		Owner(owner).
		Repo(repo).
		Branch(branch).
		Root(rootPath).
		RegisterFileInterceptor(crdAwareFileInterceptor(br))
	
	// Register dirInterceptor to handle Helm charts which may contain CRDs
	// Note: When doing automatic discovery (recurse mode), dirInterceptor processes directories
	// and fileInterceptor processes files. For Helm charts, dirInterceptor extracts CRDs from
	// the chart structure, while fileInterceptor finds standalone CRD files. This ensures we
	// discover CRDs in both formats without missing any.
	gw = gw.RegisterDirInterceptor(dirInterceptor(br))

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

// parseGitURL parses a git URL and extracts owner, repo, branch, and path components
func parseGitURL(rawURL *url.URL) (owner, repo, branch, path string, err error) {
	gitRepository := giturlparse.NewGitRepository("", "", rawURL.String(), "")
	if err := gitRepository.Parse("", 0, ""); err != nil {
		return "", "", "", "", err
	}

	owner = gitRepository.Owner
	repo = gitRepository.Name
	branch = gitRepository.Branch
	if branch == "" {
		branch = "main"
	}
	path = gitRepository.Path

	if owner == "" || repo == "" {
		return "", "", "", "", fmt.Errorf("invalid git URL format: must have at least owner/repo in path: %s", rawURL.String())
	}

	return owner, repo, branch, path, nil
}

func (gr GitRepo) extractRepoDetailsFromSourceURL() (owner, repo, branch, root string, err error) {
	owner, repo, branch, root, err = parseGitURL(gr.URL)
	if err != nil {
		err = ErrInvalidGitHubSourceURL(err)
		return
	}
	
	// If root is empty, we'll use "/**" for recursive traversal in GetContent
	// This enables automatic CRD discovery
	
	return
}

func (gr GitRepo) ExtractRepoDetailsFromSourceURL() (owner, repo, branch, root string, err error) {
	return gr.extractRepoDetailsFromSourceURL()
}

// fileInterceptor processes all files (original behavior)
func fileInterceptor(br *bufio.Writer) walker.FileInterceptor {
	return func(file walker.File) error {
		tempPath := filepath.Join(os.TempDir(), utils.GetRandomAlphabetsOfDigit(5))
		return ProcessContent(br, tempPath, file.Path)
	}
}

// crdAwareFileInterceptor only processes files that contain CRDs
// This enables automatic CRD discovery without requiring specific directory paths
func crdAwareFileInterceptor(br *bufio.Writer) walker.FileInterceptor {
	return func(file walker.File) error {
		// Check if the file is a YAML/JSON file that might contain CRDs
		fileName := strings.ToLower(file.Name)
		isYAML := strings.HasSuffix(fileName, ".yaml") || strings.HasSuffix(fileName, ".yml")
		isJSON := strings.HasSuffix(fileName, ".json")
		
		if !isYAML && !isJSON {
			// Skip non-YAML/JSON files
			return nil
		}
		
		// Check if the file content contains a CRD
		// Handle both single-document and multi-document YAML files
		content := file.Content
		
		// For multi-document YAML, split by document separator and check each
		documents := strings.Split(content, "\n---\n")
		// Also handle documents separated by "---" at the start of a line
		if len(documents) == 1 {
			// Try splitting by lines starting with "---"
			lines := strings.Split(content, "\n")
			var docs []string
			var currentDoc strings.Builder
			for _, line := range lines {
				if strings.TrimSpace(line) == "---" && currentDoc.Len() > 0 {
					docs = append(docs, currentDoc.String())
					currentDoc.Reset()
				} else {
					if currentDoc.Len() > 0 {
						currentDoc.WriteString("\n")
					}
					currentDoc.WriteString(line)
				}
			}
			if currentDoc.Len() > 0 {
				docs = append(docs, currentDoc.String())
			}
			if len(docs) > 1 {
				documents = docs
			}
		}
		
		// Check each document for CRD
		hasCRD := false
		for _, doc := range documents {
			doc = strings.TrimSpace(doc)
			if doc == "" {
				continue
			}
			// Check for YAML format
			if match, _ := regexp.MatchString(`kind:\s*CustomResourceDefinition`, doc); match {
				hasCRD = true
				break
			}
			// Check for JSON format
			if match, _ := regexp.MatchString(`"kind"\s*:\s*"CustomResourceDefinition"`, doc); match {
				hasCRD = true
				break
			}
		}
		
		if !hasCRD {
			// File doesn't contain a CRD, skip it
			return nil
		}
		
		// File contains a CRD, process it
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
