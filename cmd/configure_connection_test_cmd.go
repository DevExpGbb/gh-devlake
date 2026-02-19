package cmd

import (
	"fmt"
	"strings"

	"github.com/DevExpGBB/gh-devlake/internal/devlake"
	"github.com/DevExpGBB/gh-devlake/internal/prompt"
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
	testConnectionCmd.Flags().StringVar(&connTestPlugin, "plugin", "", "Plugin to test (github, gh-copilot)")
	testConnectionCmd.Flags().IntVar(&connTestID, "id", 0, "Connection ID to test")
	configureConnectionsCmd.AddCommand(testConnectionCmd)
}

func runTestConnection(cmd *cobra.Command, args []string) error {
	fmt.Println()
	fmt.Println("════════════════════════════════════════")
	fmt.Println("  DevLake — Test Connection")
	fmt.Println("════════════════════════════════════════")

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

	// ── Determine if we're in flag mode or interactive mode ──
	if connTestPlugin != "" && connTestID > 0 {
		// ── Flag mode: validate plugin and use provided ID ──
		def := FindConnectionDef(connTestPlugin)
		if def == nil || !def.Available {
			slugs := availablePluginSlugs()
			return fmt.Errorf("unknown plugin %q — choose: %s", connTestPlugin, strings.Join(slugs, ", "))
		}
		plugin = connTestPlugin
		connID = connTestID
	} else {
		// ── Interactive mode: list all connections and let user pick ──
		fmt.Println("\n📋 Fetching connections...")

		// Collect connections from all available plugins
		type connRow struct {
			plugin       string
			pluginLabel  string
			id           int
			name         string
			organization string
			enterprise   string
		}
		var rows []connRow

		for _, def := range AvailableConnections() {
			conns, err := client.ListConnections(def.Plugin)
			if err != nil {
				fmt.Printf("\n⚠️  Could not list %s connections: %v\n", def.DisplayName, err)
				continue
			}
			for _, c := range conns {
				rows = append(rows, connRow{
					plugin:       def.Plugin,
					pluginLabel:  def.DisplayName,
					id:           c.ID,
					name:         c.Name,
					organization: c.Organization,
					enterprise:   c.Enterprise,
				})
			}
		}

		if len(rows) == 0 {
			return fmt.Errorf("no connections found — create one first with 'gh devlake configure connection'")
		}

		// Build selection menu items
		var menuItems []string
		for _, r := range rows {
			var parts []string
			parts = append(parts, fmt.Sprintf("%s (ID=%d)", r.pluginLabel, r.id))
			if r.name != "" {
				parts = append(parts, r.name)
			}
			if r.organization != "" {
				parts = append(parts, fmt.Sprintf("org=%s", r.organization))
			}
			if r.enterprise != "" {
				parts = append(parts, fmt.Sprintf("enterprise=%s", r.enterprise))
			}
			menuItems = append(menuItems, strings.Join(parts, " — "))
		}

		fmt.Println()
		selected := prompt.Select("Select connection to test", menuItems)
		if selected == "" {
			return fmt.Errorf("connection selection is required")
		}

		// Find the selected row
		var selectedRow *connRow
		for i, item := range menuItems {
			if item == selected {
				selectedRow = &rows[i]
				break
			}
		}
		if selectedRow == nil {
			return fmt.Errorf("invalid selection")
		}

		plugin = selectedRow.plugin
		connID = selectedRow.id
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
