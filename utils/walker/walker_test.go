package walker

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
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

	if err := g.readFile(info, path); err != nil {
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

	err = NewGit().MaxFileSize(2).readFile(info, path)
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
