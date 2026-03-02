package cmd

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/DevExpGBB/gh-devlake/internal/devlake"
)

func newProjectListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all DevLake projects",
		Long: `Lists all DevLake projects.

Example:
  gh devlake configure project list`,
		RunE: runProjectList,
	}
	return cmd
}

// projectListItem is the JSON representation of a single project entry.
type projectListItem struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	BlueprintID int    `json:"blueprintId,omitempty"`
}

func runProjectList(cmd *cobra.Command, args []string) error {
	// ── Discover DevLake ──
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

	// ── Fetch projects ──
	projects, err := client.ListProjects()
	if err != nil {
		return fmt.Errorf("listing projects: %w", err)
	}

	// ── JSON output path ──
	if outputJSON {
		items := make([]projectListItem, len(projects))
		for i, p := range projects {
			item := projectListItem{
				Name:        p.Name,
				Description: p.Description,
			}
			if p.Blueprint != nil {
				item.BlueprintID = p.Blueprint.ID
			}
			items[i] = item
		}
		return printJSON(items)
	}

	// ── Render table ──
	printBanner("DevLake — List Projects")
	fmt.Println()
	if len(projects) == 0 {
		fmt.Println("  No projects found.")
		fmt.Println()
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Name\tDescription\tBlueprint ID")
	fmt.Fprintln(w, strings.Repeat("─", 30)+"\t"+strings.Repeat("─", 40)+"\t"+strings.Repeat("─", 12))
	for _, p := range projects {
		blueprintID := ""
		if p.Blueprint != nil {
			blueprintID = fmt.Sprintf("%d", p.Blueprint.ID)
		}
		desc := p.Description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", p.Name, desc, blueprintID)
	}
	w.Flush()
	fmt.Println()

	return nil
}
