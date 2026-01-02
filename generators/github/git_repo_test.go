package github

import (
	"net/url"
	"testing"
)

func TestExtractRepoDetailsFromSourceURL(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantOwner  string
		wantRepo   string
		wantBranch string
		wantRoot   string
		wantErr    bool
	}{
		{
			name:      "owner and repo only",
			input:     "git://github.com/owner/repo",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantRoot:  "/**",
			wantErr:   false,
		},
		{
			name:       "owner repo and branch",
			input:      "git://github.com/owner/repo/mybranch",
			wantOwner:  "owner",
			wantRepo:   "repo",
			wantBranch: "mybranch",
			wantRoot:   "/**",
			wantErr:    false,
		},
		{
			name:       "owner repo branch and path",
			input:      "git://github.com/owner/repo/mybranch/path/to/crds",
			wantOwner:  "owner",
			wantRepo:   "repo",
			wantBranch: "mybranch",
			wantRoot:   "path/to/crds",
			wantErr:    false,
		},
		{
			name:    "invalid single component",
			input:   "git://github.com/owner",
			wantErr: true,
		},
		{
			name:      "trailing slash on repo",
			input:     "git://github.com/owner/repo/",
			wantOwner: "owner",
			wantRepo:  "repo",
			// trailing slash leads to a third empty element; branch becomes "" and root "/**"
			wantBranch: "",
			wantRoot:   "/**",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.input)
			if err != nil {
				t.Fatalf("failed to parse url %s: %v", tt.input, err)
			}
			gr := GitRepo{URL: u}
			owner, repo, branch, root, err := gr.extractRepoDetailsFromSourceURL()
			if (err != nil) != tt.wantErr {
				t.Fatalf("unexpected error state: got err=%v, wantErr=%v", err, tt.wantErr)
			}
			if err != nil {
				// on error we are done
				return
			}
			if owner != tt.wantOwner {
				t.Fatalf("owner: got %q want %q", owner, tt.wantOwner)
			}
			if repo != tt.wantRepo {
				t.Fatalf("repo: got %q want %q", repo, tt.wantRepo)
			}
			if branch != tt.wantBranch {
				t.Fatalf("branch: got %q want %q", branch, tt.wantBranch)
			}
			if root != tt.wantRoot {
				t.Fatalf("root: got %q want %q", root, tt.wantRoot)
			}
		})
	}
}
