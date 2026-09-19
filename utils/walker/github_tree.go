package walker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"

	"github.com/meshery/schemas/models/core"
)

// defaultGithubAPIBaseURL is the GitHub REST API endpoint the hybrid crawl and
// the Contents walker talk to.
const defaultGithubAPIBaseURL = "https://api.github.com"

// symlinkFileMode is the git file mode the Trees API reports for a symlink.
const symlinkFileMode = "120000"

// blobFetchConcurrency bounds how many blobs are downloaded at once. It also
// bounds how many downloaded blobs are held in memory at once, because a slot
// is only released once its blob has been handed to the interceptor.
const blobFetchConcurrency = 8

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

// ProgressHook receives ProgressUpdate values as a walk advances. How it is
// delivered differs per walker and is documented on each RegisterProgressHook:
// Git calls it one update at a time, while Github calls it from every goroutine
// its fan-out walks a node with. An implementation that touches shared state
// must therefore be safe for concurrent use.
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
// The listing is always recursive, because a picker is asking what the
// repository holds: with no Root configured that is the whole repository, and
// a Root narrows which subtree is listed, never how deep. Walk keeps its own
// historical meaning for Root - the named directory only, unless the caller
// asked for "/**" - and is unaffected by this.
//
// A truncated listing is reported through InterestingFiles.Truncated rather
// than as an error: the candidates returned are still usable, they are just
// not the whole repository.
//
// The repository must be on github.com. There is no clone to fall back to on
// this route, and the access token must never travel to a host the caller did
// not configure.
func (g *Git) ListInterestingFiles(ctx context.Context) (InterestingFiles, error) {
	if err := g.requireGithubHost(); err != nil {
		return InterestingFiles{}, err
	}
	if g.maxFileSizeInBytes == 0 {
		return InterestingFiles{}, errZeroMaxFileSize()
	}

	ctx, cancel := g.withTimeout(ctx)
	defer cancel()

	return g.listInterestingFiles(ctx, true)
}

func (g *Git) listInterestingFiles(ctx context.Context, recursive bool) (InterestingFiles, error) {
	ref := g.apiRef()

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
		Candidates: g.rankTree(tree.Tree, recursive),
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
// Downloads overlap, up to blobFetchConcurrency at a time, but the interceptor
// is still called once at a time and in order, so it does not have to be safe
// for concurrent use.
//
// File.Path on the intercepted file is the repository-relative path, not a
// local filesystem path: nothing is written to disk on this route.
//
// The repository must be on github.com, for the reason ListInterestingFiles
// gives.
func (g *Git) FetchCandidates(ctx context.Context, candidates []CandidateFile) error {
	if err := g.requireGithubHost(); err != nil {
		return err
	}

	ctx, cancel := g.withTimeout(ctx)
	defer cancel()

	return g.fetchCandidates(ctx, candidates)
}

func (g *Git) fetchCandidates(ctx context.Context, candidates []CandidateFile) error {
	if g.fileInterceptor == nil {
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type blob struct {
		content string
		err     error
	}

	results := make([]chan blob, len(candidates))
	for i := range results {
		results[i] = make(chan blob, 1)
	}
	// A slot is taken before a download starts and only given back once that
	// blob has been delivered, so neither the requests in flight nor the
	// contents held in memory outrun the interceptor.
	slots := make(chan struct{}, blobFetchConcurrency)

	go func() {
		for i := range candidates {
			select {
			case slots <- struct{}{}:
			case <-ctx.Done():
				return
			}

			go func(i int) {
				content, err := g.fetchBlob(ctx, candidates[i])
				results[i] <- blob{content: content, err: err}
			}(i)
		}
	}()

	for i, candidate := range candidates {
		var fetched blob
		select {
		case fetched = <-results[i]:
		case <-ctx.Done():
			return ErrFetchingGitBlob(ctx.Err(), candidate.Path)
		}
		if fetched.err != nil {
			return fetched.err
		}

		g.reportProgress(ProgressUpdate{
			Stage:   ProgressStageFetchBlob,
			Message: candidate.Path,
			Current: i + 1,
			Total:   len(candidates),
		})

		if err := g.fileInterceptor(File{
			Name:    candidate.Name,
			Path:    candidate.Path,
			Content: fetched.content,
		}); err != nil {
			return err
		}

		<-slots
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
		return false, errZeroMaxFileSize()
	}

	listing, err := g.listInterestingFiles(ctx, g.recurse)
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

// apiRef reports which reference the API route should resolve. An explicitly
// set ReferenceName wins, then an explicitly set Branch; when the caller set
// neither, HEAD stands for the repository's default branch, which is what a
// clone with no reference configured checks out.
func (g *Git) apiRef() string {
	if ref := g.ref(); ref != "" {
		return ref
	}
	return "HEAD"
}

// escapeRefPath escapes a git reference for the path of a request. A name like
// release/1.2 is several path segments, which is how the commits endpoint
// spells a ref, so each segment is escaped on its own: the separators survive
// while nothing inside a segment can introduce one.
//
// A ref git itself would refuse is refused here rather than escaped. An empty
// segment, or one that opens with a dot or ends in .lock, is not a name git
// would accept (see git-check-ref-format), and a segment such as "." or ".."
// would travel as a path element rather than as part of the reference.
func escapeRefPath(ref string) (string, error) {
	segments := strings.Split(ref, "/")
	for i, segment := range segments {
		if segment == "" || strings.HasPrefix(segment, ".") || strings.HasSuffix(segment, ".lock") {
			return "", fmt.Errorf("%q is not a git reference git would accept", ref)
		}
		segments[i] = url.PathEscape(segment)
	}
	return strings.Join(segments, "/"), nil
}

// resolveRef turns a branch, tag or reference name into a commit SHA.
func (g *Git) resolveRef(ctx context.Context, ref string) (string, error) {
	escaped, err := escapeRefPath(ref)
	if err != nil {
		return "", ErrResolvingGitRef(err, ref)
	}

	endpoint := fmt.Sprintf("%s/repos/%s/%s/commits/%s", g.apiBaseURL, g.owner, g.repo, escaped)

	body, err := g.get(ctx, endpoint, 0)
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
	endpoint := fmt.Sprintf("%s/repos/%s/%s/git/trees/%s?recursive=1", g.apiBaseURL, g.owner, g.repo, url.PathEscape(commitSHA))

	body, err := g.get(ctx, endpoint, 0)
	if err != nil {
		return githubTreeAPI{}, ErrFetchingGitTree(err, commitSHA)
	}

	tree := githubTreeAPI{}
	if err := json.Unmarshal(body, &tree); err != nil {
		return githubTreeAPI{}, ErrFetchingGitTree(err, commitSHA)
	}

	return tree, nil
}

// fetchBlob downloads a single blob and returns its decoded contents. Nothing
// larger than MaxFileSize is read: a candidate that reports an oversize is
// refused before the request, and the response itself is read only as far as
// the limit allows, so a candidate that understates its size cannot buffer
// more than one blob's worth of it either.
func (g *Git) fetchBlob(ctx context.Context, candidate CandidateFile) (string, error) {
	if candidate.Size > g.maxFileSizeInBytes {
		return "", errOversizedBlob(candidate.Path, g.maxFileSizeInBytes)
	}

	endpoint := fmt.Sprintf("%s/repos/%s/%s/git/blobs/%s", g.apiBaseURL, g.owner, g.repo, url.PathEscape(candidate.SHA))

	body, err := g.get(ctx, endpoint, blobResponseLimit(g.maxFileSizeInBytes))
	if err != nil {
		if errors.Is(err, errResponseTooLarge) {
			return "", errOversizedBlob(candidate.Path, g.maxFileSizeInBytes)
		}
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

// errResponseTooLarge reports a response body that ran past the ceiling the
// caller's MaxFileSize allows, so it was never buffered whole.
var errResponseTooLarge = errors.New("the response ran past the configured file size limit")

// blobResponseLimit is how many bytes of a blob response are worth reading for
// a file of at most maxFileSizeInBytes: GitHub hands the contents over as
// base64, which inflates them by four thirds, wrapped in lines and in a JSON
// envelope of a few fields.
func blobResponseLimit(maxFileSizeInBytes int64) int64 {
	return (maxFileSizeInBytes+2)/3*4 + maxFileSizeInBytes/50 + 8192
}

func errOversizedBlob(path string, maxFileSizeInBytes int64) error {
	return ErrInvalidSizeFile(fmt.Errorf("%s exceeds the %d byte file size limit", path, maxFileSizeInBytes))
}

// get issues an authenticated GET against the GitHub API. The token travels in
// the Authorization header and is never placed in a URL or an error message.
//
// A limit above zero bounds how much of the body is read: a response that runs
// past it fails with errResponseTooLarge rather than being buffered whole or
// silently truncated.
func (g *Git) get(ctx context.Context, endpoint string, limit int64) ([]byte, error) {
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

	var reader io.Reader = response.Body
	if limit > 0 {
		reader = io.LimitReader(response.Body, limit+1)
	}

	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if limit > 0 && int64(len(body)) > limit {
		return nil, errResponseTooLarge
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
func (g *Git) rankTree(entries []githubTreeEntry, recursive bool) []CandidateFile {
	candidates := []CandidateFile{}
	root := strings.Trim(g.root, "/")

	for _, entry := range entries {
		// The Trees API reports a symlink as a blob whose content is the link
		// target path rather than the target's contents, so it is neither
		// worth ranking nor safe to hand to an interceptor as a file.
		if entry.Type != "blob" || entry.Mode == symlinkFileMode {
			continue
		}
		if !underRoot(root, entry.Path, recursive) {
			continue
		}
		// The tree already reports the size, so an oversized blob is skipped
		// before it is ever downloaded.
		if entry.Size > g.maxFileSizeInBytes {
			continue
		}

		kind, score, interesting := classifyPath(entry.Path)
		// A root naming one exact file is an explicit request for it, which
		// the clone route honours whatever the file is called.
		if !interesting && entry.Path != root {
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

// underRoot scopes an entry to the configured root: recursively everything
// below it counts, otherwise only the files sitting directly in it, and a root
// naming a single file matches only that file. An empty root is the whole
// repository when recursive, its top level otherwise.
func underRoot(root, entryPath string, recursive bool) bool {
	if root == "" {
		return recursive || !strings.Contains(entryPath, "/")
	}
	if entryPath == root {
		return true
	}
	if !strings.HasPrefix(entryPath, root+"/") {
		return false
	}
	if recursive {
		return true
	}
	return path.Dir(entryPath) == root
}

// classifyPath infers the IaC file type of a path and scores how interesting
// it is for a design import, from the path alone. The third return value
// reports whether the path is worth importing at all.
//
// The inference is deliberately name based because it runs before any content
// has been downloaded. An empty kind means the path is worth fetching but not
// distinctive enough to name a type; files.IdentifyFile makes that call once
// the contents are available.
func classifyPath(filePath string) (core.IaCFileTypes, int, bool) {
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

	if stem == "kustomization" && (ext == ".yaml" || ext == ".yml") {
		return core.K8sKustomize, ScoreKustomization, true
	}

	if stem == "design" || strings.HasSuffix(stem, ".design") {
		if ext == ".yaml" || ext == ".yml" || ext == ".json" {
			return core.MesheryDesign, ScoreMesheryDesign, true
		}
	}

	// Chart and OCI artifacts, which Helm only ever packages as a gzipped
	// tarball. The wider files.ValidHelmChartFileExtensions table describes what
	// an uploaded chart may arrive as; applying it here would label every .zip,
	// .gz and .tar in a repository a Helm chart.
	if ext == ".tgz" || ext == ".tar.gz" {
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
