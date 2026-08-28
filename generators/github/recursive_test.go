package github

import (
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
	tempDir, err := os.MkdirTemp("", "test-recursive-walk")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	r, err := git.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}
	w, err := r.Worktree()
	if err != nil {
		t.Fatalf("Failed to get worktree: %v", err)
	}

	createFile(t, tempDir, "root.yaml", "content")
	createFile(t, tempDir, "root.json", "content")
	createFile(t, tempDir, "level1/level1.yaml", "content")
	createFile(t, tempDir, "level1/level1.txt", "content")
	createFile(t, tempDir, "level1/level2/level2.yaml", "content")

	_, err = w.Add(".")
	if err != nil {
		t.Fatalf("Failed to add files: %v", err)
	}
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

	tests := []struct {
		name       string
		recursive  bool
		maxDepth   int
		extensions []string
		wantFiles  []string
	}{
		{
			name:       "Non-recursive (Root only, all types)",
			recursive:  false,
			maxDepth:   0,
			extensions: nil,
			wantFiles:  []string{"root.yaml", "root.json"},
		},
		{
			name:       "Recursive Unlimited (All types)",
			recursive:  true,
			maxDepth:   0,
			extensions: nil,
			wantFiles:  []string{"root.yaml", "root.json", "level1.yaml", "level1.txt", "level2.yaml"},
		},
		{
			name:       "Recursive MaxDepth 1 (All types)",
			recursive:  true,
			maxDepth:   1,
			extensions: nil,
			wantFiles:  []string{"root.yaml", "root.json", "level1.yaml", "level1.txt"},
		},
		{
			name:       "Recursive MaxDepth 2 (All types)",
			recursive:  true,
			maxDepth:   2,
			extensions: nil,
			wantFiles:  []string{"root.yaml", "root.json", "level1.yaml", "level1.txt", "level2.yaml"},
		},
		{
			name:       "Recursive Filtered (.yaml only)",
			recursive:  true,
			maxDepth:   0,
			extensions: []string{".yaml"},
			wantFiles:  []string{"root.yaml", "level1.yaml", "level2.yaml"},
		},
		{
			name:       "Recursive Filtered (.yaml, .json)",
			recursive:  true,
			maxDepth:   0,
			extensions: []string{".yaml", ".json"},
			wantFiles:  []string{"root.yaml", "root.json", "level1.yaml", "level2.yaml"},
		},
		{
			name:       "Recursive MaxDepth 1 Filtered (.yaml)",
			recursive:  true,
			maxDepth:   1,
			extensions: []string{".yaml"},
			wantFiles:  []string{"root.yaml", "level1.yaml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
				MaxDepth(tt.maxDepth).
				AllowedExtensions(tt.extensions)

			var collectedFiles []string
			walkerInst.RegisterFileInterceptor(func(f walker.File) error {
				collectedFiles = append(collectedFiles, f.Name)
				return nil
			})

			if tt.recursive {
				walkerInst.Root("/**")
			} else {
				walkerInst.Root("")
			}

			err := walkerInst.Walk()
			if err != nil {
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
	failed := false
	for _, f := range want {
		if _, ok := gotMap[f]; !ok {
			t.Errorf("missing expected file: %s", f)
			failed = true
		} else {
			delete(gotMap, f)
		}
	}
	if len(gotMap) > 0 {
		for f := range gotMap {
			t.Errorf("unexpected file collected: %s", f)
			failed = true
		}
	}
	if failed {
		t.Errorf("Got: %v", got)
		t.Errorf("Want: %v", want)
	}
}

func createFile(t *testing.T, base, path, content string) {
	fullPath := filepath.Join(base, path)
	err := os.MkdirAll(filepath.Dir(fullPath), 0755)
	if err != nil {
		t.Fatalf("Failed to create dirs: %v", err)
	}
	err = os.WriteFile(fullPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
}
