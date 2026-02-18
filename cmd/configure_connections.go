package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/DevExpGBB/gh-devlake/internal/devlake"
	"github.com/DevExpGBB/gh-devlake/internal/prompt"
	"github.com/DevExpGBB/gh-devlake/internal/token"
	"github.com/spf13/cobra"
)

var (
	connPlugin     string
	connOrg        string
	connEnterprise string
	connToken      string
	connEnvFile    string
	connSkipClean  bool
)

var configureConnectionsCmd = &cobra.Command{
	Use:   "connections",
	Short: "Create a plugin connection in DevLake",
	Long: `Creates a single DevLake plugin connection.

If --plugin is not specified, prompts interactively. Run multiple times to
add connections for additional plugins.

Available plugins:  github, gh-copilot
Coming soon:        gitlab, azure-devops

Token resolution order:
  --token flag → .devlake.env → $GITHUB_TOKEN/$GH_TOKEN → masked prompt`,
	RunE: runConfigureConnections,
}

func init() {
	configureConnectionsCmd.Flags().StringVar(&connPlugin, "plugin", "", "Plugin to configure (github, gh-copilot)")
	configureConnectionsCmd.Flags().StringVar(&connOrg, "org", "", "GitHub organization slug")
	configureConnectionsCmd.Flags().StringVar(&connEnterprise, "enterprise", "", "GitHub enterprise slug")
	configureConnectionsCmd.Flags().StringVar(&connToken, "token", "", "GitHub PAT")
	configureConnectionsCmd.Flags().StringVar(&connEnvFile, "env-file", ".devlake.env", "Path to env file containing GITHUB_PAT")
	configureConnectionsCmd.Flags().BoolVar(&connSkipClean, "skip-cleanup", false, "Do not delete .devlake.env after setup")
}

func runConfigureConnections(cmd *cobra.Command, args []string) error {
	// ── Select plugin ──
	def, err := selectPlugin(connPlugin)
	if err != nil {
		return err
	}

	// ── Prompt for org if needed ──
	org := connOrg
	if def.NeedsOrg && org == "" {
		org = prompt.ReadLine("GitHub organization slug")
		if org == "" {
			return fmt.Errorf("--org is required for %s", def.DisplayName)
		}
	}

	// ── Discover DevLake ──
	fmt.Println("🔍 Discovering DevLake instance...")
	disc, err := devlake.Discover(cfgURL)
	if err != nil {
		return err
	}
	fmt.Printf("   Found DevLake at %s (via %s)\n", disc.URL, disc.Source)

	client := devlake.NewClient(disc.URL)

	// ── Resolve token ──
	fmt.Println("\n🔑 Resolving PAT...")
	tokResult, err := token.Resolve(connToken, connEnvFile)
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
	}
	result, err := buildAndCreateConnection(client, def, params, org)
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
		if c.Plugin == def.Plugin {
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
	fmt.Println("\n" + strings.Repeat("─", 50))
	fmt.Printf("✅ %s connection configured!\n", def.DisplayName)
	fmt.Printf("   ID=%d  %q\n", result.ConnectionID, result.Name)
	fmt.Println(strings.Repeat("─", 50))

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
