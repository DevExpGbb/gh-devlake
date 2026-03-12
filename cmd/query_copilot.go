package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	queryCopilotProject   string
	queryCopilotTimeframe string
)

var queryCopilotCmd = &cobra.Command{
	Use:   "copilot",
	Short: "Query Copilot usage metrics (requires DevLake metrics API)",
	Long: `Query GitHub Copilot usage metrics for a project.

NOTE: This command requires DevLake to expose a metrics API endpoint.
Currently, Copilot metrics are stored in the gh-copilot plugin tables
and visualized in Grafana dashboards, but not available via the REST API.

To view Copilot metrics today, use the Grafana dashboards at your DevLake
Grafana endpoint (shown in 'gh devlake status').

Planned output format:
{
  "project": "my-team",
  "timeframe": "30d",
  "metrics": {
    "totalSeats": 45,
    "activeUsers": 38,
    "acceptanceRate": 0.34,
    "topLanguages": [
      { "language": "TypeScript", "acceptances": 1200, "suggestions": 3500 }
    ],
    "topEditors": [
      { "editor": "vscode", "users": 30 }
    ]
  }
}`,
	RunE: runQueryCopilot,
}

func init() {
	queryCopilotCmd.Flags().StringVar(&queryCopilotProject, "project", "", "Project name (required)")
	queryCopilotCmd.Flags().StringVar(&queryCopilotTimeframe, "timeframe", "30d", "Time window for metrics (e.g., 7d, 30d, 90d)")
	queryCopilotCmd.MarkFlagRequired("project")
	queryCmd.AddCommand(queryCopilotCmd)
}

func runQueryCopilot(cmd *cobra.Command, args []string) error {
	return fmt.Errorf(`Copilot metrics query is not yet implemented.

DevLake does not currently expose a metrics API endpoint. Copilot metrics are
stored in the gh-copilot plugin's database tables and visualized in Grafana
dashboards, but not accessible via the REST API.

To view Copilot metrics, visit your Grafana endpoint (shown in 'gh devlake status')
and navigate to the Copilot dashboards.

Future implementation will require:
  1. Upstream DevLake metrics API endpoint for Copilot plugin
  2. OR direct database query support (requires DB credentials)
  3. OR Grafana API integration to fetch dashboard data

Track progress at: https://github.com/DevExpGBB/gh-devlake/issues`)
}
