package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/DevExpGBB/gh-devlake/internal/prompt"
)

var (
	scopeDeletePlugin  string
	scopeDeleteConnID  int
	scopeDeleteScopeID string
	scopeDeleteForce   bool
)

func newScopeDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Remove a scope from a connection",
		Long: `Removes a scope from an existing DevLake plugin connection.

If --plugin, --connection-id, and --scope-id are not specified, prompts interactively.

Examples:
  gh devlake configure scope delete
  gh devlake configure scope delete --plugin github --connection-id 1 --scope-id 12345678`,
		RunE: runScopeDelete,
	}

	cmd.Flags().StringVar(&scopeDeletePlugin, "plugin", "", fmt.Sprintf("Plugin of the connection (%s)", strings.Join(availablePluginSlugs(), ", ")))
	cmd.Flags().IntVar(&scopeDeleteConnID, "connection-id", 0, "Connection ID")
	cmd.Flags().StringVar(&scopeDeleteScopeID, "scope-id", "", "Scope ID to delete")
	cmd.Flags().BoolVar(&scopeDeleteForce, "force", false, "Skip confirmation prompt")

	return cmd
}

func runScopeDelete(cmd *cobra.Command, args []string) error {
	printBanner("DevLake \u2014 Delete Scope")

	pluginFlagSet := cmd.Flags().Changed("plugin")
	connIDFlagSet := cmd.Flags().Changed("connection-id")
	scopeIDFlagSet := cmd.Flags().Changed("scope-id")

	// If any flag is set, all three must be provided
	if pluginFlagSet || connIDFlagSet || scopeIDFlagSet {
		if !pluginFlagSet || !connIDFlagSet || !scopeIDFlagSet ||
			scopeDeletePlugin == "" || scopeDeleteConnID == 0 || scopeDeleteScopeID == "" {
			return fmt.Errorf("--plugin, --connection-id, and --scope-id must all be provided together")
		}
	}

	if scopeDeletePlugin != "" {
		if _, err := requirePlugin(scopeDeletePlugin); err != nil {
			return err
		}
	}

	client, _, err := discoverClient(cfgURL)
	if err != nil {
		return err
	}

	selectedPlugin := scopeDeletePlugin
	selectedConnID := scopeDeleteConnID
	selectedScopeID := scopeDeleteScopeID

	if !(pluginFlagSet && connIDFlagSet && scopeIDFlagSet) {
		// Interactive: pick connection first
		picked, err := pickConnection(client, "Select a connection to delete a scope from")
		if err != nil {
			if err.Error() == "no connections found \u2014 create one with 'gh devlake configure connection add'" {
				fmt.Println("\n  No connections found.")
				return nil
			}
			return err
		}
		selectedPlugin = picked.Plugin
		selectedConnID = picked.ID

		// List scopes for picked connection
		fmt.Printf("\n\U0001f4cb Listing scopes for %s connection ID=%d...\n", selectedPlugin, selectedConnID)
		resp, err := client.ListScopes(selectedPlugin, selectedConnID)
		if err != nil {
			return fmt.Errorf("failed to list scopes: %w", err)
		}
		if len(resp.Scopes) == 0 {
			fmt.Println("\n  No scopes found on this connection.")
			return nil
		}

		// Build entries for selection
		type scopeEntry struct {
			id    string
			label string
		}
		var entries []scopeEntry
		var labels []string
		for _, s := range resp.Scopes {
			id := s.Scope.ID
			if id == "" {
				id = strconv.Itoa(s.Scope.GithubID)
			}
			name := s.Scope.FullName
			if name == "" {
				name = s.Scope.Name
			}
			label := fmt.Sprintf("[%s] %s", id, name)
			entries = append(entries, scopeEntry{id: id, label: label})
			labels = append(labels, label)
		}

		fmt.Println()
		chosen := prompt.Select("Select a scope to delete", labels)
		if chosen == "" {
			return fmt.Errorf("scope selection is required")
		}
		for _, e := range entries {
			if e.label == chosen {
				selectedScopeID = e.id
				break
			}
		}
		if selectedScopeID == "" {
			return fmt.Errorf("invalid scope selection")
		}
	}

	// Confirm deletion
	fmt.Printf("\n\u26a0\ufe0f  This will delete scope ID=%s from %s connection ID=%d.\n", selectedScopeID, selectedPlugin, selectedConnID)
	fmt.Println("   Blueprints referencing this scope may be affected.")

	fmt.Println()
	if !scopeDeleteForce && !prompt.Confirm("Are you sure you want to delete this scope?") {
		fmt.Println("\n  Deletion cancelled.")
		return nil
	}

	// Delete scope
	fmt.Printf("\n\U0001f5d1\ufe0f  Deleting scope %s from %s connection ID=%d...\n", selectedScopeID, selectedPlugin, selectedConnID)
	if err := client.DeleteScope(selectedPlugin, selectedConnID, selectedScopeID); err != nil {
		return fmt.Errorf("failed to delete scope: %w", err)
	}
	fmt.Println("   \u2705 Scope deleted")

	fmt.Println("\n" + strings.Repeat("\u2500", 40))
	fmt.Printf("\u2705 Scope deleted (plugin: %s, connection ID=%d, scope ID=%s)\n", selectedPlugin, selectedConnID, selectedScopeID)
	fmt.Println(strings.Repeat("\u2500", 40))
	fmt.Println()

	return nil
}
