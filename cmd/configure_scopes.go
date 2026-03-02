package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/DevExpGBB/gh-devlake/internal/devlake"
	"github.com/DevExpGBB/gh-devlake/internal/gh"
	"github.com/DevExpGBB/gh-devlake/internal/prompt"
	"github.com/DevExpGBB/gh-devlake/internal/repofile"
	"github.com/spf13/cobra"
)

// ScopeOpts holds all options for scope and project commands, replacing
// the former package-level scope* variables.
type ScopeOpts struct {
	Org           string
	Enterprise    string
	Plugin        string
	Repos         string
	ReposFile     string
	ConnectionID  int
	ProjectName   string
	DeployPattern string
	ProdPattern   string
	IncidentLabel string
	TimeAfter     string
	Cron          string
	SkipSync      bool
	Wait          bool
	Timeout       string
}

func newConfigureScopesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "scope",
		Aliases: []string{"scopes"},
		Short:   "Manage scopes on DevLake connections",
		Long: `Manage scopes (repos, orgs) on existing DevLake connections.

Use subcommands to add, list, or delete scopes.`,
	}

	cmd.AddCommand(newScopeAddCmd(), newScopeListCmd(), newScopeDeleteCmd())

	return cmd
}

// scopeGitHubResult holds the outputs from scoping a GitHub connection.
type scopeGitHubResult struct {
	Connection  devlake.BlueprintConnection
	Repos       []string
	RepoDetails []*gh.RepoDetails
}

// scopeGitHub resolves repos, creates scope config, and PUTs repo scopes
// for a GitHub connection. Returns the BlueprintConnection entry and repo list.
func scopeGitHub(client *devlake.Client, connID int, org string, opts *ScopeOpts) (*scopeGitHubResult, error) {
	fmt.Println("\n\U0001f4e6 Resolving repositories...")
	repos, err := resolveRepos(org, opts)
	if err != nil {
		return nil, err
	}
	if len(repos) == 0 {
		return nil, fmt.Errorf("at least one repository is required")
	}
	fmt.Printf("   Repos to configure: %s\n", strings.Join(repos, ", "))

	fmt.Println("\n\U0001f50e Looking up repo details...")
	var repoDetails []*gh.RepoDetails
	for _, repo := range repos {
		detail, err := gh.GetRepoDetails(repo)
		if err != nil {
			fmt.Printf("   \u26a0\ufe0f  Could not fetch details for %q: %v\n", repo, err)
			continue
		}
		repoDetails = append(repoDetails, detail)
		fmt.Printf("   %s (ID: %d)\n", detail.FullName, detail.ID)
	}
	if len(repoDetails) == 0 {
		return nil, fmt.Errorf("could not resolve any repository details \u2014 verify repos exist and gh CLI is authenticated")
	}

	fmt.Println("\n\u2699\ufe0f  Creating DORA scope config...")
	scopeConfigID, err := ensureScopeConfig(client, "github", connID, opts)
	if err != nil {
		fmt.Printf("   \u26a0\ufe0f  Could not create scope config: %v\n", err)
	} else {
		fmt.Printf("   Scope config ID: %d\n", scopeConfigID)
	}

	fmt.Println("\n\U0001f4dd Adding repository scopes...")
	err = putGitHubScopes(client, connID, scopeConfigID, repoDetails)
	if err != nil {
		return nil, fmt.Errorf("failed to add repo scopes: %w", err)
	}
	fmt.Printf("   \u2705 Added %d repo scope(s)\n", len(repoDetails))

	var ghScopes []devlake.BlueprintScope
	for _, d := range repoDetails {
		ghScopes = append(ghScopes, devlake.BlueprintScope{
			ScopeID:   strconv.Itoa(d.ID),
			ScopeName: d.FullName,
		})
	}

	return &scopeGitHubResult{
		Connection: devlake.BlueprintConnection{
			PluginName:   "github",
			ConnectionID: connID,
			Scopes:       ghScopes,
		},
		Repos:       repos,
		RepoDetails: repoDetails,
	}, nil
}

// scopeCopilot PUTs the org/enterprise scope for a Copilot connection.
func scopeCopilot(client *devlake.Client, connID int, org, enterprise string) (*devlake.BlueprintConnection, error) {
	fmt.Println("\n\U0001f4dd Adding Copilot scope...")
	scopeID := copilotScopeID(org, enterprise)
	err := putCopilotScope(client, connID, org, enterprise)
	if err != nil {
		return nil, fmt.Errorf("could not add Copilot scope: %w", err)
	}
	fmt.Printf("   \u2705 Copilot scope added: %s\n", scopeID)

	return &devlake.BlueprintConnection{
		PluginName:   "gh-copilot",
		ConnectionID: connID,
		Scopes: []devlake.BlueprintScope{
			{ScopeID: scopeID, ScopeName: scopeID},
		},
	}, nil
}

// resolveConnectionID finds a connection ID from flag, state, or API.
func resolveConnectionID(client *devlake.Client, state *devlake.State, plugin string, flagValue int) (int, error) {
	if flagValue > 0 {
		return flagValue, nil
	}
	if state != nil {
		for _, c := range state.Connections {
			if c.Plugin == plugin {
				return c.ConnectionID, nil
			}
		}
	}
	conns, err := client.ListConnections(plugin)
	if err != nil {
		return 0, fmt.Errorf("could not list %s connections: %w", plugin, err)
	}
	if len(conns) == 1 {
		return conns[0].ID, nil
	}
	if len(conns) > 1 {
		fmt.Printf("   Multiple %s connections found:\n", plugin)
		for _, c := range conns {
			fmt.Printf("     ID=%d  Name=%q\n", c.ID, c.Name)
		}
		input := prompt.ReadLine(fmt.Sprintf("   Enter the %s connection ID to use", plugin))
		id, err := strconv.Atoi(input)
		if err != nil {
			return 0, fmt.Errorf("invalid connection ID: %s", input)
		}
		return id, nil
	}
	return 0, fmt.Errorf("no %s connections found \u2014 run 'configure connection' first", plugin)
}

// resolveOrg determines the organization from flag, state, or prompt.
func resolveOrg(state *devlake.State, flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if state != nil {
		for _, c := range state.Connections {
			if c.Organization != "" {
				return c.Organization
			}
		}
	}
	return prompt.ReadLine("Enter organization slug")
}

// resolveEnterprise determines the enterprise slug from flag, state, or API.
func resolveEnterprise(state *devlake.State, flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if state != nil {
		for _, c := range state.Connections {
			if c.Enterprise != "" {
				return c.Enterprise
			}
		}
	}
	return ""
}

// repoListLimit is the maximum number of repos fetched from the gh CLI for
// the interactive picker. Increase this to reduce "repo not found" surprises
// in large orgs.
const repoListLimit = 100

// resolveRepos determines repos from flags, file, or interactive gh CLI selection.
func resolveRepos(org string, opts *ScopeOpts) ([]string, error) {
	if opts.Repos != "" {
		var repos []string
		for _, r := range strings.Split(opts.Repos, ",") {
			r = strings.TrimSpace(r)
			if r != "" {
				repos = append(repos, r)
			}
		}
		return repos, nil
	}
	if opts.ReposFile != "" {
		repos, err := repofile.Parse(opts.ReposFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read repos file: %w", err)
		}
		fmt.Printf("   Loaded %d repo(s) from file\n", len(repos))
		return repos, nil
	}
	if gh.IsAvailable() {
		fmt.Printf("   Listing repos in %q via gh CLI...\n", org)
		available, err := gh.ListRepos(org, repoListLimit)
		if err != nil {
			fmt.Printf("   \u26a0\ufe0f  Could not list repos: %v\n", err)
		}
		if err != nil || len(available) == 0 {
			fmt.Printf("   No repos found in %q \u2014 enter repos manually\n", org)
		} else {
			const manualOpt = "Enter repos manually instead"
			fmt.Println()
			selected := prompt.SelectMulti(
				fmt.Sprintf("Available repos in %s (showing up to %d)", org, repoListLimit),
				append(available, manualOpt),
			)
			var picked []string
			for _, s := range selected {
				if s != manualOpt {
					picked = append(picked, s)
				}
			}
			if len(picked) > 0 {
				return picked, nil
			}
		}
	}
	fmt.Println()
	input := prompt.ReadLine("Enter repos (comma-separated, e.g. org/repo1,org/repo2)")
	var repos []string
	for _, r := range strings.Split(input, ",") {
		r = strings.TrimSpace(r)
		if r != "" {
			repos = append(repos, r)
		}
	}
	return repos, nil
}

// ensureScopeConfig creates a DORA scope config or returns an existing one.
func ensureScopeConfig(client *devlake.Client, plugin string, connID int, opts *ScopeOpts) (int, error) {
	cfg := &devlake.ScopeConfig{
		Name:              "dora-config",
		ConnectionID:      connID,
		DeploymentPattern: opts.DeployPattern,
		ProductionPattern: opts.ProdPattern,
		IssueTypeIncident: opts.IncidentLabel,
		Refdiff: &devlake.RefdiffConfig{
			TagsPattern: ".*",
			TagsLimit:   10,
			TagsOrder:   "reverse semver",
		},
	}
	result, err := client.CreateScopeConfig(plugin, connID, cfg)
	if err == nil {
		return result.ID, nil
	}
	existing, listErr := client.ListScopeConfigs(plugin, connID)
	if listErr != nil {
		return 0, fmt.Errorf("create failed: %w; list failed: %v", err, listErr)
	}
	if len(existing) > 0 {
		return existing[0].ID, nil
	}
	return 0, err
}

// putGitHubScopes adds repo scopes to the GitHub connection.
func putGitHubScopes(client *devlake.Client, connID, scopeConfigID int, details []*gh.RepoDetails) error {
	var data []any
	for _, d := range details {
		entry := devlake.GitHubRepoScope{
			GithubID:     d.ID,
			ConnectionID: connID,
			Name:         d.Name,
			FullName:     d.FullName,
			HTMLURL:      d.HTMLURL,
			CloneURL:     d.CloneURL,
		}
		if scopeConfigID > 0 {
			entry.ScopeConfigID = scopeConfigID
		}
		data = append(data, entry)
	}
	return client.PutScopes("github", connID, &devlake.ScopeBatchRequest{Data: data})
}

// putCopilotScope adds the organization/enterprise scope to the Copilot connection.
func putCopilotScope(client *devlake.Client, connID int, org, enterprise string) error {
	scopeID := copilotScopeID(org, enterprise)
	data := []any{
		devlake.CopilotScope{
			ID:           scopeID,
			ConnectionID: connID,
			Organization: org,
			Enterprise:   enterprise,
			Name:         scopeID,
			FullName:     scopeID,
		},
	}
	return client.PutScopes("gh-copilot", connID, &devlake.ScopeBatchRequest{Data: data})
}

// copilotScopeID computes the scope ID matching the plugin's convention.
func copilotScopeID(org, enterprise string) string {
	org = strings.TrimSpace(org)
	enterprise = strings.TrimSpace(enterprise)
	if enterprise != "" {
		if org != "" {
			return enterprise + "/" + org
		}
		return enterprise
	}
	return org
}
