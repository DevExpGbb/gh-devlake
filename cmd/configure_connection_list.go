package cmd

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var connListPlugin string

var listConnectionsCmd = &cobra.Command{
	Use:   "list",
	Short: "List all plugin connections in DevLake",
	Long: `Lists all DevLake plugin connections, optionally filtered by plugin.

Examples:
  gh devlake configure connection list
  gh devlake configure connection list --plugin gh-copilot`,
	RunE: runListConnections,
}

func init() {
	listConnectionsCmd.Flags().StringVar(&connListPlugin, "plugin", "", fmt.Sprintf("Filter by plugin (%s)", strings.Join(availablePluginSlugs(), ", ")))
	configureConnectionsCmd.AddCommand(listConnectionsCmd)
}

func runListConnections(cmd *cobra.Command, args []string) error {
	printBanner("DevLake — List Connections")

	// ── Validate --plugin flag ──
	if connListPlugin != "" {
		if _, err := requirePlugin(connListPlugin); err != nil {
			return err
		}
	}

	// ── Discover DevLake ──
	client, _, err := discoverClient(cfgURL)
	if err != nil {
		return err
	}

	// ── Determine which plugins to query ──
	var defs []*ConnectionDef
	if connListPlugin != "" {
		defs = []*ConnectionDef{FindConnectionDef(connListPlugin)}
	} else {
		defs = AvailableConnections()
	}

	// ── Collect connections from all relevant plugins ──
	type row struct {
		plugin       string
		id           int
		name         string
		organization string
		enterprise   string
	}
	var rows []row

	for _, def := range defs {
		conns, err := client.ListConnections(def.Plugin)
		if err != nil {
			fmt.Printf("\n⚠️  Could not list %s connections: %v\n", def.DisplayName, err)
			continue
		}
		for _, c := range conns {
			rows = append(rows, row{
				plugin:       def.Plugin,
				id:           c.ID,
				name:         c.Name,
				organization: c.Organization,
				enterprise:   c.Enterprise,
			})
		}
	}

	// ── Render table ──
	fmt.Println()
	if len(rows) == 0 {
		fmt.Println("  No connections found.")
		fmt.Println()
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Plugin\tID\tName\tOrganization\tEnterprise")
	fmt.Fprintln(w, strings.Repeat("─", 10)+"\t"+strings.Repeat("─", 4)+"\t"+strings.Repeat("─", 30)+"\t"+strings.Repeat("─", 14)+"\t"+strings.Repeat("─", 12))
	for _, r := range rows {
		fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s\n", r.plugin, r.id, r.name, r.organization, r.enterprise)
	}
	w.Flush()
	fmt.Println()

	return nil
}

// availablePluginSlugs returns the Plugin slugs of all available connections.
func availablePluginSlugs() []string {
	defs := AvailableConnections()
	slugs := make([]string, len(defs))
	for i, d := range defs {
		slugs[i] = d.Plugin
	}
	return slugs
}
