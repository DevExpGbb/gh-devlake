package cmd

import "github.com/spf13/cobra"

var configureConnectionsCmd = &cobra.Command{
	Use:     "connection",
	Aliases: []string{"connections"},
	Short:   "Manage plugin connections in DevLake",
	Long: `Manage DevLake plugin connections.

Use subcommands to add, list, update, delete, or test connections.`,
}
