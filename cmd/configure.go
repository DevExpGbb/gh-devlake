package cmd

import (
	"github.com/spf13/cobra"
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Configure connections, scopes, and blueprints",
	Long:  `Set up DevLake data connections and collection scopes.`,
}

func init() {
	configureCmd.GroupID = "configure"
	rootCmd.AddCommand(configureCmd)
}
