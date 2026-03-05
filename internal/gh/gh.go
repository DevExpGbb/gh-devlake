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
	return parseListOutput(out), nil
}

// parseListOutput parses newline-separated repo names from gh repo list output.
func parseListOutput(data []byte) []string {
	var repos []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			repos = append(repos, line)
		}
	}
	return repos
}

// GetRepoDetails fetches details for a single repo via gh api.
func GetRepoDetails(fullName string) (*RepoDetails, error) {
	out, err := exec.Command("gh", "api", fmt.Sprintf("repos/%s", fullName),
		"--jq", `{id: .id, name: .name, full_name: .full_name, html_url: .html_url, clone_url: .clone_url}`,
	).Output()
	if err != nil {
		return nil, fmt.Errorf("gh api repos/%s failed: %w", fullName, err)
	}
	details, err := parseRepoDetails(out)
	if err != nil {
		return nil, fmt.Errorf("failed to parse repo details for %s: %w", fullName, err)
	}
	return details, nil
}

// parseRepoDetails unmarshals JSON output from gh api into a RepoDetails struct.
func parseRepoDetails(data []byte) (*RepoDetails, error) {
	var details RepoDetails
	if err := json.Unmarshal(data, &details); err != nil {
		return nil, err
	}
	return &details, nil
}

// AddAssigneesResponse holds the response from the addAssigneesToAssignable mutation.
type AddAssigneesResponse struct {
	Data struct {
		AddAssigneesToAssignable struct {
			Assignable struct {
				Assignees struct {
					Nodes []struct {
						Login string `json:"login"`
					} `json:"nodes"`
				} `json:"assignees"`
			} `json:"assignable"`
		} `json:"addAssigneesToAssignable"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// AddAssigneesByLogin adds assignees to an issue or pull request using their login names.
// This function tests if the GraphQL addAssigneesToAssignable mutation works with bot login names
// instead of node_id. The assignableID should be the node_id of the issue or pull request.
func AddAssigneesByLogin(assignableID string, assigneeLogins []string) (*AddAssigneesResponse, error) {
	if len(assigneeLogins) == 0 {
		return nil, fmt.Errorf("assigneeLogins cannot be empty")
	}

	// Build the GraphQL mutation with login names instead of node IDs
	mutation := `mutation($assignableId: ID!, $assigneeIds: [ID!]!) {
		addAssigneesToAssignable(input: {assignableId: $assignableId, assigneeIds: $assigneeIds}) {
			assignable {
				assignees(first: 10) {
					nodes {
						login
					}
				}
			}
		}
	}`

	// Create variables object
	variables := map[string]interface{}{
		"assignableId": assignableID,
		"assigneeIds":  assigneeLogins,
	}

	// Build the full GraphQL request
	request := map[string]interface{}{
		"query":     mutation,
		"variables": variables,
	}

	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal GraphQL request: %w", err)
	}

	// Execute the GraphQL mutation via gh api graphql
	out, err := exec.Command("gh", "api", "graphql", "-f", fmt.Sprintf("query=%s", string(requestJSON))).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("gh api graphql failed: %w (stderr: %s)", err, string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("gh api graphql failed: %w", err)
	}

	// Parse the response
	var response AddAssigneesResponse
	if err := json.Unmarshal(out, &response); err != nil {
		return nil, fmt.Errorf("failed to parse GraphQL response: %w", err)
	}

	// Check for GraphQL errors
	if len(response.Errors) > 0 {
		return &response, fmt.Errorf("GraphQL error: %s", response.Errors[0].Message)
	}

	return &response, nil
}
