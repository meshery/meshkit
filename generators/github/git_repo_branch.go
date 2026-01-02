package github

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func GetDefaultBranchFromGitHub(owner, repo string) (string, error) {
	return getDefaultBranchFromGitHub(owner, repo)
}

func getDefaultBranchFromGitHub(owner, repo string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("github api: %d", resp.StatusCode)
	}
	var out struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.DefaultBranch, nil
}
