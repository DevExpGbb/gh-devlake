package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/DevExpGBB/gh-devlake/internal/devlake"
	"github.com/DevExpGBB/gh-devlake/internal/prompt"
)

// ── Discovery ────────────────────────────────────────────────────

// discoverClient discovers the DevLake instance and returns a ready client.
func discoverClient(cfgURL string) (*devlake.Client, *devlake.DiscoveryResult, error) {
	fmt.Println("\n🔍 Discovering DevLake instance...")
	disc, err := devlake.Discover(cfgURL)
	if err != nil {
		return nil, nil, err
	}
	fmt.Printf("   Backend API: %s (via %s)\n", disc.URL, disc.Source)
	if disc.ConfigUIURL != "" {
		fmt.Printf("   Config UI:   %s\n", disc.ConfigUIURL)
	}
	if disc.GrafanaURL != "" {
		fmt.Printf("   Grafana:    %s\n", disc.GrafanaURL)
	}
	return devlake.NewClient(disc.URL), disc, nil
}

// ── Plugin validation ────────────────────────────────────────────

// requirePlugin validates a plugin slug and returns its definition.
func requirePlugin(slug string) (*ConnectionDef, error) {
	def := FindConnectionDef(slug)
	if def == nil || !def.Available {
		slugs := availablePluginSlugs()
		return nil, fmt.Errorf("unknown plugin %q — choose: %s", slug, strings.Join(slugs, ", "))
	}
	return def, nil
}

// ── Banner ───────────────────────────────────────────────────────

// printBanner prints a Unicode box-drawing header banner.
func printBanner(title string) {
	fmt.Println()
	fmt.Println("════════════════════════════════════════")
	fmt.Printf("  %s\n", title)
	fmt.Println("════════════════════════════════════════")
}

// printPhaseBanner prints a bordered phase header for multi-step wizards.
func printPhaseBanner(title string) {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Printf("║  %-37s║\n", title)
	fmt.Println("╚══════════════════════════════════════╝")
	fmt.Println()
}

// printJSON marshals v to compact JSON and writes it to stdout followed by a newline.
func printJSON(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}
	_, err = fmt.Fprintf(os.Stdout, "%s\n", data)
	return err
}

// ── Interactive connection picker ────────────────────────────────

// pickedConnection holds the result of an interactive connection selection.
type pickedConnection struct {
	Plugin string
	ID     int
	Name   string
	Conn   devlake.Connection
}

// pickConnection lists all connections across available plugins and lets the user pick one.
func pickConnection(client *devlake.Client, promptLabel string) (*pickedConnection, error) {
	fmt.Println("\n📋 Fetching connections...")

	type entry struct {
		plugin string
		conn   devlake.Connection
		label  string
	}
	var entries []entry

	for _, def := range AvailableConnections() {
		conns, err := client.ListConnections(def.Plugin)
		if err != nil {
			fmt.Printf("\n⚠️  Could not list %s connections: %v\n", def.DisplayName, err)
			continue
		}
		for _, c := range conns {
			label := fmt.Sprintf("[%s] ID=%d  %s", def.Plugin, c.ID, c.Name)
			entries = append(entries, entry{plugin: def.Plugin, conn: c, label: label})
		}
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("no connections found — create one with 'gh devlake configure connection'")
	}

	labels := make([]string, len(entries))
	for i, e := range entries {
		labels[i] = e.label
	}

	fmt.Println()
	chosen := prompt.Select(promptLabel, labels)
	if chosen == "" {
		return nil, fmt.Errorf("connection selection is required")
	}

	for _, e := range entries {
		if e.label == chosen {
			return &pickedConnection{
				Plugin: e.plugin,
				ID:     e.conn.ID,
				Name:   e.conn.Name,
				Conn:   e.conn,
			}, nil
		}
	}
	return nil, fmt.Errorf("invalid selection")
}

// ── Display name ─────────────────────────────────────────────────

// pluginDisplayName returns a friendly name for a plugin slug, sourced from the registry.
func pluginDisplayName(plugin string) string {
	if def := FindConnectionDef(plugin); def != nil {
		return def.DisplayName
	}
	return plugin
}

// deduplicateResults removes duplicate (Plugin, ConnectionID) pairs,
// keeping the first occurrence. Multiple connections of the same plugin
// with different IDs are preserved.
func deduplicateResults(results []ConnSetupResult) []ConnSetupResult {
	type key struct {
		plugin string
		id     int
	}
	seen := make(map[key]bool)
	var out []ConnSetupResult
	for _, r := range results {
		k := key{r.Plugin, r.ConnectionID}
		if !seen[k] {
			seen[k] = true
			out = append(out, r)
		}
	}
	return out
}

// ── Health polling ───────────────────────────────────────────────

// waitForReady polls the DevLake /ping endpoint until it responds 200 or
// maxAttempts is exhausted. interval is the pause between attempts.
func waitForReady(baseURL string, maxAttempts int, interval time.Duration) error {
	httpClient := &http.Client{Timeout: 5 * time.Second}
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		resp, err := httpClient.Get(baseURL + "/ping")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				fmt.Println("   ✅ DevLake is responding!")
				return nil
			}
		}
		fmt.Printf("   Attempt %d/%d — waiting...\n", attempt, maxAttempts)
		time.Sleep(interval)
	}
	return fmt.Errorf("DevLake not ready after %d attempts — check logs", maxAttempts)
}

// waitForMigration polls until DevLake finishes database migration.
// During migration the API returns 428 (Precondition Required).
func waitForMigration(baseURL string, maxAttempts int, interval time.Duration) error {
	httpClient := &http.Client{Timeout: 5 * time.Second}
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		resp, err := httpClient.Get(baseURL + "/ping")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				fmt.Println("   ✅ Migration complete!")
				return nil
			}
		}
		fmt.Printf("   Migrating... (%d/%d)\n", attempt, maxAttempts)
		time.Sleep(interval)
	}
	return fmt.Errorf("migration did not complete after %d attempts", maxAttempts)
}

// ── Scope orchestration ─────────────────────────────────────────

// scopeAllConnections iterates connection results and configures scopes
// for each, prompting for DORA patterns on GitHub connections.
func scopeAllConnections(client *devlake.Client, results []ConnSetupResult) {
	for _, r := range results {
		fmt.Printf("\n📡 Configuring scopes for %s (connection %d)...\n",
			pluginDisplayName(r.Plugin), r.ConnectionID)

		switch r.Plugin {
		case "github":
			scopeOpts := &ScopeOpts{
				DeployPattern: "(?i)deploy",
				ProdPattern:   "(?i)prod",
				IncidentLabel: "incident",
			}

			fmt.Println("   Default DORA patterns:")
			fmt.Printf("     Deployment: %s\n", scopeOpts.DeployPattern)
			fmt.Printf("     Production: %s\n", scopeOpts.ProdPattern)
			fmt.Printf("     Incidents:  label=%s\n", scopeOpts.IncidentLabel)
			fmt.Println()
			if !prompt.Confirm("   Use these defaults?") {
				v := prompt.ReadLine("   Deployment workflow regex")
				if v != "" {
					scopeOpts.DeployPattern = v
				}
				v = prompt.ReadLine("   Production environment regex")
				if v != "" {
					scopeOpts.ProdPattern = v
				}
				v = prompt.ReadLine("   Incident issue label")
				if v != "" {
					scopeOpts.IncidentLabel = v
				}
			}

			_, err := scopeGitHub(client, r.ConnectionID, r.Organization, scopeOpts)
			if err != nil {
				fmt.Printf("   ⚠️  GitHub scope setup failed: %v\n", err)
			}

		case "gh-copilot":
			_, err := scopeCopilot(client, r.ConnectionID, r.Organization, r.Enterprise)
			if err != nil {
				fmt.Printf("   ⚠️  Copilot scope setup failed: %v\n", err)
			}

		default:
			fmt.Printf("   ⚠️  Scope configuration for %q is not yet supported\n", r.Plugin)
		}
	}
}

// ── Project collection + finalization ────────────────────────────

// collectProjectOpts holds parameters for collectAndFinalizeProject.
type collectProjectOpts struct {
	Client      *devlake.Client
	Results     []ConnSetupResult
	StatePath   string
	State       *devlake.State
	Org         string
	ProjectName string
	Wait        bool
	Timeout     time.Duration
}

// collectAndFinalizeProject gathers scoped connections from results,
// prompts for a project name, and calls finalizeProject.
func collectAndFinalizeProject(opts collectProjectOpts) error {
	defaultProject := opts.Org
	if defaultProject == "" {
		defaultProject = "my-project"
	}
	projectName := opts.ProjectName
	if projectName == "" {
		projectName = prompt.ReadLine(fmt.Sprintf("\nProject name [%s]", defaultProject))
		if projectName == "" {
			projectName = defaultProject
		}
	}

	var connections []devlake.BlueprintConnection
	var allRepos []string
	var pluginNames []string

	for _, r := range opts.Results {
		choice := connChoice{
			plugin:     r.Plugin,
			id:         r.ConnectionID,
			label:      fmt.Sprintf("%s (ID: %d)", pluginDisplayName(r.Plugin), r.ConnectionID),
			enterprise: r.Enterprise,
		}
		ac, err := listConnectionScopes(opts.Client, choice)
		if err != nil {
			fmt.Printf("   ⚠️  Could not list scopes for %s: %v\n", choice.label, err)
			continue
		}
		connections = append(connections, ac.bpConn)
		allRepos = append(allRepos, ac.repos...)
		pluginNames = append(pluginNames, pluginDisplayName(r.Plugin))
	}

	if len(connections) == 0 {
		return fmt.Errorf("no scoped connections available — cannot create project")
	}

	return finalizeProject(finalizeProjectOpts{
		Client:      opts.Client,
		StatePath:   opts.StatePath,
		State:       opts.State,
		ProjectName: projectName,
		Org:         opts.Org,
		Connections: connections,
		Repos:       allRepos,
		PluginNames: pluginNames,
		Cron:        "0 0 * * *",
		Wait:        opts.Wait,
		Timeout:     opts.Timeout,
	})
}

// ── Connection discovery (moved from configure_projects.go) ─────

// ConfigureAllOpts holds parameters for the shared configureAllPhases orchestrator.
type ConfigureAllOpts struct {
	Token     string
	EnvFile   string
	SkipClean bool
	ReAddLoop bool // true for init wizard (re-prompt), false for configure full (one-shot)
}

// configureAllPhases runs the connection → scope → project pipeline.
// It is the single implementation used by both init (Phase 2-4) and configure full.
func configureAllPhases(opts ConfigureAllOpts) error {
	available := AvailableConnections()
	var results []ConnSetupResult
	var client *devlake.Client
	var statePath string
	var state *devlake.State

	if opts.ReAddLoop {
		// Interactive re-add loop (init wizard)
		for {
			remaining := available
			if len(results) > 0 {
				fmt.Println()
				fmt.Println("   " + strings.Repeat("─", 44))
				fmt.Println("   Connections configured so far:")
				for _, r := range results {
					name := r.Plugin
					if def := FindConnectionDef(r.Plugin); def != nil {
						name = def.DisplayName
					}
					fmt.Printf("     ✅ %-18s  ID=%d  %q\n", name, r.ConnectionID, r.Name)
				}
				fmt.Println("   " + strings.Repeat("─", 44))
			}

			var remainingLabels []string
			for _, d := range remaining {
				remainingLabels = append(remainingLabels, d.DisplayName)
			}

			fmt.Println()
			selectedLabels := prompt.SelectMulti("Which connections to set up?", remainingLabels)
			if len(selectedLabels) == 0 {
				if len(results) == 0 {
					return fmt.Errorf("at least one connection is required")
				}
				break
			}

			var selectedDefs []*ConnectionDef
			for _, label := range selectedLabels {
				for _, d := range remaining {
					if d.DisplayName == label {
						selectedDefs = append(selectedDefs, d)
						break
					}
				}
			}
			if len(selectedDefs) == 0 {
				if len(results) == 0 {
					return fmt.Errorf("at least one connection is required")
				}
				break
			}

			newResults, c, sp, st, err := runConnectionsInternal(selectedDefs, "", "", opts.Token, opts.EnvFile, opts.SkipClean)
			if err != nil {
				if len(results) == 0 {
					return fmt.Errorf("connection setup failed: %w", err)
				}
				fmt.Printf("\n   ⚠️  %v\n", err)
				if !prompt.Confirm("\nWould you like to try another connection?") {
					break
				}
				continue
			}
			client = c
			statePath = sp
			state = st
			for _, r := range newResults {
				dupe := false
				for _, existing := range results {
					if existing.Plugin == r.Plugin && existing.ConnectionID == r.ConnectionID {
						dupe = true
						break
					}
				}
				if !dupe {
					results = append(results, r)
				}
			}
			fmt.Println("\n   ✅ Connections configured.")

			if !prompt.Confirm("\nWould you like to add another connection?") {
				break
			}
		}
	} else {
		// One-shot selection (configure full)
		var labels []string
		for _, d := range available {
			labels = append(labels, d.DisplayName)
		}
		fmt.Println()
		selectedLabels := prompt.SelectMulti("Which connections to configure?", labels)
		var defs []*ConnectionDef
		for _, label := range selectedLabels {
			for _, d := range available {
				if d.DisplayName == label {
					defs = append(defs, d)
					break
				}
			}
		}
		if len(defs) == 0 {
			return fmt.Errorf("at least one connection is required")
		}

		var err error
		results, client, statePath, state, err = runConnectionsInternal(defs, "", "", opts.Token, opts.EnvFile, opts.SkipClean)
		if err != nil {
			return fmt.Errorf("connection setup failed: %w", err)
		}
		if len(results) == 0 {
			return fmt.Errorf("no connections were created — cannot continue")
		}
	}

	if client == nil {
		// Re-discover if connections loop didn't run (shouldn't happen)
		var err error
		client, _, err = discoverClient(cfgURL)
		if err != nil {
			return err
		}
	}

	// Resolve org from connection results
	org := ""
	for _, r := range results {
		if r.Organization != "" {
			org = r.Organization
			break
		}
	}

	// ── Scopes ──
	printPhaseBanner("Configure Scopes")
	results = deduplicateResults(results)
	scopeAllConnections(client, results)
	fmt.Println("\n   ✅ Scopes configured.")

	// ── Project ──
	printPhaseBanner("Project Setup")
	err := collectAndFinalizeProject(collectProjectOpts{
		Client:    client,
		Results:   results,
		StatePath: statePath,
		State:     state,
		Org:       org,
		Wait:      true,
		Timeout:   5 * time.Minute,
	})
	if err != nil {
		return fmt.Errorf("project setup failed: %w", err)
	}

	return nil
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
