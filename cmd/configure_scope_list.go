package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/DevExpGBB/gh-devlake/internal/devlake"
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

// scopeListItem is the JSON representation of a single scope entry.
type scopeListItem struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	FullName string `json:"fullName,omitempty"`
}

func runScopeList(cmd *cobra.Command, args []string) error {
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

	// In JSON mode, flags are required (interactive prompts are not supported)
	if outputJSON && !(pluginFlagSet && connIDFlagSet) {
		return fmt.Errorf("--plugin and --connection-id are required with --json")
	}

	// Discover client (quietly in JSON mode to keep stdout clean)
	var client *devlake.Client
	if outputJSON {
		disc, err := devlake.Discover(cfgURL)
		if err != nil {
			return err
		}
		client = devlake.NewClient(disc.URL)
	} else {
		printBanner("DevLake \u2014 List Scopes")
		c, _, err := discoverClient(cfgURL)
		if err != nil {
			return err
		}
		client = c
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

	if !outputJSON {
		fmt.Printf("\n\U0001f4cb Listing scopes for %s connection ID=%d...\n", selectedPlugin, selectedConnID)
	}

	resp, err := client.ListScopes(selectedPlugin, selectedConnID)
	if err != nil {
		return fmt.Errorf("failed to list scopes: %w", err)
	}

	// JSON output path
	if outputJSON {
		items := make([]scopeListItem, len(resp.Scopes))
		for i, s := range resp.Scopes {
			scopeID := s.Scope.ID
			if scopeID == "" {
				scopeID = strconv.Itoa(s.Scope.GithubID)
			}
			items[i] = scopeListItem{
				ID:       scopeID,
				Name:     s.Scope.Name,
				FullName: s.Scope.FullName,
			}
		}
		return printJSON(items)
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
