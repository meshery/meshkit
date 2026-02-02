package github

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/meshery/meshkit/utils/walker"
)

func TestRecursiveWalkFunctional(t *testing.T) {
	// 1. Create a temporary local git repo
	tempDir, err := ioutil.TempDir("", "test-recursive-walk")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Initialize git repo
	r, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}
	w, err := r.Worktree()
	if err != nil {
		t.Fatalf("Failed to get worktree: %v", err)
	}

	// 2. Commit a file structure
	// root/
	//   root.yaml
	//   level1/
	//     level1.yaml
	//     level2/
	//       level2.yaml

	createFile(t, tempDir, "root.yaml", "content")
	createFile(t, tempDir, "level1/level1.yaml", "content")
	createFile(t, tempDir, "level1/level2/level2.yaml", "content")

	_, err = w.Add(".")
	if err != nil {
		t.Fatalf("Failed to add files: %v", err)
	}
	// Use a fixed time for commit to be deterministic if needed, but not strictly required here.
	_, err = w.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// 3. Test Cases
	tests := []struct {
		name      string
		recursive bool
		maxDepth  int
		wantFiles []string
	}{
		{
			name:      "Non-recursive (Root only)",
			recursive: false,
			maxDepth:  0,
			wantFiles: []string{"root.yaml"},
		},
		{
			name:      "Recursive Unlimited",
			recursive: true,
			maxDepth:  0,
			wantFiles: []string{"root.yaml", "level1.yaml", "level2.yaml"},
		},
		{
			name:      "Recursive MaxDepth 1",
			recursive: true,
			maxDepth:  1,
			wantFiles: []string{"root.yaml", "level1.yaml"},
		},
		{
			name:      "Recursive MaxDepth 2",
			recursive: true,
			maxDepth:  2,
			wantFiles: []string{"root.yaml", "level1.yaml", "level2.yaml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Construct file URL for local repo
			// Convert tempDir to file URI (e.g., file:///C:/...)
			// ToSlash is important for Windows
			repoPath := filepath.ToSlash(tempDir)
			if !strings.HasPrefix(repoPath, "/") {
				repoPath = "/" + repoPath
			}
			fileURL := "file://" + repoPath

			walkerInst := walker.NewGit().
				Owner("").
				Repo("").
				BaseURL(fileURL).
				Branch("master").
				Root("").
				MaxDepth(tt.maxDepth)

			var collectedFiles []string
			walkerInst.RegisterFileInterceptor(func(f walker.File) error {
				collectedFiles = append(collectedFiles, f.Name)
				return nil
			})

			// Adjust root to trigger recursion internal flag
			if tt.recursive {
				walkerInst.Root("/**")
			} else {
				walkerInst.Root("")
			}

			// Run Walk
			err := walkerInst.Walk()
			if err != nil {
				// We expect this to work for local repos. Fail if it doesn't.
				t.Fatalf("Walk failed: %v", err)
			}

			assertFilesEqual(t, collectedFiles, tt.wantFiles)
		})
	}
}

func assertFilesEqual(t *testing.T, got, want []string) {
	gotMap := make(map[string]struct{})
	for _, f := range got {
		gotMap[f] = struct{}{}
	}
	for _, f := range want {
		if _, ok := gotMap[f]; !ok {
			t.Errorf("missing expected file: %s", f)
		} else {
			delete(gotMap, f)
		}
	}
	if len(gotMap) > 0 {
		for f := range gotMap {
			t.Errorf("unexpected file collected: %s", f)
		}
	}
	if len(got) != len(want) {
		t.Errorf("got %d files, want %d", len(got), len(want))
	}
}

func createFile(t *testing.T, base, path, content string) {
	fullPath := filepath.Join(base, path)
	err := os.MkdirAll(filepath.Dir(fullPath), 0755)
	if err != nil {
		t.Fatalf("Failed to create dirs: %v", err)
	}
	err = ioutil.WriteFile(fullPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
}
