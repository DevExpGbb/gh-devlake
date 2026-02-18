package cmd

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/DevExpGBB/gh-devlake/internal/devlake"
	"github.com/DevExpGBB/gh-devlake/internal/prompt"
	"github.com/spf13/cobra"
)

var (
	initOrg        string
	initEnterprise string
	initToken      string
	initEnvFile    string
	initRepos      string
	initReposFile  string
)

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Guided setup wizard — deploy and configure DevLake in one step",
		Long: `Walks you through deploying and configuring DevLake from scratch.

The wizard will:
  1. Ask where to deploy (local Docker or Azure)
  2. Deploy DevLake and wait for it to be ready
  3. Create GitHub and Copilot connections
  4. Configure repository scopes, DORA metrics, and trigger the first sync

You can also pass flags to pre-fill answers and skip prompts:
  gh devlake init --org my-org --repos owner/repo1,owner/repo2`,
		RunE: runInit,
	}

	cmd.Flags().StringVar(&initOrg, "org", "", "GitHub organization slug")
	cmd.Flags().StringVar(&initEnterprise, "enterprise", "", "GitHub enterprise slug (for Copilot enterprise metrics)")
	cmd.Flags().StringVar(&initToken, "token", "", "GitHub PAT")
	cmd.Flags().StringVar(&initEnvFile, "env-file", ".devlake.env", "Path to env file containing GITHUB_PAT")
	cmd.Flags().StringVar(&initRepos, "repos", "", "Comma-separated repos (owner/repo)")
	cmd.Flags().StringVar(&initReposFile, "repos-file", "", "Path to file with repos (one per line)")

	return cmd
}

func runInit(cmd *cobra.Command, args []string) error {
	fmt.Println()
	fmt.Println("════════════════════════════════════════")
	fmt.Println("  DevLake — Setup Wizard")
	fmt.Println("  Deploy → Connect → Configure")
	fmt.Println("════════════════════════════════════════")

	// ── Phase 1: Choose deployment target ──
	targets := []string{"local - Docker Compose on this machine", "azure - Azure Container Apps"}
	choice := prompt.Select("Where would you like to deploy DevLake?", targets)
	if choice == "" {
		return fmt.Errorf("deployment target is required")
	}
	target := strings.SplitN(choice, " ", 2)[0] // "local" or "azure"

	fmt.Printf("\n   Selected: %s\n", target)

	// ── Phase 2: Deploy ──
	fmt.Println("\n╔══════════════════════════════════════╗")
	fmt.Println("║  PHASE 1: Deploy DevLake             ║")
	fmt.Println("╚══════════════════════════════════════╝")

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

	// ── Phase 3: Verify DevLake is reachable ──
	fmt.Println("\n🔍 Verifying DevLake is reachable...")
	disc, err := devlake.Discover(cfgURL)
	if err != nil {
		return fmt.Errorf("cannot reach DevLake after deploy: %w", err)
	}
	fmt.Printf("   ✅ DevLake at %s (via %s)\n", disc.URL, disc.Source)

	// ── Phase 4: Configure connections ──
	fmt.Println("\n╔══════════════════════════════════════╗")
	fmt.Println("║  PHASE 2: Configure Connections      ║")
	fmt.Println("╚══════════════════════════════════════╝")

	if initOrg == "" {
		initOrg = prompt.ReadLine("GitHub organization slug")
		if initOrg == "" {
			return fmt.Errorf("--org is required")
		}
	}

	// ── Select connections ──
	available := AvailableConnections()
	var availLabels []string
	for _, d := range available {
		availLabels = append(availLabels, d.DisplayName)
	}
	selectedLabels := prompt.SelectMultiWithDefaults(
		"Which connections to set up? (GitHub + Copilot recommended)",
		availLabels,
		[]int{1, 2},
	)
	var selectedDefs []*ConnectionDef
	for _, label := range selectedLabels {
		for _, d := range available {
			if d.DisplayName == label {
				selectedDefs = append(selectedDefs, d)
				break
			}
		}
	}
	if len(selectedDefs) == 0 {
		selectedDefs = available // fallback: configure all
	}

	results, devlakeURL, _, err := runConnectionsInternal(selectedDefs, initOrg, initEnterprise, initToken, initEnvFile, true)
	if err != nil {
		return fmt.Errorf("connection setup failed: %w", err)
	}
	fmt.Println("\n   ✅ Connections configured.")

	// ── Phase 5: Configure scopes ──
	fmt.Println("\n╔══════════════════════════════════════╗")
	fmt.Println("║  PHASE 3: Project, Scopes & Sync    ║")
	fmt.Println("╚══════════════════════════════════════╝")

	// Wire connection results into scope vars
	scopeOrg = initOrg
	scopeSkipCopilot = true
	scopeSkipGitHub = true
	for _, r := range results {
		switch r.Plugin {
		case "github":
			scopeGHConnID = r.ConnectionID
			scopeSkipGitHub = false
		case "gh-copilot":
			scopeCopilotConnID = r.ConnectionID
			scopeSkipCopilot = false
		}
	}
	if devlakeURL != "" {
		cfgURL = devlakeURL
	}

	// Wire repo flags if provided
	if initRepos != "" {
		scopeRepos = initRepos
	}
	if initReposFile != "" {
		scopeReposFile = initReposFile
	}

	// Use sensible DORA defaults — prompt to confirm
	fmt.Println("\n   Default DORA patterns:")
	fmt.Printf("     Deployment: %s\n", scopeDeployPattern)
	fmt.Printf("     Production: %s\n", scopeProdPattern)
	fmt.Printf("     Incidents:  label=%s\n", scopeIncidentLabel)
	if !prompt.Confirm("Use these defaults?") {
		scopeDeployPattern = prompt.ReadLine("Deployment workflow regex")
		if scopeDeployPattern == "" {
			scopeDeployPattern = "(?i)deploy"
		}
		scopeProdPattern = prompt.ReadLine("Production environment regex")
		if scopeProdPattern == "" {
			scopeProdPattern = "(?i)prod"
		}
		scopeIncidentLabel = prompt.ReadLine("Incident issue label")
		if scopeIncidentLabel == "" {
			scopeIncidentLabel = "incident"
		}
	}

	if err := runConfigureScopes(cmd, args); err != nil {
		return fmt.Errorf("scope configuration failed: %w", err)
	}

	// ── Summary ──
	fmt.Println("\n════════════════════════════════════════")
	fmt.Println("  ✅ DevLake is ready!")
	fmt.Println("════════════════════════════════════════")
	fmt.Println()

	disc, _ = devlake.Discover(cfgURL)
	if disc != nil {
		fmt.Printf("\n  Backend:  %s\n", disc.URL)
		if disc.GrafanaURL != "" {
			fmt.Printf("  Grafana:  %s\n", disc.GrafanaURL)
		}
	}
	fmt.Printf("  Org:      %s\n", initOrg)
	fmt.Println("\nNext steps:")
	fmt.Println("  • Open Grafana and explore the DORA dashboard")
	fmt.Println("  • Run 'gh devlake status' to check health")
	fmt.Println("  • Run 'gh devlake cleanup' when finished")

	return nil
}

// runInitLocal handles the local deployment path of the wizard.
func runInitLocal(cmd *cobra.Command, args []string) error {
	// Use defaults for local deploy
	deployLocalDir = "."
	deployLocalVersion = "latest"
	deployLocalQuiet = true // wizard handles next steps

	if err := runDeployLocal(cmd, args); err != nil {
		return err
	}

	// Start containers and wait for health
	absDir, _ := filepath.Abs(deployLocalDir)
	backendURL, err := startLocalContainers(absDir)
	if err != nil {
		return err
	}
	cfgURL = backendURL

	// Trigger migration
	fmt.Println("\n🔄 Triggering database migration...")
	client := devlake.NewClient(backendURL)
	if err := client.TriggerMigration(); err != nil {
		fmt.Printf("   ⚠️  Migration may need manual trigger: %v\n", err)
	} else {
		fmt.Println("   ✅ Migration triggered")
		// Give migration a moment
		time.Sleep(5 * time.Second)
	}

	return nil
}

// runInitAzure handles the Azure deployment path of the wizard.
func runInitAzure(cmd *cobra.Command, args []string) error {
	deployAzureQuiet = true // wizard handles next steps

	// Ask whether to use official images or a custom build
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
		// Prompt for repo path / URL if not already set
		if azureRepoURL == "" {
			azureRepoURL = prompt.ReadLine("Path or URL to DevLake repo (leave blank to auto-detect)")
		}
	}

	// runDeployAzure already has interactive prompts for region + RG
	if err := runDeployAzure(cmd, args); err != nil {
		return err
	}

	// Read endpoint from state file
	state, _ := devlake.LoadState(".devlake-azure.json")
	if state != nil && state.Endpoints.Backend != "" {
		cfgURL = state.Endpoints.Backend
	}

	return nil
}
