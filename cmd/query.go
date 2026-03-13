package cmd

import (
	"github.com/spf13/cobra"
)

var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "Query DevLake data and metrics",
	Long: `Query DevLake's aggregated data and metrics.

Retrieve DORA metrics, Copilot usage data, pipeline status, and other
metrics in a structured format (JSON by default, --format table for
human-readable output).

Examples:
  gh devlake query pipelines --project my-team
  gh devlake query pipelines --limit 20
  gh devlake query pipelines --status TASK_COMPLETED`,
}

func init() {
	queryCmd.GroupID = "operate"
	rootCmd.AddCommand(queryCmd)
}
