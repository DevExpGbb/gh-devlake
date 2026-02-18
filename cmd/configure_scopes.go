package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/DevExpGBB/gh-devlake/internal/devlake"
	"github.com/DevExpGBB/gh-devlake/internal/gh"
	"github.com/DevExpGBB/gh-devlake/internal/prompt"
	"github.com/DevExpGBB/gh-devlake/internal/repofile"
	"github.com/spf13/cobra"
)

var (
	scopeOrg           string
	scopeRepos         string
	scopeReposFile     string
	scopeGHConnID      int
	scopeCopilotConnID int
	scopeProjectName   string
	scopeDeployPattern string
	scopeProdPattern   string
	scopeIncidentLabel string
	scopeTimeAfter     string
	scopeCron          string
	scopeSkipSync      bool
	scopeSkipCopilot   bool
	scopeWait          bool
	scopeTimeout       time.Duration
)

func newConfigureScopesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scopes",
		Short: "Configure collection scopes for existing connections",
		Long: `Adds repository scopes and scope-configs to existing DevLake connections,
creates a project with DORA metrics, configures a blueprint, and triggers
the first data sync.

Example:
  gh devlake configure scopes --org my-org --repos owner/repo1,owner/repo2
  gh devlake configure scopes --org my-org --repos-file repos.txt`,
		RunE: runConfigureScopes,
	}

	cmd.Flags().StringVar(&scopeOrg, "org", "", "GitHub organization slug")
	cmd.Flags().StringVar(&scopeRepos, "repos", "", "Comma-separated repos (owner/repo)")
	cmd.Flags().StringVar(&scopeReposFile, "repos-file", "", "Path to file with repos (one per line)")
	cmd.Flags().IntVar(&scopeGHConnID, "github-connection-id", 0, "GitHub connection ID (auto-detected if omitted)")
	cmd.Flags().IntVar(&scopeCopilotConnID, "copilot-connection-id", 0, "Copilot connection ID (auto-detected if omitted)")
	cmd.Flags().StringVar(&scopeProjectName, "project-name", "", "DevLake project name (defaults to org name)")
	cmd.Flags().StringVar(&scopeDeployPattern, "deployment-pattern", "(?i)deploy", "Regex to match deployment workflows")
	cmd.Flags().StringVar(&scopeProdPattern, "production-pattern", "(?i)prod", "Regex to match production environment")
	cmd.Flags().StringVar(&scopeIncidentLabel, "incident-label", "incident", "Issue label for incidents")
	cmd.Flags().StringVar(&scopeTimeAfter, "time-after", "", "Only collect data after this date (default: 6 months ago)")
	cmd.Flags().StringVar(&scopeCron, "cron", "0 0 * * *", "Blueprint cron schedule")
	cmd.Flags().BoolVar(&scopeSkipSync, "skip-sync", false, "Skip triggering the first data sync")
	cmd.Flags().BoolVar(&scopeSkipCopilot, "skip-copilot", false, "Skip adding Copilot scope")
	cmd.Flags().BoolVar(&scopeWait, "wait", true, "Wait for pipeline to complete")
	cmd.Flags().DurationVar(&scopeTimeout, "timeout", 5*time.Minute, "Max time to wait for pipeline")

	return cmd
}

func init() {}

func runConfigureScopes(cmd *cobra.Command, args []string) error {
	fmt.Println()

	// ── Step 1: Discover DevLake ──
	fmt.Println("🔍 Discovering DevLake instance...")
	disc, err := devlake.Discover(cfgURL)
	if err != nil {
		return err
	}
	fmt.Printf("   Found DevLake at %s (via %s)\n", disc.URL, disc.Source)

	client := devlake.NewClient(disc.URL)
	statePath, state := devlake.FindStateFile(disc.URL, disc.GrafanaURL)

	// ── Step 2: Resolve connection IDs ──
	fmt.Println("\n🔗 Resolving connections...")
	ghConnID, err := resolveConnectionID(client, state, "github", scopeGHConnID)
	if err != nil {
		return fmt.Errorf("GitHub connection: %w", err)
	}
	fmt.Printf("   GitHub connection ID: %d\n", ghConnID)

	copilotConnID := 0
	if !scopeSkipCopilot {
		copilotConnID, err = resolveConnectionID(client, state, "gh-copilot", scopeCopilotConnID)
		if err != nil {
			fmt.Printf("   ⚠️  Copilot connection not found, skipping: %v\n", err)
			scopeSkipCopilot = true
		} else {
			fmt.Printf("   Copilot connection ID: %d\n", copilotConnID)
		}
	}

	// ── Step 3: Resolve organization ──
	org := resolveOrg(state, scopeOrg)
	if org == "" {
		return fmt.Errorf("organization is required (use --org)")
	}
	fmt.Printf("   Organization: %s\n", org)

	projectName := scopeProjectName
	if projectName == "" {
		projectName = org
	}

	timeAfter := scopeTimeAfter
	if timeAfter == "" {
		timeAfter = time.Now().AddDate(0, -6, 0).Format("2006-01-02T00:00:00Z")
	}

	// ── Step 4: Resolve repositories ──
	fmt.Println("\n📦 Resolving repositories...")
	repos, err := resolveRepos(org)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		return fmt.Errorf("at least one repository is required")
	}
	fmt.Printf("   Repos to configure: %s\n", strings.Join(repos, ", "))

	// ── Step 5: Look up repo details ──
	fmt.Println("\n🔎 Looking up repo details...")
	var repoDetails []*gh.RepoDetails
	for _, repo := range repos {
		detail, err := gh.GetRepoDetails(repo)
		if err != nil {
			fmt.Printf("   ⚠️  Could not fetch details for %q: %v\n", repo, err)
			continue
		}
		repoDetails = append(repoDetails, detail)
		fmt.Printf("   %s (ID: %d)\n", detail.FullName, detail.ID)
	}
	if len(repoDetails) == 0 {
		return fmt.Errorf("could not resolve any repository details — verify repos exist and gh CLI is authenticated")
	}

	// ── Step 6: Create DORA scope config ──
	fmt.Println("\n⚙️  Creating DORA scope config...")
	scopeConfigID, err := ensureScopeConfig(client, ghConnID)
	if err != nil {
		fmt.Printf("   ⚠️  Could not create scope config: %v\n", err)
	} else {
		fmt.Printf("   Scope config ID: %d\n", scopeConfigID)
	}

	// ── Step 7: PUT GitHub repo scopes ──
	fmt.Println("\n📝 Adding repository scopes...")
	err = putGitHubScopes(client, ghConnID, scopeConfigID, repoDetails)
	if err != nil {
		return fmt.Errorf("failed to add repo scopes: %w", err)
	}
	fmt.Printf("   ✅ Added %d repo scope(s)\n", len(repoDetails))

	// ── Step 8: PUT Copilot scope ──
	if !scopeSkipCopilot && copilotConnID > 0 {
		fmt.Println("\n📝 Adding Copilot scope...")
		err = putCopilotScope(client, copilotConnID, org)
		if err != nil {
			fmt.Printf("   ⚠️  Could not add Copilot scope: %v\n", err)
		} else {
			fmt.Printf("   ✅ Copilot scope added: %s\n", org)
		}
	}

	// ── Step 9: Create project ──
	fmt.Println("\n🏗️  Creating DevLake project...")
	fmt.Println("   A DevLake project groups data from multiple connections — useful")
	fmt.Println("   per team or business unit. It automatically creates a sync schedule.")
	blueprintID, err := ensureProject(client, projectName, org)
	if err != nil {
		return fmt.Errorf("failed to create project: %w", err)
	}
	fmt.Printf("   Project: %s, Blueprint ID: %d\n", projectName, blueprintID)

	// ── Step 10: PATCH blueprint ──
	fmt.Println("\n📋 Configuring blueprint...")
	err = patchBlueprint(client, blueprintID, ghConnID, copilotConnID, org, repoDetails, timeAfter)
	if err != nil {
		return fmt.Errorf("failed to configure blueprint: %w", err)
	}
	fmt.Printf("   ✅ Blueprint configured with %d repo(s)\n", len(repoDetails))
	fmt.Printf("   Schedule: %s | Data since: %s\n", scopeCron, timeAfter)

	// ── Step 11: Trigger sync ──
	if !scopeSkipSync {
		fmt.Println("\n🚀 Triggering first data sync...")
		fmt.Println("   This collects repository, PR, and Copilot data from GitHub.")
		fmt.Println("   Depending on repo size and history, this may take 5–30 minutes.")
		err = triggerAndPoll(client, blueprintID)
		if err != nil {
			fmt.Printf("   ⚠️  %v\n", err)
		}
	}

	// ── Step 12: Update state file ──
	state.Project = &devlake.StateProject{
		Name:         projectName,
		BlueprintID:  blueprintID,
		Repos:        repos,
		Organization: org,
	}
	state.ScopesConfiguredAt = time.Now().Format(time.RFC3339)
	if err := devlake.SaveState(statePath, state); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Could not update state file: %v\n", err)
	} else {
		fmt.Printf("\n💾 State saved to %s\n", statePath)
	}

	// ── Summary ──
	fmt.Println("\n" + strings.Repeat("─", 50))
	fmt.Println("✅ Scopes configured successfully!")
	fmt.Printf("   Project: %s\n", projectName)
	fmt.Printf("   Repos:   %s\n", strings.Join(repos, ", "))
	if !scopeSkipCopilot {
		fmt.Printf("   Copilot: %s\n", org)
	}
	fmt.Println(strings.Repeat("─", 50))

	return nil
}

// resolveConnectionID finds a connection ID from flag, state, or API.
func resolveConnectionID(client *devlake.Client, state *devlake.State, plugin string, flagValue int) (int, error) {
	if flagValue > 0 {
		return flagValue, nil
	}

	// Try state file
	if state != nil {
		for _, c := range state.Connections {
			if c.Plugin == plugin {
				return c.ConnectionID, nil
			}
		}
	}

	// Try API
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

	return 0, fmt.Errorf("no %s connections found — run 'configure connections' first", plugin)
}

// resolveOrg determines the organization from flag, state, or prompt.
func resolveOrg(state *devlake.State, flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if state != nil {
		for _, c := range state.Connections {
			if c.Plugin == "gh-copilot" && c.Organization != "" {
				return c.Organization
			}
		}
	}
	return prompt.ReadLine("Enter your GitHub organization slug")
}

// resolveRepos determines repos from flags, file, or interactive gh CLI selection.
func resolveRepos(org string) ([]string, error) {
	// From --repos flag
	if scopeRepos != "" {
		var repos []string
		for _, r := range strings.Split(scopeRepos, ",") {
			r = strings.TrimSpace(r)
			if r != "" {
				repos = append(repos, r)
			}
		}
		return repos, nil
	}

	// From --repos-file flag
	if scopeReposFile != "" {
		repos, err := repofile.Parse(scopeReposFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read repos file: %w", err)
		}
		fmt.Printf("   Loaded %d repo(s) from file\n", len(repos))
		return repos, nil
	}

	// Interactive: try gh CLI
	if gh.IsAvailable() {
		fmt.Printf("   Listing repos in %q via gh CLI...\n", org)
		available, err := gh.ListRepos(org, 30)
		if err == nil && len(available) > 0 {
			selected := prompt.SelectMulti(fmt.Sprintf("Available repos in %s (up to 30)", org), available)
			return selected, nil
		}
	}

	// Manual prompt
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
func ensureScopeConfig(client *devlake.Client, connID int) (int, error) {
	cfg := &devlake.ScopeConfig{
		Name:              "dora-config",
		ConnectionID:      connID,
		DeploymentPattern: scopeDeployPattern,
		ProductionPattern: scopeProdPattern,
		IssueTypeIncident: scopeIncidentLabel,
		Refdiff: &devlake.RefdiffConfig{
			TagsPattern: ".*",
			TagsLimit:   10,
			TagsOrder:   "reverse semver",
		},
	}

	result, err := client.CreateScopeConfig("github", connID, cfg)
	if err == nil {
		return result.ID, nil
	}

	// Try to find existing
	existing, listErr := client.ListScopeConfigs("github", connID)
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

// putCopilotScope adds the organization scope to the Copilot connection.
func putCopilotScope(client *devlake.Client, connID int, org string) error {
	data := []any{
		devlake.CopilotScope{
			ID:           org,
			ConnectionID: connID,
			Organization: org,
			Name:         org,
			FullName:     org,
		},
	}
	return client.PutScopes("gh-copilot", connID, &devlake.ScopeBatchRequest{Data: data})
}

// ensureProject creates a project or returns an existing one's blueprint ID.
func ensureProject(client *devlake.Client, name, org string) (int, error) {
	project := &devlake.Project{
		Name:        name,
		Description: fmt.Sprintf("DORA metrics and Copilot adoption for %s", org),
		Metrics: []devlake.ProjectMetric{
			{PluginName: "dora", Enable: true},
		},
	}

	result, err := client.CreateProject(project)
	if err == nil && result.Blueprint != nil {
		return result.Blueprint.ID, nil
	}

	// Try to get existing project
	existing, getErr := client.GetProject(name)
	if getErr != nil {
		return 0, fmt.Errorf("create failed: %v; get failed: %v", err, getErr)
	}
	if existing != nil && existing.Blueprint != nil {
		return existing.Blueprint.ID, nil
	}
	return 0, fmt.Errorf("project %q has no blueprint", name)
}

// patchBlueprint updates the blueprint with connections, scopes, and schedule.
func patchBlueprint(client *devlake.Client, blueprintID, ghConnID, copilotConnID int, org string, details []*gh.RepoDetails, timeAfter string) error {
	var ghScopes []devlake.BlueprintScope
	for _, d := range details {
		ghScopes = append(ghScopes, devlake.BlueprintScope{
			ScopeID:   strconv.Itoa(d.ID),
			ScopeName: d.FullName,
		})
	}

	connections := []devlake.BlueprintConnection{
		{
			PluginName:   "github",
			ConnectionID: ghConnID,
			Scopes:       ghScopes,
		},
	}

	if !scopeSkipCopilot && copilotConnID > 0 {
		connections = append(connections, devlake.BlueprintConnection{
			PluginName:   "gh-copilot",
			ConnectionID: copilotConnID,
			Scopes: []devlake.BlueprintScope{
				{ScopeID: org, ScopeName: org},
			},
		})
	}

	enable := true
	patch := &devlake.BlueprintPatch{
		Enable:      &enable,
		CronConfig:  scopeCron,
		TimeAfter:   timeAfter,
		Connections: connections,
	}

	_, err := client.PatchBlueprint(blueprintID, patch)
	return err
}

// triggerAndPoll triggers a blueprint sync and monitors progress.
func triggerAndPoll(client *devlake.Client, blueprintID int) error {
	pipeline, err := client.TriggerBlueprint(blueprintID)
	if err != nil {
		return fmt.Errorf("could not trigger sync: %w", err)
	}
	fmt.Printf("   Pipeline started (ID: %d)\n", pipeline.ID)

	if !scopeWait {
		return nil
	}

	fmt.Println("   Monitoring progress...")
	deadline := time.Now().Add(scopeTimeout)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		p, err := client.GetPipeline(pipeline.ID)
		if err != nil {
			elapsed := time.Since(deadline.Add(-scopeTimeout)).Truncate(time.Second)
			fmt.Printf("   [%s] Could not check status...\n", elapsed)
		} else {
			elapsed := time.Since(deadline.Add(-scopeTimeout)).Truncate(time.Second)
			fmt.Printf("   [%s] Status: %s | Tasks: %d/%d\n", elapsed, p.Status, p.FinishedTasks, p.TotalTasks)

			switch p.Status {
			case "TASK_COMPLETED":
				fmt.Println("\n   ✅ Data sync completed!")
				return nil
			case "TASK_FAILED":
				return fmt.Errorf("pipeline failed — check DevLake logs")
			}
		}

		if time.Now().After(deadline) {
			fmt.Println("\n   ⏳ Monitoring timed out. Pipeline is still running.")
			fmt.Printf("   Check status: GET /pipelines/%d\n", pipeline.ID)
			return nil
		}
	}
	return nil
}

// marshalJSON is unused but available for debug output.
func marshalJSON(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}
