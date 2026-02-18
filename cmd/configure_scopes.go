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
	scopeSkipGitHub    bool
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

// scopeGitHubResult holds the outputs from scoping a GitHub connection.
type scopeGitHubResult struct {
	Connection  devlake.BlueprintConnection
	Repos       []string
	RepoDetails []*gh.RepoDetails
}

// scopeGitHub resolves repos, creates scope config, and PUTs repo scopes
// for a GitHub connection. Returns the BlueprintConnection entry and repo list.
func scopeGitHub(client *devlake.Client, connID int, org string) (*scopeGitHubResult, error) {
	// ── Resolve repositories ──
	fmt.Println("\n📦 Resolving repositories...")
	repos, err := resolveRepos(org)
	if err != nil {
		return nil, err
	}
	if len(repos) == 0 {
		return nil, fmt.Errorf("at least one repository is required")
	}
	fmt.Printf("   Repos to configure: %s\n", strings.Join(repos, ", "))

	// ── Look up repo details ──
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
		return nil, fmt.Errorf("could not resolve any repository details — verify repos exist and gh CLI is authenticated")
	}

	// ── Create DORA scope config ──
	fmt.Println("\n⚙️  Creating DORA scope config...")
	scopeConfigID, err := ensureScopeConfig(client, connID)
	if err != nil {
		fmt.Printf("   ⚠️  Could not create scope config: %v\n", err)
	} else {
		fmt.Printf("   Scope config ID: %d\n", scopeConfigID)
	}

	// ── PUT GitHub repo scopes ──
	fmt.Println("\n📝 Adding repository scopes...")
	err = putGitHubScopes(client, connID, scopeConfigID, repoDetails)
	if err != nil {
		return nil, fmt.Errorf("failed to add repo scopes: %w", err)
	}
	fmt.Printf("   ✅ Added %d repo scope(s)\n", len(repoDetails))

	// Build the BlueprintConnection entry
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

// scopeCopilot PUTs the org scope for a Copilot connection.
// Returns the BlueprintConnection entry.
func scopeCopilot(client *devlake.Client, connID int, org string) (*devlake.BlueprintConnection, error) {
	fmt.Println("\n📝 Adding Copilot scope...")
	err := putCopilotScope(client, connID, org)
	if err != nil {
		return nil, fmt.Errorf("could not add Copilot scope: %w", err)
	}
	fmt.Printf("   ✅ Copilot scope added: %s\n", org)

	return &devlake.BlueprintConnection{
		PluginName:   "gh-copilot",
		ConnectionID: connID,
		Scopes: []devlake.BlueprintScope{
			{ScopeID: org, ScopeName: org},
		},
	}, nil
}

// finalizeProjectOpts holds the parameters for finalizeProject.
type finalizeProjectOpts struct {
	Client      *devlake.Client
	StatePath   string
	State       *devlake.State
	ProjectName string
	Org         string
	Connections []devlake.BlueprintConnection
	Repos       []string
	HasGitHub   bool
	HasCopilot  bool
}

// finalizeProject creates the project, patches the blueprint with all
// accumulated connections, triggers the first sync, and saves state.
func finalizeProject(opts finalizeProjectOpts) error {
	timeAfter := scopeTimeAfter
	if timeAfter == "" {
		timeAfter = time.Now().AddDate(0, -6, 0).Format("2006-01-02T00:00:00Z")
	}

	// ── Create project ──
	fmt.Println("\n🏗️  Creating DevLake project...")
	blueprintID, err := ensureProjectWithFlags(opts.Client, opts.ProjectName, opts.Org, opts.HasGitHub, opts.HasCopilot)
	if err != nil {
		return fmt.Errorf("failed to create project: %w", err)
	}
	fmt.Printf("   Project: %s, Blueprint ID: %d\n", opts.ProjectName, blueprintID)

	// ── PATCH blueprint ──
	fmt.Println("\n📋 Configuring blueprint...")
	enable := true
	patch := &devlake.BlueprintPatch{
		Enable:      &enable,
		CronConfig:  scopeCron,
		TimeAfter:   timeAfter,
		Connections: opts.Connections,
	}
	_, err = opts.Client.PatchBlueprint(blueprintID, patch)
	if err != nil {
		return fmt.Errorf("failed to configure blueprint: %w", err)
	}
	fmt.Printf("   ✅ Blueprint configured with %d connection(s)\n", len(opts.Connections))
	fmt.Printf("   Schedule: %s | Data since: %s\n", scopeCron, timeAfter)

	// ── Trigger sync ──
	if !scopeSkipSync {
		fmt.Println("\n🚀 Triggering first data sync...")
		fmt.Println("   This collects repository, PR, and Copilot data from GitHub.")
		fmt.Println("   Depending on repo size and history, this may take 5–30 minutes.")
		if err := triggerAndPoll(opts.Client, blueprintID); err != nil {
			fmt.Printf("   ⚠️  %v\n", err)
		}
	}

	// ── Update state file ──
	opts.State.Project = &devlake.StateProject{
		Name:         opts.ProjectName,
		BlueprintID:  blueprintID,
		Repos:        opts.Repos,
		Organization: opts.Org,
	}
	opts.State.ScopesConfiguredAt = time.Now().Format(time.RFC3339)
	if err := devlake.SaveState(opts.StatePath, opts.State); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Could not update state file: %v\n", err)
	} else {
		fmt.Printf("\n💾 State saved to %s\n", opts.StatePath)
	}

	// ── Summary ──
	fmt.Println("\n" + strings.Repeat("─", 50))
	fmt.Println("✅ Scopes configured successfully!")
	fmt.Printf("   Project: %s\n", opts.ProjectName)
	if len(opts.Repos) > 0 {
		fmt.Printf("   Repos:   %s\n", strings.Join(opts.Repos, ", "))
	}
	if opts.HasCopilot {
		fmt.Printf("   Copilot: %s\n", opts.Org)
	}
	fmt.Println(strings.Repeat("─", 50))

	return nil
}

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
	ghConnID := 0
	if !scopeSkipGitHub {
		ghConnID, err = resolveConnectionID(client, state, "github", scopeGHConnID)
		if err != nil {
			fmt.Printf("   ⚠️  GitHub connection not found, skipping: %v\n", err)
			scopeSkipGitHub = true
		} else {
			fmt.Printf("   GitHub connection ID: %d\n", ghConnID)
		}
	}

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

	if scopeSkipGitHub && scopeSkipCopilot {
		return fmt.Errorf("no connections available — run 'configure connections' first")
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

	// ── Scope connections ──
	var connections []devlake.BlueprintConnection
	var repos []string

	if !scopeSkipGitHub {
		result, err := scopeGitHub(client, ghConnID, org)
		if err != nil {
			return err
		}
		connections = append(connections, result.Connection)
		repos = result.Repos
	}

	if !scopeSkipCopilot && copilotConnID > 0 {
		conn, err := scopeCopilot(client, copilotConnID, org)
		if err != nil {
			fmt.Printf("   ⚠️  %v\n", err)
		} else {
			connections = append(connections, *conn)
		}
	}

	// ── Finalize ──
	return finalizeProject(finalizeProjectOpts{
		Client:      client,
		StatePath:   statePath,
		State:       state,
		ProjectName: projectName,
		Org:         org,
		Connections: connections,
		Repos:       repos,
		HasGitHub:   !scopeSkipGitHub,
		HasCopilot:  !scopeSkipCopilot,
	})
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
			if c.Organization != "" {
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
		if err != nil {
			fmt.Printf("   ⚠️  Could not list repos: %v\n", err)
		} else if len(available) == 0 {
			fmt.Println("   ⚠️  No repos found — verify the org name and PAT scopes (read:org)")
		} else {
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

// ensureProjectWithFlags creates a project or returns an existing one's blueprint ID.
// Uses explicit hasGitHub/hasCopilot flags instead of package globals.
func ensureProjectWithFlags(client *devlake.Client, name, org string, hasGitHub, hasCopilot bool) (int, error) {
	desc := fmt.Sprintf("DORA metrics and Copilot adoption for %s", org)
	if !hasGitHub {
		desc = fmt.Sprintf("Copilot adoption for %s", org)
	} else if !hasCopilot {
		desc = fmt.Sprintf("DORA metrics for %s", org)
	}
	project := &devlake.Project{
		Name:        name,
		Description: desc,
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
