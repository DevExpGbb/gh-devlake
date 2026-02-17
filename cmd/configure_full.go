package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/DevExpGBB/gh-devlake/internal/devlake"
	"github.com/DevExpGBB/gh-devlake/internal/prompt"
	"github.com/DevExpGBB/gh-devlake/internal/token"
	"github.com/spf13/cobra"
)

// ConfigureConnectionsResult bundles what configure-connections produces,
// so configure-full can chain into configure-scopes.
type ConfigureConnectionsResult struct {
	DevLakeURL          string
	GrafanaURL          string
	GitHubConnectionID  int
	CopilotConnectionID int
	Organization        string
}

var (
	fullOrg        string
	fullEnterprise string
	fullToken      string
	fullEnvFile    string
	fullSkipClean  bool
	// scope flags reused via scopeXxx package vars
)

var configureFullCmd = &cobra.Command{
	Use:   "full",
	Short: "Run connections + scopes configuration in one step",
	Long: `Combines 'configure connections' and 'configure scopes' into a single
workflow. Creates connections, then immediately configures scopes, project,
and triggers the first sync.

Example:
  gh devlake configure full --org my-org --repos owner/repo1,owner/repo2`,
	RunE: runConfigureFull,
}

func init() {
	// Connection flags
	configureFullCmd.Flags().StringVar(&fullOrg, "org", "", "GitHub organization name")
	configureFullCmd.Flags().StringVar(&fullEnterprise, "enterprise", "", "GitHub enterprise slug")
	configureFullCmd.Flags().StringVar(&fullToken, "token", "", "GitHub PAT")
	configureFullCmd.Flags().StringVar(&fullEnvFile, "env-file", ".devlake.env", "Path to env file containing GITHUB_PAT")
	configureFullCmd.Flags().BoolVar(&fullSkipClean, "skip-cleanup", false, "Do not delete .devlake.env after setup")

	// Scope flags (reuse the package-level vars from configure_scopes.go)
	configureFullCmd.Flags().StringVar(&scopeRepos, "repos", "", "Comma-separated repos (owner/repo)")
	configureFullCmd.Flags().StringVar(&scopeReposFile, "repos-file", "", "Path to file with repos")
	configureFullCmd.Flags().StringVar(&scopeProjectName, "project-name", "", "DevLake project name")
	configureFullCmd.Flags().StringVar(&scopeDeployPattern, "deployment-pattern", "(?i)deploy", "Deployment workflow regex")
	configureFullCmd.Flags().StringVar(&scopeProdPattern, "production-pattern", "(?i)prod", "Production environment regex")
	configureFullCmd.Flags().StringVar(&scopeIncidentLabel, "incident-label", "incident", "Incident issue label")
	configureFullCmd.Flags().StringVar(&scopeTimeAfter, "time-after", "", "Only collect data after this date")
	configureFullCmd.Flags().StringVar(&scopeCron, "cron", "0 0 * * *", "Blueprint cron schedule")
	configureFullCmd.Flags().BoolVar(&scopeSkipSync, "skip-sync", false, "Skip first data sync")
	configureFullCmd.Flags().BoolVar(&scopeSkipCopilot, "skip-copilot", false, "Skip Copilot scope")

	configureCmd.AddCommand(configureFullCmd)
}

func runConfigureFull(cmd *cobra.Command, args []string) error {
	// ── Interactive prompt for missing org ──
	if fullOrg == "" {
		fullOrg = prompt.ReadLine("GitHub organization slug")
		if fullOrg == "" {
			return fmt.Errorf("--org is required")
		}
	}

	fmt.Println("═══════════════════════════════════════")
	fmt.Println("  DevLake — Full Configuration")
	fmt.Println("  Phase 1: Configure Connections")
	fmt.Println("  Phase 2: Configure Scopes & Project")
	fmt.Println("═══════════════════════════════════════")

	// ── Phase 1: Configure Connections ──
	fmt.Println("\n╔══════════════════════════════════════╗")
	fmt.Println("║  PHASE 1: Configure Connections      ║")
	fmt.Println("╚══════════════════════════════════════╝")

	result, err := runConnectionsInternal(fullOrg, fullEnterprise, fullToken, fullEnvFile, fullSkipClean)
	if err != nil {
		return fmt.Errorf("phase 1 (connections) failed: %w", err)
	}
	fmt.Println("\n   ✅ Phase 1 complete.")

	// ── Phase 2: Configure Scopes ──
	fmt.Println("\n╔══════════════════════════════════════╗")
	fmt.Println("║  PHASE 2: Configure Scopes & Project ║")
	fmt.Println("╚══════════════════════════════════════╝")

	// Wire connection results into scope flags
	scopeOrg = fullOrg
	scopeGHConnID = result.GitHubConnectionID
	if result.CopilotConnectionID > 0 {
		scopeCopilotConnID = result.CopilotConnectionID
	} else {
		scopeSkipCopilot = true
	}
	cfgURL = result.DevLakeURL

	if err := runConfigureScopes(cmd, args); err != nil {
		return fmt.Errorf("phase 2 (scopes) failed: %w", err)
	}

	fmt.Println("\n═══════════════════════════════════════")
	fmt.Println("  ✅ Full configuration complete!")
	fmt.Println("═══════════════════════════════════════")
	return nil
}

// runConnectionsInternal runs the connection setup and returns the result struct.
func runConnectionsInternal(org, enterprise, tokenVal, envFile string, skipClean bool) (*ConfigureConnectionsResult, error) {
	// ── Step 1: Discover DevLake ──
	fmt.Println("\n🔍 Discovering DevLake instance...")
	disc, err := devlake.Discover(cfgURL)
	if err != nil {
		return nil, err
	}
	fmt.Printf("   Found DevLake at %s (via %s)\n", disc.URL, disc.Source)

	client := devlake.NewClient(disc.URL)

	// ── Step 2: Resolve token ──
	fmt.Println("\n🔑 Resolving GitHub PAT...")
	tokResult, err := token.Resolve(tokenVal, envFile)
	if err != nil {
		return nil, err
	}
	fmt.Printf("   Token loaded from: %s\n", tokResult.Source)

	// ── Step 3: GitHub connection ──
	ghConnName := fmt.Sprintf("GitHub - %s", org)
	fmt.Printf("\n📡 Creating GitHub connection %q...\n", ghConnName)

	existing, _ := client.FindConnectionByName("github", ghConnName)
	var ghConn *devlake.Connection
	if existing != nil {
		fmt.Printf("   Connection already exists (ID=%d), skipping.\n", existing.ID)
		ghConn = existing
	} else {
		testReq := &devlake.ConnectionTestRequest{
			Endpoint:         "https://api.github.com/",
			AuthMethod:       "AccessToken",
			Token:            tokResult.Token,
			EnableGraphql:    true,
			RateLimitPerHour: 4500,
			Proxy:            "",
		}
		testResult, err := client.TestConnection("github", testReq)
		if err != nil {
			return nil, fmt.Errorf("GitHub connection test failed: %w", err)
		}
		if !testResult.Success {
			return nil, fmt.Errorf("GitHub connection test failed: %s", testResult.Message)
		}
		fmt.Println("   ✅ Connection test passed")

		createReq := &devlake.ConnectionCreateRequest{
			Name:             ghConnName,
			Endpoint:         "https://api.github.com/",
			AuthMethod:       "AccessToken",
			Token:            tokResult.Token,
			EnableGraphql:    true,
			RateLimitPerHour: 4500,
		}
		ghConn, err = client.CreateConnection("github", createReq)
		if err != nil {
			return nil, fmt.Errorf("failed to create GitHub connection: %w", err)
		}
		fmt.Printf("   ✅ Created GitHub connection (ID=%d)\n", ghConn.ID)
	}

	// ── Step 4: Copilot connection ──
	copilotConnName := fmt.Sprintf("Copilot - %s", org)
	fmt.Printf("\n📡 Creating Copilot connection %q...\n", copilotConnName)

	copilotConnID := 0
	existingCopilot, _ := client.FindConnectionByName("gh-copilot", copilotConnName)
	if existingCopilot != nil {
		fmt.Printf("   Connection already exists (ID=%d), skipping.\n", existingCopilot.ID)
		copilotConnID = existingCopilot.ID
	} else {
		copilotCreateReq := &devlake.ConnectionCreateRequest{
			Name:             copilotConnName,
			Endpoint:         "https://api.github.com/",
			AuthMethod:       "AccessToken",
			Token:            tokResult.Token,
			RateLimitPerHour: 4500,
			Organization:     org,
		}
		if enterprise != "" {
			copilotCreateReq.Enterprise = enterprise
		}
		copilotConn, err := client.CreateConnection("gh-copilot", copilotCreateReq)
		if err != nil {
			fmt.Printf("   ⚠️  Could not create Copilot connection: %v\n", err)
		} else {
			copilotConnID = copilotConn.ID
			fmt.Printf("   ✅ Created Copilot connection (ID=%d)\n", copilotConn.ID)
		}
	}

	// ── Step 5: Update state file ──
	statePath, state := devlake.FindStateFile(disc.URL, disc.GrafanaURL)
	connections := []devlake.StateConnection{
		{Plugin: "github", ConnectionID: ghConn.ID, Name: ghConn.Name, Organization: org},
	}
	if copilotConnID > 0 {
		connections = append(connections, devlake.StateConnection{
			Plugin:       "gh-copilot",
			ConnectionID: copilotConnID,
			Name:         copilotConnName,
			Organization: org,
			Enterprise:   enterprise,
		})
	}
	if err := devlake.UpdateConnections(statePath, state, connections); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Could not update state file: %v\n", err)
	} else {
		fmt.Printf("\n💾 State saved to %s\n", statePath)
	}

	// ── Step 6: Cleanup ──
	if !skipClean && tokResult.EnvFilePath != "" {
		fmt.Printf("\n🧹 Cleaning up %s...\n", tokResult.EnvFilePath)
		if err := os.Remove(tokResult.EnvFilePath); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "⚠️  Could not delete env file: %v\n", err)
		} else {
			fmt.Println("   ✅ Env file deleted")
		}
	}

	fmt.Println("\n" + strings.Repeat("─", 50))
	fmt.Println("✅ Connections configured successfully!")
	fmt.Printf("   GitHub:  ID=%d  %q\n", ghConn.ID, ghConn.Name)
	if copilotConnID > 0 {
		fmt.Printf("   Copilot: ID=%d  %q\n", copilotConnID, copilotConnName)
	}
	fmt.Println(strings.Repeat("─", 50))

	return &ConfigureConnectionsResult{
		DevLakeURL:          disc.URL,
		GrafanaURL:          disc.GrafanaURL,
		GitHubConnectionID:  ghConn.ID,
		CopilotConnectionID: copilotConnID,
		Organization:        org,
	}, nil
}
