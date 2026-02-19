package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/DevExpGBB/gh-devlake/internal/devlake"
	"github.com/DevExpGBB/gh-devlake/internal/prompt"
	"github.com/spf13/cobra"
)

var (
	updateConnPlugin     string
	updateConnID         int
	updateConnToken      string
	updateConnOrg        string
	updateConnEnterprise string
	updateConnName       string
	updateConnEndpoint   string
	updateConnProxy      string
)

var updateConnectionCmd = &cobra.Command{
	Use:   "update",
	Short: "Update an existing plugin connection",
	Long: `Updates an existing DevLake plugin connection in-place.

Use this command for token rotation, endpoint changes, or org/enterprise updates
without recreating the connection (which would lose scope configs and blueprints).

Flag-based (non-interactive):
  gh devlake configure connection update --plugin github --id 1 --token ghp_newtoken
  gh devlake configure connection update --plugin gh-copilot --id 2 --org new-org

Interactive (no flags required):
  gh devlake configure connection update

In interactive mode, current values are shown as defaults. Press Enter to keep.`,
	RunE: runUpdateConnection,
}

func init() {
	updateConnectionCmd.Flags().StringVar(&updateConnPlugin, "plugin", "", "Plugin slug (github, gh-copilot)")
	updateConnectionCmd.Flags().IntVar(&updateConnID, "id", 0, "Connection ID to update")
	updateConnectionCmd.Flags().StringVar(&updateConnToken, "token", "", "New GitHub PAT for token rotation")
	updateConnectionCmd.Flags().StringVar(&updateConnOrg, "org", "", "GitHub organization slug")
	updateConnectionCmd.Flags().StringVar(&updateConnEnterprise, "enterprise", "", "GitHub enterprise slug")
	updateConnectionCmd.Flags().StringVar(&updateConnName, "name", "", "Connection display name")
	updateConnectionCmd.Flags().StringVar(&updateConnEndpoint, "endpoint", "", "API endpoint URL")
	updateConnectionCmd.Flags().StringVar(&updateConnProxy, "proxy", "", "HTTP proxy URL")
	configureConnectionsCmd.AddCommand(updateConnectionCmd)
}

func runUpdateConnection(cmd *cobra.Command, args []string) error {
	fmt.Println()
	fmt.Println("════════════════════════════════════════")
	fmt.Println("  DevLake — Update Connection")
	fmt.Println("════════════════════════════════════════")

	flagMode := updateConnPlugin != "" || updateConnID != 0

	// ── Validate flags before making any network calls ──
	if flagMode {
		if updateConnPlugin == "" || updateConnID == 0 {
			return fmt.Errorf("--plugin and --id are both required in flag-based mode")
		}

		def := FindConnectionDef(updateConnPlugin)
		if def == nil || !def.Available {
			slugs := availablePluginSlugs()
			return fmt.Errorf("unknown plugin %q — choose: %s", updateConnPlugin, strings.Join(slugs, ", "))
		}
	}

	// ── Discover DevLake ──
	fmt.Println("\n🔍 Discovering DevLake instance...")
	disc, err := devlake.Discover(cfgURL)
	if err != nil {
		return err
	}
	fmt.Printf("   Found DevLake at %s (via %s)\n", disc.URL, disc.Source)

	client := devlake.NewClient(disc.URL)

	var plugin string
	var connID int

	if flagMode {
		plugin = updateConnPlugin
		connID = updateConnID
	} else {
		// ── Interactive: list all connections, let user pick ──
		var entries []struct {
			plugin string
			conn   devlake.Connection
		}
		var labels []string

		for _, def := range AvailableConnections() {
			conns, err := client.ListConnections(def.Plugin)
			if err != nil {
				fmt.Printf("\n⚠️  Could not list %s connections: %v\n", def.DisplayName, err)
				continue
			}
			for _, c := range conns {
				entries = append(entries, struct {
					plugin string
					conn   devlake.Connection
				}{plugin: def.Plugin, conn: c})
				label := fmt.Sprintf("%s / %d / %s", def.Plugin, c.ID, c.Name)
				labels = append(labels, label)
			}
		}

		if len(entries) == 0 {
			return fmt.Errorf("no connections found — create one with 'gh devlake configure connection'")
		}

		fmt.Println()
		chosen := prompt.Select("Select connection to update", labels)
		if chosen == "" {
			return fmt.Errorf("connection selection is required")
		}
		for i, label := range labels {
			if label == chosen {
				plugin = entries[i].plugin
				connID = entries[i].conn.ID
				break
			}
		}
	}

	// ── GET current connection ──
	fmt.Printf("\n🔍 Fetching connection %s/%d...\n", plugin, connID)
	current, err := client.GetConnection(plugin, connID)
	if err != nil {
		return fmt.Errorf("could not retrieve connection: %w", err)
	}

	def := FindConnectionDef(plugin)

	// ── Build PATCH request ──
	var req *devlake.ConnectionUpdateRequest
	if flagMode {
		req = buildUpdateRequestFromFlags(cmd, current)
	} else {
		req, err = buildUpdateRequestInteractive(current, def)
		if err != nil {
			return err
		}
	}

	// ── Show current values before update ──
	fmt.Printf("\n📋 Current values for %q (ID=%d):\n", current.Name, current.ID)
	fmt.Printf("   Name:       %s\n", current.Name)
	fmt.Printf("   Endpoint:   %s\n", current.Endpoint)
	if current.Organization != "" {
		fmt.Printf("   Org:        %s\n", current.Organization)
	}
	if current.Enterprise != "" {
		fmt.Printf("   Enterprise: %s\n", current.Enterprise)
	}
	if current.Token != "" {
		fmt.Printf("   Token:      %s\n", maskToken(current.Token))
	}

	// ── Apply PATCH ──
	fmt.Printf("\n📡 Updating %s connection (ID=%d)...\n", plugin, connID)
	updated, err := client.UpdateConnection(plugin, connID, req)
	if err != nil {
		return fmt.Errorf("failed to update connection: %w", err)
	}
	fmt.Printf("   ✅ Updated (ID=%d) %q\n", updated.ID, updated.Name)

	// ── Test updated connection ──
	fmt.Println("\n🔌 Testing updated connection...")
	testResult, err := client.TestSavedConnection(plugin, updated.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Connection test failed: %v\n", err)
	} else if !testResult.Success {
		fmt.Fprintf(os.Stderr, "⚠️  Connection test failed: %s\n", testResult.Message)
	} else {
		fmt.Println("   ✅ Connection test passed")
	}

	// ── Update state file ──
	statePath, state := devlake.FindStateFile(disc.URL, disc.GrafanaURL)
	for i, c := range state.Connections {
		if c.Plugin == plugin && c.ConnectionID == updated.ID {
			state.Connections[i].Name = updated.Name
			state.Connections[i].Organization = updated.Organization
			state.Connections[i].Enterprise = updated.Enterprise
			break
		}
	}
	if err := devlake.UpdateConnections(statePath, state, state.Connections); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Could not update state file: %v\n", err)
	} else {
		fmt.Printf("\n💾 State saved to %s\n", statePath)
	}

	// ── Summary ──
	fmt.Println("\n" + strings.Repeat("─", 50))
	fmt.Printf("✅ Connection updated (ID=%d) %q\n", updated.ID, updated.Name)
	fmt.Println(strings.Repeat("─", 50))
	fmt.Println()

	return nil
}

// buildUpdateRequestFromFlags builds a PATCH request using only explicitly set flags.
func buildUpdateRequestFromFlags(cmd *cobra.Command, current *devlake.Connection) *devlake.ConnectionUpdateRequest {
	req := &devlake.ConnectionUpdateRequest{}
	if cmd.Flags().Changed("name") {
		req.Name = updateConnName
	}
	if cmd.Flags().Changed("token") {
		req.Token = updateConnToken
		req.AuthMethod = "AccessToken"
	}
	if cmd.Flags().Changed("org") {
		req.Organization = updateConnOrg
	}
	if cmd.Flags().Changed("enterprise") {
		req.Enterprise = updateConnEnterprise
	}
	if cmd.Flags().Changed("endpoint") {
		req.Endpoint = updateConnEndpoint
	}
	if cmd.Flags().Changed("proxy") {
		req.Proxy = updateConnProxy
	}
	return req
}

// buildUpdateRequestInteractive prompts the user for each field, showing current values as defaults.
func buildUpdateRequestInteractive(current *devlake.Connection, def *ConnectionDef) (*devlake.ConnectionUpdateRequest, error) {
	req := &devlake.ConnectionUpdateRequest{}

	fmt.Println()
	fmt.Println("   Press Enter to keep current value, or type a new one.")

	// Name
	newName := prompt.ReadLine(fmt.Sprintf("   Name [%s]", current.Name))
	if newName != "" && newName != current.Name {
		req.Name = newName
	}

	// Endpoint
	endpointDefault := current.Endpoint
	if endpointDefault == "" && def != nil {
		endpointDefault = def.Endpoint
	}
	newEndpoint := prompt.ReadLine(fmt.Sprintf("   Endpoint [%s]", endpointDefault))
	if newEndpoint != "" && newEndpoint != endpointDefault {
		req.Endpoint = newEndpoint
	}

	// Org (if relevant)
	if def == nil || def.NeedsOrg || current.Organization != "" {
		newOrg := prompt.ReadLine(fmt.Sprintf("   Organization [%s]", current.Organization))
		if newOrg != "" && newOrg != current.Organization {
			req.Organization = newOrg
		}
	}

	// Enterprise (if relevant)
	if def == nil || def.NeedsEnterprise || current.Enterprise != "" {
		newEnt := prompt.ReadLine(fmt.Sprintf("   Enterprise [%s]", current.Enterprise))
		if newEnt != "" && newEnt != current.Enterprise {
			req.Enterprise = newEnt
		}
	}

	// Token (masked display)
	tokenDisplay := "(hidden)"
	if current.Token != "" {
		tokenDisplay = maskToken(current.Token)
	}
	fmt.Println()
	newToken := prompt.ReadSecret(fmt.Sprintf("   Token [%s] (Enter to keep)", tokenDisplay))
	if newToken != "" {
		req.Token = newToken
		req.AuthMethod = "AccessToken"
	}

	return req, nil
}

// tokenVisibleChars is the number of trailing characters shown when masking a token.
const tokenVisibleChars = 4

// maskToken returns a masked version of a token, showing only the last tokenVisibleChars characters.
// Tokens with tokenVisibleChars or fewer characters are returned unchanged.
func maskToken(token string) string {
	if len(token) <= tokenVisibleChars {
		return token
	}
	return strings.Repeat("*", len(token)-tokenVisibleChars) + token[len(token)-tokenVisibleChars:]
}
