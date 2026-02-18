package cmd

import (
	"fmt"
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
  1. Show your existing connections and let you choose which to include
  2. Let you pick which repos to collect data from
  3. Create the project with DORA metrics enabled
  4. Configure a sync blueprint with your connections and scopes
  5. Trigger the first data collection

Example:
  gh devlake configure projects --org my-org --repos my-org/app1,my-org/app2
  gh devlake configure projects --org my-org  # interactive repo selection`,
		RunE: runConfigureProjects,
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

// connChoice represents a discovered connection for the interactive picker.
type connChoice struct {
	plugin string
	id     int
	label  string
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

	// ── Project name ──
	if scopeProjectName == "" {
		def := scopeOrg
		if def == "" {
			def = "my-project"
		}
		custom := prompt.ReadLine(fmt.Sprintf("\nProject name [%s]", def))
		if custom != "" {
			scopeProjectName = custom
		} else {
			scopeProjectName = def
		}
	}

	// ── Discover connections ──
	// If connection IDs were passed via flags, skip the picker entirely.
	if scopeGHConnID == 0 || (!scopeSkipCopilot && scopeCopilotConnID == 0) {
		fmt.Println("\n🔍 Discovering DevLake connections...")
		disc, err := devlake.Discover(cfgURL)
		if err != nil {
			return err
		}
		client := devlake.NewClient(disc.URL)
		_, state := devlake.FindStateFile(disc.URL, disc.GrafanaURL)

		choices := discoverConnections(client, state)
		if len(choices) == 0 {
			return fmt.Errorf("no connections found — run 'gh devlake configure connections' first")
		}

		// Show connections and let user choose
		labels := make([]string, len(choices))
		defaults := make([]int, len(choices))
		for i, c := range choices {
			labels[i] = c.label
			defaults[i] = i + 1 // all selected by default
		}

		fmt.Println()
		fmt.Println("   These connections were found. Choose which to include in")
		fmt.Println("   this project (they determine what data gets collected).")
		fmt.Println()
		selected := prompt.SelectMultiWithDefaults("Connections to include", labels, defaults)

		// Map selections back to connection IDs
		hasGitHub := false
		hasCopilot := false
		for _, sel := range selected {
			for _, c := range choices {
				if c.label == sel {
					switch c.plugin {
					case "github":
						scopeGHConnID = c.id
						hasGitHub = true
					case "gh-copilot":
						scopeCopilotConnID = c.id
						hasCopilot = true
					}
				}
			}
		}
		if !hasGitHub && !hasCopilot {
			return fmt.Errorf("at least one connection is required — select GitHub, Copilot, or both")
		}
		if !hasGitHub {
			scopeSkipGitHub = true
		}
		if !hasCopilot {
			scopeSkipCopilot = true
		}
	}

	// ── Explain what happens next ──
	fmt.Println()
	fmt.Println("   Next steps:")
	if !scopeSkipGitHub {
		fmt.Println("     • Select which GitHub repos to collect data from")
		fmt.Println("       (PRs, commits, deployments → DORA metrics)")
	}
	if !scopeSkipCopilot {
		fmt.Println("     • Add Copilot usage data for your org")
	}
	fmt.Println("     • Create a daily sync schedule")
	fmt.Println("     • Trigger the first data collection")

	return runConfigureScopes(cmd, args)
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
			choices = append(choices, connChoice{plugin: c.Plugin, id: c.ConnectionID, label: label})
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
			choices = append(choices, connChoice{plugin: plugin, id: c.ID, label: label})
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
