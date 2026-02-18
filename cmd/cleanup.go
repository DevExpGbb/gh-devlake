package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/DevExpGBB/gh-devlake/internal/azure"
	dockerpkg "github.com/DevExpGBB/gh-devlake/internal/docker"
	"github.com/DevExpGBB/gh-devlake/internal/prompt"
	"github.com/spf13/cobra"
)

var (
	cleanupForce  bool
	cleanupState  string
	cleanupKeepRG bool
	cleanupAzure  bool
	cleanupLocal  bool
)

func newCleanupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Tear down DevLake resources",
		Long: `Removes DevLake resources. Auto-detects deployment type from state files.

For local: stops Docker Compose containers.
For Azure: deletes the resource group (or individual resources with --keep-resource-group).

Example:
  gh devlake cleanup
  gh devlake cleanup --azure --force
  gh devlake cleanup --local`,
		RunE: runCleanup,
	}

	cmd.Flags().BoolVar(&cleanupForce, "force", false, "Skip confirmation prompt")
	cmd.Flags().StringVar(&cleanupState, "state-file", "", "Path to state file (auto-detected if omitted)")
	cmd.Flags().BoolVar(&cleanupKeepRG, "keep-resource-group", false, "Delete resources but keep the Azure resource group")
	cmd.Flags().BoolVar(&cleanupAzure, "azure", false, "Force Azure cleanup mode")
	cmd.Flags().BoolVar(&cleanupLocal, "local", false, "Force local cleanup mode")

	return cmd
}

// azureStateData holds the parsed Azure state file.
type azureStateData struct {
	DeployedAt    string `json:"deployedAt"`
	ResourceGroup string `json:"resourceGroup"`
	Region        string `json:"region"`
	Resources     struct {
		ACR        any      `json:"acr"`
		KeyVault   string   `json:"keyVault"`
		MySQL      string   `json:"mysql"`
		Containers []string `json:"containers"`
	} `json:"resources"`
	Endpoints struct {
		Backend  string `json:"backend"`
		ConfigUI string `json:"configUi"`
		Grafana  string `json:"grafana"`
	} `json:"endpoints"`
}

func runCleanup(cmd *cobra.Command, args []string) error {
	// Determine mode
	mode := detectCleanupMode()
	if mode == "" {
		return fmt.Errorf("could not determine deployment type.\nUse --azure or --local, or ensure a state file exists")
	}

	switch mode {
	case "azure":
		return runAzureCleanup()
	case "local":
		return runLocalCleanup()
	default:
		return fmt.Errorf("unknown cleanup mode: %s", mode)
	}
}

func detectCleanupMode() string {
	if cleanupAzure {
		return "azure"
	}
	if cleanupLocal {
		return "local"
	}

	// Check for state files
	if cleanupState != "" {
		if _, err := os.Stat(cleanupState); err == nil {
			// Guess from filename
			if contains(cleanupState, "azure") {
				return "azure"
			}
			return "local"
		}
	}

	if _, err := os.Stat(".devlake-azure.json"); err == nil {
		return "azure"
	}
	if _, err := os.Stat(".devlake-local.json"); err == nil {
		return "local"
	}
	return ""
}

func runAzureCleanup() error {
	fmt.Println()
	fmt.Println("════════════════════════════════════════")
	fmt.Println("  DevLake Azure Cleanup")
	fmt.Println("════════════════════════════════════════")

	stateFile := cleanupState
	if stateFile == "" {
		stateFile = ".devlake-azure.json"
	}

	data, err := os.ReadFile(stateFile)
	if err != nil {
		return fmt.Errorf("state file not found: %s\nExpected .devlake-azure.json or use --state-file", stateFile)
	}

	var state azureStateData
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("invalid state file: %w", err)
	}

	fmt.Printf("\nDeployment found:\n")
	fmt.Printf("  Deployed:       %s\n", state.DeployedAt)
	fmt.Printf("  Resource Group: %s\n", state.ResourceGroup)
	fmt.Printf("  Region:         %s\n", state.Region)

	fmt.Printf("\nResources to delete:\n")
	if acrName, ok := state.Resources.ACR.(string); ok && acrName != "" {
		fmt.Printf("  Container Registry: %s\n", acrName)
	}
	fmt.Printf("  Key Vault:   %s\n", state.Resources.KeyVault)
	fmt.Printf("  MySQL:       %s\n", state.Resources.MySQL)
	for _, c := range state.Resources.Containers {
		fmt.Printf("  Container:   %s\n", c)
	}

	fmt.Printf("\nEndpoints that will be removed:\n")
	fmt.Printf("  Backend:  %s\n", state.Endpoints.Backend)
	fmt.Printf("  Config UI: %s\n", state.Endpoints.ConfigUI)
	fmt.Printf("  Grafana:  %s\n", state.Endpoints.Grafana)

	if !cleanupForce {
		if !prompt.Confirm("\nAre you sure you want to delete ALL these resources?") {
			fmt.Println("Cleanup cancelled.")
			return nil
		}
	}

	// Check Azure login
	fmt.Println("\n🔑 Checking Azure CLI login...")
	_, err = azure.CheckLogin()
	if err != nil {
		fmt.Println("   Not logged in. Running az login...")
		if loginErr := azure.Login(); loginErr != nil {
			return fmt.Errorf("az login failed: %w", loginErr)
		}
	}

	if cleanupKeepRG {
		// Delete individual resources
		for _, container := range state.Resources.Containers {
			fmt.Printf("\n   Deleting container %s...\n", container)
			_ = azure.DeleteResource("container", container, state.ResourceGroup)
		}
		fmt.Println("\n   Deleting MySQL server...")
		_ = azure.DeleteResource("mysql", state.Resources.MySQL, state.ResourceGroup)

		if acrName, ok := state.Resources.ACR.(string); ok && acrName != "" {
			fmt.Println("\n   Deleting Container Registry...")
			_ = azure.DeleteResource("acr", acrName, state.ResourceGroup)
		}

		fmt.Println("\n   Deleting Key Vault...")
		_ = azure.DeleteResource("keyvault", state.Resources.KeyVault, state.ResourceGroup)

		fmt.Println("   Purging soft-deleted Key Vault...")
		_ = azure.PurgeKeyVault(state.Resources.KeyVault, state.Region)

		fmt.Printf("\n   Resource group %q kept.\n", state.ResourceGroup)
	} else {
		fmt.Printf("\n   Deleting resource group %q...\n", state.ResourceGroup)
		if err := azure.DeleteResourceGroup(state.ResourceGroup); err != nil {
			return fmt.Errorf("failed to delete resource group: %w", err)
		}
		fmt.Println("   Deletion initiated (running in background)")
	}

	// Remove state file
	fmt.Println("\n   Removing state file...")
	if err := os.Remove(stateFile); err != nil && !os.IsNotExist(err) {
		fmt.Printf("   ⚠️  Could not remove state file: %v\n", err)
	} else {
		fmt.Println("   ✅ State file removed")
	}

	fmt.Println("\n════════════════════════════════════════")
	fmt.Println("  ✅ Cleanup Complete!")
	fmt.Println("════════════════════════════════════════")
	fmt.Println()

	if !cleanupKeepRG {
		fmt.Println("\nNote: Resource group deletion runs in background.")
		fmt.Printf("Check status: az group show --name %s\n", state.ResourceGroup)
	}

	return nil
}

func runLocalCleanup() error {
	fmt.Println()
	fmt.Println("════════════════════════════════════════")
	fmt.Println("  DevLake Local Cleanup")
	fmt.Println("════════════════════════════════════════")

	stateFile := cleanupState
	if stateFile == "" {
		stateFile = ".devlake-local.json"
	}

	if !cleanupForce {
		if !prompt.Confirm("\nStop and remove local DevLake Docker containers?") {
			fmt.Println("Cleanup cancelled.")
			return nil
		}
	}

	fmt.Println("\n🐳 Running docker compose down...")
	cwd, _ := os.Getwd()
	if err := dockerpkg.ComposeDown(cwd); err != nil {
		fmt.Printf("   ⚠️  docker compose down failed: %v\n", err)
		fmt.Println("   You may need to stop containers manually.")
	} else {
		fmt.Println("   ✅ Containers stopped and removed")
	}

	// Remove state file
	if _, err := os.Stat(stateFile); err == nil {
		fmt.Printf("\n   Removing %s...\n", stateFile)
		if err := os.Remove(stateFile); err != nil {
			fmt.Printf("   ⚠️  Could not remove state file: %v\n", err)
		} else {
			fmt.Println("   ✅ State file removed")
		}
	}

	fmt.Println("\n════════════════════════════════════════")
	fmt.Println("  ✅ Cleanup Complete!")
	fmt.Println("════════════════════════════════════════")
	fmt.Println()

	return nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
