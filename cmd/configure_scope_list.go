package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var (
	scopeListPlugin string
	scopeListConnID int
)

func newScopeListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List scopes on a connection",
		Long: `Lists all scopes configured on a DevLake plugin connection.

If --plugin and --connection-id are not specified, prompts interactively.

Examples:
  gh devlake configure scope list
  gh devlake configure scope list --plugin github --connection-id 1`,
		RunE: runScopeList,
	}

	cmd.Flags().StringVar(&scopeListPlugin, "plugin", "", fmt.Sprintf("Plugin to query (%s)", strings.Join(availablePluginSlugs(), ", ")))
	cmd.Flags().IntVar(&scopeListConnID, "connection-id", 0, "Connection ID")

	return cmd
}

func runScopeList(cmd *cobra.Command, args []string) error {
	printBanner("DevLake \u2014 List Scopes")

	pluginFlagSet := cmd.Flags().Changed("plugin")
	connIDFlagSet := cmd.Flags().Changed("connection-id")

	// If any flag is set, both must be provided
	if pluginFlagSet || connIDFlagSet {
		if !pluginFlagSet || !connIDFlagSet || scopeListPlugin == "" || scopeListConnID == 0 {
			return fmt.Errorf("both --plugin and --connection-id must be provided together")
		}
	}

	if scopeListPlugin != "" {
		if _, err := requirePlugin(scopeListPlugin); err != nil {
			return err
		}
	}

	client, _, err := discoverClient(cfgURL)
	if err != nil {
		return err
	}

	selectedPlugin := scopeListPlugin
	selectedConnID := scopeListConnID

	if !(pluginFlagSet && connIDFlagSet) {
		picked, err := pickConnection(client, "Select a connection to list scopes")
		if err != nil {
			if err.Error() == "no connections found \u2014 create one with 'gh devlake configure connection add'" {
				fmt.Println("\n  No connections found.")
				return nil
			}
			return err
		}
		selectedPlugin = picked.Plugin
		selectedConnID = picked.ID
	}

	fmt.Printf("\n\U0001f4cb Listing scopes for %s connection ID=%d...\n", selectedPlugin, selectedConnID)

	resp, err := client.ListScopes(selectedPlugin, selectedConnID)
	if err != nil {
		return fmt.Errorf("failed to list scopes: %w", err)
	}

	if len(resp.Scopes) == 0 {
		fmt.Println("  No scopes found.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Scope ID\tName\tFull Name")
	fmt.Fprintln(w, strings.Repeat("\u2500", 10)+"\t"+strings.Repeat("\u2500", 20)+"\t"+strings.Repeat("\u2500", 30))
	for _, s := range resp.Scopes {
		scopeID := s.Scope.ID
		if scopeID == "" {
			scopeID = strconv.Itoa(s.Scope.GithubID)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", scopeID, s.Scope.Name, s.Scope.FullName)
	}
	w.Flush()
	fmt.Println()

	return nil
}
