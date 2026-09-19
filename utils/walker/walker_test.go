package walker

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	meshkiterrors "github.com/meshery/meshkit/errors"
)

func TestWalkLocalDirectory(t *testing.T) {
	dir := t.TempDir()
	nestedDir := filepath.Join(dir, "nested")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	rootFile := filepath.Join(dir, "root.txt")
	nestedFile := filepath.Join(nestedDir, "child.yaml")
	if err := os.WriteFile(rootFile, []byte("root content"), 0o644); err != nil {
		t.Fatalf("failed to write root file: %v", err)
	}
	if err := os.WriteFile(nestedFile, []byte("nested content"), 0o644); err != nil {
		t.Fatalf("failed to write nested file: %v", err)
	}

	files, err := WalkLocalDirectory(dir)
	if err != nil {
		t.Fatalf("WalkLocalDirectory() returned error: %v", err)
	}

	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}

	got := map[string]string{}
	for _, file := range files {
		got[file.Name] = file.Content
	}

	if got["root.txt"] != "root content" {
		t.Errorf("expected root file content to be %q, got %q", "root content", got["root.txt"])
	}
	if got["child.yaml"] != "nested content" {
		t.Errorf("expected nested file content to be %q, got %q", "nested content", got["child.yaml"])
	}
}

func TestGitConfigurationMethods(t *testing.T) {
	g := NewGit()
	if g.branch != "master" {
		t.Fatalf("expected default branch to be %q, got %q", "master", g.branch)
	}
	if g.baseURL != "https://github.com" {
		t.Fatalf("expected default base URL to be %q, got %q", "https://github.com", g.baseURL)
	}
	if g.maxFileSizeInBytes != 50000000 {
		t.Fatalf("expected default max file size to be %d, got %d", 50000000, g.maxFileSizeInBytes)
	}

	fileInterceptor := func(File) error { return nil }
	dirInterceptor := func(Directory) error { return nil }

	if g.BaseURL("https://example.com") != g {
		t.Fatal("BaseURL should return the same Git instance")
	}
	if g.MaxFileSize(2048) != g {
		t.Fatal("MaxFileSize should return the same Git instance")
	}
	if g.ShowLogs() != g {
		t.Fatal("ShowLogs should return the same Git instance")
	}
	if g.Owner("meshery") != g {
		t.Fatal("Owner should return the same Git instance")
	}
	if g.Repo("meshkit") != g {
		t.Fatal("Repo should return the same Git instance")
	}
	if g.Branch("main") != g {
		t.Fatal("Branch should return the same Git instance")
	}
	if g.Root("configs/**") != g {
		t.Fatal("Root should return the same Git instance")
	}
	if g.ReferenceName("refs/heads/main") != g {
		t.Fatal("ReferenceName should return the same Git instance")
	}
	if g.RegisterFileInterceptor(fileInterceptor) != g {
		t.Fatal("RegisterFileInterceptor should return the same Git instance")
	}
	if g.RegisterDirInterceptor(dirInterceptor) != g {
		t.Fatal("RegisterDirInterceptor should return the same Git instance")
	}

	if g.baseURL != "https://example.com" {
		t.Errorf("expected base URL to be updated, got %q", g.baseURL)
	}
	if g.maxFileSizeInBytes != 2048 {
		t.Errorf("expected max file size to be updated, got %d", g.maxFileSizeInBytes)
	}
	if !g.showLogs {
		t.Error("expected showLogs to be enabled")
	}
	if g.owner != "meshery" {
		t.Errorf("expected owner to be %q, got %q", "meshery", g.owner)
	}
	if g.repo != "meshkit" {
		t.Errorf("expected repo to be %q, got %q", "meshkit", g.repo)
	}
	if g.branch != "main" {
		t.Errorf("expected branch to be %q, got %q", "main", g.branch)
	}
	if g.root != "/configs" {
		t.Errorf("expected root to be %q, got %q", "/configs", g.root)
	}
	if !g.recurse {
		t.Error("expected recurse to be enabled for /** root")
	}
	if string(g.referenceName) != "refs/heads/main" {
		t.Errorf("expected reference name to be %q, got %q", "refs/heads/main", string(g.referenceName))
	}
	if g.fileInterceptor == nil {
		t.Error("expected file interceptor to be registered")
	}
	if g.dirInterceptor == nil {
		t.Error("expected dir interceptor to be registered")
	}
}

func TestGithubConfigurationMethods(t *testing.T) {
	g := NewGithub()
	if g.branch != "main" {
		t.Fatalf("expected default branch to be %q, got %q", "main", g.branch)
	}

	fileInterceptor := func(GithubContentAPI) error { return nil }
	dirInterceptor := func(GithubDirectoryContentAPI) error { return nil }

	if g.Owner("meshery") != g {
		t.Fatal("Owner should return the same Github instance")
	}
	if g.Repo("meshkit") != g {
		t.Fatal("Repo should return the same Github instance")
	}
	if g.Branch("master") != g {
		t.Fatal("Branch should return the same Github instance")
	}
	if g.Root("utils/**") != g {
		t.Fatal("Root should return the same Github instance")
	}
	if g.RegisterFileInterceptor(fileInterceptor) != g {
		t.Fatal("RegisterFileInterceptor should return the same Github instance")
	}
	if g.RegisterDirInterceptor(dirInterceptor) != g {
		t.Fatal("RegisterDirInterceptor should return the same Github instance")
	}

	if g.owner != "meshery" {
		t.Errorf("expected owner to be %q, got %q", "meshery", g.owner)
	}
	if g.repo != "meshkit" {
		t.Errorf("expected repo to be %q, got %q", "meshkit", g.repo)
	}
	if g.branch != "master" {
		t.Errorf("expected branch to be %q, got %q", "master", g.branch)
	}
	if g.root != "utils" {
		t.Errorf("expected root to be %q, got %q", "utils", g.root)
	}
	if !g.recurse {
		t.Error("expected recurse to be enabled for /** root")
	}
	if g.fileInterceptor == nil {
		t.Error("expected file interceptor to be registered")
	}
	if g.dirInterceptor == nil {
		t.Error("expected dir interceptor to be registered")
	}
}

func TestGitWalkReturnsInvalidSizeErrorWhenLimitIsZero(t *testing.T) {
	err := NewGit().MaxFileSize(0).Walk()
	if err == nil {
		t.Fatal("expected Walk to return an error when max file size is zero")
	}
	if got := meshkiterrors.GetCode(err); got != ErrInvalidSizeFileCode {
		t.Fatalf("expected error code %q, got %q", ErrInvalidSizeFileCode, got)
	}
}

func TestGitReadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat test file: %v", err)
	}

	var intercepted File
	g := NewGit().
		MaxFileSize(1024).
		RegisterFileInterceptor(func(file File) error {
			intercepted = file
			return nil
		})

	if err := g.readFile(info, dir, path); err != nil {
		t.Fatalf("readFile() returned error: %v", err)
	}

	if intercepted.Name != "sample.txt" {
		t.Errorf("expected intercepted name to be %q, got %q", "sample.txt", intercepted.Name)
	}
	if intercepted.Path != path {
		t.Errorf("expected intercepted path to be %q, got %q", path, intercepted.Path)
	}
	if intercepted.Content != "hello world" {
		t.Errorf("expected intercepted content to be %q, got %q", "hello world", intercepted.Content)
	}
}

func TestGitReadFileRejectsOversizedFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.txt")
	if err := os.WriteFile(path, []byte("12345"), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat test file: %v", err)
	}

	err = NewGit().MaxFileSize(2).readFile(info, dir, path)
	if err == nil {
		t.Fatal("expected readFile to reject oversized files")
	}
	if got := meshkiterrors.GetCode(err); got != ErrInvalidSizeFileCode {
		t.Fatalf("expected error code %q, got %q", ErrInvalidSizeFileCode, got)
	}
}

func TestGitWalkTraversesLocalRepository(t *testing.T) {
	baseDir := t.TempDir()
	repoPath := filepath.Join(baseDir, "owner", "sample")
	createCommittedRepo(t, repoPath, map[string]string{
		"README.md":                "repo root",
		"configs/root.txt":         "root file",
		"configs/nested/child.yml": "nested file",
	})

	t.Run("recursive root", func(t *testing.T) {
		var mu sync.Mutex
		files := map[string]string{}
		dirs := map[string]struct{}{}

		g := NewGit().
			BaseURL("file://" + baseDir).
			Owner("owner").
			Repo("sample").
			Root("configs/**").
			RegisterFileInterceptor(func(file File) error {
				mu.Lock()
				defer mu.Unlock()
				files[file.Name] = file.Content
				return nil
			}).
			RegisterDirInterceptor(func(dir Directory) error {
				mu.Lock()
				defer mu.Unlock()
				dirs[dir.Name] = struct{}{}
				return nil
			})

		if err := g.Walk(); err != nil {
			t.Fatalf("Walk() returned error: %v", err)
		}

		if len(files) != 2 {
			t.Fatalf("expected 2 intercepted files, got %d", len(files))
		}
		if files["root.txt"] != "root file" {
			t.Errorf("expected root file content to be %q, got %q", "root file", files["root.txt"])
		}
		if files["child.yml"] != "nested file" {
			t.Errorf("expected nested file content to be %q, got %q", "nested file", files["child.yml"])
		}
		if _, ok := dirs["configs"]; !ok {
			t.Error("expected root directory to be intercepted in recursive mode")
		}
		if _, ok := dirs["nested"]; !ok {
			t.Error("expected nested directory to be intercepted in recursive mode")
		}
	})

	t.Run("non-recursive root", func(t *testing.T) {
		var mu sync.Mutex
		files := map[string]string{}
		dirs := map[string]struct{}{}

		g := NewGit().
			BaseURL("file://" + baseDir).
			Owner("owner").
			Repo("sample").
			Root("configs").
			RegisterFileInterceptor(func(file File) error {
				mu.Lock()
				defer mu.Unlock()
				files[file.Name] = file.Content
				return nil
			}).
			RegisterDirInterceptor(func(dir Directory) error {
				mu.Lock()
				defer mu.Unlock()
				dirs[dir.Name] = struct{}{}
				return nil
			})

		if err := g.Walk(); err != nil {
			t.Fatalf("Walk() returned error: %v", err)
		}

		if len(files) != 1 {
			t.Fatalf("expected 1 intercepted file, got %d", len(files))
		}
		if files["root.txt"] != "root file" {
			t.Errorf("expected root file content to be %q, got %q", "root file", files["root.txt"])
		}
		if _, ok := dirs["nested"]; !ok {
			t.Error("expected immediate child directory to be intercepted in non-recursive mode")
		}
		if _, ok := files["child.yml"]; ok {
			t.Error("did not expect nested files to be intercepted in non-recursive mode")
		}
	})

	t.Run("file root", func(t *testing.T) {
		var intercepted File
		g := NewGit().
			BaseURL("file://" + baseDir).
			Owner("owner").
			Repo("sample").
			Root("configs/root.txt").
			RegisterFileInterceptor(func(file File) error {
				intercepted = file
				return nil
			})

		if err := g.Walk(); err != nil {
			t.Fatalf("Walk() returned error: %v", err)
		}

		if intercepted.Name != "root.txt" {
			t.Errorf("expected file root to intercept %q, got %q", "root.txt", intercepted.Name)
		}
		if intercepted.Content != "root file" {
			t.Errorf("expected file root content to be %q, got %q", "root file", intercepted.Content)
		}
	})
}

func createCommittedRepo(t *testing.T, repoPath string, files map[string]string) {
	t.Helper()

	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("failed to create repo directory: %v", err)
	}

	repo, err := git.PlainInit(repoPath, false)
	if err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatalf("failed to get worktree: %v", err)
	}

	for name, content := range files {
		fullPath := filepath.Join(repoPath, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatalf("failed to create parent directory for %s: %v", name, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write %s: %v", name, err)
		}
		if _, err := worktree.Add(name); err != nil {
			t.Fatalf("failed to add %s to repo: %v", name, err)
		}
	}

	if _, err := worktree.Commit("init", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
			When:  time.Now(),
		},
	}); err != nil {
		t.Fatalf("failed to commit test repo: %v", err)
	}
}

// addCommittedSymlink commits link as a symlink to target inside repoPath.
func addCommittedSymlink(t *testing.T, repoPath, link, target string) {
	t.Helper()

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		t.Fatalf("failed to open repo: %v", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatalf("failed to get worktree: %v", err)
	}

	fullPath := filepath.Join(repoPath, filepath.FromSlash(link))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("failed to create parent directory for %s: %v", link, err)
	}
	if err := os.Symlink(filepath.FromSlash(target), fullPath); err != nil {
		t.Fatalf("failed to create symlink %s: %v", link, err)
	}
	if _, err := worktree.Add(link); err != nil {
		t.Fatalf("failed to add %s to repo: %v", link, err)
	}
	if _, err := worktree.Commit("symlink", &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@example.com", When: time.Now()},
	}); err != nil {
		t.Fatalf("failed to commit %s: %v", link, err)
	}
}

func TestGitCloneReferenceName(t *testing.T) {
	tests := []struct {
		name          string
		branch        string
		referenceName string
		want          string
	}{
		{
			name: "neither set clones the remote default branch",
			want: "",
		},
		{
			name:   "an explicit branch becomes a branch reference",
			branch: "feature",
			want:   "refs/heads/feature",
		},
		{
			name:          "an explicit reference name is used as is",
			referenceName: "refs/tags/v1.2.3",
			want:          "refs/tags/v1.2.3",
		},
		{
			name:          "an explicit reference name wins over a branch",
			branch:        "feature",
			referenceName: "refs/tags/v1.2.3",
			want:          "refs/tags/v1.2.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGit()
			if tt.branch != "" {
				g = g.Branch(tt.branch)
			}
			if tt.referenceName != "" {
				g = g.ReferenceName(tt.referenceName)
			}

			if got := string(g.cloneReferenceName()); got != tt.want {
				t.Errorf("expected clone reference name %q, got %q", tt.want, got)
			}
		})
	}
}

func TestGitRef(t *testing.T) {
	tests := []struct {
		name          string
		branch        string
		referenceName string
		want          string
	}{
		{name: "empty when neither is set, so the default branch is resolved", want: ""},
		{name: "uses the configured branch", branch: "release", want: "release"},
		{name: "uses the short name of a reference", referenceName: "refs/tags/v1.2.3", want: "v1.2.3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGit()
			if tt.branch != "" {
				g = g.Branch(tt.branch)
			}
			if tt.referenceName != "" {
				g = g.ReferenceName(tt.referenceName)
			}

			if got := g.ref(); got != tt.want {
				t.Errorf("expected ref %q, got %q", tt.want, got)
			}
		})
	}
}

func TestGitWalkHonoursConfiguredBranch(t *testing.T) {
	baseDir := t.TempDir()
	repoPath := filepath.Join(baseDir, "owner", "sample")
	createCommittedRepo(t, repoPath, map[string]string{"configs/root.txt": "default branch"})
	commitOnBranch(t, repoPath, "feature", "configs/root.txt", "feature branch")

	tests := []struct {
		name   string
		branch string
		want   string
	}{
		{name: "unset branch keeps the remote default", want: "default branch"},
		{name: "explicit branch is cloned", branch: "feature", want: "feature branch"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var intercepted File
			g := NewGit().
				BaseURL("file://" + baseDir).
				Owner("owner").
				Repo("sample").
				Root("configs/root.txt").
				RegisterFileInterceptor(func(file File) error {
					intercepted = file
					return nil
				})
			if tt.branch != "" {
				g = g.Branch(tt.branch)
			}

			if err := g.Walk(); err != nil {
				t.Fatalf("Walk() returned error: %v", err)
			}
			if intercepted.Content != tt.want {
				t.Errorf("expected content %q, got %q", tt.want, intercepted.Content)
			}
		})
	}
}

func TestGitWalkPathsMatchTheRouteTheCallerOptedInTo(t *testing.T) {
	// A caller that enabled the hybrid crawl handles repository-relative paths,
	// so the clone it falls back to must hand it the same kind of path, for
	// directories as much as for files. A caller that never opted in keeps the
	// absolute path into the clone, which is the only openable form.
	baseDir := t.TempDir()
	repoPath := filepath.Join(baseDir, "owner", "sample")
	createCommittedRepo(t, repoPath, map[string]string{"configs/nested/child.yml": "nested file"})

	walk := func(t *testing.T, useAPI bool) (File, []string) {
		t.Helper()

		var intercepted File
		directories := []string{}
		g := NewGit().
			BaseURL("file://" + baseDir).
			Owner("owner").
			Repo("sample").
			Root("configs/**").
			RegisterFileInterceptor(func(file File) error {
				intercepted = file
				return nil
			}).
			RegisterDirInterceptor(func(dir Directory) error {
				directories = append(directories, dir.Path)
				return nil
			})
		if useAPI {
			g = g.UseGithubAPI()
		}

		if err := g.Walk(); err != nil {
			t.Fatalf("Walk() returned error: %v", err)
		}
		return intercepted, directories
	}

	t.Run("clone-only callers keep the absolute clone path", func(t *testing.T) {
		intercepted, directories := walk(t, false)

		underClone := filepath.Join(os.TempDir(), "sample") + string(os.PathSeparator)
		if !filepath.IsAbs(intercepted.Path) {
			t.Fatalf("expected an absolute path into the clone, got %q", intercepted.Path)
		}
		if !strings.HasPrefix(intercepted.Path, underClone) {
			t.Errorf("expected the path to sit under the temporary clone, got %q", intercepted.Path)
		}
		if !strings.HasSuffix(intercepted.Path, filepath.Join("configs", "nested", "child.yml")) {
			t.Errorf("expected the path to end at the walked file, got %q", intercepted.Path)
		}

		if len(directories) != 2 {
			t.Fatalf("expected the root and the nested directory to be intercepted, got %v", directories)
		}
		for _, directory := range directories {
			if !filepath.IsAbs(directory) || !strings.HasPrefix(directory, underClone) {
				t.Errorf("expected an absolute directory path into the clone, got %q", directory)
			}
		}
		if !strings.HasSuffix(directories[1], filepath.Join("configs", "nested")) {
			t.Errorf("expected the nested directory to end at its path, got %q", directories[1])
		}
	})

	t.Run("callers that opted into the api get repository-relative paths", func(t *testing.T) {
		intercepted, directories := walk(t, true)

		if intercepted.Path != "configs/nested/child.yml" {
			t.Errorf("expected the repository-relative file path, got %q", intercepted.Path)
		}
		if intercepted.Content != "nested file" {
			t.Errorf("expected the file contents to be unchanged, got %q", intercepted.Content)
		}
		if want := []string{"configs", "configs/nested"}; !reflect.DeepEqual(directories, want) {
			t.Errorf("expected the repository-relative directory paths %v, got %v", want, directories)
		}
	})
}

func TestGitWalkCloneRouteFiltersOnlyForOptedInCallers(t *testing.T) {
	// The truncated-tree fallback runs the clone route for a caller that opted
	// into the hybrid crawl, so that route owes it the same ranked,
	// size-bounded set the API route would have delivered. A caller that never
	// opted in keeps receiving every file, oversize error included.
	baseDir := t.TempDir()
	createCommittedRepo(t, filepath.Join(baseDir, "owner", "plain"), map[string]string{
		"configs/deployment.yaml": "kind: ConfigMap",
		"configs/README.md":       "docs",
		"configs/notes.txt":       "notes",
	})
	createCommittedRepo(t, filepath.Join(baseDir, "owner", "oversized"), map[string]string{
		"configs/deployment.yaml": "kind: ConfigMap",
		"configs/huge.yaml":       strings.Repeat("x", 2000),
	})

	walk := func(t *testing.T, repo string, useAPI bool) ([]string, error) {
		t.Helper()

		delivered := []string{}
		g := NewGit().
			BaseURL("file://" + baseDir).
			Owner("owner").
			Repo(repo).
			Root("configs/**").
			MaxFileSize(1000).
			RegisterFileInterceptor(func(file File) error {
				delivered = append(delivered, filepath.Base(file.Path))
				return nil
			})
		if useAPI {
			g = g.UseGithubAPI()
		}

		err := g.Walk()
		return delivered, err
	}

	t.Run("clone-only callers receive every file", func(t *testing.T) {
		delivered, err := walk(t, "plain", false)
		if err != nil {
			t.Fatalf("Walk() returned error: %v", err)
		}
		if want := []string{"README.md", "deployment.yaml", "notes.txt"}; !reflect.DeepEqual(delivered, want) {
			t.Errorf("expected every file to be delivered as %v, got %v", want, delivered)
		}
	})

	t.Run("callers that opted into the api receive only ranked candidates", func(t *testing.T) {
		delivered, err := walk(t, "plain", true)
		if err != nil {
			t.Fatalf("Walk() returned error: %v", err)
		}
		if want := []string{"deployment.yaml"}; !reflect.DeepEqual(delivered, want) {
			t.Errorf("expected only the interesting file to be delivered as %v, got %v", want, delivered)
		}
	})

	t.Run("clone-only callers still fail on an oversized file", func(t *testing.T) {
		_, err := walk(t, "oversized", false)
		if err == nil {
			t.Fatal("expected the walk to fail on a file over the size limit")
		}
		if code := meshkiterrors.GetCode(err); code != ErrCloningRepoCode {
			t.Fatalf("expected error code %q, got %q: %v", ErrCloningRepoCode, code, err)
		}
	})

	t.Run("callers that opted into the api skip an oversized file", func(t *testing.T) {
		delivered, err := walk(t, "oversized", true)
		if err != nil {
			t.Fatalf("expected an oversized file to be skipped, got error: %v", err)
		}
		if want := []string{"deployment.yaml"}; !reflect.DeepEqual(delivered, want) {
			t.Errorf("expected the oversized file to be skipped, leaving %v, got %v", want, delivered)
		}
	})
}

func TestGitSkipOnCloneFollowsLinksOnlyInsideTheRepositoryCopy(t *testing.T) {
	// A link is read only while its target stays inside the repository copy.
	// A caller that opted into the GitHub API skips every link instead, since
	// the Trees route offers none.
	clonePath := filepath.Join(t.TempDir(), "clone")
	outsidePath := t.TempDir()

	for _, dir := range []string{"configs", "data"} {
		if err := os.MkdirAll(filepath.Join(clonePath, dir), 0o755); err != nil {
			t.Fatalf("failed to create %s: %v", dir, err)
		}
	}
	for path, content := range map[string]string{
		filepath.Join(clonePath, "configs", "real.yaml"): "kind: ConfigMap",
		filepath.Join(clonePath, "data", "shared.yaml"):  "kind: Secret",
		filepath.Join(outsidePath, "shared.yaml"):        "kind: Secret",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write %s: %v", path, err)
		}
	}
	for link, target := range map[string]string{
		"inside.yaml":   filepath.Join("..", "data", "shared.yaml"),
		"chained.yaml":  "inside.yaml",
		"outside.yaml":  filepath.Join(outsidePath, "shared.yaml"),
		"dangling.yaml": "missing.yaml",
	} {
		if err := os.Symlink(target, filepath.Join(clonePath, "configs", link)); err != nil {
			t.Fatalf("failed to create symlink %s: %v", link, err)
		}
	}

	tests := []struct {
		name         string
		entry        string
		wantSkipped  bool
		wantSkippedO bool
	}{
		{name: "a regular file is read either way", entry: "real.yaml", wantSkipped: false, wantSkippedO: false},
		{name: "a link inside the copy is read unless the caller opted in", entry: "inside.yaml", wantSkipped: false, wantSkippedO: true},
		{name: "a chain of links inside the copy is read too", entry: "chained.yaml", wantSkipped: false, wantSkippedO: true},
		{name: "a link resolving outside the copy is never read", entry: "outside.yaml", wantSkipped: true, wantSkippedO: true},
		{name: "a link that cannot be resolved is never read", entry: "dangling.yaml", wantSkipped: true, wantSkippedO: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entryPath := filepath.Join(clonePath, "configs", tt.entry)
			info, err := os.Lstat(entryPath)
			if err != nil {
				t.Fatalf("failed to stat %s: %v", tt.entry, err)
			}

			g := NewGit().MaxFileSize(1000).Root("configs/**")
			if got := g.skipOnClone(clonePath, entryPath, info); got != tt.wantSkipped {
				t.Errorf("expected skipOnClone to report %t for a clone-only caller, got %t", tt.wantSkipped, got)
			}
			if got := g.UseGithubAPI().skipOnClone(clonePath, entryPath, info); got != tt.wantSkippedO {
				t.Errorf("expected skipOnClone to report %t for a caller that opted in, got %t", tt.wantSkippedO, got)
			}
		})
	}
}

func TestGitWalkCloneRouteSkipsSymlinksForOptedInCallers(t *testing.T) {
	// The API route never offers a symlink, whose blob holds the link target
	// rather than the target's contents, so neither does the clone an opted-in
	// caller falls back to - it must not read through the link instead.
	baseDir := t.TempDir()
	repoPath := filepath.Join(baseDir, "owner", "linked")
	createCommittedRepo(t, repoPath, map[string]string{
		"configs/real.yaml":   "kind: ConfigMap",
		"secrets/secret.yaml": "kind: Secret",
	})
	addCommittedSymlink(t, repoPath, "configs/link.yaml", "../secrets/secret.yaml")

	walk := func(t *testing.T, useAPI bool) []string {
		t.Helper()

		delivered := []string{}
		g := NewGit().
			BaseURL("file://" + baseDir).
			Owner("owner").
			Repo("linked").
			Root("configs/**").
			RegisterFileInterceptor(func(file File) error {
				delivered = append(delivered, filepath.Base(file.Path))
				return nil
			})
		if useAPI {
			g = g.UseGithubAPI()
		}

		if err := g.Walk(); err != nil {
			t.Fatalf("Walk() returned error: %v", err)
		}
		return delivered
	}

	t.Run("callers that opted into the api never read the link", func(t *testing.T) {
		want := []string{"real.yaml"}
		if got := walk(t, true); !reflect.DeepEqual(got, want) {
			t.Errorf("expected the symlink to be skipped, leaving %v, got %v", want, got)
		}
	})

	t.Run("a root naming the symlink itself is not read either", func(t *testing.T) {
		delivered := []string{}
		err := NewGit().
			BaseURL("file://" + baseDir).
			Owner("owner").
			Repo("linked").
			Root("configs/link.yaml").
			UseGithubAPI().
			RegisterFileInterceptor(func(file File) error {
				delivered = append(delivered, filepath.Base(file.Path))
				return nil
			}).
			Walk()
		if err != nil {
			t.Fatalf("Walk() returned error: %v", err)
		}
		if len(delivered) != 0 {
			t.Errorf("expected an explicitly named symlink to be skipped, got %v", delivered)
		}
	})
}

func TestGitWalkContextRespectsCancellation(t *testing.T) {
	baseDir := t.TempDir()
	repoPath := filepath.Join(baseDir, "owner", "sample")
	createCommittedRepo(t, repoPath, map[string]string{"configs/root.txt": "root file"})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := NewGit().
		BaseURL("file://" + baseDir).
		Owner("owner").
		Repo("sample").
		Root("configs").
		RegisterFileInterceptor(func(File) error { return nil }).
		WalkContext(ctx)
	if err == nil {
		t.Fatal("expected WalkContext to fail once the context is cancelled")
	}
	if got := meshkiterrors.GetCode(err); got != ErrCloningRepoCode {
		t.Fatalf("expected error code %q, got %q", ErrCloningRepoCode, got)
	}
}

// commitOnBranch creates branch and commits path with content on it, leaving
// the repository checked out on its original branch.
func commitOnBranch(t *testing.T, repoPath, branch, path, content string) {
	t.Helper()

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		t.Fatalf("failed to open repo: %v", err)
	}

	head, err := repo.Head()
	if err != nil {
		t.Fatalf("failed to read HEAD: %v", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatalf("failed to get worktree: %v", err)
	}

	if err := worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(branch),
		Create: true,
	}); err != nil {
		t.Fatalf("failed to create branch %s: %v", branch, err)
	}

	fullPath := filepath.Join(repoPath, filepath.FromSlash(path))
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
	if _, err := worktree.Add(path); err != nil {
		t.Fatalf("failed to add %s: %v", path, err)
	}
	if _, err := worktree.Commit("branch commit", &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@example.com", When: time.Now()},
	}); err != nil {
		t.Fatalf("failed to commit on %s: %v", branch, err)
	}

	if err := worktree.Checkout(&git.CheckoutOptions{Branch: head.Name()}); err != nil {
		t.Fatalf("failed to check out %s: %v", head.Name(), err)
	}
}
