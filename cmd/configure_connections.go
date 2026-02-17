package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/DevExpGBB/gh-devlake/internal/devlake"
	"github.com/DevExpGBB/gh-devlake/internal/envfile"
	"github.com/DevExpGBB/gh-devlake/internal/prompt"
	"github.com/DevExpGBB/gh-devlake/internal/token"
	"github.com/spf13/cobra"
)

var (
	connOrg        string
	connEnterprise string
	connToken      string
	connEnvFile    string
	connSkipClean  bool
)

var configureConnectionsCmd = &cobra.Command{
	Use:   "connections",
	Short: "Create GitHub and Copilot connections in DevLake",
	Long: `Creates two DevLake connections using a GitHub PAT:
  1. GitHub connection — for repository and PR data
  2. GitHub Copilot connection — for Copilot usage metrics

Token resolution order:
  --token flag → .devlake.env file → $GITHUB_TOKEN/$GH_TOKEN → interactive prompt`,
	RunE: runConfigureConnections,
}

func init() {
	configureConnectionsCmd.Flags().StringVar(&connOrg, "org", "", "GitHub organization name")
	configureConnectionsCmd.Flags().StringVar(&connEnterprise, "enterprise", "", "GitHub enterprise slug (optional, needed for Copilot enterprise metrics)")
	configureConnectionsCmd.Flags().StringVar(&connToken, "token", "", "GitHub PAT (if not using .devlake.env or env vars)")
	configureConnectionsCmd.Flags().StringVar(&connEnvFile, "env-file", ".devlake.env", "Path to env file containing GITHUB_PAT")
	configureConnectionsCmd.Flags().BoolVar(&connSkipClean, "skip-cleanup", false, "Do not delete .devlake.env after successful setup")
	configureCmd.AddCommand(configureConnectionsCmd)
}

func runConfigureConnections(cmd *cobra.Command, args []string) error {
	// ── Interactive prompt for missing org ──
	if connOrg == "" {
		connOrg = prompt.ReadLine("GitHub organization slug")
		if connOrg == "" {
			return fmt.Errorf("--org is required")
		}
	}

	// ── Step 1: Discover DevLake ──
	fmt.Println("🔍 Discovering DevLake instance...")
	disc, err := devlake.Discover(cfgURL)
	if err != nil {
		return err
	}
	fmt.Printf("   Found DevLake at %s (via %s)\n", disc.URL, disc.Source)

	client := devlake.NewClient(disc.URL)

	// ── Step 2: Resolve token ──
	fmt.Println("\n🔑 Resolving GitHub PAT...")
	result, err := token.Resolve(connToken, connEnvFile)
	if err != nil {
		return err
	}
	fmt.Printf("   Token loaded from: %s\n", result.Source)

	// ── Step 3: Test + create GitHub connection ──
	ghConnName := fmt.Sprintf("GitHub - %s", connOrg)
	fmt.Printf("\n📡 Creating GitHub connection %q...\n", ghConnName)

	existing, _ := client.FindConnectionByName("github", ghConnName)
	var ghConn *devlake.Connection
	if existing != nil {
		fmt.Printf("   Connection already exists (ID=%d), skipping creation.\n", existing.ID)
		ghConn = existing
	} else {
		// Test first
		testReq := &devlake.ConnectionTestRequest{
			Endpoint:         "https://api.github.com/",
			AuthMethod:       "AccessToken",
			Token:            result.Token,
			EnableGraphql:    true,
			RateLimitPerHour: 4500,
			Proxy:            "",
		}
		testResult, err := client.TestConnection("github", testReq)
		if err != nil {
			return fmt.Errorf("GitHub connection test failed: %w", err)
		}
		if !testResult.Success {
			return fmt.Errorf("GitHub connection test failed: %s", testResult.Message)
		}
		fmt.Println("   ✅ Connection test passed")

		createReq := &devlake.ConnectionCreateRequest{
			Name:             ghConnName,
			Endpoint:         "https://api.github.com/",
			AuthMethod:       "AccessToken",
			Token:            result.Token,
			EnableGraphql:    true,
			RateLimitPerHour: 4500,
		}
		ghConn, err = client.CreateConnection("github", createReq)
		if err != nil {
			return fmt.Errorf("failed to create GitHub connection: %w", err)
		}
		fmt.Printf("   ✅ Created GitHub connection (ID=%d)\n", ghConn.ID)
	}

	// ── Step 4: Test + create Copilot connection ──
	copilotConnName := fmt.Sprintf("Copilot - %s", connOrg)
	fmt.Printf("\n📡 Creating Copilot connection %q...\n", copilotConnName)

	existingCopilot, _ := client.FindConnectionByName("gh-copilot", copilotConnName)
	var copilotConn *devlake.Connection
	if existingCopilot != nil {
		fmt.Printf("   Connection already exists (ID=%d), skipping creation.\n", existingCopilot.ID)
		copilotConn = existingCopilot
	} else {
		copilotCreateReq := &devlake.ConnectionCreateRequest{
			Name:             copilotConnName,
			Endpoint:         "https://api.github.com/",
			AuthMethod:       "AccessToken",
			Token:            result.Token,
			RateLimitPerHour: 4500,
			Organization:     connOrg,
		}
		if connEnterprise != "" {
			copilotCreateReq.Enterprise = connEnterprise
		}
		copilotConn, err = client.CreateConnection("gh-copilot", copilotCreateReq)
		if err != nil {
			return fmt.Errorf("failed to create Copilot connection: %w", err)
		}
		fmt.Printf("   ✅ Created Copilot connection (ID=%d)\n", copilotConn.ID)
	}

	// ── Step 5: Update state file ──
	statePath, state := devlake.FindStateFile(disc.URL, disc.GrafanaURL)
	connections := []devlake.StateConnection{
		{Plugin: "github", ConnectionID: ghConn.ID, Name: ghConn.Name, Organization: connOrg},
		{Plugin: "gh-copilot", ConnectionID: copilotConn.ID, Name: copilotConn.Name, Organization: connOrg, Enterprise: connEnterprise},
	}
	if err := devlake.UpdateConnections(statePath, state, connections); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Could not update state file: %v\n", err)
	} else {
		fmt.Printf("\n💾 State saved to %s\n", statePath)
	}

	// ── Step 6: Cleanup env file ──
	if !connSkipClean && result.EnvFilePath != "" {
		fmt.Printf("\n🧹 Cleaning up %s...\n", result.EnvFilePath)
		if err := envfile.Delete(result.EnvFilePath); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  Could not delete env file: %v\n", err)
		} else {
			fmt.Println("   ✅ Env file deleted")
		}
	}

	// ── Summary ──
	fmt.Println("\n" + strings.Repeat("─", 50))
	fmt.Println("✅ Connections configured successfully!")
	fmt.Printf("   GitHub:  ID=%d  %q\n", ghConn.ID, ghConn.Name)
	fmt.Printf("   Copilot: ID=%d  %q\n", copilotConn.ID, copilotConn.Name)
	fmt.Println(strings.Repeat("─", 50))

	return nil
}
