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
	Long: `Combines 'configure connection' and 'configure project' into a single
workflow. Prompts to select which plugins to connect, then creates a project,
configures scopes, and triggers the first sync.

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
	configureFullCmd.Flags().StringVar(&scopePlugin, "plugin", "", "Limit to one plugin (github, gh-copilot)")

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
	configureFullCmd.Flags().BoolVar(&scopeSkipCopilot, "skip-copilot", false, "Deprecated: use --plugin github instead")
	_ = configureFullCmd.Flags().MarkHidden("skip-copilot")
}

func runConfigureFull(cmd *cobra.Command, args []string) error {
	fmt.Println()
	fmt.Println("════════════════════════════════════════")
	fmt.Println("  DevLake — Full Configuration")
	fmt.Println("════════════════════════════════════════")

	// ── Select connections ──
	available := AvailableConnections()
	var defs []*ConnectionDef
	if scopePlugin != "" {
		// --plugin limits to one plugin: skip the interactive picker
		for _, d := range available {
			if d.Plugin == scopePlugin {
				defs = append(defs, d)
				break
			}
		}
		if len(defs) == 0 {
			return fmt.Errorf("unknown plugin %q — choose: github, gh-copilot", scopePlugin)
		}
	} else {
		var labels []string
		for _, d := range available {
			labels = append(labels, d.DisplayName)
		}
		selectedLabels := prompt.SelectMultiWithDefaults("Which connections to configure?", labels, []int{1, 2})
		for _, label := range selectedLabels {
			for _, d := range available {
				if d.DisplayName == label {
					defs = append(defs, d)
					break
				}
			}
		}
	}
	if len(defs) == 0 {
		return fmt.Errorf("at least one connection is required")
	}

	// ── Phase 1: Configure Connections ──
	fmt.Println("\n╔══════════════════════════════════════╗")
	fmt.Println("║  PHASE 1: Configure Connections      ║")
	fmt.Println("╚══════════════════════════════════════╝")

	results, devlakeURL, _, err := runConnectionsInternal(defs, fullOrg, fullEnterprise, fullToken, fullEnvFile, fullSkipClean)
	if err != nil {
		return fmt.Errorf("phase 1 (connections) failed: %w", err)
	}
	fmt.Println("\n   ✅ Phase 1 complete.")

	// ── Phase 2: Project Setup ──
	fmt.Println("\n╔══════════════════════════════════════╗")
	fmt.Println("║  PHASE 2: Project Setup              ║")
	fmt.Println("╚══════════════════════════════════════╝")

	// Wire connection results into scope vars
	scopeSkipCopilot = true
	scopeSkipGitHub = true
	for _, r := range results {
		switch r.Plugin {
		case "github":
			scopeGHConnID = r.ConnectionID
			scopeSkipGitHub = false
			if scopeOrg == "" {
				scopeOrg = r.Organization
			}
		case "gh-copilot":
			scopeCopilotConnID = r.ConnectionID
			scopeSkipCopilot = false
			if scopeEnterprise == "" && r.Enterprise != "" {
				scopeEnterprise = r.Enterprise
			}
		}
	}
	if fullOrg != "" {
		scopeOrg = fullOrg
	}
	if fullEnterprise != "" {
		scopeEnterprise = fullEnterprise
	}
	cfgURL = devlakeURL

	if err := runConfigureProjects(cmd, args); err != nil {
		return fmt.Errorf("phase 2 (project setup) failed: %w", err)
	}

	fmt.Println("\n════════════════════════════════════════")
	fmt.Println("  ✅ Full configuration complete!")
	fmt.Println("════════════════════════════════════════")
	fmt.Println()
	return nil
}

// runConnectionsInternal creates connections for the given defs using a shared token.
// Returns (results, devlakeURL, grafanaURL, error).
func runConnectionsInternal(defs []*ConnectionDef, org, enterprise, tokenVal, envFile string, skipClean bool) ([]ConnSetupResult, string, string, error) {
	// ── Discover DevLake ──
	fmt.Println("\n🔍 Discovering DevLake instance...")
	disc, err := devlake.Discover(cfgURL)
	if err != nil {
		return nil, "", "", err
	}
	fmt.Printf("   Found DevLake at %s (via %s)\n", disc.URL, disc.Source)

	client := devlake.NewClient(disc.URL)

	// ── Resolve token ──
	fmt.Println("\n🔑 Resolving GitHub PAT...")
	scopeHint := aggregateScopeHints(defs)
	tokResult, err := token.Resolve(tokenVal, envFile, scopeHint)
	if err != nil {
		return nil, "", "", err
	}
	fmt.Printf("   Token loaded from: %s\n", tokResult.Source)

	// ── Prompt for org once if any def needs it ──
	for _, def := range defs {
		if def.NeedsOrg && org == "" {
			org = prompt.ReadLine("GitHub organization slug")
			break
		}
	}

	// ── Create connections ──
	var results []ConnSetupResult
	for _, def := range defs {
		fmt.Printf("\n📡 Creating %s connection...\n", def.DisplayName)
		params := ConnectionParams{
			Token:      tokResult.Token,
			Org:        org,
			Enterprise: enterprise,
		}
		r, err := buildAndCreateConnection(client, def, params, org, false)
		if err != nil {
			// Non-fatal: log and continue (e.g. Copilot may need extra permissions)
			fmt.Printf("   ⚠️  Could not create %s connection: %v\n", def.DisplayName, err)
			continue
		}
		results = append(results, *r)
	}

	// ── Update state file ──
	statePath, state := devlake.FindStateFile(disc.URL, disc.GrafanaURL)
	var stateConns []devlake.StateConnection
	for _, r := range results {
		stateConns = append(stateConns, devlake.StateConnection{
			Plugin:       r.Plugin,
			ConnectionID: r.ConnectionID,
			Name:         r.Name,
			Organization: r.Organization,
			Enterprise:   r.Enterprise,
		})
	}
	if err := devlake.UpdateConnections(statePath, state, stateConns); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Could not update state file: %v\n", err)
	} else {
		fmt.Printf("\n💾 State saved to %s\n", statePath)
	}

	// ── Cleanup env file ──
	if !skipClean && tokResult.EnvFilePath != "" {
		fmt.Printf("\n🧹 Cleaning up %s...\n", tokResult.EnvFilePath)
		if err := os.Remove(tokResult.EnvFilePath); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "⚠️  Could not delete env file: %v\n", err)
		} else {
			fmt.Println("   ✅ Env file deleted")
		}
	}

	// ── Summary ──
	fmt.Println("\n" + strings.Repeat("─", 50))
	fmt.Println("✅ Connections configured successfully!")
	for _, r := range results {
		name := r.Plugin
		if def := FindConnectionDef(r.Plugin); def != nil {
			name = def.DisplayName
		}
		fmt.Printf("   %-18s  ID=%d  %q\n", name, r.ConnectionID, r.Name)
	}
	fmt.Println(strings.Repeat("─", 50))

	return results, disc.URL, disc.GrafanaURL, nil
}
