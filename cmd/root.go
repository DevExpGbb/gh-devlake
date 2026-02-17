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
Automates connection setup, scope configuration, and health monitoring.`,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgURL, "url", "", "DevLake API base URL (auto-discovered if omitted)")
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
