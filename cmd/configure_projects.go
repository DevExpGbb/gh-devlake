package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/DevExpGBB/gh-devlake/internal/devlake"
	"github.com/DevExpGBB/gh-devlake/internal/prompt"
	"github.com/spf13/cobra"
)

// ProjectOpts holds options for the project command.
type ProjectOpts struct {
	ProjectName string
	TimeAfter   string
	Cron        string
	SkipSync    bool
	Wait        bool
	Timeout     time.Duration
	// Pre-resolved fields set by orchestrators (full, init)
	Client    *devlake.Client
	StatePath string
	State     *devlake.State
}

func newConfigureProjectsCmd() *cobra.Command {
	var opts ProjectOpts
	cmd := &cobra.Command{
		Use:     "project",
		Aliases: []string{"projects"},
		Short:   "Create a DevLake project and start data collection",
		Long: `Creates a DevLake project that groups data from your connections.

A project ties together existing scopes (repos, orgs) from your connections
into a single view with DORA metrics. It creates a sync schedule (blueprint)
that collects data on a cron schedule (daily by default).

Prerequisites: run 'gh devlake configure scope' first to add scopes.

This command will:
  1. Discover existing scopes on your connections
  2. Let you choose which scopes to include
  3. Create the project with DORA metrics enabled
  4. Configure a sync blueprint
  5. Trigger the first data collection

Example:
  gh devlake configure project
  gh devlake configure project --project-name my-team`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigureProjects(cmd, args, &opts)
		},
	}

	cmd.Flags().StringVar(&opts.ProjectName, "project-name", "", "DevLake project name")
	cmd.Flags().StringVar(&opts.TimeAfter, "time-after", "", "Only collect data after this date (default: 6 months ago)")
	cmd.Flags().StringVar(&opts.Cron, "cron", "0 0 * * *", "Blueprint cron schedule")
	cmd.Flags().BoolVar(&opts.SkipSync, "skip-sync", false, "Skip triggering the first data sync")
	cmd.Flags().BoolVar(&opts.Wait, "wait", true, "Wait for pipeline to complete")
	cmd.Flags().DurationVar(&opts.Timeout, "timeout", 5*time.Minute, "Max time to wait for pipeline")

	return cmd
}

// connChoice represents a discovered connection for the interactive picker.
type connChoice struct {
	plugin     string
	id         int
	label      string
	enterprise string
}

// addedConnection tracks a connection whose scopes are included in the project.
type addedConnection struct {
	plugin  string
	connID  int
	label   string
	summary string
	bpConn  devlake.BlueprintConnection
	repos   []string
}

func runConfigureProjects(cmd *cobra.Command, args []string, opts *ProjectOpts) error {
	fmt.Println()
	fmt.Println("\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550")
	fmt.Println("  DevLake \u2014 Project Setup")
	fmt.Println("\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550")
	fmt.Println()
	fmt.Println("   A DevLake project groups data from multiple connections into a")
	fmt.Println("   single view with DORA metrics. Think of it as one project per")
	fmt.Println("   team or business unit.")

	client := opts.Client
	statePath := opts.StatePath
	state := opts.State

	// Discover DevLake if not pre-resolved by an orchestrator
	if client == nil {
		fmt.Println("\n\U0001f50d Discovering DevLake instance...")
		disc, err := devlake.Discover(cfgURL)
		if err != nil {
			return err
		}
		fmt.Printf("   Found DevLake at %s (via %s)\n", disc.URL, disc.Source)

		client = devlake.NewClient(disc.URL)
		statePath, state = devlake.FindStateFile(disc.URL, disc.GrafanaURL)
	}

	// Project name — derive default from state connections
	defaultName := "my-project"
	if state != nil {
		for _, c := range state.Connections {
			if c.Organization != "" {
				defaultName = c.Organization
				break
			}
		}
	}
	projectName := opts.ProjectName
	if projectName == "" {
		custom := prompt.ReadLine(fmt.Sprintf("\nProject name [%s]", defaultName))
		if custom != "" {
			projectName = custom
		} else {
			projectName = defaultName
		}
	}

	// Discover connections
	fmt.Println("\n\U0001f50d Discovering connections...")
	choices := discoverConnections(client, state)
	if len(choices) == 0 {
		return fmt.Errorf("no connections found \u2014 run 'gh devlake configure connection' first")
	}

	// Iterative connection addition loop
	var added []addedConnection
	remaining := make([]connChoice, len(choices))
	copy(remaining, choices)

	for {
		if len(remaining) == 0 {
			if len(added) == 0 {
				return fmt.Errorf("at least one connection is required")
			}
			fmt.Println("\n   All available connections have been added.")
			break
		}

		var picked connChoice

		if len(added) > 0 {
			fmt.Println()
			fmt.Println("   " + strings.Repeat("\u2500", 44))
			fmt.Println("   Added so far:")
			for _, a := range added {
				fmt.Printf("     \u2705 %s\n", a.summary)
			}
			fmt.Println("   " + strings.Repeat("\u2500", 44))
		}

		fmt.Println()
		fmt.Println("   Choose a connection to add to this project.")
		fmt.Println()

		labels := make([]string, len(remaining))
		for i, c := range remaining {
			labels[i] = c.label
		}
		chosen := prompt.Select("Add connection", labels)
		if chosen == "" {
			if len(added) == 0 {
				return fmt.Errorf("at least one connection is required")
			}
			break
		}
		for _, c := range remaining {
			if c.label == chosen {
				picked = c
				break
			}
		}

		// List existing scopes on the picked connection
		ac, err := listConnectionScopes(client, picked)
		if err != nil {
			fmt.Printf("   \u26a0\ufe0f  Could not list scopes for %s: %v\n", picked.label, err)
			fmt.Println("   Run 'gh devlake configure scope' to add scopes first.")
			remaining = removeChoice(remaining, picked)
			continue
		}
		added = append(added, *ac)
		remaining = removeChoice(remaining, picked)

		if len(remaining) == 0 {
			fmt.Println("\n   All available connections have been added.")
			break
		}
		if !prompt.Confirm("\nWould you like to add another connection?") {
			break
		}
	}

	if len(added) == 0 {
		return fmt.Errorf("at least one connection is required")
	}

	// Accumulate results
	var connections []devlake.BlueprintConnection
	var allRepos []string
	var pluginNames []string
	for _, a := range added {
		connections = append(connections, a.bpConn)
		allRepos = append(allRepos, a.repos...)
		pluginNames = append(pluginNames, pluginDisplayName(a.plugin))
	}

	// Show what will happen
	fmt.Println()
	fmt.Println("   Ready to finalize:")
	for _, a := range added {
		fmt.Printf("     \u2022 %s\n", a.summary)
	}
	fmt.Println("     \u2022 Create project with DORA metrics")
	fmt.Println("     \u2022 Configure daily sync schedule")
	if !opts.SkipSync {
		fmt.Println("     \u2022 Trigger the first data collection")
	}

	// Finalize
	return finalizeProject(finalizeProjectOpts{
		Client:      client,
		StatePath:   statePath,
		State:       state,
		ProjectName: projectName,
		Connections: connections,
		Repos:       allRepos,
		PluginNames: pluginNames,
		Cron:        opts.Cron,
		TimeAfter:   opts.TimeAfter,
		SkipSync:    opts.SkipSync,
		Wait:        opts.Wait,
		Timeout:     opts.Timeout,
	})
}

// listConnectionScopes lists existing scopes on a connection and builds an
// addedConnection from them. Returns an error if no scopes are found.
func listConnectionScopes(client *devlake.Client, c connChoice) (*addedConnection, error) {
	fmt.Printf("\n\U0001f4e6 Listing scopes on %s...\n", c.label)
	resp, err := client.ListScopes(c.plugin, c.id)
	if err != nil {
		return nil, fmt.Errorf("could not list scopes: %w", err)
	}
	if resp == nil || len(resp.Scopes) == 0 {
		return nil, fmt.Errorf("no scopes found on connection %d \u2014 run 'gh devlake configure scope' first", c.id)
	}

	var bpScopes []devlake.BlueprintScope
	var repos []string
	for _, w := range resp.Scopes {
		s := w.Scope
		// Resolve scope ID: GitHub uses githubId (int), Copilot uses id (string)
		scopeID := s.ID
		if c.plugin == "github" && s.GithubID > 0 {
			scopeID = fmt.Sprintf("%d", s.GithubID)
		}
		scopeName := s.FullName
		if scopeName == "" {
			scopeName = s.Name
		}
		bpScopes = append(bpScopes, devlake.BlueprintScope{
			ScopeID:   scopeID,
			ScopeName: scopeName,
		})
		if c.plugin == "github" && s.FullName != "" {
			repos = append(repos, s.FullName)
		}
		fmt.Printf("   %s (ID: %s)\n", scopeName, scopeID)
	}
	fmt.Printf("   \u2705 Found %d scope(s)\n", len(bpScopes))

	summary := fmt.Sprintf("%s (ID: %d, %d scope(s))", pluginDisplayName(c.plugin), c.id, len(bpScopes))
	return &addedConnection{
		plugin:  c.plugin,
		connID:  c.id,
		label:   c.label,
		summary: summary,
		bpConn: devlake.BlueprintConnection{
			PluginName:   c.plugin,
			ConnectionID: c.id,
			Scopes:       bpScopes,
		},
		repos: repos,
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
	PluginNames []string // display names of active plugins
	Cron        string
	TimeAfter   string
	SkipSync    bool
	Wait        bool
	Timeout     time.Duration
}

// finalizeProject creates the project, patches the blueprint with all
// accumulated connections, triggers the first sync, and saves state.
func finalizeProject(opts finalizeProjectOpts) error {
	timeAfter := opts.TimeAfter
	if timeAfter == "" {
		timeAfter = time.Now().AddDate(0, -6, 0).Format("2006-01-02T00:00:00Z")
	}
	cron := opts.Cron
	if cron == "" {
		cron = "0 0 * * *"
	}

	fmt.Println("\n\U0001f3d7\ufe0f  Creating DevLake project...")
	blueprintID, err := ensureProjectWithFlags(opts.Client, opts.ProjectName, opts.PluginNames)
	if err != nil {
		return fmt.Errorf("failed to create project: %w", err)
	}
	fmt.Printf("   Project: %s, Blueprint ID: %d\n", opts.ProjectName, blueprintID)

	fmt.Println("\n\U0001f4cb Configuring blueprint...")
	enable := true
	patch := &devlake.BlueprintPatch{
		Enable:      &enable,
		CronConfig:  cron,
		TimeAfter:   timeAfter,
		Connections: opts.Connections,
	}
	_, err = opts.Client.PatchBlueprint(blueprintID, patch)
	if err != nil {
		return fmt.Errorf("failed to configure blueprint: %w", err)
	}
	fmt.Printf("   \u2705 Blueprint configured with %d connection(s)\n", len(opts.Connections))
	fmt.Printf("   Schedule: %s | Data since: %s\n", cron, timeAfter)

	if !opts.SkipSync {
		fmt.Println("\n\U0001f680 Triggering first data sync...")
		fmt.Println("   Depending on data volume and history, this may take 5\u201330 minutes.")
		if err := triggerAndPoll(opts.Client, blueprintID, opts.Wait, opts.Timeout); err != nil {
			fmt.Printf("   \u26a0\ufe0f  %v\n", err)
		}
	}

	// Update state file
	opts.State.Project = &devlake.StateProject{
		Name:         opts.ProjectName,
		BlueprintID:  blueprintID,
		Repos:        opts.Repos,
		Organization: opts.Org,
	}
	opts.State.ScopesConfiguredAt = time.Now().Format(time.RFC3339)
	if err := devlake.SaveState(opts.StatePath, opts.State); err != nil {
		fmt.Fprintf(os.Stderr, "\u26a0\ufe0f  Could not update state file: %v\n", err)
	} else {
		fmt.Printf("\n\U0001f4be State saved to %s\n", opts.StatePath)
	}

	fmt.Println("\n" + strings.Repeat("\u2500", 50))
	fmt.Println("\u2705 Project configured successfully!")
	fmt.Printf("   Project: %s\n", opts.ProjectName)
	if len(opts.Repos) > 0 {
		fmt.Printf("   Repos:   %s\n", strings.Join(opts.Repos, ", "))
	}
	for _, pn := range opts.PluginNames {
		fmt.Printf("   Plugin:  %s\n", pn)
	}
	fmt.Println(strings.Repeat("\u2500", 50))

	return nil
}

// ensureProjectWithFlags creates a project or returns an existing one's blueprint ID.
func ensureProjectWithFlags(client *devlake.Client, name string, pluginNames []string) (int, error) {
	desc := fmt.Sprintf("DevLake metrics for %s", name)
	if len(pluginNames) > 0 {
		desc = fmt.Sprintf("DevLake metrics for %s (%s)", name, strings.Join(pluginNames, ", "))
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
func triggerAndPoll(client *devlake.Client, blueprintID int, wait bool, timeout time.Duration) error {
	pipeline, err := client.TriggerBlueprint(blueprintID)
	if err != nil {
		return fmt.Errorf("could not trigger sync: %w", err)
	}
	fmt.Printf("   Pipeline started (ID: %d)\n", pipeline.ID)

	if !wait {
		return nil
	}
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	fmt.Println("   Monitoring progress...")
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		p, err := client.GetPipeline(pipeline.ID)
		if err != nil {
			elapsed := time.Since(deadline.Add(-timeout)).Truncate(time.Second)
			fmt.Printf("   [%s] Could not check status...\n", elapsed)
		} else {
			elapsed := time.Since(deadline.Add(-timeout)).Truncate(time.Second)
			fmt.Printf("   [%s] Status: %s | Tasks: %d/%d\n", elapsed, p.Status, p.FinishedTasks, p.TotalTasks)

			switch p.Status {
			case "TASK_COMPLETED":
				fmt.Println("\n   \u2705 Data sync completed!")
				return nil
			case "TASK_FAILED":
				return fmt.Errorf("pipeline failed \u2014 check DevLake logs")
			}
		}

		if time.Now().After(deadline) {
			fmt.Println("\n   \u231b Monitoring timed out. Pipeline is still running.")
			fmt.Printf("   Check status: GET /pipelines/%d\n", pipeline.ID)
			return nil
		}
	}
	return nil
}

// marshalJSON is available for debug output.
func marshalJSON(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

// removeChoice returns choices with the specified entry removed.
func removeChoice(choices []connChoice, remove connChoice) []connChoice {
	var out []connChoice
	for _, c := range choices {
		if c.plugin == remove.plugin && c.id == remove.id {
			continue
		}
		out = append(out, c)
	}
	return out
}

// discoverConnections finds all available connections from state and API.
func discoverConnections(client *devlake.Client, state *devlake.State) []connChoice {
	seen := make(map[string]bool)
	var choices []connChoice
	if state != nil {
		for _, c := range state.Connections {
			key := fmt.Sprintf("%s:%d", c.Plugin, c.ConnectionID)
			seen[key] = true
			label := fmt.Sprintf("%s (ID: %d, Name: %q)", pluginDisplayName(c.Plugin), c.ConnectionID, c.Name)
			choices = append(choices, connChoice{plugin: c.Plugin, id: c.ConnectionID, label: label, enterprise: c.Enterprise})
		}
	}
	for _, def := range AvailableConnections() {
		conns, err := client.ListConnections(def.Plugin)
		if err != nil {
			continue
		}
		for _, c := range conns {
			key := fmt.Sprintf("%s:%d", def.Plugin, c.ID)
			if seen[key] {
				continue
			}
			seen[key] = true
			label := fmt.Sprintf("%s (ID: %d, Name: %q)", def.DisplayName, c.ID, c.Name)
			choices = append(choices, connChoice{plugin: def.Plugin, id: c.ID, label: label, enterprise: c.Enterprise})
		}
	}
	return choices
}

// filterChoicesByPlugin returns only the connections matching the given plugin slug.
func filterChoicesByPlugin(choices []connChoice, plugin string) []connChoice {
	var out []connChoice
	for _, c := range choices {
		if c.plugin == plugin {
			out = append(out, c)
		}
	}
	return out
}

// pluginDisplayName returns a friendly name for a plugin slug, sourced from the registry.
func pluginDisplayName(plugin string) string {
	if def := FindConnectionDef(plugin); def != nil {
		return def.DisplayName
	}
	return plugin
}
