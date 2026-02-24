package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

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

	// Preflight: if the user is inside a git repo, recommend exiting and re-running
	// from a dedicated directory so copy/paste commands work naturally.
	if repoRoot, ok := findGitRepoRoot("."); ok {
		fmt.Println()
		homeDirTip("devlake")
		fmt.Printf("\n⚠️  You're running inside a git repository: %s\n", repoRoot)
		fmt.Println("   It's recommended to run this wizard from a dedicated directory.")
		fmt.Println("   (A CLI cannot change your terminal's working directory.)")
		fmt.Println()
		choices := []string{
			"exit - show mkdir/cd commands and re-run (recommended)",
			"continue - keep going in this directory",
		}
		picked := prompt.Select("How do you want to proceed?", choices)
		if strings.HasPrefix(picked, "exit") {
			printDedicatedDirCopyPaste(target)
			fmt.Println("\n✅ Exiting wizard. Re-run after changing directory.")
			fmt.Println()
			return nil
		}
	}

	printPhaseBanner("PHASE 1: Deploy DevLake")

	switch target {
	case "local":
		if err := runInitLocal(cmd, args); err != nil {
			return fmt.Errorf("deployment failed: %w", err)
		}
	case "azure":
		if err := runInitAzure(cmd, args); err != nil {
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

	client := devlake.NewClient(disc.URL)
	statePath, state := devlake.FindStateFile(disc.URL, disc.GrafanaURL)

	// ── Phase 2: Configure Connections ───────────────────────────
	printPhaseBanner("PHASE 2: Configure Connections")

	available := AvailableConnections()
	var results []ConnSetupResult

	for {
		// Always show all available plugins so users can add multiple
		// connections of the same plugin (e.g. multiple GitHub connections).
		remaining := available

		if len(results) > 0 {
			fmt.Println()
			fmt.Println("   " + strings.Repeat("─", 44))
			fmt.Println("   Connections configured so far:")
			for _, r := range results {
				name := r.Plugin
				if def := FindConnectionDef(r.Plugin); def != nil {
					name = def.DisplayName
				}
				fmt.Printf("     ✅ %-18s  ID=%d  %q\n", name, r.ConnectionID, r.Name)
			}
			fmt.Println("   " + strings.Repeat("─", 44))
		}

		var remainingLabels []string
		for _, d := range remaining {
			remainingLabels = append(remainingLabels, d.DisplayName)
		}

		fmt.Println()
		selectedLabels := prompt.SelectMulti("Which connections to set up?", remainingLabels)
		if len(selectedLabels) == 0 {
			if len(results) == 0 {
				return fmt.Errorf("at least one connection is required")
			}
			break
		}

		var selectedDefs []*ConnectionDef
		for _, label := range selectedLabels {
			for _, d := range remaining {
				if d.DisplayName == label {
					selectedDefs = append(selectedDefs, d)
					break
				}
			}
		}
		if len(selectedDefs) == 0 {
			if len(results) == 0 {
				return fmt.Errorf("at least one connection is required")
			}
			break
		}

		newResults, _, _, _, err := runConnectionsInternal(selectedDefs, "", "", initToken, initEnvFile, initSkipClean)
		if err != nil {
			if len(results) == 0 {
				return fmt.Errorf("connection setup failed: %w", err)
			}
			fmt.Printf("\n   ⚠️  %v\n", err)
			if !prompt.Confirm("\nWould you like to try another connection?") {
				break
			}
			continue
		}
		for _, r := range newResults {
			results = append(results, r)
		}
		fmt.Println("\n   ✅ Connections configured.")

		// Reload state after connections were saved
		statePath, state = devlake.FindStateFile(disc.URL, disc.GrafanaURL)

		if !prompt.Confirm("\nWould you like to add another connection?") {
			break
		}
	}

	// Resolve org and enterprise from connection results
	org := ""
	enterprise := ""
	for _, r := range results {
		if r.Organization != "" && org == "" {
			org = r.Organization
		}
		if r.Enterprise != "" && enterprise == "" {
			enterprise = r.Enterprise
		}
	}

	// ── Phase 3: Configure Scopes ────────────────────────────────
	printPhaseBanner("PHASE 3: Configure Scopes")
	scopeAllConnections(client, results)
	fmt.Println("\n   ✅ Scopes configured.")

	// ── Phase 4: Create Project ──────────────────────────────────
	printPhaseBanner("PHASE 4: Project Setup")

	err = collectAndFinalizeProject(collectProjectOpts{
		Client:    client,
		Results:   results,
		StatePath: statePath,
		State:     state,
		Org:       org,
		Wait:      true,
		Timeout:   5 * time.Minute,
	})
	if err != nil {
		return fmt.Errorf("project setup failed: %w", err)
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
	fmt.Printf("  Org:       %s\n", org)
	fmt.Println("\nNext steps:")
	fmt.Println("  • Open Grafana and explore the DORA dashboard")
	fmt.Println("  • Run 'gh devlake status' to check health")
	fmt.Println("  • Run 'gh devlake cleanup' when finished")

	return nil
}

func printDedicatedDirCopyPaste(target string) {
	// Choose a sensible default folder name.
	home, _ := os.UserHomeDir()
	folder := "devlake"
	if target == "azure" {
		folder = "devlake-azure"
	}
	defaultDir := folder
	if home != "" {
		defaultDir = filepath.Join(home, folder)
	}

	fmt.Println("\nCopy/paste to use a dedicated directory:")
	if runtime.GOOS == "windows" {
		fmt.Println("  PowerShell:")
		fmt.Printf("    $dir = \"%s\"\n", defaultDir)
		fmt.Println("    New-Item -ItemType Directory -Force -Path $dir | Out-Null")
		fmt.Println("    Set-Location $dir")
		fmt.Println("    gh devlake init")
		return
	}

	// POSIX shells
	fmt.Println("  Bash/Zsh:")
	fmt.Printf("    dir=\"%s\"\n", defaultDir)
	fmt.Println("    mkdir -p \"$dir\"")
	fmt.Println("    cd \"$dir\"")
	fmt.Println("    gh devlake init")
}

// runInitLocal handles the local deployment path of the wizard.
func runInitLocal(cmd *cobra.Command, args []string) error {
	deployLocalDir = "."
	deployLocalVersion = "latest"
	deployLocalQuiet = true
	deployLocalOfficial = true // default; overridden by prompt below

	// If the user selects a dedicated directory, the wizard should operate from
	// that directory so state files (.devlake-local.json) land next to the
	// downloaded docker-compose.yml and .env.

	// Suggest a dedicated directory when running inside a git repo.
	if repoRoot, ok := findGitRepoRoot("."); ok {
		homeDirTip("devlake")
		fmt.Printf("\n⚠️  You're running inside a git repository: %s\n", repoRoot)
		fmt.Println("   This wizard will download docker-compose.yml and write .env in the current directory.")
		fmt.Println("")
		if prompt.Confirm("Use a dedicated directory instead?") {
			home, _ := os.UserHomeDir()
			defaultDir := "devlake"
			if home != "" {
				defaultDir = filepath.Join(home, "devlake")
			}
			fmt.Println()
			chosen := prompt.ReadLine(fmt.Sprintf("Target directory (e.g. %s)", defaultDir))
			if chosen != "" {
				deployLocalDir = chosen
			}
		}
	}

	imageChoices := []string{
		"official - Apache DevLake images from GitHub releases (recommended)",
		"custom  - Skip download, use your own docker-compose.yml",
	}
	fmt.Println()
	imgChoice := prompt.Select("Which DevLake images to use?", imageChoices)
	if imgChoice == "" {
		return fmt.Errorf("image choice is required")
	}
	deployLocalOfficial = strings.HasPrefix(imgChoice, "official")

	if err := runDeployLocal(cmd, args); err != nil {
		return err
	}

	absDir, _ := filepath.Abs(deployLocalDir)
	if absDir != "" && deployLocalDir != "." {
		if err := os.Chdir(absDir); err != nil {
			return fmt.Errorf("failed to change directory to %s: %w", absDir, err)
		}
	}

	backendURL, err := startLocalContainers(absDir)
	if err != nil {
		return err
	}
	cfgURL = backendURL

	fmt.Println("\n🔄 Triggering database migration...")
	migClient := devlake.NewClient(backendURL)
	if err := migClient.TriggerMigration(); err != nil {
		fmt.Printf("   ⚠️  Migration may need manual trigger: %v\n", err)
	} else {
		fmt.Println("   ✅ Migration triggered")
		time.Sleep(5 * time.Second)
	}

	return nil
}

// runInitAzure handles the Azure deployment path of the wizard.
func runInitAzure(cmd *cobra.Command, args []string) error {
	deployAzureQuiet = true

	// Keep wizard state files with the Azure deployment state file when --dir is used.

	// Suggest a dedicated directory when running inside a git repo.
	if repoRoot, ok := findGitRepoRoot("."); ok {
		homeDirTip("devlake")
		fmt.Printf("\n⚠️  You're running inside a git repository: %s\n", repoRoot)
		fmt.Println("   This wizard will write .devlake-azure.json (state) in the current directory.")
		fmt.Println("")
		if prompt.Confirm("Use a dedicated directory instead?") {
			home, _ := os.UserHomeDir()
			defaultDir := "devlake-azure"
			if home != "" {
				defaultDir = filepath.Join(home, "devlake-azure")
			}
			fmt.Println()
			chosen := prompt.ReadLine(fmt.Sprintf("Target directory (e.g. %s)", defaultDir))
			if chosen != "" {
				deployAzureDir = chosen
			}
		}
	}

	imageChoices := []string{
		"official - Apache DevLake images from Docker Hub (recommended)",
		"custom  - Build from a DevLake repository (fork or clone)",
	}
	imgChoice := prompt.Select("Which DevLake images to use?", imageChoices)
	if imgChoice == "" {
		return fmt.Errorf("image choice is required")
	}
	if strings.HasPrefix(imgChoice, "official") {
		azureOfficial = true
	} else {
		azureOfficial = false
		if azureRepoURL == "" {
			azureRepoURL = prompt.ReadLine("Path or URL to DevLake repo (leave blank to auto-detect)")
		}
	}

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
