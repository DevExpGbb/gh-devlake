package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/DevExpGBB/gh-devlake/internal/devlake"
	"github.com/DevExpGBB/gh-devlake/internal/prompt"
	"github.com/spf13/cobra"
)

var (
	initToken     string
	initEnvFile   string
	initSkipClean bool
)

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Guided setup wizard — deploy and configure DevLake in one step",
		Long: `Walks you through deploying and configuring DevLake from scratch.

The wizard will:
  1. Ask where to deploy (local Docker or Azure)
  2. Deploy DevLake and wait for it to be ready
  3. Create connections for your chosen plugins
  4. Configure scopes (repos for GitHub, org for Copilot, etc.)
  5. Create a project and trigger the first data sync

This command is fully interactive — use 'gh devlake configure full' for
flag-driven setup, or individual commands (configure connection, configure
scope, configure project) for fine-grained control.`,
		RunE: runInit,
	}

	cmd.Flags().StringVar(&initToken, "token", "", "Personal access token (avoids interactive prompt)")
	cmd.Flags().StringVar(&initEnvFile, "env-file", ".devlake.env", "Path to env file containing PAT")
	cmd.Flags().BoolVar(&initSkipClean, "skip-cleanup", false, "Do not delete .devlake.env after setup")

	return cmd
}

func runInit(cmd *cobra.Command, args []string) error {
	printBanner("DevLake — Setup Wizard\n  Deploy → Connect → Scope → Project")

	// ── Phase 1: Deploy ──────────────────────────────────────────
	targets := []string{"local - Docker Compose on this machine", "azure - Azure Container Apps"}
	choice := prompt.Select("\nWhere would you like to deploy DevLake?", targets)
	if choice == "" {
		return fmt.Errorf("deployment target is required")
	}
	target := strings.SplitN(choice, " ", 2)[0]
	fmt.Printf("\n   Selected: %s\n", target)

	if suggestDedicatedDir(target, "gh devlake init") {
		return nil
	}

	printPhaseBanner("PHASE 1: Deploy DevLake")

	switch target {
	case "local":
		if err := runInitLocal(cmd, args); err != nil {
			fmt.Println("\n💡 To clean up partial artifacts:")
			fmt.Println("   gh devlake cleanup --local --force")
			return fmt.Errorf("deployment failed: %w", err)
		}
	case "azure":
		if err := runInitAzure(cmd, args); err != nil {
			fmt.Println("\n💡 To clean up partial artifacts:")
			fmt.Println("   gh devlake cleanup --azure --force")
			return fmt.Errorf("deployment failed: %w", err)
		}
	}

	fmt.Println("\n🔍 Verifying DevLake is reachable...")
	disc, err := devlake.Discover(cfgURL)
	if err != nil {
		return fmt.Errorf("cannot reach DevLake after deploy: %w", err)
	}
	if disc.ConfigUIURL == "" || disc.GrafanaURL == "" {
		if loadedState, _ := devlake.LoadStateFromCwd(); loadedState != nil {
			if disc.ConfigUIURL == "" && loadedState.Endpoints.ConfigUI != "" {
				disc.ConfigUIURL = loadedState.Endpoints.ConfigUI
			}
			if disc.GrafanaURL == "" && loadedState.Endpoints.Grafana != "" {
				disc.GrafanaURL = loadedState.Endpoints.Grafana
			}
		}
	}
	fmt.Printf("   ✅ Backend API: %s (via %s)\n", disc.URL, disc.Source)
	if disc.ConfigUIURL != "" {
		fmt.Printf("   ✅ Config UI:   %s\n", disc.ConfigUIURL)
	}
	if disc.GrafanaURL != "" {
		fmt.Printf("   ✅ Grafana:     %s\n", disc.GrafanaURL)
	}

	// ── Phases 2–4: Configure (connections → scopes → project) ──
	printPhaseBanner("PHASE 2: Configure")

	if err := configureAllPhases(ConfigureAllOpts{
		Token:     initToken,
		EnvFile:   initEnvFile,
		SkipClean: initSkipClean,
		ReAddLoop: true,
	}); err != nil {
		return err
	}

	// ── Summary ──────────────────────────────────────────────────
	printBanner("✅ DevLake is ready!")

	fmt.Printf("  Backend:   %s\n", disc.URL)
	if disc.ConfigUIURL != "" {
		fmt.Printf("  Config UI: %s\n", disc.ConfigUIURL)
	}
	if disc.GrafanaURL != "" {
		fmt.Printf("  Grafana:   %s\n", disc.GrafanaURL)
	}
	fmt.Println("\nNext steps:")
	fmt.Println("  • Open Grafana and explore the DORA dashboard")
	fmt.Println("  • Run 'gh devlake status' to check health")
	fmt.Println("  • Run 'gh devlake cleanup' when finished")

	return nil
}

// runInitLocal handles the local deployment path of the wizard.
// It delegates entirely to runDeployLocal which owns the image-choice prompt,
// download/clone, and container startup.
func runInitLocal(cmd *cobra.Command, args []string) error {
	deployLocalDir = "."
	deployLocalVersion = "latest"
	deployLocalQuiet = true
	deployLocalSource = "" // let runDeployLocal prompt interactively

	if err := runDeployLocal(cmd, args); err != nil {
		return err
	}

	absDir, _ := filepath.Abs(deployLocalDir)
	if absDir != "" && deployLocalDir != "." {
		if err := os.Chdir(absDir); err != nil {
			return fmt.Errorf("failed to change directory to %s: %w", absDir, err)
		}
	}

	return nil
}

// runInitAzure handles the Azure deployment path of the wizard.
// It delegates entirely to runDeployAzure which owns the image-choice prompt
// and Azure provisioning.
func runInitAzure(cmd *cobra.Command, args []string) error {
	deployAzureQuiet = true

	if err := runDeployAzure(cmd, args); err != nil {
		return err
	}
	if deployAzureDir != "" {
		absDir, _ := filepath.Abs(deployAzureDir)
		if absDir != "" {
			if err := os.Chdir(absDir); err != nil {
				return fmt.Errorf("failed to change directory to %s: %w", absDir, err)
			}
		}
	}

	statePath := ".devlake-azure.json"
	if deployAzureDir != "" {
		statePath = filepath.Join(deployAzureDir, ".devlake-azure.json")
	}
	loadedState, _ := devlake.LoadState(statePath)
	if loadedState != nil && loadedState.Endpoints.Backend != "" {
		cfgURL = loadedState.Endpoints.Backend
	}

	return nil
}
