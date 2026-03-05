package gh_test

import (
	"fmt"

	"github.com/DevExpGBB/gh-devlake/internal/gh"
)

// ExampleAddAssigneesByLogin demonstrates how to add assignees to an issue or PR
// using bot login names instead of node IDs.
func ExampleAddAssigneesByLogin() {
	// The node_id of the issue or pull request (e.g., from GitHub GraphQL API)
	assignableID := "MDU6SXNzdWUxMjM0NTY3ODk="

	// Bot login names to assign - this is what we're testing!
	// Previously, you would need to get the node_id for each user/bot.
	// Now you can use login names directly.
	botLogins := []string{
		"github-actions[bot]",
		"Claude",
		"octocat",
	}

	// Add the assignees
	response, err := gh.AddAssigneesByLogin(assignableID, botLogins)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Print the assigned users
	fmt.Println("Successfully assigned:")
	for _, assignee := range response.Data.AddAssigneesToAssignable.Assignable.Assignees.Nodes {
		fmt.Printf("  - %s\n", assignee.Login)
	}
}

// ExampleAddAssigneesByLogin_singleBot demonstrates assigning a single bot
func ExampleAddAssigneesByLogin_singleBot() {
	assignableID := "MDU6SXNzdWUxMjM0NTY3ODk="
	botLogin := []string{"Claude"}

	response, err := gh.AddAssigneesByLogin(assignableID, botLogin)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if len(response.Data.AddAssigneesToAssignable.Assignable.Assignees.Nodes) > 0 {
		fmt.Printf("Bot assigned: %s\n",
			response.Data.AddAssigneesToAssignable.Assignable.Assignees.Nodes[0].Login)
	}
}
