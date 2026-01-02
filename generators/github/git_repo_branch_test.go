package github

import (
	"testing"
)

func TestGetGithubRepoBranch(t *testing.T) {
	tests := []struct {
		name       string
		repo       string
		owner      string
		wantBranch string
		wantErr    bool
	}{
		{
			name:       "Repo with main branch",
			repo:       "gonotify",
			owner:      "muhammadolammi",
			wantBranch: "main",
			wantErr:    false,
		},
		{
			name:       "Repo with master branch",
			repo:       "meshkit",
			owner:      "muhammadolammi",
			wantBranch: "master",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			branch, err := GetDefaultBranchFromGitHub(tt.owner, tt.repo)

			if (err != nil) != tt.wantErr {
				t.Fatalf("unexpected error state: got err=%v, wantErr=%v", err, tt.wantErr)
			}

			if branch != tt.wantBranch {
				t.Fatalf("branch: got %q want %q", branch, tt.wantBranch)
			}
		})
	}
}
