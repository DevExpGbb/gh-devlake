package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	queryDoraProject   string
	queryDoraTimeframe string
)

var queryDoraCmd = &cobra.Command{
	Use:   "dora",
	Short: "Query DORA metrics (requires DevLake metrics API)",
	Long: `Query DORA (DevOps Research and Assessment) metrics for a project.

NOTE: This command requires DevLake to expose a metrics API endpoint.
Currently, DORA metrics are calculated in Grafana dashboards but not
available via the REST API. This is a placeholder for future enhancement.

To view DORA metrics today, use the Grafana dashboards at your DevLake
Grafana endpoint (shown in 'gh devlake status').

Planned output format:
{
  "project": "my-team",
  "timeframe": "30d",
  "metrics": {
    "deploymentFrequency": { "value": 4.2, "unit": "per_week", "rating": "high" },
    "leadTimeForChanges": { "value": 2.3, "unit": "hours", "rating": "elite" },
    "changeFailureRate": { "value": 0.08, "unit": "ratio", "rating": "high" },
    "meanTimeToRestore": { "value": 1.5, "unit": "hours", "rating": "elite" }
  }
}`,
	RunE: runQueryDora,
}

func init() {
	queryDoraCmd.Flags().StringVar(&queryDoraProject, "project", "", "Project name (required)")
	queryDoraCmd.Flags().StringVar(&queryDoraTimeframe, "timeframe", "30d", "Time window for metrics (e.g., 7d, 30d, 90d)")
	queryDoraCmd.MarkFlagRequired("project")
	queryCmd.AddCommand(queryDoraCmd)
}

func runQueryDora(cmd *cobra.Command, args []string) error {
	return fmt.Errorf(`DORA metrics query is not yet implemented.

DevLake does not currently expose a metrics API endpoint. DORA metrics are
calculated in Grafana dashboards using SQL queries against the domain layer.

To view DORA metrics, visit your Grafana endpoint (shown in 'gh devlake status')
and navigate to the DORA dashboards.

Future implementation will require:
  1. Upstream DevLake metrics API endpoint
  2. OR direct database query support (requires DB credentials)
  3. OR Grafana API integration to fetch dashboard data

Track progress at: https://github.com/DevExpGBB/gh-devlake/issues`)
}
