package cmd

import (
	"fmt"

	"github.com/DevExpGBB/gh-devlake/internal/devlake"
	"github.com/DevExpGBB/gh-devlake/internal/query"
	"github.com/spf13/cobra"
)

var (
	queryCopilotProject   string
	queryCopilotTimeframe string
)

var queryCopilotCmd = &cobra.Command{
	Use:   "copilot",
	Short: "Query Copilot usage metrics (limited by available API data)",
	Long: `Query GitHub Copilot usage metrics for a project.

NOTE: GitHub Copilot usage metrics (total seats, active users, acceptance rates,
language breakdowns, editor usage) are stored in _tool_gh_copilot_* tables and
visualized in Grafana dashboards, but DevLake does not expose a /metrics or
/copilot API endpoint.

This command returns available connection metadata and explains what additional
API endpoints would be needed to retrieve Copilot metrics via CLI.

Copilot metrics are currently available in Grafana dashboards at your DevLake
Grafana endpoint (shown in 'gh devlake status').`,
	RunE: runQueryCopilot,
}

func init() {
	queryCopilotCmd.Flags().StringVar(&queryCopilotProject, "project", "", "Project name (required)")
	queryCopilotCmd.Flags().StringVar(&queryCopilotTimeframe, "timeframe", "30d", "Time window for metrics (e.g., 7d, 30d, 90d)")
	queryCmd.AddCommand(queryCopilotCmd)
}

func runQueryCopilot(cmd *cobra.Command, args []string) error {
	// Validate project flag
	if queryCopilotProject == "" {
		return fmt.Errorf("--project flag is required")
	}

	// Discover DevLake instance
	disc, err := devlake.Discover(cfgURL)
	if err != nil {
		return fmt.Errorf("discovering DevLake: %w", err)
	}
	client := devlake.NewClient(disc.URL)

	// Get the query definition
	queryDef, err := query.Get("copilot")
	if err != nil {
		return fmt.Errorf("getting copilot query: %w", err)
	}

	// Build parameters
	params := map[string]interface{}{
		"project":   queryCopilotProject,
		"timeframe": queryCopilotTimeframe,
	}

	// Execute the query
	engine := query.NewEngine(client)
	result, err := engine.Execute(queryDef, params)
	if err != nil {
		return fmt.Errorf("executing copilot query: %w", err)
	}

	// Output result as JSON
	return printJSON(result)
}
