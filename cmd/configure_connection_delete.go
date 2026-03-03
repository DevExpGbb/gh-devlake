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
	printBanner("DevLake — Delete Connection")

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
		if _, err := requirePlugin(connDeletePlugin); err != nil {
			return err
		}
	}

	// ── Discover DevLake ──
	client, disc, err := discoverClient(cfgURL)
	if err != nil {
		return err
	}

	// ── Resolve plugin + ID ──
	plugin := connDeletePlugin
	connID := connDeleteID

	if !(pluginFlagSet && idFlagSet) {
		// Interactive: let user select one
		picked, err := pickConnection(client, "Select a connection to delete")
		if err != nil {
			if picked == nil && err.Error() == "no connections found — create one with 'gh devlake configure connection add'" {
				fmt.Println("\n  No connections found.")
				fmt.Println()
				return nil
			}
			return err
		}
		plugin = picked.Plugin
		connID = picked.ID
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

	fmt.Println("\n" + strings.Repeat("─", 40))
	fmt.Printf("✅ Connection deleted (plugin: %s, ID=%d)\n", plugin, connID)
	fmt.Println(strings.Repeat("─", 40))
	fmt.Println()

	return nil
}
