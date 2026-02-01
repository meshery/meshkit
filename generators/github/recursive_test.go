package github

import (
	"net/url"
	"testing"
)

func TestRecursiveWalk(t *testing.T) {
	gr := GitRepo{
		URL:         &url.URL{Path: "/owner/repo/branch/root"},
		Recursive:   true,
		MaxDepth:    2,
		PackageName: "test-package",
	}

	if !gr.Recursive {
		t.Error("GitRepo.Recursive should be true")
	}
	if gr.MaxDepth != 2 {
		t.Errorf("GitRepo.MaxDepth should be 2, got %d", gr.MaxDepth)
	}

	ghpm := GitHubPackageManager{
		PackageName: "test",
		SourceURL:   "https://github.com/owner/repo",
		Recursive:   true,
		MaxDepth:    3,
	}

	if !ghpm.Recursive {
		t.Error("GitHubPackageManager.Recursive should be true")
	}
	if ghpm.MaxDepth != 3 {
		t.Errorf("GitHubPackageManager.MaxDepth should be 3, got %d", ghpm.MaxDepth)
	}
}
