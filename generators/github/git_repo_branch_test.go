package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetGithubRepoBranch(t *testing.T) {
	tests := []struct {
		name       string
		owner      string
		repo       string
		mockStatus int
		mockBody   any // Using any allows us to pass structs or raw strings
		wantBranch string
		wantErr    bool
	}{
		{
			name:       "valid repo",
			owner:      "octocat",
			repo:       "Hello-World",
			mockStatus: http.StatusOK,
			mockBody:   map[string]string{"default_branch": "main"},
			wantBranch: "main",
			wantErr:    false,
		},
		{
			name:       "valid repo with master branch",
			owner:      "octocat",
			repo:       "Hello-Mesh",
			mockStatus: http.StatusOK,
			mockBody:   map[string]string{"default_branch": "master"},
			wantBranch: "master",
			wantErr:    false,
		},
		{
			name:       "repo not found",
			owner:      "octocat",
			repo:       "NonExistentRepo",
			mockStatus: http.StatusNotFound,
			mockBody:   map[string]string{"message": "Not Found"},
			wantBranch: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := fmt.Sprintf("/repos/%s/%s", tt.owner, tt.repo)
				if r.URL.Path != expectedPath {
					t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
				}

				w.WriteHeader(tt.mockStatus)
				json.NewEncoder(w).Encode(tt.mockBody)
			}))
			defer server.Close()

			branch, err := GetDefaultBranchFromGitHub(server.URL, tt.owner, tt.repo, server.Client())

			if (err != nil) != tt.wantErr {
				t.Fatalf("GetDefaultBranchFromGitHub() unexpected error state: %v", err)
			}
			if branch != tt.wantBranch {
				t.Errorf("got branch %q, want %q", branch, tt.wantBranch)
			}
		})
	}
}
