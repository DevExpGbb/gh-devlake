package cmd

import (
	"encoding/json"
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
	Jobs          string
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

// resolveJenkinsJobs determines Jenkins jobs from flags or interactive remote-scope browsing.
func resolveJenkinsJobs(client *devlake.Client, connID int, opts *ScopeOpts) ([]string, error) {
	if opts.Jobs != "" {
		var jobs []string
		for _, j := range strings.Split(opts.Jobs, ",") {
			j = strings.TrimSpace(j)
			if j != "" {
				jobs = append(jobs, j)
			}
		}
		if len(jobs) == 0 {
			return nil, fmt.Errorf("no Jenkins jobs provided via --jobs")
		}
		return jobs, nil
	}

	fmt.Println("\n\U0001f50e Discovering Jenkins jobs...")
	scopes, err := listJenkinsRemoteJobs(client, connID)
	if err != nil {
		return nil, fmt.Errorf("could not list Jenkins jobs: %w", err)
	}
	if len(scopes) == 0 {
		return nil, fmt.Errorf("no Jenkins jobs found on connection %d", connID)
	}

	var labels []string
	labelToFullName := make(map[string]string)
	for _, s := range scopes {
		label := s.FullName
		if label == "" {
			label = s.Name
		}
		if label == "" {
			label = s.ID
		}
		if label == "" {
			continue
		}
		labels = append(labels, label)
		if s.FullName != "" {
			labelToFullName[label] = s.FullName
		} else {
			labelToFullName[label] = label
		}
	}

	fmt.Println()
	selected := prompt.SelectMulti("Select Jenkins jobs to collect", labels)
	var jobs []string
	for _, choice := range selected {
		if full := labelToFullName[choice]; full != "" {
			jobs = append(jobs, full)
		}
	}
	return jobs, nil
}

// listJenkinsRemoteJobs walks the Jenkins remote-scope tree and returns all job scopes.
func listJenkinsRemoteJobs(client *devlake.Client, connID int) ([]devlake.RemoteScopeChild, error) {
	var (
		allJobs    []devlake.RemoteScopeChild
		groupQueue = []string{""}
	)
	for len(groupQueue) > 0 {
		groupID := groupQueue[0]
		groupQueue = groupQueue[1:]

		pageToken := ""
		for {
			resp, err := client.ListRemoteScopes("jenkins", connID, groupID, pageToken)
			if err != nil {
				return nil, fmt.Errorf("listing remote scopes: %w", err)
			}
			for _, child := range resp.Children {
				switch child.Type {
				case "group":
					groupQueue = append(groupQueue, child.ID)
				case "scope":
					allJobs = append(allJobs, child)
				}
			}
			if resp.NextPageToken == "" {
				break
			}
			pageToken = resp.NextPageToken
		}
	}
	return allJobs, nil
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

// scopeGitHubHandler is the ScopeHandler for the github plugin.
// When opts is nil (interactive context), it prompts for DORA patterns before scoping.
func scopeGitHubHandler(client *devlake.Client, connID int, org, enterprise string, opts *ScopeOpts) (*devlake.BlueprintConnection, error) {
	if opts == nil {
		opts = &ScopeOpts{
			DeployPattern: "(?i)deploy",
			ProdPattern:   "(?i)prod",
			IncidentLabel: "incident",
		}
		fmt.Println("   Default DORA patterns:")
		fmt.Printf("   Deployment: %s\n", opts.DeployPattern)
		fmt.Printf("   Production: %s\n", opts.ProdPattern)
		fmt.Printf("   Incidents:  label=%s\n", opts.IncidentLabel)
		fmt.Println()
		if !prompt.Confirm("   Use these defaults?") {
			v := prompt.ReadLine("   Deployment workflow regex")
			if v != "" {
				opts.DeployPattern = v
			}
			v = prompt.ReadLine("   Production environment regex")
			if v != "" {
				opts.ProdPattern = v
			}
			v = prompt.ReadLine("   Incident issue label")
			if v != "" {
				opts.IncidentLabel = v
			}
		}
	}
	result, err := scopeGitHub(client, connID, org, opts)
	if err != nil {
		return nil, err
	}
	return &result.Connection, nil
}

// scopeCopilotHandler is the ScopeHandler for the gh-copilot plugin.
func scopeCopilotHandler(client *devlake.Client, connID int, org, enterprise string, opts *ScopeOpts) (*devlake.BlueprintConnection, error) {
	return scopeCopilot(client, connID, org, enterprise)
}

// scopeGitLabHandler is the ScopeHandler for the gitlab plugin.
// It resolves projects via the DevLake remote-scope API and PUTs the selected
// projects as scopes on the connection.
func scopeGitLabHandler(client *devlake.Client, connID int, org, enterprise string, opts *ScopeOpts) (*devlake.BlueprintConnection, error) {
	projects, err := resolveGitLabProjects(client, connID, org, opts)
	if err != nil {
		return nil, err
	}
	if len(projects) == 0 {
		return nil, fmt.Errorf("at least one GitLab project is required")
	}

	fmt.Println("\n📝 Adding GitLab project scopes...")
	if err := putGitLabScopes(client, connID, projects); err != nil {
		return nil, fmt.Errorf("failed to add GitLab project scopes: %w", err)
	}
	fmt.Printf("   ✅ Added %d GitLab project scope(s)\n", len(projects))

	var bpScopes []devlake.BlueprintScope
	for _, p := range projects {
		bpScopes = append(bpScopes, devlake.BlueprintScope{
			ScopeID:   strconv.Itoa(p.GitlabID),
			ScopeName: p.PathWithNamespace,
		})
	}
	return &devlake.BlueprintConnection{
		PluginName:   "gitlab",
		ConnectionID: connID,
		Scopes:       bpScopes,
	}, nil
}

// resolveGitLabProjects determines which GitLab projects to scope via flags or
// interactive browsing of the DevLake remote-scope hierarchy.
func resolveGitLabProjects(client *devlake.Client, connID int, group string, opts *ScopeOpts) ([]*devlake.GitLabProjectScope, error) {
	fmt.Println("\n📦 Resolving GitLab projects...")
	if opts != nil && opts.Repos != "" {
		var paths []string
		for _, p := range strings.Split(opts.Repos, ",") {
			if p = strings.TrimSpace(p); p != "" {
				paths = append(paths, p)
			}
		}
		return searchGitLabProjectsByPath(client, connID, paths)
	}
	if opts != nil && opts.ReposFile != "" {
		paths, err := repofile.Parse(opts.ReposFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read repos file: %w", err)
		}
		fmt.Printf("   Loaded %d project path(s) from file\n", len(paths))
		return searchGitLabProjectsByPath(client, connID, paths)
	}
	return browseGitLabProjectsInteractively(client, connID, group)
}

// searchGitLabProjectsByPath looks up GitLab projects by path using the
// search-remote-scopes API, then returns them as GitLabProjectScope entries.
func searchGitLabProjectsByPath(client *devlake.Client, connID int, paths []string) ([]*devlake.GitLabProjectScope, error) {
	var projects []*devlake.GitLabProjectScope
	for _, path := range paths {
		fmt.Printf("\n🔍 Searching for GitLab project %q...\n", path)
		resp, err := client.SearchRemoteScopes("gitlab", connID, path, 1, 20)
		if err != nil {
			return nil, fmt.Errorf("searching for GitLab project %q: %w", path, err)
		}
		var found *devlake.GitLabProjectScope
		for i := range resp.Children {
			child := &resp.Children[i]
			if child.Type != "scope" {
				continue
			}
			p := parseGitLabProject(child)
			if p == nil {
				continue
			}
			if p.PathWithNamespace == path || p.Name == path {
				found = p
				break
			}
			if found == nil {
				found = p // use first match if no exact match
			}
		}
		if found == nil {
			return nil, fmt.Errorf("GitLab project %q not found", path)
		}
		found.ConnectionID = connID
		projects = append(projects, found)
		fmt.Printf("   Found: %s (ID=%d)\n", found.PathWithNamespace, found.GitlabID)
	}
	return projects, nil
}

// browseGitLabProjectsInteractively walks the group→project hierarchy via the
// DevLake remote-scope API, prompting the user to select a group then projects.
func browseGitLabProjectsInteractively(client *devlake.Client, connID int, group string) ([]*devlake.GitLabProjectScope, error) {
	groupID := group

	if groupID == "" {
		fmt.Println("\n🔍 Listing GitLab groups...")
		resp, err := client.ListRemoteScopes("gitlab", connID, "", "")
		if err != nil {
			return nil, fmt.Errorf("listing GitLab groups: %w", err)
		}

		var groupLabels []string
		groupIDByLabel := make(map[string]string)
		for _, child := range resp.Children {
			if child.Type == "group" {
				label := child.FullName
				if label == "" {
					label = child.Name
				}
				groupLabels = append(groupLabels, label)
				groupIDByLabel[label] = child.ID
			}
		}
		if len(groupLabels) == 0 {
			return nil, fmt.Errorf("no GitLab groups found — verify your token has read_api scope")
		}

		fmt.Println()
		selected := prompt.Select("Select a GitLab group", groupLabels)
		if selected == "" {
			return nil, fmt.Errorf("no group selected")
		}
		groupID = groupIDByLabel[selected]
		if groupID == "" {
			groupID = selected
		}
	}

	fmt.Printf("\n🔍 Listing projects in group %q...\n", groupID)
	resp, err := client.ListRemoteScopes("gitlab", connID, groupID, "")
	if err != nil {
		return nil, fmt.Errorf("listing projects in group %q: %w", groupID, err)
	}

	var projectLabels []string
	projectByLabel := make(map[string]*devlake.RemoteScopeChild)
	for i := range resp.Children {
		child := &resp.Children[i]
		if child.Type != "scope" {
			continue
		}
		label := child.FullName
		if label == "" {
			label = child.Name
		}
		projectLabels = append(projectLabels, label)
		projectByLabel[label] = child
	}
	if len(projectLabels) == 0 {
		return nil, fmt.Errorf("no projects found in group %q", groupID)
	}

	fmt.Println()
	selectedLabels := prompt.SelectMulti("Select GitLab projects to configure", projectLabels)
	var projects []*devlake.GitLabProjectScope
	for _, label := range selectedLabels {
		child, ok := projectByLabel[label]
		if !ok {
			continue
		}
		p := parseGitLabProject(child)
		if p != nil {
			p.ConnectionID = connID
			projects = append(projects, p)
		}
	}
	return projects, nil
}

// parseGitLabProject extracts project fields from a RemoteScopeChild's Data payload.
func parseGitLabProject(child *devlake.RemoteScopeChild) *devlake.GitLabProjectScope {
	var p devlake.GitLabProjectScope
	if err := json.Unmarshal(child.Data, &p); err != nil {
		return nil
	}
	if p.PathWithNamespace == "" {
		p.PathWithNamespace = child.FullName
	}
	if p.Name == "" {
		p.Name = child.Name
	}
	return &p
}

// putGitLabScopes batch-upserts GitLab project scopes on a connection.
func putGitLabScopes(client *devlake.Client, connID int, projects []*devlake.GitLabProjectScope) error {
	var data []any
	for _, p := range projects {
		data = append(data, p)
	}
	return client.PutScopes("gitlab", connID, &devlake.ScopeBatchRequest{Data: data})
}

// scopeJenkinsHandler is the ScopeHandler for the jenkins plugin.
func scopeJenkinsHandler(client *devlake.Client, connID int, org, enterprise string, opts *ScopeOpts) (*devlake.BlueprintConnection, error) {
	if opts == nil {
		opts = &ScopeOpts{}
	}
	jobs, err := resolveJenkinsJobs(client, connID, opts)
	if err != nil {
		return nil, err
	}
	if len(jobs) == 0 {
		return nil, fmt.Errorf("at least one Jenkins job is required")
	}

	fmt.Println("\n📝 Adding Jenkins job scopes...")
	var (
		data   []any
		scopes []devlake.BlueprintScope
	)
	for _, job := range jobs {
		data = append(data, devlake.JenkinsJobScope{
			ConnectionID: connID,
			FullName:     job,
			Name:         job,
		})
		scopes = append(scopes, devlake.BlueprintScope{
			ScopeID:   job,
			ScopeName: job,
		})
	}
	if err := client.PutScopes("jenkins", connID, &devlake.ScopeBatchRequest{Data: data}); err != nil {
		return nil, fmt.Errorf("failed to add Jenkins job scopes: %w", err)
	}
	fmt.Printf("   ✅ Added %d Jenkins job scope(s)\n", len(jobs))

	return &devlake.BlueprintConnection{
		PluginName:   "jenkins",
		ConnectionID: connID,
		Scopes:       scopes,
	}, nil
}

// scopeJiraHandler is the ScopeHandler for the jira plugin.
func scopeJiraHandler(client *devlake.Client, connID int, org, enterprise string, opts *ScopeOpts) (*devlake.BlueprintConnection, error) {
	fmt.Println("\n📋 Fetching Jira boards...")
	remoteScopes, err := client.ListRemoteScopes("jira", connID, "", "")
	if err != nil {
		return nil, fmt.Errorf("failed to list Jira boards: %w", err)
	}

	// Aggregate all pages of remote scopes
	allChildren := remoteScopes.Children
	nextToken := remoteScopes.NextPageToken
	for nextToken != "" {
		page, err := client.ListRemoteScopes("jira", connID, "", nextToken)
		if err != nil {
			return nil, fmt.Errorf("failed to list Jira boards (page token %s): %w", nextToken, err)
		}
		allChildren = append(allChildren, page.Children...)
		nextToken = page.NextPageToken
	}

	// Extract boards from remote-scope response
	var boardOptions []string
	boardMap := make(map[string]*devlake.RemoteScopeChild)
	for i := range allChildren {
		child := &allChildren[i]
		if child.Type == "scope" {
			label := child.Name
			if child.ID != "" {
				label = fmt.Sprintf("%s (ID: %s)", child.Name, child.ID)
			}
			boardOptions = append(boardOptions, label)
			boardMap[label] = child
		}
	}

	if len(boardOptions) == 0 {
		return nil, fmt.Errorf("no Jira boards found for connection %d", connID)
	}

	fmt.Println()
	selectedLabels := prompt.SelectMulti("Select Jira boards to track", boardOptions)
	if len(selectedLabels) == 0 {
		return nil, fmt.Errorf("at least one board must be selected")
	}

	// Build scope data for PUT
	fmt.Println("\n📝 Adding Jira board scopes...")
	var scopeData []any
	var blueprintScopes []devlake.BlueprintScope
	for _, label := range selectedLabels {
		child := boardMap[label]
		// Parse boardId from child.ID (should be a string representation of uint64)
		if child.ID == "" {
			fmt.Printf("   ⚠️  Skipping board %q: empty ID\n", child.Name)
			continue
		}
		boardID, err := strconv.ParseUint(child.ID, 10, 64)
		if err != nil {
			fmt.Printf("   ⚠️  Skipping board %q: invalid ID %q\n", child.Name, child.ID)
			continue
		}
		scopeData = append(scopeData, devlake.JiraBoardScope{
			BoardID:      boardID,
			ConnectionID: connID,
			Name:         child.Name,
		})
		blueprintScopes = append(blueprintScopes, devlake.BlueprintScope{
			ScopeID:   child.ID,
			ScopeName: child.Name,
		})
	}

	if len(scopeData) == 0 {
		return nil, fmt.Errorf("no valid boards to add")
	}

	err = client.PutScopes("jira", connID, &devlake.ScopeBatchRequest{Data: scopeData})
	if err != nil {
		return nil, fmt.Errorf("failed to add Jira board scopes: %w", err)
	}
	fmt.Printf("   ✅ Added %d board scope(s)\n", len(scopeData))

	return &devlake.BlueprintConnection{
		PluginName:   "jira",
		ConnectionID: connID,
		Scopes:       blueprintScopes,
	}, nil
	}, nil
}
