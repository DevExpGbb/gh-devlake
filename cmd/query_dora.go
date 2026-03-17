package cmd

import (
	"fmt"

	"github.com/DevExpGBB/gh-devlake/internal/devlake"
	"github.com/DevExpGBB/gh-devlake/internal/query"
	"github.com/spf13/cobra"
)

var (
	queryDoraProject   string
	queryDoraTimeframe string
)

func newQueryDoraCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dora",
		Short: "Query DORA metrics (limited by available API data)",
		Long: `Query DORA (DevOps Research and Assessment) metrics for a project.

NOTE: Full DORA metric calculations (deployment frequency, lead time, change
failure rate, MTTR) require SQL queries against DevLake's domain layer tables.
DevLake does not expose database credentials or a metrics API endpoint.

This command returns project metadata and explains what additional API
endpoints would be needed to compute DORA metrics via CLI.

DORA metrics are currently available in Grafana dashboards at your DevLake
Grafana endpoint (shown in 'gh devlake status').`,
		RunE: runQueryDora,
	}
	cmd.Flags().StringVar(&queryDoraProject, "project", "", "Project name (required)")
	cmd.Flags().StringVar(&queryDoraTimeframe, "timeframe", "30d", "Time window for metrics (e.g., 7d, 30d, 90d)")
	return cmd
}

func runQueryDora(cmd *cobra.Command, args []string) error {
	if queryDoraProject == "" {
		return fmt.Errorf("--project flag is required")
	}

	disc, err := devlake.Discover(cfgURL)
	if err != nil {
		return fmt.Errorf("discovering DevLake: %w", err)
	}
	client := devlake.NewClient(disc.URL)

	queryDef, err := query.Get("dora")
	if err != nil {
		return fmt.Errorf("getting dora query: %w", err)
	}

	params := map[string]interface{}{
		"project":   queryDoraProject,
		"timeframe": queryDoraTimeframe,
	}

	engine := query.NewEngine(client)
	result, err := engine.Execute(queryDef, params)
	if err != nil {
		return fmt.Errorf("executing dora query: %w", err)
	}

	return printJSON(result)
}
