package github

import (
	"testing"

	"github.com/jarcoal/httpmock"
)

// use httpmock to mock github api responses
func TestGetGithubRepoBranch(t *testing.T) {
	tests := []struct {
		name       string
		owner      string
		repo       string
		mockStatus int
		mockBody   string
		wantBranch string
		wantErr    bool
	}{
		{
			name:       "valid repo",
			owner:      "octocat",
			repo:       "Hello-World",
			mockStatus: 200,
			mockBody:   `{"default_branch":"main"}`,
			wantBranch: "main",
			wantErr:    false,
		},
		{
			name:       "valid repo with master branch",
			owner:      "octocat",
			repo:       "Hello-Mesh",
			mockStatus: 200,
			mockBody:   `{"default_branch":"master"}`,
			wantBranch: "master",
			wantErr:    false,
		},
		{
			name:       "repo not found",
			owner:      "octocat",
			repo:       "NonExistentRepo",
			mockStatus: 404,
			mockBody:   `{"message":"Not Found"}`,
			wantBranch: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// setup httpmock
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()

			url := "https://api.github.com/repos/" + tt.owner + "/" + tt.repo
			httpmock.RegisterResponder("GET", url,
				httpmock.NewStringResponder(tt.mockStatus, tt.mockBody))

			branch, err := GetDefaultBranchFromGitHub(tt.owner, tt.repo, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetDefaultBranchFromGitHub() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if branch != tt.wantBranch {
				t.Errorf("GetDefaultBranchFromGitHub() = %v, want %v", branch, tt.wantBranch)
			}
		})
	}
}
