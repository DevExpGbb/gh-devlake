package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/DevExpGBB/gh-devlake/internal/devlake"
	"github.com/DevExpGBB/gh-devlake/internal/prompt"
	"github.com/spf13/cobra"
)

func newConfigureProjectsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "projects",
		Short: "Create a DevLake project and start data collection",
		Long: `Creates a DevLake project that groups data from your connections.

A project ties together your GitHub repos and Copilot organization into a
single view with DORA metrics. It automatically creates a sync schedule
(blueprint) that collects data on a cron schedule (daily by default).

This command will:
  1. Walk you through adding connections one-by-one
  2. Scope each connection (pick repos for GitHub, org for Copilot)
  3. Create the project with DORA metrics enabled
  4. Configure a sync blueprint with all selected connections
  5. Trigger the first data collection

Example:
  gh devlake configure projects --org my-org --repos my-org/app1,my-org/app2
  gh devlake configure projects --org my-org  # interactive repo selection`,
		RunE: runConfigureProjects,
	}

	cmd.Flags().StringVar(&scopeOrg, "org", "", "GitHub organization slug")
	cmd.Flags().StringVar(&scopeEnterprise, "enterprise", "", "GitHub enterprise slug (enables enterprise-level Copilot metrics)")
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

// connChoice represents a discovered connection for the interactive picker.
type connChoice struct {
	plugin     string
	id         int
	label      string
	enterprise string // enterprise slug from state/API, if available
}

// addedConnection tracks a connection that has been scoped and is ready
// for inclusion in the final blueprint.
type addedConnection struct {
	plugin  string
	connID  int
	label   string
	summary string // short summary shown in "Added so far" list
	bpConn  devlake.BlueprintConnection
	repos   []string // only populated for GitHub connections
}

func runConfigureProjects(cmd *cobra.Command, args []string) error {
	fmt.Println()
	fmt.Println("════════════════════════════════════════")
	fmt.Println("  DevLake — Project Setup")
	fmt.Println("════════════════════════════════════════")
	fmt.Println()
	fmt.Println("   A DevLake project groups data from multiple connections into a")
	fmt.Println("   single view with DORA metrics. Think of it as one project per")
	fmt.Println("   team or business unit.")

	// ── Discover DevLake ──
	fmt.Println("\n🔍 Discovering DevLake instance...")
	disc, err := devlake.Discover(cfgURL)
	if err != nil {
		return err
	}
	fmt.Printf("   Found DevLake at %s (via %s)\n", disc.URL, disc.Source)

	client := devlake.NewClient(disc.URL)
	statePath, state := devlake.FindStateFile(disc.URL, disc.GrafanaURL)

	// ── Resolve organization ──
	org := resolveOrg(state, scopeOrg)
	if org == "" {
		return fmt.Errorf("organization is required (use --org)")
	}
	fmt.Printf("   Organization: %s\n", org)

	// ── Resolve enterprise ──
	enterprise := resolveEnterprise(state, scopeEnterprise)
	if enterprise != "" {
		fmt.Printf("   Enterprise: %s\n", enterprise)
	}

	// ── Project name ──
	if scopeProjectName == "" {
		def := org
		custom := prompt.ReadLine(fmt.Sprintf("\nProject name [%s]", def))
		if custom != "" {
			scopeProjectName = custom
		} else {
			scopeProjectName = def
		}
	}

	// ── Non-interactive fast path: flags provide connection IDs ──
	if scopeGHConnID > 0 || scopeCopilotConnID > 0 {
		return runConfigureScopes(cmd, args)
	}

	// ── Discover connections ──
	fmt.Println("\n🔍 Discovering connections...")
	choices := discoverConnections(client, state)
	if len(choices) == 0 {
		return fmt.Errorf("no connections found — run 'gh devlake configure connections' first")
	}

	// ── Iterative connection addition loop ──
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

		// Auto-add if only one connection exists and nothing added yet
		var picked connChoice
		if len(remaining) == 1 && len(added) == 0 {
			picked = remaining[0]
			fmt.Printf("\n📡 One connection available — adding %s automatically.\n", picked.label)
		} else {
			// Show what's been added so far
			if len(added) > 0 {
				fmt.Println()
				fmt.Println("   " + strings.Repeat("─", 44))
				fmt.Println("   Added so far:")
				for _, a := range added {
					fmt.Printf("     ✅ %s\n", a.summary)
				}
				fmt.Println("   " + strings.Repeat("─", 44))
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
		}

		// Scope the picked connection
		ac, err := scopeConnection(client, picked, org, enterprise)
		if err != nil {
			fmt.Printf("   ⚠️  Could not scope %s: %v\n", picked.label, err)
			// Remove from remaining so user doesn't loop on a failing connection
			remaining = removeChoice(remaining, picked)
			continue
		}
		added = append(added, *ac)

		// Remove from remaining
		remaining = removeChoice(remaining, picked)

		// If no connections left, done
		if len(remaining) == 0 {
			fmt.Println("\n   All available connections have been added.")
			break
		}

		// Ask whether to add another
		if !prompt.Confirm("\nWould you like to add another connection?") {
			break
		}
	}

	if len(added) == 0 {
		return fmt.Errorf("at least one connection is required")
	}

	// ── Accumulate results ──
	var connections []devlake.BlueprintConnection
	var allRepos []string
	hasGitHub := false
	hasCopilot := false
	for _, a := range added {
		connections = append(connections, a.bpConn)
		allRepos = append(allRepos, a.repos...)
		switch a.plugin {
		case "github":
			hasGitHub = true
		case "gh-copilot":
			hasCopilot = true
		}
	}

	// ── Show what will happen ──
	fmt.Println()
	fmt.Println("   Ready to finalize:")
	for _, a := range added {
		fmt.Printf("     • %s\n", a.summary)
	}
	fmt.Println("     • Create project with DORA metrics")
	fmt.Println("     • Configure daily sync schedule")
	if !scopeSkipSync {
		fmt.Println("     • Trigger the first data collection")
	}

	// ── Finalize ──
	return finalizeProject(finalizeProjectOpts{
		Client:      client,
		StatePath:   statePath,
		State:       state,
		ProjectName: scopeProjectName,
		Org:         org,
		Connections: connections,
		Repos:       allRepos,
		HasGitHub:   hasGitHub,
		HasCopilot:  hasCopilot,
	})
}

// scopeConnection scopes a single connection (GitHub repos or Copilot org)
// and returns an addedConnection with the BlueprintConnection entry.
func scopeConnection(client *devlake.Client, c connChoice, org, enterprise string) (*addedConnection, error) {
	switch c.plugin {
	case "github":
		result, err := scopeGitHub(client, c.id, org)
		if err != nil {
			return nil, err
		}
		repoCount := len(result.Repos)
		summary := fmt.Sprintf("GitHub (ID: %d, %d repo(s))", c.id, repoCount)
		return &addedConnection{
			plugin:  c.plugin,
			connID:  c.id,
			label:   c.label,
			summary: summary,
			bpConn:  result.Connection,
			repos:   result.Repos,
		}, nil

	case "gh-copilot":
		// Prefer enterprise from the connection's own state, fall back to resolved value
		ent := enterprise
		if c.enterprise != "" {
			ent = c.enterprise
		}
		conn, err := scopeCopilot(client, c.id, org, ent)
		if err != nil {
			return nil, err
		}
		scopeID := copilotScopeID(org, ent)
		summary := fmt.Sprintf("GitHub Copilot (ID: %d, scope: %s)", c.id, scopeID)
		return &addedConnection{
			plugin:  c.plugin,
			connID:  c.id,
			label:   c.label,
			summary: summary,
			bpConn:  *conn,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported plugin %q", c.plugin)
	}
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
	seen := make(map[string]bool) // key: "plugin:id"
	var choices []connChoice

	// From state file first
	if state != nil {
		for _, c := range state.Connections {
			key := fmt.Sprintf("%s:%d", c.Plugin, c.ConnectionID)
			seen[key] = true
			label := fmt.Sprintf("%s (ID: %d, Name: %q)", pluginDisplayName(c.Plugin), c.ConnectionID, c.Name)
			choices = append(choices, connChoice{plugin: c.Plugin, id: c.ConnectionID, label: label, enterprise: c.Enterprise})
		}
	}

	// Also check API for connections not in state
	for _, plugin := range []string{"github", "gh-copilot"} {
		conns, err := client.ListConnections(plugin)
		if err != nil {
			continue
		}
		for _, c := range conns {
			key := fmt.Sprintf("%s:%d", plugin, c.ID)
			if seen[key] {
				continue
			}
			seen[key] = true
			label := fmt.Sprintf("%s (ID: %d, Name: %q)", pluginDisplayName(plugin), c.ID, c.Name)
			choices = append(choices, connChoice{plugin: plugin, id: c.ID, label: label, enterprise: c.Enterprise})
		}
	}
	return choices
}

// pluginDisplayName returns a friendly name for a plugin slug.
func pluginDisplayName(plugin string) string {
	switch plugin {
	case "github":
		return "GitHub"
	case "gh-copilot":
		return "GitHub Copilot"
	default:
		return plugin
	}
}
