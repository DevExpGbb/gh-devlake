// Package gh wraps the GitHub CLI (gh) for repository operations.
package gh

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// RepoDetails holds information about a GitHub repository.
type RepoDetails struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	HTMLURL  string `json:"html_url"`
	CloneURL string `json:"clone_url"`
}

// IsAvailable checks whether the gh CLI is installed and accessible.
func IsAvailable() bool {
	_, err := exec.LookPath("gh")
	return err == nil
}

// ListRepos returns up to limit repos for an org using gh CLI.
func ListRepos(org string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 30
	}
	out, err := exec.Command("gh", "repo", "list", org,
		"--limit", fmt.Sprintf("%d", limit),
		"--json", "nameWithOwner",
		"--jq", ".[].nameWithOwner",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("gh repo list failed: %w", err)
	}

	var repos []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			repos = append(repos, line)
		}
	}
	return repos, nil
}

// GetRepoDetails fetches details for a single repo via gh api.
func GetRepoDetails(fullName string) (*RepoDetails, error) {
	out, err := exec.Command("gh", "api", fmt.Sprintf("repos/%s", fullName),
		"--jq", `{id: .id, name: .name, full_name: .full_name, html_url: .html_url, clone_url: .clone_url}`,
	).Output()
	if err != nil {
		return nil, fmt.Errorf("gh api repos/%s failed: %w", fullName, err)
	}

	var details RepoDetails
	if err := json.Unmarshal(out, &details); err != nil {
		return nil, fmt.Errorf("failed to parse repo details for %s: %w", fullName, err)
	}
	return &details, nil
}
