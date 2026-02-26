package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/DevExpGBB/gh-devlake/internal/devlake"
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

// connectionListItem is the JSON representation of a single connection entry.
type connectionListItem struct {
	ID           int    `json:"id"`
	Plugin       string `json:"plugin"`
	Name         string `json:"name"`
	Endpoint     string `json:"endpoint,omitempty"`
	Organization string `json:"organization,omitempty"`
	Enterprise   string `json:"enterprise,omitempty"`
}

func runListConnections(cmd *cobra.Command, args []string) error {
	// ── Validate --plugin flag ──
	if connListPlugin != "" {
		if _, err := requirePlugin(connListPlugin); err != nil {
			return err
		}
	}

	// ── Discover DevLake (quietly in JSON mode to keep stdout clean) ──
	var client *devlake.Client
	if outputJSON {
		disc, err := devlake.Discover(cfgURL)
		if err != nil {
			return err
		}
		client = devlake.NewClient(disc.URL)
	} else {
		c, _, err := discoverClient(cfgURL)
		if err != nil {
			return err
		}
		client = c
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
		ID           int
		name         string
		endpoint     string
		organization string
		enterprise   string
	}
	var rows []row

	for _, def := range defs {
		conns, err := client.ListConnections(def.Plugin)
		if err != nil {
			if outputJSON {
				fmt.Fprintf(os.Stderr, "⚠️  Could not list %s connections: %v\n", def.DisplayName, err)
			} else {
				fmt.Printf("\n⚠️  Could not list %s connections: %v\n", def.DisplayName, err)
			}
			continue
		}
		for _, c := range conns {
			rows = append(rows, row{
				plugin:       def.Plugin,
				ID:           c.ID,
				name:         c.Name,
				endpoint:     c.Endpoint,
				organization: c.Organization,
				enterprise:   c.Enterprise,
			})
		}
	}

	// ── JSON output path ──
	if outputJSON {
		items := make([]connectionListItem, len(rows))
		for i, r := range rows {
			items[i] = connectionListItem{
				ID:           r.ID,
				Plugin:       r.plugin,
				Name:         r.name,
				Endpoint:     r.endpoint,
				Organization: r.organization,
				Enterprise:   r.enterprise,
			}
		}
		return printJSON(items)
	}

	// ── Render table ──
	printBanner("DevLake — List Connections")
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
		fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s\n", r.plugin, r.ID, r.name, r.organization, r.enterprise)
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
