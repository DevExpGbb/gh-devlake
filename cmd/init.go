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
	initToken   string
	initEnvFile string
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

	return cmd
}

func runInit(cmd *cobra.Command, args []string) error {
	fmt.Println()
	fmt.Println("════════════════════════════════════════")
	fmt.Println("  DevLake — Setup Wizard")
	fmt.Println("  Deploy → Connect → Scope → Project")
	fmt.Println("════════════════════════════════════════")

	// ── Phase 1: Deploy ──────────────────────────────────────────
	targets := []string{"local - Docker Compose on this machine", "azure - Azure Container Apps"}
	choice := prompt.Select("\nWhere would you like to deploy DevLake?", targets)
	if choice == "" {
		return fmt.Errorf("deployment target is required")
	}
	target := strings.SplitN(choice, " ", 2)[0]
	fmt.Printf("\n   Selected: %s\n", target)

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

	fmt.Println("\n🔍 Verifying DevLake is reachable...")
	disc, err := devlake.Discover(cfgURL)
	if err != nil {
		return fmt.Errorf("cannot reach DevLake after deploy: %w", err)
	}
	fmt.Printf("   ✅ DevLake at %s (via %s)\n", disc.URL, disc.Source)

	client := devlake.NewClient(disc.URL)
	statePath, state := devlake.FindStateFile(disc.URL, disc.GrafanaURL)

	// ── Phase 2: Configure Connections ───────────────────────────
	fmt.Println("\n╔══════════════════════════════════════╗")
	fmt.Println("║  PHASE 2: Configure Connections      ║")
	fmt.Println("╚══════════════════════════════════════╝")

	available := AvailableConnections()
	var availLabels []string
	for _, d := range available {
		availLabels = append(availLabels, d.DisplayName)
	}

	fmt.Println()
	selectedLabels := prompt.SelectMulti("Which connections to set up?", availLabels)
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
		return fmt.Errorf("at least one connection is required")
	}

	results, _, _, _, err := runConnectionsInternal(selectedDefs, "", "", initToken, initEnvFile, true)
	if err != nil {
		return fmt.Errorf("connection setup failed: %w", err)
	}
	if len(results) == 0 {
		return fmt.Errorf("no connections were created — cannot continue")
	}
	fmt.Println("\n   ✅ Connections configured.")

	// Reload state after connections were saved
	statePath, state = devlake.FindStateFile(disc.URL, disc.GrafanaURL)

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
	fmt.Println("\n╔══════════════════════════════════════╗")
	fmt.Println("║  PHASE 3: Configure Scopes           ║")
	fmt.Println("╚══════════════════════════════════════╝")

	for _, r := range results {
		fmt.Printf("\n📡 Configuring scopes for %s (connection %d)...\n",
			pluginDisplayName(r.Plugin), r.ConnectionID)

		switch r.Plugin {
		case "github":
			scopeOpts := &ScopeOpts{
				DeployPattern: "(?i)deploy",
				ProdPattern:   "(?i)prod",
				IncidentLabel: "incident",
			}

			fmt.Println("\n   Default DORA patterns:")
			fmt.Printf("     Deployment: %s\n", scopeOpts.DeployPattern)
			fmt.Printf("     Production: %s\n", scopeOpts.ProdPattern)
			fmt.Printf("     Incidents:  label=%s\n", scopeOpts.IncidentLabel)
			if !prompt.Confirm("   Use these defaults?") {
				v := prompt.ReadLine("   Deployment workflow regex")
				if v != "" {
					scopeOpts.DeployPattern = v
				}
				v = prompt.ReadLine("   Production environment regex")
				if v != "" {
					scopeOpts.ProdPattern = v
				}
				v = prompt.ReadLine("   Incident issue label")
				if v != "" {
					scopeOpts.IncidentLabel = v
				}
			}

			_, err := scopeGitHub(client, r.ConnectionID, r.Organization, scopeOpts)
			if err != nil {
				fmt.Printf("   ⚠️  GitHub scope setup failed: %v\n", err)
			}

		case "gh-copilot":
			_, err := scopeCopilot(client, r.ConnectionID, r.Organization, r.Enterprise)
			if err != nil {
				fmt.Printf("   ⚠️  Copilot scope setup failed: %v\n", err)
			}

		default:
			fmt.Printf("   ⚠️  Scope configuration for %q is not yet supported\n", r.Plugin)
		}
	}
	fmt.Println("\n   ✅ Scopes configured.")

	// ── Phase 4: Create Project ──────────────────────────────────
	fmt.Println("\n╔══════════════════════════════════════╗")
	fmt.Println("║  PHASE 4: Project Setup              ║")
	fmt.Println("╚══════════════════════════════════════╝")

	defaultProject := org
	if defaultProject == "" {
		defaultProject = "my-project"
	}
	projectName := prompt.ReadLine(fmt.Sprintf("\nProject name [%s]", defaultProject))
	if projectName == "" {
		projectName = defaultProject
	}

	// List existing scopes on each connection
	var connections []devlake.BlueprintConnection
	var allRepos []string
	var pluginNames []string

	for _, r := range results {
		choice := connChoice{
			plugin:     r.Plugin,
			id:         r.ConnectionID,
			label:      fmt.Sprintf("%s (ID: %d)", pluginDisplayName(r.Plugin), r.ConnectionID),
			enterprise: r.Enterprise,
		}
		ac, err := listConnectionScopes(client, choice, r.Organization, r.Enterprise)
		if err != nil {
			fmt.Printf("   ⚠️  Could not list scopes for %s: %v\n", choice.label, err)
			continue
		}
		connections = append(connections, ac.bpConn)
		allRepos = append(allRepos, ac.repos...)
		pluginNames = append(pluginNames, pluginDisplayName(r.Plugin))
	}

	if len(connections) == 0 {
		return fmt.Errorf("no scoped connections available — cannot create project")
	}

	err = finalizeProject(finalizeProjectOpts{
		Client:      client,
		StatePath:   statePath,
		State:       state,
		ProjectName: projectName,
		Org:         org,
		Connections: connections,
		Repos:       allRepos,
		PluginNames: pluginNames,
		Cron:        "0 0 * * *",
		Wait:        true,
		Timeout:     5 * time.Minute,
	})
	if err != nil {
		return fmt.Errorf("project setup failed: %w", err)
	}

	// ── Summary ──────────────────────────────────────────────────
	fmt.Println("\n════════════════════════════════════════")
	fmt.Println("  ✅ DevLake is ready!")
	fmt.Println("════════════════════════════════════════")
	fmt.Println()

	fmt.Printf("  Backend:  %s\n", disc.URL)
	if disc.GrafanaURL != "" {
		fmt.Printf("  Grafana:  %s\n", disc.GrafanaURL)
	}
	fmt.Printf("  Org:      %s\n", org)
	fmt.Printf("  Project:  %s\n", projectName)
	fmt.Println("\nNext steps:")
	fmt.Println("  • Open Grafana and explore the DORA dashboard")
	fmt.Println("  • Run 'gh devlake status' to check health")
	fmt.Println("  • Run 'gh devlake cleanup' when finished")

	return nil
}

// runInitLocal handles the local deployment path of the wizard.
func runInitLocal(cmd *cobra.Command, args []string) error {
	deployLocalDir = "."
	deployLocalVersion = "latest"
	deployLocalQuiet = true

	if err := runDeployLocal(cmd, args); err != nil {
		return err
	}

	absDir, _ := filepath.Abs(deployLocalDir)
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

	loadedState, _ := devlake.LoadState(".devlake-azure.json")
	if loadedState != nil && loadedState.Endpoints.Backend != "" {
		cfgURL = loadedState.Endpoints.Backend
	}

	return nil
}
