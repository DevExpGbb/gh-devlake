package cmd

import (
	"github.com/spf13/cobra"
)

var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "Query DevLake data and metrics",
	Long: `Query DevLake's aggregated data and metrics.

Retrieve pipeline status plus DORA/Copilot query results in structured
JSON output. Individual subcommands may provide extra formatting options
such as query pipelines --format table for human-readable output.

Examples:
  gh devlake query pipelines --project my-team
  gh devlake query pipelines --limit 20
  gh devlake query pipelines --status TASK_COMPLETED`,
}

func init() {
	queryCmd.GroupID = "operate"
	rootCmd.AddCommand(queryCmd)
}
