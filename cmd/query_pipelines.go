package cmd

import (
	"fmt"
	"strings"

	"github.com/DevExpGBB/gh-devlake/internal/devlake"
	"github.com/DevExpGBB/gh-devlake/internal/query"
	"github.com/spf13/cobra"
)

var (
	queryPipelinesProject string
	queryPipelinesStatus  string
	queryPipelinesLimit   int
	queryPipelinesFormat  string
)

var queryPipelinesCmd = &cobra.Command{
	Use:   "pipelines",
	Short: "Query recent pipeline runs",
	Long: `Query recent pipeline runs for a project or across all projects.

Retrieves pipeline execution history with status, timing, and task completion
information. Output is JSON by default; use --format table for human-readable display.

Examples:
  gh devlake query pipelines
  gh devlake query pipelines --project my-team
  gh devlake query pipelines --status TASK_COMPLETED --limit 10
  gh devlake query pipelines --format table`,
	RunE: runQueryPipelines,
}

func init() {
	queryPipelinesCmd.Flags().StringVar(&queryPipelinesProject, "project", "", "Filter by project name")
	queryPipelinesCmd.Flags().StringVar(&queryPipelinesStatus, "status", "", "Filter by status (TASK_CREATED, TASK_RUNNING, TASK_COMPLETED, TASK_FAILED)")
	queryPipelinesCmd.Flags().IntVar(&queryPipelinesLimit, "limit", 20, "Maximum number of pipelines to return")
	queryPipelinesCmd.Flags().StringVar(&queryPipelinesFormat, "format", "json", "Output format (json or table)")
	queryCmd.AddCommand(queryPipelinesCmd)
}

func runQueryPipelines(cmd *cobra.Command, args []string) error {
	// Validate format flag
	if queryPipelinesFormat != "json" && queryPipelinesFormat != "table" {
		return fmt.Errorf("invalid --format value %q: must be 'json' or 'table'", queryPipelinesFormat)
	}

	// Discover DevLake instance
	var client *devlake.Client
	var err error

	// Use quiet discovery for JSON output, verbose for table
	if outputJSON || queryPipelinesFormat == "json" {
		// Quiet discovery for JSON output
		disc, err := devlake.Discover(cfgURL)
		if err != nil {
			return fmt.Errorf("discovering DevLake: %w", err)
		}
		client = devlake.NewClient(disc.URL)
	} else {
		// Verbose discovery for table output
		var disc *devlake.DiscoveryResult
		client, disc, err = discoverClient(cfgURL)
		if err != nil {
			return fmt.Errorf("discovering DevLake: %w", err)
		}
		_ = disc // disc is used by discoverClient for output
	}

	// Get the query definition
	queryDef, err := query.Get("pipelines")
	if err != nil {
		return fmt.Errorf("getting pipelines query: %w", err)
	}

	// Build parameters
	params := map[string]interface{}{
		"limit": queryPipelinesLimit,
	}
	if queryPipelinesProject != "" {
		params["project"] = queryPipelinesProject
	}
	if queryPipelinesStatus != "" {
		params["status"] = queryPipelinesStatus
	}

	// Execute the query
	engine := query.NewEngine(client)
	result, err := engine.Execute(queryDef, params)
	if err != nil {
		return fmt.Errorf("executing pipelines query: %w", err)
	}

	// Cast result to slice of PipelineResult
	pipelines, ok := result.([]query.PipelineResult)
	if !ok {
		return fmt.Errorf("unexpected result type: %T", result)
	}

	// Output
	if outputJSON || queryPipelinesFormat == "json" {
		return printJSON(pipelines)
	}

	// Table format
	printBanner("DevLake — Pipeline Query")
	if len(pipelines) == 0 {
		fmt.Println("\n  No pipelines found.")
		return nil
	}

	fmt.Printf("\n  Found %d pipeline(s)\n", len(pipelines))
	fmt.Println("  " + strings.Repeat("─", 80))
	fmt.Printf("  %-6s  %-15s  %-10s  %-20s\n", "ID", "STATUS", "TASKS", "FINISHED AT")
	fmt.Println("  " + strings.Repeat("─", 80))
	for _, p := range pipelines {
		status := p.Status
		tasks := fmt.Sprintf("%d/%d", p.FinishedTasks, p.TotalTasks)
		finished := p.FinishedAt
		if finished == "" {
			finished = "(running)"
		}
		fmt.Printf("  %-6d  %-15s  %-10s  %-20s\n", p.ID, status, tasks, finished)
	}
	fmt.Println()

	return nil
}
