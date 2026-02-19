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
	connDeletePlugin string
	connDeleteID     int
)

var deleteConnectionCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a plugin connection from DevLake",
	Long: `Deletes a DevLake plugin connection by plugin and ID.

If --plugin and --id are not specified, prompts interactively.

Examples:
  gh devlake configure connection delete
  gh devlake configure connection delete --plugin github --id 3`,
	RunE: runDeleteConnection,
}

func init() {
	deleteConnectionCmd.Flags().StringVar(&connDeletePlugin, "plugin", "", "Plugin of the connection to delete")
	deleteConnectionCmd.Flags().IntVar(&connDeleteID, "id", 0, "ID of the connection to delete")
	configureConnectionsCmd.AddCommand(deleteConnectionCmd)
}

func runDeleteConnection(cmd *cobra.Command, args []string) error {
	fmt.Println()
	fmt.Println("════════════════════════════════════════")
	fmt.Println("  DevLake — Delete Connection")
	fmt.Println("════════════════════════════════════════")

	// ── Validate flags early before any I/O ──
	pluginFlagSet := cmd.Flags().Changed("plugin")
	idFlagSet := cmd.Flags().Changed("id")

	if pluginFlagSet || idFlagSet {
		// Flag mode: both flags must be provided
		if !pluginFlagSet || !idFlagSet || connDeletePlugin == "" || connDeleteID == 0 {
			return fmt.Errorf("both --plugin and --id must be provided together")
		}
	}

	if connDeletePlugin != "" {
		def := FindConnectionDef(connDeletePlugin)
		if def == nil || !def.Available {
			slugs := availablePluginSlugs()
			return fmt.Errorf("unknown plugin %q — choose: %s", connDeletePlugin, strings.Join(slugs, ", "))
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

	// ── Resolve plugin + ID ──
	plugin := connDeletePlugin
	connID := connDeleteID

	if !(pluginFlagSet && idFlagSet) {
		// Interactive: list connections and let user select one
		type connEntry struct {
			plugin string
			id     int
			label  string
		}

		fmt.Println("\n📋 Fetching connections...")
		var entries []connEntry
		for _, def := range AvailableConnections() {
			conns, err := client.ListConnections(def.Plugin)
			if err != nil {
				fmt.Printf("\n⚠️  Could not list %s connections: %v\n", def.DisplayName, err)
				continue
			}
			for _, c := range conns {
				label := fmt.Sprintf("[%s] ID=%d  %s", def.Plugin, c.ID, c.Name)
				entries = append(entries, connEntry{plugin: def.Plugin, id: c.ID, label: label})
			}
		}

		if len(entries) == 0 {
			fmt.Println("\n  No connections found.")
			fmt.Println()
			return nil
		}

		labels := make([]string, len(entries))
		for i, e := range entries {
			labels[i] = e.label
		}

		fmt.Println()
		chosen := prompt.Select("Select a connection to delete", labels)
		if chosen == "" {
			return fmt.Errorf("connection selection is required")
		}

		for _, e := range entries {
			if e.label == chosen {
				plugin = e.plugin
				connID = e.id
				break
			}
		}
	}

	if plugin == "" || connID == 0 {
		return fmt.Errorf("--plugin and --id are required (or omit both for interactive mode)")
	}

	// ── Confirm deletion ──
	fmt.Printf("\n⚠️  This will delete connection ID=%d (plugin: %s).\n", connID, plugin)
	fmt.Println("   Any scopes and blueprint references for this connection will also be lost.")
	fmt.Println()
	if !prompt.Confirm("Are you sure you want to delete this connection?") {
		fmt.Println("\n  Deletion cancelled.")
		fmt.Println()
		return nil
	}

	// ── Delete connection ──
	fmt.Printf("\n🗑️  Deleting %s connection ID=%d...\n", plugin, connID)
	if err := client.DeleteConnection(plugin, connID); err != nil {
		return fmt.Errorf("failed to delete connection: %w", err)
	}
	fmt.Println("   ✅ Connection deleted")

	// ── Update state file ──
	statePath, state := devlake.FindStateFile(disc.URL, disc.GrafanaURL)
	var updated []devlake.StateConnection
	for _, c := range state.Connections {
		if c.Plugin == plugin && c.ConnectionID == connID {
			continue
		}
		updated = append(updated, c)
	}
	if err := devlake.UpdateConnections(statePath, state, updated); err != nil {
		fmt.Fprintf(os.Stderr, "\n⚠️  Could not update state file: %v\n", err)
	} else {
		fmt.Printf("\n💾 State saved to %s\n", statePath)
	}

	fmt.Println("\n" + strings.Repeat("─", 50))
	fmt.Printf("✅ Connection deleted (plugin: %s, ID=%d)\n", plugin, connID)
	fmt.Println(strings.Repeat("─", 50))
	fmt.Println()

	return nil
}
