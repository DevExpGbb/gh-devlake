package cmd

import (
	"fmt"
	"strings"

	"github.com/DevExpGBB/gh-devlake/internal/devlake"
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

type pipelineQueryResult struct {
	ID            int    `json:"id"`
	Status        string `json:"status"`
	BlueprintID   int    `json:"blueprintId,omitempty"`
	CreatedAt     string `json:"createdAt,omitempty"`
	BeganAt       string `json:"beganAt,omitempty"`
	FinishedAt    string `json:"finishedAt,omitempty"`
	FinishedTasks int    `json:"finishedTasks"`
	TotalTasks    int    `json:"totalTasks"`
	Message       string `json:"message,omitempty"`
}

func runQueryPipelines(cmd *cobra.Command, args []string) error {
	// Discover DevLake instance
	var disc *devlake.DiscoveryResult
	var client *devlake.Client
	var err error

	if !outputJSON && queryPipelinesFormat != "table" {
		client, disc, err = discoverClient(cfgURL)
		if err != nil {
			return fmt.Errorf("discovering DevLake: %w", err)
		}
	} else {
		// Quiet discovery for JSON/table output
		disc, err = devlake.Discover(cfgURL)
		if err != nil {
			return fmt.Errorf("discovering DevLake: %w", err)
		}
		client = devlake.NewClient(disc.URL)
	}

	// If --project is specified, resolve it to a blueprint ID
	var blueprintID int
	if queryPipelinesProject != "" {
		proj, err := client.GetProject(queryPipelinesProject)
		if err != nil {
			return fmt.Errorf("getting project %q: %w", queryPipelinesProject, err)
		}
		if proj.Blueprint != nil {
			blueprintID = proj.Blueprint.ID
		} else {
			return fmt.Errorf("project %q has no blueprint", queryPipelinesProject)
		}
	}

	// Query pipelines
	resp, err := client.ListPipelines(queryPipelinesStatus, blueprintID, 1, queryPipelinesLimit)
	if err != nil {
		return fmt.Errorf("listing pipelines: %w", err)
	}

	// Transform to output format
	results := make([]pipelineQueryResult, len(resp.Pipelines))
	for i, p := range resp.Pipelines {
		results[i] = pipelineQueryResult{
			ID:            p.ID,
			Status:        p.Status,
			BlueprintID:   p.BlueprintID,
			CreatedAt:     p.CreatedAt,
			BeganAt:       p.BeganAt,
			FinishedAt:    p.FinishedAt,
			FinishedTasks: p.FinishedTasks,
			TotalTasks:    p.TotalTasks,
			Message:       p.Message,
		}
	}

	// Output
	if outputJSON || queryPipelinesFormat == "json" {
		return printJSON(results)
	}

	// Table format
	printBanner("DevLake — Pipeline Query")
	if len(results) == 0 {
		fmt.Println("\n  No pipelines found.")
		return nil
	}

	fmt.Printf("\n  Found %d pipeline(s)\n", len(results))
	fmt.Println("  " + strings.Repeat("─", 80))
	fmt.Printf("  %-6s  %-15s  %-10s  %-20s\n", "ID", "STATUS", "TASKS", "FINISHED AT")
	fmt.Println("  " + strings.Repeat("─", 80))
	for _, r := range results {
		status := r.Status
		tasks := fmt.Sprintf("%d/%d", r.FinishedTasks, r.TotalTasks)
		finished := r.FinishedAt
		if finished == "" {
			finished = "(running)"
		}
		fmt.Printf("  %-6d  %-15s  %-10s  %-20s\n", r.ID, status, tasks, finished)
	}
	fmt.Println()

	return nil
}
