package cmd

import (
	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy a DevLake instance (local Docker or Azure)",
	Long: `Deploy an Apache DevLake stack to your local machine (Docker Compose)
or to Azure (Container Apps + Azure Database for MySQL).`,
}

func init() {
	deployCmd.GroupID = "deploy"
	rootCmd.AddCommand(deployCmd)
	deployCmd.AddCommand(newDeployLocalCmd())
	deployCmd.AddCommand(newDeployAzureCmd())

	cleanupCmd := newCleanupCmd()
	cleanupCmd.GroupID = "operate"
	rootCmd.AddCommand(cleanupCmd)
}
