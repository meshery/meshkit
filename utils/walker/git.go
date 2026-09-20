package walker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

// Git represents the Git Walker
type Git struct {
	baseURL            string
	owner              string
	repo               string
	branch             string
	root               string // If the root ends with "/**", then recurse is set to true
	recurse            bool
	showLogs           bool  // By default the logs of gitwalker are not displayed
	maxFileSizeInBytes int64 //defaults to 50 MB
	fileInterceptor    FileInterceptor
	dirInterceptor     DirInterceptor
	referenceName      plumbing.ReferenceName
	// branchSet records whether Branch was called explicitly. Only an
	// explicitly set branch is turned into a ReferenceName, so callers that
	// set neither Branch nor ReferenceName keep cloning the remote's default
	// branch exactly as before.
	branchSet    bool
	token        string
	apiBaseURL   string
	useAPI       bool
	timeout      time.Duration
	progressHook ProgressHook
}

// NewGit returns a pointer to an instance of Git
func NewGit() *Git {
	return &Git{
		branch:             "master",
		baseURL:            "https://github.com", //defaults to a github repo if the url is not set with URL method
		maxFileSizeInBytes: 50000000,             // ~50MB file size limit
		apiBaseURL:         defaultGithubAPIBaseURL,
	}
}

type File struct {
	Name    string `json:"name,omitempty"`
	Content string `json:"content,omitempty"`
	Path    string `json:"path,omitempty"`
}
type Directory struct {
	Name string `json:"name,omitempty"`
	Path string `json:"path,omitempty"`
}
type FileInterceptor func(File) error
type DirInterceptor func(Directory) error

// BaseURL sets git repository base URL and returns a pointer
// to the same Git instance
func (g *Git) BaseURL(baseurl string) *Git {
	g.baseURL = baseurl
	return g
}

// BaseURL sets git repository base URL and returns a pointer
// to the same Git instance
func (g *Git) MaxFileSize(size int64) *Git {
	g.maxFileSizeInBytes = size
	return g
}

// ShowLogs enable the logs and returns a pointer
// to the same Git instance
func (g *Git) ShowLogs() *Git {
	g.showLogs = true
	return g
}

// Owner sets git repository owner and returns a pointer
// to the same Git instance
func (g *Git) Owner(owner string) *Git {
	g.owner = owner
	return g
}

// Repo sets github repository and returns a pointer
// to the same Git instance
func (g *Git) Repo(repo string) *Git {
	g.repo = repo
	return g
}

// Branch sets git repository branch which
// will be cloned and returns a pointer
// to the same Git instance
func (g *Git) Branch(branch string) *Git {
	g.branch = branch
	g.branchSet = true
	return g
}

// Root sets git repository root node from where
// Git walker needs to start traversing and returns
// a pointer to the same Git instance
//
// If the root parameter ends with a "/**" then github walker
// will run in "traversal" mode, ie. it will look into each sub
// directory of the root node
// If path will be prefixed with "/" if not already.
//
// A root of "" or "/" scopes nothing: a walk reads the repository root exactly
// as it always has, and ListInterestingFiles reads it as the whole repository.
func (g *Git) Root(root string) *Git {
	if !strings.HasPrefix(root, "/") {
		root = "/" + root
	}
	g.root = root

	if strings.HasSuffix(root, "/**") {
		g.recurse = true
		g.root = strings.TrimSuffix(root, "/**")
	}
	if strings.Trim(g.root, "/") == "" {
		g.root = ""
	}

	return g
}

func (g *Git) ReferenceName(refName string) *Git {
	g.referenceName = plumbing.ReferenceName(refName)
	return g
}

// Walk will initiate traversal process.
//
// It is equivalent to WalkContext with a background context and is retained
// for callers that predate context support.
func (g *Git) Walk() error {
	return g.WalkContext(context.Background())
}

// WalkContext initiates the traversal process under ctx.
//
// When UseGithubAPI has been enabled and the configured base URL points at
// github.com, the traversal runs over the Git Trees API: the reference is
// resolved to a commit, the tree is listed once recursively, the entries are
// filtered and ranked, and only the surviving blobs are downloaded. Every
// other case - a non-github.com host, a truncated tree, or the API path not
// being enabled - walks a go-git clone exactly as Walk always has.
func (g *Git) WalkContext(ctx context.Context) error {
	ctx, cancel := g.withTimeout(ctx)
	defer cancel()

	if g.useAPI {
		isGithub, err := g.isGithubHost()
		if err != nil {
			return err
		}
		if isGithub {
			walked, standingInForTrees, err := g.treeWalk(ctx)
			if err != nil {
				return err
			}
			if walked {
				return nil
			}
			// treeWalk declined - a truncated tree, a registered directory
			// interceptor, or a root that is itself a symlink - so the clone
			// below is the only complete answer. treeWalk reports whether that
			// clone stands in for a Trees walk of the same repository, which
			// only the truncated tree does; the other two decline the API
			// route to get what the clone already answers, not a different
			// result set.
			g.reportProgress(ProgressUpdate{Stage: ProgressStageClone, Message: "the Trees API could not answer completely, falling back to a clone"})
			return clonewalkContext(ctx, g, standingInForTrees)
		}
	}

	return clonewalkContext(ctx, g, false)
}

// Token sets the GitHub App or OAuth access token used to authenticate against
// the GitHub API and against the go-git clone, so private repositories are
// reachable and the authenticated rate limit applies.
//
// The token is never logged, never placed in an error message and never
// reported through the progress hook.
func (g *Git) Token(token string) *Git {
	g.token = token
	return g
}

// UseGithubAPI opts the walker into the hybrid GitHub crawl: Trees API plus
// selective blob downloads, with a go-git clone as the fallback. It is off by
// default so existing callers keep the clone-and-filter behaviour.
func (g *Git) UseGithubAPI() *Git {
	g.useAPI = true
	return g
}

// Timeout bounds the whole traversal. A zero duration, the default, leaves the
// traversal bounded only by the context passed to WalkContext.
func (g *Git) Timeout(d time.Duration) *Git {
	g.timeout = d
	return g
}

// RegisterProgressHook registers a callback invoked as the walk advances
// through its stages. The hook is called synchronously, so it should return
// promptly.
func (g *Git) RegisterProgressHook(h ProgressHook) *Git {
	g.progressHook = h
	return g
}

func (g *Git) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if g.timeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, g.timeout)
}

func (g *Git) reportProgress(update ProgressUpdate) {
	if g.progressHook == nil {
		return
	}
	g.progressHook(update)
}

// isGithubHost reports whether the configured base URL points at github.com.
// The host is read from the base URL rather than assumed so that GitHub
// Enterprise and other forges keep taking the go-git path.
func (g *Git) isGithubHost() (bool, error) {
	parsed, err := url.Parse(g.baseURL)
	if err != nil {
		return false, ErrInvalidBaseURL(err, g.baseURL)
	}
	return strings.ToLower(parsed.Hostname()) == "github.com", nil
}

// requireGithubHost refuses a repository the Git Trees API cannot answer for.
// The API endpoint is github.com's whatever BaseURL says, so a walker pointed
// at another forge would otherwise send that forge's access token to
// github.com and read an unrelated repository's listing back.
func (g *Git) requireGithubHost() error {
	isGithub, err := g.isGithubHost()
	if err != nil {
		return err
	}
	if !isGithub {
		return ErrInvalidBaseURL(errors.New("the Git Trees API only answers for github.com repositories"), g.baseURL)
	}
	return nil
}

// cloneReferenceName resolves the reference the clone should check out. An
// explicitly set ReferenceName always wins; otherwise an explicitly set branch
// is expanded to its refs/heads form. When neither was set the zero value is
// returned and go-git clones the remote's default branch.
func (g *Git) cloneReferenceName() plumbing.ReferenceName {
	if g.referenceName != "" {
		return g.referenceName
	}
	if g.branchSet && g.branch != "" {
		return plumbing.NewBranchReferenceName(g.branch)
	}
	return ""
}

// ref returns the git reference the GitHub API should resolve: the short name
// of an explicitly set ReferenceName, else an explicitly set branch. It is
// empty when the caller set neither, which means the repository's default
// branch on the API route exactly as it already does on the clone route.
func (g *Git) ref() string {
	if g.referenceName != "" {
		return g.referenceName.Short()
	}
	if g.branchSet && g.branch != "" {
		return g.branch
	}
	return ""
}
func (g *Git) RegisterFileInterceptor(i FileInterceptor) *Git {
	g.fileInterceptor = i
	return g
}

func (g *Git) RegisterDirInterceptor(i DirInterceptor) *Git {
	g.dirInterceptor = i
	return g
}

// errZeroMaxFileSize reports a MaxFileSize of zero, which would silently drop
// every file instead of reading any of them.
func errZeroMaxFileSize() error {
	return ErrInvalidSizeFile(errors.New("max file size passed as 0. Will not read any file"))
}

func clonewalkContext(ctx context.Context, g *Git, standingInForTrees bool) error {
	if g.maxFileSizeInBytes == 0 {
		return errZeroMaxFileSize()
	}

	clonePath := filepath.Join(os.TempDir(), g.repo, strconv.FormatInt(time.Now().UTC().UnixNano(), 10))
	var wg sync.WaitGroup
	defer func() {
		wg.Wait()
		_ = os.RemoveAll(clonePath)
	}()
	var err error
	cloneOptions := &git.CloneOptions{
		URL:          fmt.Sprintf("%s/%s/%s", g.baseURL, g.owner, g.repo),
		SingleBranch: true,
		Depth:        1,
	}

	if refName := g.cloneReferenceName(); refName != "" {
		cloneOptions.ReferenceName = refName
	}

	if g.token != "" {
		cloneOptions.Auth = &githttp.BasicAuth{Username: "x-access-token", Password: g.token}
	}

	if g.showLogs {
		cloneOptions.Progress = os.Stdout
	}

	g.reportProgress(ProgressUpdate{Stage: ProgressStageClone, Message: fmt.Sprintf("cloning %s/%s", g.owner, g.repo)})

	// The working copy is scratch space no caller reads directly, and with a
	// token it holds a private repository, so it is created owner-only before
	// go-git can create it world-readable. Only this clone's own directory is:
	// the parent is shared with every other user's clones of the same
	// repository name, so it keeps the mode it has always had.
	if err = os.MkdirAll(filepath.Dir(clonePath), 0o755); err != nil {
		return ErrCloningRepo(err)
	}
	if err = os.Mkdir(clonePath, 0o700); err != nil {
		return ErrCloningRepo(err)
	}

	_, err = git.PlainCloneContext(ctx, clonePath, false, cloneOptions)
	if err != nil {
		return ErrCloningRepo(err)
	}

	g.reportProgress(ProgressUpdate{Stage: ProgressStageClone, Message: "clone complete"})

	rootPath := filepath.Join(clonePath, g.root)
	info, err := os.Stat(rootPath)
	if err != nil {
		// A root that is a link the walk cannot resolve names nothing it can
		// read, which is what the API route reports for a root its tree does
		// not hold. A root that is missing outright stays a clone failure.
		if linkInfo, lerr := os.Lstat(rootPath); lerr == nil && linkInfo.Mode()&os.ModeSymlink != 0 {
			return ErrRootNotFound(strings.Trim(g.root, "/"), g.apiRef())
		}
		return ErrCloningRepo(err)
	}

	// A root reaching outside the repository copy, by link or by "../"
	// traversal, names nothing of the repository either. Only the configured
	// root fails this way: a link encountered under it is still passed over
	// silently while the walk carries on.
	if !resolvesInsideClone(clonePath, rootPath) {
		return ErrRootNotFound(strings.Trim(g.root, "/"), g.apiRef())
	}

	if !info.IsDir() {
		entryInfo := info
		if linkInfo, lerr := os.Lstat(rootPath); lerr == nil {
			entryInfo = linkInfo
		}
		// The root is the one file the caller named, so the filtering a clone
		// standing in for a Trees walk applies to everything else must not
		// drop it silently: it is refused for its size the way that walk
		// refuses it, and otherwise read on the ordinary clone's terms - a
		// link included, having already been proven to resolve inside the
		// copy above.
		if standingInForTrees && entryInfo.Mode()&os.ModeSymlink == 0 && entryInfo.Size() > g.maxFileSizeInBytes {
			return errOversizedBlob(strings.Trim(g.root, "/"), g.maxFileSizeInBytes)
		}
		if g.skipOnClone(clonePath, rootPath, entryInfo, false) {
			return nil
		}
		err = g.readFile(info, clonePath, rootPath)
		if err != nil {
			return ErrCloningRepo(err)
		}
		return nil
	}

	// If recurse mode is on, we will walk the tree
	if g.recurse {
		err = filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, er error) error {
			if d.IsDir() && g.dirInterceptor != nil {
				return g.dirInterceptor(Directory{
					Name: d.Name(),
					Path: path,
				})
			}
			if d.IsDir() {
				return nil
			}
			f, errInfo := d.Info()
			if err != nil {
				return errInfo
			}
			if g.skipOnClone(clonePath, path, f, standingInForTrees) {
				return nil
			}
			return g.readFile(f, clonePath, path)
		})
		if err != nil {
			return ErrCloningRepo(err)
		}
		return nil
	}

	// If recurse mode is off, we only walk the root directory passed with g.root
	entries, err := os.ReadDir(filepath.Join(clonePath, g.root))
	if err != nil {
		return err
	}
	files := make([]fs.FileInfo, 0, len(entries))
	for _, entry := range entries {
		file, err := entry.Info()
		if err != nil {
			return ErrCloningRepo(err)
		}
		files = append(files, file)
	}

	for _, f := range files {
		fPath := filepath.Join(clonePath, g.root, f.Name())
		if f.IsDir() && g.dirInterceptor != nil {
			name := f.Name()
			wg.Add(1)
			go func(name string, path string, filename string) {
				defer wg.Done()
				err := g.dirInterceptor(Directory{
					Name: filename,
					Path: path,
				})
				if err != nil {
					fmt.Println(err.Error())
				}
			}(name, fPath, f.Name())
			continue
		}
		if f.IsDir() {
			continue
		}
		if g.skipOnClone(clonePath, fPath, f, standingInForTrees) {
			continue
		}
		err := g.readFile(f, clonePath, fPath)
		if err != nil {
			fmt.Println(err.Error())
		}
	}

	return nil
}

// skipOnClone decides whether a file found in the clone is read at all.
//
// A symlink is followed only while it stays inside the repository copy: a link
// whose target resolves outside the copy, or that cannot be resolved, is
// passed over. That holds for every caller.
//
// When the clone is standing in for a Trees walk of the same repository it
// additionally gives what rankTree would have given - every symlink skipped,
// since the Trees route offers none, the same filtered set, and an oversized
// file skipped rather than failing the walk. Any other clone keeps delivering
// each file under Root, oversize error included.
//
// info has to come from an lstat for the symlink bit to survive.
func (g *Git) skipOnClone(clonePath, entryPath string, info fs.FileInfo, standingInForTrees bool) bool {
	if info.Mode()&os.ModeSymlink != 0 {
		return standingInForTrees || !resolvesInsideClone(clonePath, entryPath)
	}
	if !standingInForTrees {
		return false
	}
	if info.Size() > g.maxFileSizeInBytes {
		return true
	}

	relative := g.interceptedPath(clonePath, entryPath)
	if relative == strings.Trim(g.root, "/") {
		return false
	}

	_, _, interesting := classifyPath(relative)
	return !interesting
}

// resolvesInsideClone reports whether entryPath, with every link along it
// followed, names a path inside the repository copy at clonePath. Resolution
// is by lstat and readlink alone, so nothing is opened to decide it, and a
// path that cannot be resolved counts as outside: nothing is read that has not
// been shown to stay inside the copy.
func resolvesInsideClone(clonePath, entryPath string) bool {
	root, err := filepath.EvalSymlinks(clonePath)
	if err != nil {
		return false
	}
	resolved, err := filepath.EvalSymlinks(entryPath)
	if err != nil {
		return false
	}

	relative, err := filepath.Rel(root, resolved)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
}

// interceptedPath reports the path a file read out of a clone is handed to its
// interceptor with. A caller that opted into the GitHub API sees the
// repository-relative path the Trees route would have given it, so the two
// routes stay interchangeable when the walk falls back. Every other caller
// keeps the absolute path into the clone. A Directory is not named this way:
// only the clone route ever produces one, and an interceptor handed a
// directory has nothing to do with it but open it.
func (g *Git) interceptedPath(clonePath, entryPath string) string {
	if !g.useAPI {
		return entryPath
	}
	relative, err := filepath.Rel(clonePath, entryPath)
	if err != nil {
		return entryPath
	}
	return filepath.ToSlash(relative)
}

func (g *Git) readFile(f fs.FileInfo, clonePath, filePath string) error {
	if f.Size() > g.maxFileSizeInBytes {
		return ErrInvalidSizeFile(errors.New("File exceeding size limit"))
	}
	filename, err := os.Open(filePath)
	if err != nil {
		return err
	}
	content, err := io.ReadAll(filename)
	if err != nil {
		return err
	}
	err = g.fileInterceptor(File{
		Name:    f.Name(),
		Path:    g.interceptedPath(clonePath, filePath),
		Content: string(content),
	})
	if err != nil {
		fmt.Println("Could not intercept the file ", f.Name())
	}
	return err
}
