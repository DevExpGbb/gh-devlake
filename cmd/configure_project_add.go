package cmd

import (
	"time"

	"github.com/spf13/cobra"
)

func newProjectAddCmd() *cobra.Command {
	var opts ProjectOpts
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create a DevLake project and start data collection",
		Long: `Creates a DevLake project that groups data from your connections.

A project ties together existing scopes (repos, orgs) from your connections
into a single view with DORA metrics. It creates a sync schedule (blueprint)
that collects data on a cron schedule (daily by default).

Prerequisites: run 'gh devlake configure scope' first to add scopes.

This command will:
  1. Discover existing scopes on your connections
  2. Let you choose which scopes to include
  3. Create the project with DORA metrics enabled
  4. Configure a sync blueprint
  5. Trigger the first data collection

Example:
  gh devlake configure project add
  gh devlake configure project add --project-name my-team`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProjectAdd(cmd, args, &opts)
		},
	}

	cmd.Flags().StringVar(&opts.ProjectName, "project-name", "", "DevLake project name")
	cmd.Flags().StringVar(&opts.TimeAfter, "time-after", "", "Only collect data after this date (default: 6 months ago)")
	cmd.Flags().StringVar(&opts.Cron, "cron", "0 0 * * *", "Blueprint cron schedule")
	cmd.Flags().BoolVar(&opts.SkipSync, "skip-sync", false, "Skip triggering the first data sync")
	cmd.Flags().BoolVar(&opts.Wait, "wait", true, "Wait for pipeline to complete")
	cmd.Flags().DurationVar(&opts.Timeout, "timeout", 5*time.Minute, "Max time to wait for pipeline")

	return cmd
}

func runProjectAdd(cmd *cobra.Command, args []string, opts *ProjectOpts) error {
	return runConfigureProjects(cmd, args, opts)
}
