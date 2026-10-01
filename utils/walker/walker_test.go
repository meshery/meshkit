package walker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
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
	// A caller that enabled the hybrid crawl handles repository-relative file
	// paths, so the clone it falls back to hands it the same kind of path. A
	// directory has no second route to match - only a clone produces one - and
	// an interceptor handed a directory can do nothing with it but open it, so
	// its path stays the real one on disk for every caller.
	baseDir := t.TempDir()
	repoPath := filepath.Join(baseDir, "owner", "sample")
	createCommittedRepo(t, repoPath, map[string]string{"configs/nested/child.yml": "nested file"})

	// The clone is removed once the walk returns, so a directory is opened
	// while the interceptor holds it, which is the only time it is of any use.
	walk := func(t *testing.T, useAPI bool) (File, []string, []string) {
		t.Helper()

		var intercepted File
		directories := []string{}
		unreadable := []string{}
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
				if _, err := os.ReadDir(dir.Path); err != nil {
					unreadable = append(unreadable, dir.Path)
				}
				return nil
			})
		if useAPI {
			g = g.UseGithubAPI()
		}

		if err := g.Walk(); err != nil {
			t.Fatalf("Walk() returned error: %v", err)
		}
		return intercepted, directories, unreadable
	}

	t.Run("clone-only callers keep the absolute clone path", func(t *testing.T) {
		intercepted, directories, unreadable := walk(t, false)

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
		if len(unreadable) != 0 {
			t.Errorf("expected every intercepted directory to be readable, got %v", unreadable)
		}
	})

	t.Run("callers that opted into the api get repository-relative file paths", func(t *testing.T) {
		intercepted, directories, unreadable := walk(t, true)

		if intercepted.Path != "configs/nested/child.yml" {
			t.Errorf("expected the repository-relative file path, got %q", intercepted.Path)
		}
		if intercepted.Content != "nested file" {
			t.Errorf("expected the file contents to be unchanged, got %q", intercepted.Content)
		}

		if len(directories) != 2 {
			t.Fatalf("expected the root and the nested directory to be intercepted, got %v", directories)
		}
		for _, directory := range directories {
			if !filepath.IsAbs(directory) {
				t.Errorf("expected a directory path on disk, got %q", directory)
			}
		}
		if len(unreadable) != 0 {
			t.Errorf("expected every intercepted directory to be readable, got %v", unreadable)
		}
	})
}

func TestGitCloneRouteFiltersOnlyWhenStandingInForTheTreesWalk(t *testing.T) {
	// A clone standing in for a truncated Trees walk owes the caller the set
	// that walk would have delivered. Every other clone - a non-github.com
	// host, a registered directory interceptor - behaves as it always has,
	// whether or not the caller enabled the hybrid crawl.
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

	walker := func(repo string, delivered *[]string) *Git {
		return NewGit().
			BaseURL("file://" + baseDir).
			Owner("owner").
			Repo(repo).
			Root("configs/**").
			MaxFileSize(1000).
			UseGithubAPI().
			RegisterFileInterceptor(func(file File) error {
				*delivered = append(*delivered, filepath.Base(file.Path))
				return nil
			})
	}

	// A walk against a non-github.com host takes the clone route without ever
	// standing in for a Trees walk, however the caller configured the crawl.
	walk := func(t *testing.T, repo string) ([]string, error) {
		t.Helper()

		delivered := []string{}
		err := walker(repo, &delivered).Walk()
		return delivered, err
	}

	// A truncated tree is what leaves the clone standing in for the Trees walk
	// of the same repository, which no local stub can produce end to end.
	standIn := func(t *testing.T, repo string) ([]string, error) {
		t.Helper()

		delivered := []string{}
		err := clonewalkContext(context.Background(), walker(repo, &delivered), true)
		return delivered, err
	}

	t.Run("an ordinary clone receives every file", func(t *testing.T) {
		delivered, err := walk(t, "plain")
		if err != nil {
			t.Fatalf("Walk() returned error: %v", err)
		}
		if want := []string{"README.md", "deployment.yaml", "notes.txt"}; !reflect.DeepEqual(delivered, want) {
			t.Errorf("expected every file to be delivered as %v, got %v", want, delivered)
		}
	})

	t.Run("a clone standing in for a tree receives only ranked candidates", func(t *testing.T) {
		delivered, err := standIn(t, "plain")
		if err != nil {
			t.Fatalf("Walk() returned error: %v", err)
		}
		if want := []string{"deployment.yaml"}; !reflect.DeepEqual(delivered, want) {
			t.Errorf("expected only the interesting file to be delivered as %v, got %v", want, delivered)
		}
	})

	t.Run("an ordinary clone still fails on an oversized file", func(t *testing.T) {
		_, err := walk(t, "oversized")
		if err == nil {
			t.Fatal("expected the walk to fail on a file over the size limit")
		}
		if code := meshkiterrors.GetCode(err); code != ErrCloningRepoCode {
			t.Fatalf("expected error code %q, got %q: %v", ErrCloningRepoCode, code, err)
		}
	})

	t.Run("a clone standing in for a tree skips an oversized file", func(t *testing.T) {
		delivered, err := standIn(t, "oversized")
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
	// A clone standing in for a Trees walk skips every link instead, since the
	// Trees route offers none; opting into the GitHub API does not on its own
	// make a clone do that.
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
		name              string
		entry             string
		wantSkipped       bool
		wantSkippedByTree bool
	}{
		{name: "a regular file is read either way", entry: "real.yaml", wantSkipped: false, wantSkippedByTree: false},
		{name: "a link inside the copy is read unless the clone stands in for a tree", entry: "inside.yaml", wantSkipped: false, wantSkippedByTree: true},
		{name: "a chain of links inside the copy is read too", entry: "chained.yaml", wantSkipped: false, wantSkippedByTree: true},
		{name: "a link resolving outside the copy is never read", entry: "outside.yaml", wantSkipped: true, wantSkippedByTree: true},
		{name: "a link that cannot be resolved is never read", entry: "dangling.yaml", wantSkipped: true, wantSkippedByTree: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entryPath := filepath.Join(clonePath, "configs", tt.entry)
			info, err := os.Lstat(entryPath)
			if err != nil {
				t.Fatalf("failed to stat %s: %v", tt.entry, err)
			}

			g := NewGit().MaxFileSize(1000).Root("configs/**").UseGithubAPI()
			if got := g.skipOnClone(clonePath, entryPath, info, false); got != tt.wantSkipped {
				t.Errorf("expected skipOnClone to report %t for an ordinary clone, got %t", tt.wantSkipped, got)
			}
			if got := g.skipOnClone(clonePath, entryPath, info, true); got != tt.wantSkippedByTree {
				t.Errorf("expected skipOnClone to report %t while standing in for a tree, got %t", tt.wantSkippedByTree, got)
			}
		})
	}
}

func TestGitCloneRouteSkipsSymlinksWhenStandingInForTheTreesWalk(t *testing.T) {
	// The Trees route never offers a symlink, whose blob holds the link target
	// rather than the target's contents, so neither does a clone standing in
	// for it. An ordinary clone still reads a link that stays inside the copy.
	baseDir := t.TempDir()
	repoPath := filepath.Join(baseDir, "owner", "linked")
	createCommittedRepo(t, repoPath, map[string]string{
		"configs/real.yaml":   "kind: ConfigMap",
		"secrets/secret.yaml": "kind: Secret",
	})
	addCommittedSymlink(t, repoPath, "configs/link.yaml", "../secrets/secret.yaml")

	walker := func(root string, delivered *[]string) *Git {
		return NewGit().
			BaseURL("file://" + baseDir).
			Owner("owner").
			Repo("linked").
			Root(root).
			UseGithubAPI().
			RegisterFileInterceptor(func(file File) error {
				*delivered = append(*delivered, filepath.Base(file.Path))
				return nil
			})
	}

	walk := func(t *testing.T, standingInForTrees bool) []string {
		t.Helper()

		delivered := []string{}
		if err := clonewalkContext(context.Background(), walker("configs/**", &delivered), standingInForTrees); err != nil {
			t.Fatalf("the clone walk returned error: %v", err)
		}
		return delivered
	}

	t.Run("an ordinary clone reads a link that stays inside the copy", func(t *testing.T) {
		want := []string{"link.yaml", "real.yaml"}
		if got := walk(t, false); !reflect.DeepEqual(got, want) {
			t.Errorf("expected the link inside the copy to be read, delivering %v, got %v", want, got)
		}
	})

	t.Run("a clone standing in for a tree never reads the link", func(t *testing.T) {
		want := []string{"real.yaml"}
		if got := walk(t, true); !reflect.DeepEqual(got, want) {
			t.Errorf("expected the symlink to be skipped, leaving %v, got %v", want, got)
		}
	})

	t.Run("a root naming the symlink itself is read by either clone", func(t *testing.T) {
		// The root is the one file the caller named. An ordinary clone reads
		// it - that is the clone a walk declining the API route falls back to
		// - and so does a clone standing in for a Trees walk, which reaches
		// this root only under a truncated tree and must not turn a working
		// single-file import into an empty successful one.
		for _, standingInForTrees := range []bool{false, true} {
			delivered := []string{}
			if err := clonewalkContext(context.Background(), walker("configs/link.yaml", &delivered), standingInForTrees); err != nil {
				t.Fatalf("the clone walk returned error with standingInForTrees=%t: %v", standingInForTrees, err)
			}
			if want := []string{"link.yaml"}; !reflect.DeepEqual(delivered, want) {
				t.Errorf("expected the named link to be read with standingInForTrees=%t, delivering %v, got %v", standingInForTrees, want, delivered)
			}
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

func TestGitCloneDirectoryIsNotReadableByOtherLocalUsers(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX directory permissions are not enforced on Windows")
	}

	// A token makes private repositories clonable, so the working copy the
	// clone checks out has to stay readable to this process alone for as long
	// as it exists. Every clone gets the same treatment: the directory is
	// scratch space no caller reads directly. Its parent, <tmp>/<repo>, is
	// shared with every other user cloning a repository of that name, so it
	// has to stay traversable - and the repository is named uniquely here so
	// that this walk is the one creating it.
	baseDir := t.TempDir()
	repoName := fmt.Sprintf("meshkit-walker-perms-%d", time.Now().UnixNano())
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Join(os.TempDir(), repoName)) })
	repoPath := filepath.Join(baseDir, "owner", repoName)
	createCommittedRepo(t, repoPath, map[string]string{"configs/child.yml": "kind: ConfigMap"})

	var clonePath string
	var cloneMode os.FileMode
	var parentMode os.FileMode
	err := NewGit().
		BaseURL("file://" + baseDir).
		Owner("owner").
		Repo(repoName).
		Root("configs/**").
		RegisterFileInterceptor(func(File) error { return nil }).
		RegisterDirInterceptor(func(dir Directory) error {
			// The clone is removed once the walk returns, so it is read here,
			// from the root the intercepted directory sits under.
			if clonePath != "" {
				return nil
			}
			clonePath = filepath.Dir(dir.Path)
			info, err := os.Stat(clonePath)
			if err != nil {
				return err
			}
			cloneMode = info.Mode().Perm()

			// The parent is shared with every other user's clones of a
			// repository of the same name, so it has to stay traversable.
			parentInfo, err := os.Stat(filepath.Dir(clonePath))
			if err != nil {
				return err
			}
			parentMode = parentInfo.Mode().Perm()
			return nil
		}).
		Walk()
	if err != nil {
		t.Fatalf("Walk() returned error: %v", err)
	}

	if clonePath == "" {
		t.Fatal("expected the walk to intercept a directory inside the clone")
	}
	if !strings.HasPrefix(clonePath, filepath.Join(os.TempDir(), repoName)+string(os.PathSeparator)) {
		t.Fatalf("expected the clone to sit under the temporary directory, got %q", clonePath)
	}
	if cloneMode&0o077 != 0 {
		t.Errorf("expected the clone directory to deny group and other access, got %#o", cloneMode)
	}
	if cloneMode&0o700 != 0o700 {
		t.Errorf("expected the clone directory to stay fully accessible to its owner, got %#o", cloneMode)
	}
	// mkdir masks the requested mode with the process umask, so the parent is
	// held against a directory created here the same ordinary way rather than
	// against fixed bits: what matters is that it was not tightened.
	control := filepath.Join(t.TempDir(), "control")
	if err := os.MkdirAll(control, 0o755); err != nil {
		t.Fatalf("failed to create the control directory: %v", err)
	}
	controlInfo, err := os.Stat(control)
	if err != nil {
		t.Fatalf("failed to read the control directory: %v", err)
	}
	if want := controlInfo.Mode().Perm(); parentMode != want {
		t.Errorf("expected the shared parent directory to keep the ordinary directory mode %#o, got %#o", want, parentMode)
	}
}

// createCommittedRepoWithLinks commits files and then symlinks. Git stores a
// link's target verbatim, an absolute one included, and go-git checks it back
// out as a real link.
func createCommittedRepoWithLinks(t *testing.T, repoPath string, files map[string]string, links map[string]string) {
	t.Helper()

	createCommittedRepo(t, repoPath, files)

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		t.Fatalf("failed to open repo: %v", err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatalf("failed to get worktree: %v", err)
	}

	for name, target := range links {
		fullPath := filepath.Join(repoPath, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatalf("failed to create parent directory for %s: %v", name, err)
		}
		if err := os.Symlink(target, fullPath); err != nil {
			t.Fatalf("failed to create symlink %s: %v", name, err)
		}
		if _, err := worktree.Add(name); err != nil {
			t.Fatalf("failed to add %s to repo: %v", name, err)
		}
	}

	if _, err := worktree.Commit("links", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
			When:  time.Now(),
		},
	}); err != nil {
		t.Fatalf("failed to commit links: %v", err)
	}
}

func TestGitCloneWalkReadsALinkedRootOnlyWhenItStaysInsideTheRepository(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("committed symlinks are not checked out as links on Windows")
	}

	// The configured root can itself be a committed symlink, and the
	// containment rule holds there too - but for the root it fails the walk
	// rather than skipping, since nothing would be delivered: a root whose
	// target lands outside the repository copy, or that cannot be resolved at
	// all, is refused with the root-not-found error. A root resolving inside
	// the copy is walked as usual.
	baseDir := t.TempDir()

	// The clone lands at <tmp>/<repo>/<nanos>, so this target sits two levels
	// above it - outside the copy, while still inside the temporary directory.
	escapeName := fmt.Sprintf("meshkit-walker-escape-%d", time.Now().UnixNano())
	escapePath := filepath.Join(os.TempDir(), escapeName)
	if err := os.MkdirAll(escapePath, 0o700); err != nil {
		t.Fatalf("failed to create the directory outside the repository: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(escapePath) })
	if err := os.WriteFile(filepath.Join(escapePath, "secret.yaml"), []byte("kind: Secret"), 0o600); err != nil {
		t.Fatalf("failed to write the file outside the repository: %v", err)
	}

	createCommittedRepoWithLinks(t,
		filepath.Join(baseDir, "owner", "sample"),
		map[string]string{"inside/app.yaml": "kind: ConfigMap"},
		map[string]string{
			"escaping":     filepath.Join("..", "..", escapeName),
			"unresolvable": filepath.Join("missing", "target"),
			"contained":    "inside",
		},
	)

	walk := func(t *testing.T, root string) ([]string, error) {
		t.Helper()

		delivered := []string{}
		err := NewGit().
			BaseURL("file://" + baseDir).
			Owner("owner").
			Repo("sample").
			Root(root).
			RegisterFileInterceptor(func(file File) error {
				delivered = append(delivered, filepath.Base(file.Path)+"="+file.Content)
				return nil
			}).
			Walk()
		return delivered, err
	}

	walked := func(t *testing.T, root string) []string {
		t.Helper()

		delivered, err := walk(t, root)
		if err != nil {
			t.Fatalf("Walk() returned error: %v", err)
		}
		return delivered
	}

	refused := func(t *testing.T, root string) {
		t.Helper()

		delivered, err := walk(t, root)
		if err == nil {
			t.Fatal("expected a root reaching outside the repository copy to fail")
		}
		if code := meshkiterrors.GetCode(err); code != ErrRootNotFoundCode {
			t.Fatalf("expected error code %q, got %q: %v", ErrRootNotFoundCode, code, err)
		}
		if len(delivered) != 0 {
			t.Errorf("expected nothing from outside the repository copy, got %v", delivered)
		}
	}

	t.Run("a root linking outside the copy is refused", func(t *testing.T) {
		refused(t, "escaping")
	})

	t.Run("a root that cannot be resolved is refused", func(t *testing.T) {
		refused(t, "unresolvable")
	})

	t.Run("a root linking inside the copy is walked as usual", func(t *testing.T) {
		want := []string{"app.yaml=kind: ConfigMap"}
		if delivered := walked(t, "contained"); !reflect.DeepEqual(delivered, want) {
			t.Errorf("expected the linked directory to be walked as %v, got %v", want, delivered)
		}
	})

	t.Run("an ordinary root is unaffected", func(t *testing.T) {
		want := []string{"app.yaml=kind: ConfigMap"}
		if delivered := walked(t, "inside"); !reflect.DeepEqual(delivered, want) {
			t.Errorf("expected the real directory to be walked as %v, got %v", want, delivered)
		}
	})
}

func TestGitCloneWalkReadsAFileRootOnlyWhenItStaysInsideTheRepository(t *testing.T) {
	// Root is caller supplied - in the import flow it is a user-selected path -
	// and filepath.Join cleans its "../" segments away before the walk stats
	// it, so containment has to hold for a file root as it does for a
	// directory one, with no symlink involved.
	baseDir := t.TempDir()

	// The clone lands at <tmp>/<repo>/<nanos>, so this file sits two levels
	// above it - outside the copy, while still inside the temporary directory.
	escapeName := fmt.Sprintf("meshkit-walker-file-escape-%d", time.Now().UnixNano())
	escapePath := filepath.Join(os.TempDir(), escapeName)
	if err := os.MkdirAll(escapePath, 0o700); err != nil {
		t.Fatalf("failed to create the directory outside the repository: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(escapePath) })
	if err := os.WriteFile(filepath.Join(escapePath, "secret.yaml"), []byte("kind: Secret"), 0o600); err != nil {
		t.Fatalf("failed to write the file outside the repository: %v", err)
	}

	createCommittedRepo(t, filepath.Join(baseDir, "owner", "sample"), map[string]string{
		"inside/app.yaml": "kind: ConfigMap",
	})

	walk := func(t *testing.T, root string) ([]string, error) {
		t.Helper()

		delivered := []string{}
		err := NewGit().
			BaseURL("file://" + baseDir).
			Owner("owner").
			Repo("sample").
			Root(root).
			RegisterFileInterceptor(func(file File) error {
				delivered = append(delivered, filepath.Base(file.Path)+"="+file.Content)
				return nil
			}).
			Walk()
		return delivered, err
	}

	t.Run("a file root traversing outside the copy is refused", func(t *testing.T) {
		delivered, err := walk(t, filepath.Join("..", "..", escapeName, "secret.yaml"))
		if err == nil {
			t.Fatal("expected a file root reaching outside the repository copy to fail")
		}
		if code := meshkiterrors.GetCode(err); code != ErrRootNotFoundCode {
			t.Fatalf("expected error code %q, got %q: %v", ErrRootNotFoundCode, code, err)
		}
		if len(delivered) != 0 {
			t.Errorf("expected nothing from outside the repository copy, got %v", delivered)
		}
	})

	t.Run("a file root inside the copy is read as usual", func(t *testing.T) {
		want := []string{"app.yaml=kind: ConfigMap"}
		delivered, err := walk(t, filepath.Join("inside", "app.yaml"))
		if err != nil {
			t.Fatalf("Walk() returned error: %v", err)
		}
		if !reflect.DeepEqual(delivered, want) {
			t.Errorf("expected the file to be delivered as %v, got %v", want, delivered)
		}
	})
}

func TestStandInCloneRefusesAnOversizeExactFileRoot(t *testing.T) {
	// A truncated tree sends the walk to a clone standing in for the Trees
	// walk. That clone filters the way the ranking would, but the root is the
	// one file the caller named, so it fails for its size the way the Trees
	// route does rather than reporting an import of nothing.
	const limit = 1000

	baseDir := t.TempDir()
	createCommittedRepo(t, filepath.Join(baseDir, "owner", "sample"), map[string]string{
		"charts/values.yaml": strings.Repeat("y", limit*5),
		"charts/small.yaml":  "kind: ConfigMap",
	})

	walk := func(t *testing.T, root string, standingInForTrees bool) ([]string, error) {
		t.Helper()

		delivered := []string{}
		g := NewGit().
			BaseURL("file://" + baseDir).
			Owner("owner").
			Repo("sample").
			MaxFileSize(limit).
			Root(root).
			RegisterFileInterceptor(func(file File) error {
				delivered = append(delivered, filepath.Base(file.Path))
				return nil
			})
		return delivered, clonewalkContext(context.Background(), g, standingInForTrees)
	}

	t.Run("the exact file root fails", func(t *testing.T) {
		delivered, err := walk(t, "charts/values.yaml", true)
		if err == nil {
			t.Fatal("expected the oversize root to fail rather than deliver nothing")
		}
		if code := meshkiterrors.GetCode(err); code != ErrInvalidSizeFileCode {
			t.Fatalf("expected error code %q, got %q: %v", ErrInvalidSizeFileCode, code, err)
		}
		if len(delivered) != 0 {
			t.Errorf("expected nothing to be delivered, got %v", delivered)
		}
	})

	t.Run("a directory root still skips it", func(t *testing.T) {
		delivered, err := walk(t, "charts/**", true)
		if err != nil {
			t.Fatalf("the clone walk returned error: %v", err)
		}
		if want := []string{"small.yaml"}; !reflect.DeepEqual(delivered, want) {
			t.Errorf("expected the oversized file to be skipped, leaving %v, got %v", want, delivered)
		}
	})
}

func TestStandInCloneRefusesASymlinkedRootItCannotContain(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("committed symlinks are not checked out as links on Windows")
	}

	// A truncated tree sends the walk to a clone standing in for the Trees
	// walk, and a root naming one symlink has to end the same way there as on
	// an ordinary clone: refused when it leaves the copy or resolves to
	// nothing, never silently skipped.
	baseDir := t.TempDir()
	createCommittedRepoWithLinks(t,
		filepath.Join(baseDir, "owner", "sample"),
		map[string]string{"inside/app.yaml": "kind: ConfigMap"},
		map[string]string{
			"escaping":     filepath.Join("..", "..", "meshkit-walker-nothing-here"),
			"unresolvable": filepath.Join("missing", "target"),
		},
	)

	for _, root := range []string{"escaping", "unresolvable"} {
		t.Run(root+" is refused", func(t *testing.T) {
			delivered := []string{}
			g := NewGit().
				BaseURL("file://" + baseDir).
				Owner("owner").
				Repo("sample").
				Root(root).
				RegisterFileInterceptor(func(file File) error {
					delivered = append(delivered, filepath.Base(file.Path))
					return nil
				})

			err := clonewalkContext(context.Background(), g, true)
			if err == nil {
				t.Fatal("expected a root that leaves the repository copy to fail")
			}
			if code := meshkiterrors.GetCode(err); code != ErrRootNotFoundCode {
				t.Fatalf("expected error code %q, got %q: %v", ErrRootNotFoundCode, code, err)
			}
			if len(delivered) != 0 {
				t.Errorf("expected nothing to be delivered, got %v", delivered)
			}
		})
	}
}

func TestStandInCloneRefusesARecursiveSymlinkedRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("committed symlinks are not checked out as links on Windows")
	}

	// filepath.WalkDir hands a recursive walk the link itself and never
	// descends it, so a clone standing in for a Trees walk would filter that
	// one entry away and deliver nothing. It says so instead.
	baseDir := t.TempDir()
	createCommittedRepoWithLinks(t,
		filepath.Join(baseDir, "owner", "sample"),
		map[string]string{"deploy/charts/Chart.yaml": "name: redis"},
		map[string]string{"charts": filepath.Join("deploy", "charts")},
	)

	walk := func(t *testing.T, standingInForTrees bool) ([]string, error) {
		t.Helper()

		delivered := []string{}
		g := NewGit().
			BaseURL("file://" + baseDir).
			Owner("owner").
			Repo("sample").
			Root("charts/**").
			RegisterFileInterceptor(func(file File) error {
				delivered = append(delivered, filepath.Base(file.Path))
				return nil
			})
		return delivered, clonewalkContext(context.Background(), g, standingInForTrees)
	}

	t.Run("the stand-in clone refuses it", func(t *testing.T) {
		delivered, err := walk(t, true)
		if err == nil {
			t.Fatal("expected a recursive symlinked root to fail rather than deliver nothing")
		}
		if code := meshkiterrors.GetCode(err); code != ErrSymlinkedRootCode {
			t.Fatalf("expected error code %q, got %q: %v", ErrSymlinkedRootCode, code, err)
		}
		if len(delivered) != 0 {
			t.Errorf("expected nothing to be delivered, got %v", delivered)
		}
	})

	t.Run("an ordinary clone is left as it has always behaved", func(t *testing.T) {
		// Pre-existing: WalkDir hands it the link, readFile opens what
		// resolves to a directory, and the read failure surfaces as a clone
		// error. This change does not touch that.
		delivered, err := walk(t, false)
		if err == nil {
			t.Fatal("expected the ordinary clone to keep failing on a directory read")
		}
		if code := meshkiterrors.GetCode(err); code != ErrCloningRepoCode {
			t.Fatalf("expected error code %q, got %q: %v", ErrCloningRepoCode, code, err)
		}
		if len(delivered) != 0 {
			t.Errorf("expected nothing to be delivered, got %v", delivered)
		}
	})
}
