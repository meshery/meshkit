package github

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func GetDefaultBranchFromGitHub(owner, repo string, client *http.Client) (string, error) {
	if client == nil {
		client = http.DefaultClient
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)
	resp, err := client.Get(url)
	if err != nil {
		return "", ErrGetDefaultBranch(err, owner, repo)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", ErrGetDefaultBranch(fmt.Errorf("github api: %d", resp.StatusCode), owner, repo)
	}
	var out struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", ErrGetDefaultBranch(err, owner, repo)
	}
	return out.DefaultBranch, nil

}
