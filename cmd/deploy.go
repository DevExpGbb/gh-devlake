package cmd

import (
	"fmt"
	"strings"

	"github.com/DevExpGBB/gh-devlake/internal/prompt"
	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy a DevLake instance (local Docker or Azure)",
	Long: `Deploy an Apache DevLake stack to your local machine (Docker Compose)
or to Azure (Container Apps + Azure Database for MySQL).

Run without a subcommand for an interactive target prompt.`,
	RunE: runDeploy,
}

func runDeploy(cmd *cobra.Command, args []string) error {
	targets := []string{"local - Docker Compose on this machine", "azure - Azure Container Apps"}
	choice := prompt.Select("\nWhere would you like to deploy DevLake?", targets)
	if choice == "" {
		return fmt.Errorf("deployment target is required")
	}
	target := strings.SplitN(choice, " ", 2)[0]
	fmt.Printf("\n   Selected: %s\n", target)

	switch target {
	case "local":
		return runDeployLocal(cmd, args)
	case "azure":
		return runDeployAzure(cmd, args)
	default:
		return fmt.Errorf("unknown target: %s", target)
	}
}

func init() {
	deployCmd.GroupID = "deploy"
	rootCmd.AddCommand(deployCmd)
	deployCmd.AddCommand(newDeployAzureCmd())
	deployCmd.AddCommand(newDeployLocalCmd())

	initCmd := newInitCmd()
	initCmd.GroupID = "deploy"
	rootCmd.AddCommand(initCmd)

	cleanupCmd := newCleanupCmd()
	cleanupCmd.GroupID = "operate"
	rootCmd.AddCommand(cleanupCmd)

	startCmd := newStartCmd()
	startCmd.GroupID = "operate"
	rootCmd.AddCommand(startCmd)
}
