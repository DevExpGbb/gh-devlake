package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/DevExpGBB/gh-devlake/internal/devlake"
	"github.com/DevExpGBB/gh-devlake/internal/prompt"
	"github.com/DevExpGBB/gh-devlake/internal/token"
)

var (
	connPlugin     string
	connOrg        string
	connEnterprise string
	connToken      string
	connEnvFile    string
	connSkipClean  bool
	connName       string
	connProxy      string
	connEndpoint   string
)

var addConnectionCmd = &cobra.Command{
	Use:   "add",
	Short: "Create a new plugin connection in DevLake",
	Long: `Creates a single DevLake plugin connection.

If --plugin is not specified, prompts interactively. Run multiple times to
add connections for additional plugins.

Shared flags (all plugins):
  --plugin       Plugin to configure
  --token        Personal access token
  --name         Connection display name
  --endpoint     API endpoint override
  --proxy        HTTP proxy URL
  --env-file     Path to env file containing PAT
  --skip-cleanup Do not delete .devlake.env after setup

GitHub Copilot-specific flags:
  --enterprise   Enterprise slug
  --org          Organization slug (required unless enterprise provided)

Token resolution order:
  --token flag → .devlake.env → environment variable → masked prompt

Example (GitHub):
  gh devlake configure connection add --plugin github --token ghp_xxx --org my-org

Example (Copilot):
  gh devlake configure connection add --plugin gh-copilot --token ghp_xxx --org my-org --enterprise my-ent`,
	RunE: runAddConnection,
}

func init() {
	addConnectionCmd.Flags().StringVar(&connPlugin, "plugin", "", fmt.Sprintf("Plugin to configure (%s)", strings.Join(availablePluginSlugs(), ", ")))
	addConnectionCmd.Flags().StringVar(&connOrg, "org", "", "Organization slug")
	addConnectionCmd.Flags().StringVar(&connEnterprise, "enterprise", "", "Enterprise slug")
	addConnectionCmd.Flags().StringVar(&connToken, "token", "", "Personal access token")
	addConnectionCmd.Flags().StringVar(&connEnvFile, "env-file", ".devlake.env", "Path to env file containing PAT")
	addConnectionCmd.Flags().BoolVar(&connSkipClean, "skip-cleanup", false, "Do not delete .devlake.env after setup")
	addConnectionCmd.Flags().StringVar(&connName, "name", "", "Connection display name (defaults to \"Plugin - org\")")
	addConnectionCmd.Flags().StringVar(&connProxy, "proxy", "", "HTTP proxy URL")
	addConnectionCmd.Flags().StringVar(&connEndpoint, "endpoint", "", "API endpoint override")
	configureConnectionsCmd.AddCommand(addConnectionCmd)
}

func runAddConnection(cmd *cobra.Command, args []string) error {
	printBanner("DevLake — Configure Connection")

	// ── Select plugin ──
	def, err := selectPlugin(connPlugin)
	if err != nil {
		return err
	}

	// ── Prompt for org if needed ──
	org := connOrg
	if def.NeedsOrg && org == "" {
		orgPrompt := def.OrgPrompt
		if orgPrompt == "" {
			orgPrompt = "Organization slug"
		}
		org = prompt.ReadLine(orgPrompt)
		if org == "" {
			return fmt.Errorf("--org is required for %s", def.DisplayName)
		}
	}

	// Prompt for org optionally for plugins that don't require it,
	// so it gets saved to state for downstream commands (e.g. scopes).
	if !def.NeedsOrg && org == "" {
		org = prompt.ReadLine("Organization slug (optional, press Enter to skip)")
	}

	// ── Discover DevLake ──
	client, disc, err := discoverClient(cfgURL)
	if err != nil {
		return err
	}

	// ── Resolve token ──
	fmt.Printf("\n🔑 Resolving %s PAT...\n", def.DisplayName)
	tokResult, err := token.Resolve(token.ResolveOpts{
		FlagValue:   connToken,
		EnvFilePath: connEnvFile,
		EnvFileKeys: def.EnvFileKeys,
		EnvVarNames: def.EnvVarNames,
		DisplayName: def.DisplayName,
		ScopeHint:   def.ScopeHint,
	})
	if err != nil {
		return err
	}
	fmt.Printf("   Token loaded from: %s\n", tokResult.Source)

	// ── Create connection ──
	fmt.Printf("\n📡 Creating %s connection...\n", def.DisplayName)
	params := ConnectionParams{
		Token:      tokResult.Token,
		Org:        org,
		Enterprise: connEnterprise,
		Name:       connName,
		Proxy:      connProxy,
		Endpoint:   connEndpoint,
	}
	result, err := buildAndCreateConnection(client, def, params, org, true)
	if err != nil {
		return err
	}

	// ── Update state (replace same plugin or append) ──
	statePath, state := devlake.FindStateFile(disc.URL, disc.GrafanaURL)
	newConn := devlake.StateConnection{
		Plugin:       def.Plugin,
		ConnectionID: result.ConnectionID,
		Name:         result.Name,
		Organization: org,
		Enterprise:   connEnterprise,
	}
	replaced := false
	for i, c := range state.Connections {
		if c.Plugin == def.Plugin && c.ConnectionID == newConn.ConnectionID {
			state.Connections[i] = newConn
			replaced = true
			break
		}
	}
	if !replaced {
		state.Connections = append(state.Connections, newConn)
	}
	if err := devlake.UpdateConnections(statePath, state, state.Connections); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Could not update state file: %v\n", err)
	} else {
		fmt.Printf("\n💾 State saved to %s\n", statePath)
	}

	// ── Cleanup env file ──
	if !connSkipClean && tokResult.EnvFilePath != "" {
		fmt.Printf("\n🧹 Cleaning up %s...\n", tokResult.EnvFilePath)
		if err := os.Remove(tokResult.EnvFilePath); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "⚠️  Could not delete env file: %v\n", err)
		} else {
			fmt.Println("   ✅ Env file deleted")
		}
	}

	// ── Summary ──
	fmt.Println("\n" + strings.Repeat("─", 40))
	fmt.Printf("✅ %s connection configured!\n", def.DisplayName)
	fmt.Printf("   ID=%d  %q\n", result.ConnectionID, result.Name)
	fmt.Println(strings.Repeat("─", 40))

	// ── Next step hint ──
	hintOrg := org
	if hintOrg == "" {
		hintOrg = "<org>"
	}
	fmt.Println("\nNext steps:")
	fmt.Printf("  Run 'gh devlake configure scope add --plugin %s --org %s' to add scopes\n", def.Plugin, hintOrg)
	fmt.Println("  Then run 'gh devlake configure project add' to create a project and start collecting data.")

	return nil
}

// selectPlugin resolves a ConnectionDef from a --plugin flag or interactive selection.
func selectPlugin(pluginSlug string) (*ConnectionDef, error) {
	if pluginSlug != "" {
		def := FindConnectionDef(pluginSlug)
		if def == nil {
			return nil, fmt.Errorf("unknown plugin %q", pluginSlug)
		}
		if !def.Available {
			return nil, fmt.Errorf("%s connections are coming soon", def.DisplayName)
		}
		return def, nil
	}

	available := AvailableConnections()
	var labels []string
	for _, d := range available {
		labels = append(labels, d.DisplayName)
	}

	var comingSoon []string
	for _, d := range connectionRegistry {
		if !d.Available {
			comingSoon = append(comingSoon, d.DisplayName)
		}
	}
	if len(comingSoon) > 0 {
		fmt.Fprintf(os.Stderr, "Coming soon: %s\n\n", strings.Join(comingSoon, ", "))
	}

	chosen := prompt.Select("Which plugin to configure?", labels)
	if chosen == "" {
		return nil, fmt.Errorf("plugin selection is required")
	}
	for _, d := range available {
		if d.DisplayName == chosen {
			return d, nil
		}
	}
	return nil, fmt.Errorf("invalid selection %q", chosen)
}
