// Package cmd contains the cobra command tree for gh-devlake.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var cfgURL string // --url flag (global)

var rootCmd = &cobra.Command{
	Use:   "devlake",
	Short: "Manage Apache DevLake deployments and configuration",
	Long: `gh devlake — a GitHub CLI extension for Apache DevLake.

Deploy, configure, and manage DevLake instances from the command line.

Typical workflow:
  1. gh devlake deploy local          # spin up DevLake with Docker Compose
  2. gh devlake configure full --org myorg  # create connections + scopes
  3. gh devlake status                # verify everything is healthy
  4. gh devlake cleanup               # tear down when finished`,
}

func init() {
	cobra.EnableCommandSorting = false

	rootCmd.PersistentFlags().StringVar(&cfgURL, "url", "", "DevLake API base URL (auto-discovered if omitted)")

	rootCmd.AddGroup(
		&cobra.Group{ID: "deploy", Title: "Deployment:"},
		&cobra.Group{ID: "configure", Title: "Configuration:"},
		&cobra.Group{ID: "operate", Title: "Operations:"},
	)
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
