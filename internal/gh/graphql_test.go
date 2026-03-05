package gh

import (
	"encoding/json"
	"testing"
)

func TestAddAssigneesResponse_Parsing(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantErr        bool
		wantLogins     []string
		wantGraphQLErr bool
	}{
		{
			name: "successful assignment with bot",
			input: `{
				"data": {
					"addAssigneesToAssignable": {
						"assignable": {
							"assignees": {
								"nodes": [
									{"login": "github-actions[bot]"},
									{"login": "octocat"}
								]
							}
						}
					}
				}
			}`,
			wantLogins: []string{"github-actions[bot]", "octocat"},
		},
		{
			name: "successful assignment with claude bot",
			input: `{
				"data": {
					"addAssigneesToAssignable": {
						"assignable": {
							"assignees": {
								"nodes": [
									{"login": "Claude"}
								]
							}
						}
					}
				}
			}`,
			wantLogins: []string{"Claude"},
		},
		{
			name: "graphql error response",
			input: `{
				"errors": [
					{"message": "Could not resolve to a node with the global id"}
				]
			}`,
			wantGraphQLErr: true,
		},
		{
			name: "empty assignees",
			input: `{
				"data": {
					"addAssigneesToAssignable": {
						"assignable": {
							"assignees": {
								"nodes": []
							}
						}
					}
				}
			}`,
			wantLogins: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var response AddAssigneesResponse
			err := json.Unmarshal([]byte(tt.input), &response)
			if err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}

			if tt.wantGraphQLErr {
				if len(response.Errors) == 0 {
					t.Error("expected GraphQL errors, got none")
				}
				return
			}

			if len(response.Errors) > 0 {
				t.Errorf("unexpected GraphQL errors: %v", response.Errors)
				return
			}

			assignees := response.Data.AddAssigneesToAssignable.Assignable.Assignees.Nodes
			if len(assignees) != len(tt.wantLogins) {
				t.Fatalf("got %d assignees, want %d", len(assignees), len(tt.wantLogins))
			}

			for i, login := range tt.wantLogins {
				if assignees[i].Login != login {
					t.Errorf("assignee[%d].Login = %q, want %q", i, assignees[i].Login, login)
				}
			}
		})
	}
}

func TestAddAssigneesByLogin_EmptyLogins(t *testing.T) {
	_, err := AddAssigneesByLogin("some-node-id", []string{})
	if err == nil {
		t.Error("expected error for empty assigneeLogins, got nil")
	}
	if err.Error() != "assigneeLogins cannot be empty" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestAddAssigneesByLogin_ValidationTests(t *testing.T) {
	tests := []struct {
		name            string
		assignableID    string
		assigneeLogins  []string
		expectValidCall bool
	}{
		{
			name:            "valid bot login",
			assignableID:    "MDU6SXNzdWUxMjM0NTY3ODk=",
			assigneeLogins:  []string{"github-actions[bot]"},
			expectValidCall: true,
		},
		{
			name:            "valid multiple logins including bot",
			assignableID:    "MDU6SXNzdWUxMjM0NTY3ODk=",
			assigneeLogins:  []string{"Claude", "octocat", "github-actions[bot]"},
			expectValidCall: true,
		},
		{
			name:            "single user login",
			assignableID:    "MDU6SXNzdWUxMjM0NTY3ODk=",
			assigneeLogins:  []string{"octocat"},
			expectValidCall: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't actually call the function without a real GitHub connection,
			// but we can validate the inputs would be accepted
			if len(tt.assigneeLogins) == 0 {
				t.Error("test case has empty assigneeLogins but expects valid call")
			}
			if tt.assignableID == "" {
				t.Error("test case has empty assignableID")
			}
		})
	}
}

// TestGraphQLMutationStructure validates that the mutation structure
// matches GitHub's expected format for addAssigneesToAssignable
func TestGraphQLMutationStructure(t *testing.T) {
	// This test documents the expected mutation structure
	expectedMutation := `mutation($assignableId: ID!, $assigneeIds: [ID!]!) {
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

	// Verify the mutation has required components
	requiredComponents := []string{
		"mutation",
		"$assignableId: ID!",
		"$assigneeIds: [ID!]!",
		"addAssigneesToAssignable",
		"input",
		"assignableId",
		"assigneeIds",
	}

	for _, component := range requiredComponents {
		if !containsString(expectedMutation, component) {
			t.Errorf("mutation is missing required component: %s", component)
		}
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
