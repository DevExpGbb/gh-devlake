package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/DevExpGBB/gh-devlake/internal/devlake"
	"github.com/DevExpGBB/gh-devlake/internal/prompt"
)

func newScopeAddCmd() *cobra.Command {
	var opts ScopeOpts
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add scopes (repos, orgs) to existing connections",
		Long: `Adds repository scopes and scope-configs to existing DevLake connections.

This command only manages scopes on connections -- it does not create projects
or trigger data syncs. To create a project after scoping, run:
  gh devlake configure project add

Shared flags (all plugins):
  --plugin             Plugin to configure (required in flag mode)
  --connection-id      Connection ID (required in flag mode)
  --org                Organization slug

GitHub-specific flags:
  --repos              Comma-separated repos (owner/repo)
  --repos-file         Path to file with repos (one per line)
  --deployment-pattern Regex to match deployment workflows
  --production-pattern Regex to match production environment
  --incident-label     Issue label for incidents

GitHub Copilot-specific flags:
  --enterprise         Enterprise slug (enables enterprise-level metrics)

Example (GitHub):
  gh devlake configure scope add --plugin github --connection-id 1 --org my-org --repos org/repo1,org/repo2

Example (Copilot):
  gh devlake configure scope add --plugin gh-copilot --connection-id 2 --org my-org --enterprise my-ent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScopeAdd(cmd, args, &opts)
		},
	}

	cmd.Flags().StringVar(&opts.Org, "org", "", "Organization slug")
	cmd.Flags().StringVar(&opts.Enterprise, "enterprise", "", "Enterprise slug (enables enterprise-level metrics)")
	cmd.Flags().StringVar(&opts.Plugin, "plugin", "", fmt.Sprintf("Plugin to configure (%s)", strings.Join(availablePluginSlugs(), ", ")))
	cmd.Flags().StringVar(&opts.Repos, "repos", "", "Comma-separated repos (owner/repo)")
	cmd.Flags().StringVar(&opts.ReposFile, "repos-file", "", "Path to file with repos (one per line)")
	cmd.Flags().StringVar(&opts.Jobs, "jobs", "", "Comma-separated Jenkins job full names")
	cmd.Flags().IntVar(&opts.ConnectionID, "connection-id", 0, "Connection ID (auto-detected if omitted)")
	cmd.Flags().StringVar(&opts.DeployPattern, "deployment-pattern", "(?i)deploy", "Regex to match deployment workflows")
	cmd.Flags().StringVar(&opts.ProdPattern, "production-pattern", "(?i)prod", "Regex to match production environment")
	cmd.Flags().StringVar(&opts.IncidentLabel, "incident-label", "incident", "Issue label for incidents")

	return cmd
}

func runScopeAdd(cmd *cobra.Command, args []string, opts *ScopeOpts) error {
	printBanner("DevLake \u2014 Configure Scopes")

	// Determine which plugin to scope
	var selectedPlugin string
	if opts.Plugin != "" {
		def, err := requirePlugin(opts.Plugin)
		if err != nil {
			return err
		}
		selectedPlugin = opts.Plugin
		// Warn about flags that don't apply to the selected plugin.
		warnIrrelevantFlags(cmd, def, collectAllScopeFlagDefs())
	} else {
		flagMode := cmd.Flags().Changed("org") ||
			cmd.Flags().Changed("repos") ||
			cmd.Flags().Changed("repos-file") ||
			cmd.Flags().Changed("jobs") ||
			cmd.Flags().Changed("connection-id")
		if flagMode {
			slugs := availablePluginSlugs()
			return fmt.Errorf("--plugin is required when using flags (choose: %s)", strings.Join(slugs, ", "))
		}
		available := AvailableConnections()
		var labels []string
		for _, d := range available {
			labels = append(labels, d.DisplayName)
		}
		fmt.Println()
		chosen := prompt.Select("Which plugin to configure?", labels)
		if chosen == "" {
			return fmt.Errorf("plugin selection is required")
		}
		for _, d := range available {
			if d.DisplayName == chosen {
				selectedPlugin = d.Plugin
				// Print applicable flags and warn about irrelevant ones after
				// interactive plugin selection.
				printContextualFlagHelp(d, d.ScopeFlags, "Scope")
				warnIrrelevantFlags(cmd, d, collectAllScopeFlagDefs())
				break
			}
		}
		if selectedPlugin == "" {
			return fmt.Errorf("plugin selection is required")
		}
	}

	client, disc, err := discoverClient(cfgURL)
	if err != nil {
		return err
	}
	_, state := devlake.FindStateFile(disc.URL, disc.GrafanaURL)

	fmt.Println("\n\U0001f517 Resolving connection...")
	connID, err := resolveConnectionID(client, state, selectedPlugin, opts.ConnectionID)
	if err != nil {
		return fmt.Errorf("no %s connection found \u2014 run 'configure connection' first: %w", pluginDisplayName(selectedPlugin), err)
	}
	fmt.Printf("   %s connection ID: %d\n", pluginDisplayName(selectedPlugin), connID)

	org := resolveOrg(state, opts.Org)
	def := FindConnectionDef(selectedPlugin)
	if def == nil || def.ScopeFunc == nil {
		return fmt.Errorf("scope configuration for %q is not yet supported", selectedPlugin)
	}
	if org == "" && def.NeedsOrg {
		return fmt.Errorf("organization is required (use --org)")
	}
	if org != "" {
		fmt.Printf("   Organization: %s\n", org)
	}

	enterprise := resolveEnterprise(state, opts.Enterprise)
	if enterprise != "" {
		fmt.Printf("   Enterprise: %s\n", enterprise)
	}

	// Dispatch to plugin-specific scope handler
	_, err = def.ScopeFunc(client, connID, org, enterprise, opts)
	if err != nil {
		return err
	}

	fmt.Println("\n" + strings.Repeat("\u2500", 40))
	fmt.Printf("\u2705 %s scopes configured successfully!\n", pluginDisplayName(selectedPlugin))
	fmt.Printf("   Connection %d: scopes added\n", connID)
	fmt.Println(strings.Repeat("\u2500", 40))
	fmt.Println("\nNext step:")
	fmt.Println("  Run 'gh devlake configure project add' to create a project and start data collection.")

	return nil
}
