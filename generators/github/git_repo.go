package github

import (
	"bufio"
	"fmt"
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

func (gr GitRepo) extractRepoDetailsFromSourceURL() (owner, repo, branch, root string, err error) {
	path := strings.TrimPrefix(gr.URL.Path, "/")
	parts := strings.Split(path, "/")
	size := len(parts)
	
	// Minimum required: owner and repo
	if size < 2 {
		err = ErrInvalidGitHubSourceURL(fmt.Errorf("Source URL %s is invalid, must specify at least owner and repo", gr.URL.String()))
		return
	}
	
	owner = parts[0]
	repo = parts[1]
	
	// Remove .git suffix from repo name if present
	repo = strings.TrimSuffix(repo, ".git")
	
	// Handle standard GitHub URL formats:
	// - https://github.com/owner/repo
	// - https://github.com/owner/repo/tree/branch
	// - https://github.com/owner/repo/tree/branch/path/to/dir
	// - git://github.com/owner/repo/branch/path (legacy format)
	
	branch = "main" // default branch
	root = ""
	
	if size >= 3 {
		// Check if this is a standard GitHub URL with /tree/branch format
		if parts[2] == "tree" && size >= 4 {
			// Format: owner/repo/tree/branch[/path...]
			branch = parts[3]
			if size > 4 {
				// Reconstruct the path after branch
				root = strings.Join(parts[4:], "/")
			}
		} else if parts[2] == "blob" {
			// Format: owner/repo/blob/branch/path/to/file
			// This is a file URL, not a directory - we'll treat it as root path
			if size >= 4 {
				branch = parts[3]
				if size > 4 {
					root = strings.Join(parts[4:], "/")
				}
			}
		} else {
			// Legacy format: owner/repo/branch[/path...]
			branch = parts[2]
			if size > 3 {
				// Reconstruct the path after branch
				root = strings.Join(parts[3:], "/")
			}
		}
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
			if strings.Contains(doc, "kind: CustomResourceDefinition") {
				hasCRD = true
				break
			}
			// Check for JSON format
			if strings.Contains(doc, "\"kind\":\"CustomResourceDefinition\"") ||
				strings.Contains(doc, `"kind":"CustomResourceDefinition"`) {
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
