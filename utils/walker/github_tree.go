package walker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"

	"github.com/meshery/meshkit/files/iacext"
	"github.com/meshery/schemas/models/core"
)

// DefaultGithubAPIBaseURL is the GitHub REST API endpoint the hybrid crawl
// talks to unless APIBaseURL overrides it.
const DefaultGithubAPIBaseURL = "https://api.github.com"

// Progress stages reported through a ProgressHook.
const (
	ProgressStageResolveRef ProgressStage = "resolve-ref"
	ProgressStageListTree   ProgressStage = "list-tree"
	ProgressStageRank       ProgressStage = "rank"
	ProgressStageFetchBlob  ProgressStage = "fetch-blob"
	ProgressStageClone      ProgressStage = "clone"
)

// ProgressStage names the phase of a walk a ProgressUpdate describes.
type ProgressStage string

// ProgressUpdate describes how far a walk has advanced. Current and Total are
// only meaningful for stages that iterate over a known number of items, such
// as blob downloads; they are zero elsewhere.
//
// An update never carries the access token.
type ProgressUpdate struct {
	Stage   ProgressStage `json:"stage,omitempty"`
	Message string        `json:"message,omitempty"`
	Current int           `json:"current,omitempty"`
	Total   int           `json:"total,omitempty"`
}

// ProgressHook receives ProgressUpdate values as a walk advances. It is
// invoked synchronously on the walking goroutine.
type ProgressHook func(ProgressUpdate)

// Relevance scores assigned to a candidate from its path alone. A larger score
// ranks earlier. They are exported so that callers rendering a picker can
// group or threshold candidates the same way the walker ranks them.
const (
	ScoreHelmChartDefinition = 100
	ScoreKustomization       = 90
	ScoreDockerCompose       = 80
	ScoreMesheryDesign       = 70
	ScoreChartArchive        = 60
	ScoreGenericYAML         = 20
	ScoreGenericJSON         = 10
)

// CandidateFile is a single interesting file described from Git tree metadata
// alone. No blob has been downloaded to produce it, so Kind is inferred from
// the path and is a hint, not an identification: the authoritative answer
// comes from files.IdentifyFile once the contents are in hand.
type CandidateFile struct {
	// Path is the repository-relative path of the file.
	Path string `json:"path,omitempty"`
	// Name is the base name of Path.
	Name string `json:"name,omitempty"`
	// SHA is the blob SHA, suitable for a later blob fetch.
	SHA string `json:"sha,omitempty"`
	// Size is the blob size in bytes as reported by the tree.
	Size int64 `json:"size,omitempty"`
	// Kind is the IaC file type inferred from the path. It is empty when the
	// path is interesting but not distinctive enough to name a type.
	Kind core.IaCFileTypes `json:"kind,omitempty"`
	// Score is the relevance score used for the ranking.
	Score int `json:"score,omitempty"`
}

// InterestingFiles is the ranked candidate listing for one commit.
type InterestingFiles struct {
	// CommitSHA is the commit the tree was listed from.
	CommitSHA string `json:"commitSha,omitempty"`
	// Truncated reports that GitHub could not return the whole tree in one
	// response, so Candidates is incomplete and a clone is needed to see
	// every file.
	Truncated bool `json:"truncated,omitempty"`
	// Candidates is ranked most interesting first.
	Candidates []CandidateFile `json:"candidates,omitempty"`
}

// githubTreeAPI represents the GitHub API v3 response to
// /repos/{owner}/{repo}/git/trees/{sha}?recursive=1
type githubTreeAPI struct {
	SHA       string            `json:"sha,omitempty"`
	Truncated bool              `json:"truncated,omitempty"`
	Tree      []githubTreeEntry `json:"tree,omitempty"`
}

type githubTreeEntry struct {
	Path string `json:"path,omitempty"`
	Mode string `json:"mode,omitempty"`
	Type string `json:"type,omitempty"`
	SHA  string `json:"sha,omitempty"`
	Size int64  `json:"size,omitempty"`
}

type githubCommitAPI struct {
	SHA string `json:"sha,omitempty"`
}

type githubBlobAPI struct {
	SHA      string `json:"sha,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Content  string `json:"content,omitempty"`
	Encoding string `json:"encoding,omitempty"`
}

// ListInterestingFiles resolves the configured reference to a commit, lists
// that commit's tree recursively in a single request, and returns the ranked
// candidate files. No blob is downloaded, so the result is cheap enough to
// render an import picker from.
//
// A truncated listing is reported through InterestingFiles.Truncated rather
// than as an error: the candidates returned are still usable, they are just
// not the whole repository.
func (g *Git) ListInterestingFiles(ctx context.Context) (InterestingFiles, error) {
	ctx, cancel := g.withTimeout(ctx)
	defer cancel()

	return g.listInterestingFiles(ctx)
}

func (g *Git) listInterestingFiles(ctx context.Context) (InterestingFiles, error) {
	ref := g.ref()

	g.reportProgress(ProgressUpdate{Stage: ProgressStageResolveRef, Message: fmt.Sprintf("resolving %s", ref)})
	commitSHA, err := g.resolveRef(ctx, ref)
	if err != nil {
		return InterestingFiles{}, err
	}

	g.reportProgress(ProgressUpdate{Stage: ProgressStageListTree, Message: fmt.Sprintf("listing tree for %s", commitSHA)})
	tree, err := g.fetchTree(ctx, commitSHA)
	if err != nil {
		return InterestingFiles{}, err
	}

	listing := InterestingFiles{
		CommitSHA:  commitSHA,
		Truncated:  tree.Truncated,
		Candidates: g.rankTree(tree.Tree),
	}
	g.reportProgress(ProgressUpdate{
		Stage:   ProgressStageRank,
		Message: fmt.Sprintf("ranked %d interesting files", len(listing.Candidates)),
		Total:   len(listing.Candidates),
	})

	return listing, nil
}

// FetchCandidates downloads the blob behind each candidate and hands it to the
// registered file interceptor, in the order given. Candidates usually come
// from ListInterestingFiles, filtered down to whatever the user selected.
//
// File.Path on the intercepted file is the repository-relative path, not a
// local filesystem path: nothing is written to disk on this route.
func (g *Git) FetchCandidates(ctx context.Context, candidates []CandidateFile) error {
	ctx, cancel := g.withTimeout(ctx)
	defer cancel()

	return g.fetchCandidates(ctx, candidates)
}

func (g *Git) fetchCandidates(ctx context.Context, candidates []CandidateFile) error {
	if g.fileInterceptor == nil {
		return nil
	}

	for i, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return ErrFetchingGitBlob(err, candidate.Path)
		}

		g.reportProgress(ProgressUpdate{
			Stage:   ProgressStageFetchBlob,
			Message: candidate.Path,
			Current: i + 1,
			Total:   len(candidates),
		})

		content, err := g.fetchBlob(ctx, candidate)
		if err != nil {
			return err
		}

		if err := g.fileInterceptor(File{
			Name:    candidate.Name,
			Path:    candidate.Path,
			Content: content,
		}); err != nil {
			return err
		}
	}

	return nil
}

// treeWalk runs the hybrid GitHub crawl. It reports whether the walk was
// actually carried out over the API; false means the caller should fall back
// to a go-git clone.
func (g *Git) treeWalk(ctx context.Context) (bool, error) {
	// Directory interception needs a working tree on disk to hand to the
	// interceptor, which the API route never produces, so those callers keep
	// the clone.
	if g.dirInterceptor != nil {
		return false, nil
	}

	if g.maxFileSizeInBytes == 0 {
		return false, ErrInvalidSizeFile(fmt.Errorf("max file size passed as 0. Will not read any file"))
	}

	listing, err := g.listInterestingFiles(ctx)
	if err != nil {
		return false, err
	}

	// A truncated tree is an incomplete listing, so the clone is the only way
	// to see every file.
	if listing.Truncated {
		return false, nil
	}

	if err := g.fetchCandidates(ctx, listing.Candidates); err != nil {
		return false, err
	}

	return true, nil
}

// resolveRef turns a branch, tag or reference name into a commit SHA.
func (g *Git) resolveRef(ctx context.Context, ref string) (string, error) {
	endpoint := fmt.Sprintf("%s/repos/%s/%s/commits/%s", g.apiEndpoint(), g.owner, g.repo, url.PathEscape(ref))

	body, err := g.get(ctx, endpoint)
	if err != nil {
		return "", ErrResolvingGitRef(err, ref)
	}

	commit := githubCommitAPI{}
	if err := json.Unmarshal(body, &commit); err != nil {
		return "", ErrResolvingGitRef(err, ref)
	}
	if commit.SHA == "" {
		return "", ErrResolvingGitRef(fmt.Errorf("the GitHub API returned no commit SHA"), ref)
	}

	return commit.SHA, nil
}

// fetchTree lists the whole tree of a commit in a single recursive request.
func (g *Git) fetchTree(ctx context.Context, commitSHA string) (githubTreeAPI, error) {
	endpoint := fmt.Sprintf("%s/repos/%s/%s/git/trees/%s?recursive=1", g.apiEndpoint(), g.owner, g.repo, url.PathEscape(commitSHA))

	body, err := g.get(ctx, endpoint)
	if err != nil {
		return githubTreeAPI{}, ErrFetchingGitTree(err, commitSHA)
	}

	tree := githubTreeAPI{}
	if err := json.Unmarshal(body, &tree); err != nil {
		return githubTreeAPI{}, ErrFetchingGitTree(err, commitSHA)
	}

	return tree, nil
}

// fetchBlob downloads a single blob and returns its decoded contents.
func (g *Git) fetchBlob(ctx context.Context, candidate CandidateFile) (string, error) {
	endpoint := fmt.Sprintf("%s/repos/%s/%s/git/blobs/%s", g.apiEndpoint(), g.owner, g.repo, url.PathEscape(candidate.SHA))

	body, err := g.get(ctx, endpoint)
	if err != nil {
		return "", ErrFetchingGitBlob(err, candidate.Path)
	}

	blob := githubBlobAPI{}
	if err := json.Unmarshal(body, &blob); err != nil {
		return "", ErrFetchingGitBlob(err, candidate.Path)
	}

	if blob.Encoding != "base64" {
		return blob.Content, nil
	}

	// GitHub wraps base64 blob content at a fixed width.
	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(blob.Content, "\n", ""))
	if err != nil {
		return "", ErrFetchingGitBlob(err, candidate.Path)
	}

	return string(decoded), nil
}

func (g *Git) apiEndpoint() string {
	if g.apiBaseURL == "" {
		return DefaultGithubAPIBaseURL
	}
	return g.apiBaseURL
}

// get issues an authenticated GET against the GitHub API. The token travels in
// the Authorization header and is never placed in a URL or an error message.
func (g *Git) get(ctx context.Context, endpoint string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	if g.token != "" {
		request.Header.Set("Authorization", "Bearer "+g.token)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = response.Body.Close()
	}()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the GitHub API responded with %s: %s", response.Status, githubAPIMessage(body))
	}

	return body, nil
}

// githubAPIMessage pulls the human readable message out of a GitHub API error
// body, falling back to the status alone when there is none.
func githubAPIMessage(body []byte) string {
	payload := struct {
		Message string `json:"message,omitempty"`
	}{}
	if err := json.Unmarshal(body, &payload); err != nil || payload.Message == "" {
		return "no message returned"
	}
	return payload.Message
}

// rankTree filters the tree entries down to the ones worth importing and ranks
// them, most interesting first.
func (g *Git) rankTree(entries []githubTreeEntry) []CandidateFile {
	candidates := []CandidateFile{}
	root := strings.Trim(g.root, "/")

	for _, entry := range entries {
		if entry.Type != "blob" {
			continue
		}
		if !g.underRoot(root, entry.Path) {
			continue
		}
		// The tree already reports the size, so an oversized blob is skipped
		// before it is ever downloaded.
		if entry.Size > g.maxFileSizeInBytes {
			continue
		}

		kind, score, interesting := ClassifyPath(entry.Path)
		if !interesting {
			continue
		}

		candidates = append(candidates, CandidateFile{
			Path:  entry.Path,
			Name:  path.Base(entry.Path),
			SHA:   entry.SHA,
			Size:  entry.Size,
			Kind:  kind,
			Score: score,
		})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score
		}
		// Shallower paths first: a chart at the repository root is a more
		// likely import target than one buried in a test fixture.
		di, dj := strings.Count(candidates[i].Path, "/"), strings.Count(candidates[j].Path, "/")
		if di != dj {
			return di < dj
		}
		return candidates[i].Path < candidates[j].Path
	})

	return candidates
}

// underRoot mirrors the scoping Root applies to the clone walk: in recursive
// mode everything below the root counts, otherwise only the files sitting
// directly in it, and a root naming a single file matches only that file.
func (g *Git) underRoot(root, entryPath string) bool {
	if root == "" {
		return g.recurse || !strings.Contains(entryPath, "/")
	}
	if entryPath == root {
		return true
	}
	if !strings.HasPrefix(entryPath, root+"/") {
		return false
	}
	if g.recurse {
		return true
	}
	return path.Dir(entryPath) == root
}

// ClassifyPath infers the IaC file type of a path and scores how interesting
// it is for a design import, from the path alone. The third return value
// reports whether the path is worth importing at all.
//
// The inference is deliberately name based because it runs before any content
// has been downloaded. An empty kind means the path is worth fetching but not
// distinctive enough to name a type; files.IdentifyFile makes that call once
// the contents are available.
func ClassifyPath(filePath string) (core.IaCFileTypes, int, bool) {
	name := strings.ToLower(path.Base(filePath))
	ext := strings.ToLower(path.Ext(name))
	if strings.HasSuffix(name, ".tar.gz") {
		ext = ".tar.gz"
	}
	stem := strings.TrimSuffix(name, ext)

	switch name {
	case "chart.yaml", "chart.yml":
		return core.HelmChart, ScoreHelmChartDefinition, true
	case "docker-compose.yaml", "docker-compose.yml", "compose.yaml", "compose.yml":
		return core.DockerCompose, ScoreDockerCompose, true
	}

	if stem == "kustomization" && iacext.ValidKustomizeFileExtensions[ext] {
		return core.K8sKustomize, ScoreKustomization, true
	}

	if stem == "design" || strings.HasSuffix(stem, ".design") {
		if ext == ".yaml" || ext == ".yml" || ext == ".json" {
			return core.MesheryDesign, ScoreMesheryDesign, true
		}
	}

	// Chart and design archives, including OCI artifacts packaged as tarballs.
	if iacext.ValidHelmChartFileExtensions[ext] {
		return core.HelmChart, ScoreChartArchive, true
	}

	switch ext {
	case ".yaml", ".yml":
		return "", ScoreGenericYAML, true
	case ".json":
		return "", ScoreGenericJSON, true
	}

	return "", 0, false
}
