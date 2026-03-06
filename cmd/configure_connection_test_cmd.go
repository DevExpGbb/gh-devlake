package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var (
	connTestPlugin string
	connTestID     int
)

var testConnectionCmd = &cobra.Command{
	Use:   "test",
	Short: "Test an existing DevLake connection",
	Long: `Tests an existing DevLake plugin connection by ID.

Examples:
  gh devlake configure connection test --plugin gh-copilot --id 2
  gh devlake configure connection test   # interactive mode`,
	RunE: runTestConnection,
}

func init() {
	testConnectionCmd.Flags().StringVar(&connTestPlugin, "plugin", "", fmt.Sprintf("Plugin to test (%s)", strings.Join(availablePluginSlugs(), ", ")))
	testConnectionCmd.Flags().IntVar(&connTestID, "id", 0, "Connection ID to test")
	configureConnectionsCmd.AddCommand(testConnectionCmd)
}

func runTestConnection(cmd *cobra.Command, args []string) error {
	printBanner("DevLake — Test Connection")

	// ── Discover DevLake ──
	client, _, err := discoverClient(cfgURL)
	if err != nil {
		return err
	}

	var plugin string
	var connID int

	// ── Determine if we're in flag mode or interactive mode ──
	if connTestPlugin != "" && connTestID > 0 {
		// ── Flag mode: validate plugin and use provided ID ──
		if _, err := requirePlugin(connTestPlugin); err != nil {
			return err
		}
		plugin = canonicalPluginSlug(connTestPlugin)
		connID = connTestID
	} else {
		// ── Interactive mode: list all connections and let user pick ──
		picked, err := pickConnection(client, "Select connection to test")
		if err != nil {
			return err
		}
		plugin = picked.Plugin
		connID = picked.ID
	}

	// ── Test the connection ──
	fmt.Printf("\n🔌 Testing connection (plugin=%s, ID=%d)...\n", plugin, connID)
	result, err := client.TestSavedConnection(plugin, connID)
	if err != nil {
		return fmt.Errorf("test connection request failed: %w", err)
	}

	// ── Display result ──
	fmt.Println()
	if result.Success {
		fmt.Println("✅ Connection test passed")
		if result.Message != "" {
			fmt.Printf("   %s\n", result.Message)
		}
		fmt.Println()
		return nil
	}

	// Test failed
	msg := result.Message
	if msg == "" {
		msg = "No details provided"
	}
	fmt.Printf("❌ Connection test failed: %s\n", msg)
	def := FindConnectionDef(plugin)
	if def != nil && def.ScopeHint != "" {
		fmt.Printf("   💡 Ensure your PAT has these scopes: %s\n", def.ScopeHint)
	}
	fmt.Println()
	return fmt.Errorf("connection test failed")
}
