package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

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
	fullPlugin     string
	fullRepos      string
	fullReposFile  string
	fullProject    string
	fullDeploy     string
	fullProd       string
	fullIncident   string
	fullTimeAfter  string
	fullCron       string
	fullSkipSync   bool
)

var configureFullCmd = &cobra.Command{
	Use:   "full",
	Short: "Run connections + scopes + project in one step",
	Long: `Combines 'configure connection', 'configure scope', and 'configure project'
into a single workflow.

Example:
  gh devlake configure full --org my-org --plugin github --repos owner/repo1,owner/repo2`,
	RunE: runConfigureFull,
}

func init() {
	configureFullCmd.Flags().StringVar(&fullOrg, "org", "", "Organization slug")
	configureFullCmd.Flags().StringVar(&fullEnterprise, "enterprise", "", "Enterprise slug")
	configureFullCmd.Flags().StringVar(&fullToken, "token", "", "Personal access token")
	configureFullCmd.Flags().StringVar(&fullEnvFile, "env-file", ".devlake.env", "Path to env file containing PAT")
	configureFullCmd.Flags().BoolVar(&fullSkipClean, "skip-cleanup", false, "Do not delete .devlake.env after setup")
	configureFullCmd.Flags().StringVar(&fullPlugin, "plugin", "", fmt.Sprintf("Limit to one plugin (%s)", strings.Join(availablePluginSlugs(), ", ")))
	configureFullCmd.Flags().StringVar(&fullRepos, "repos", "", "Comma-separated repos (owner/repo)")
	configureFullCmd.Flags().StringVar(&fullReposFile, "repos-file", "", "Path to file with repos")
	configureFullCmd.Flags().StringVar(&fullProject, "project-name", "", "DevLake project name")
	configureFullCmd.Flags().StringVar(&fullDeploy, "deployment-pattern", "(?i)deploy", "Deployment workflow regex")
	configureFullCmd.Flags().StringVar(&fullProd, "production-pattern", "(?i)prod", "Production environment regex")
	configureFullCmd.Flags().StringVar(&fullIncident, "incident-label", "incident", "Incident issue label")
	configureFullCmd.Flags().StringVar(&fullTimeAfter, "time-after", "", "Only collect data after this date")
	configureFullCmd.Flags().StringVar(&fullCron, "cron", "0 0 * * *", "Blueprint cron schedule")
	configureFullCmd.Flags().BoolVar(&fullSkipSync, "skip-sync", false, "Skip first data sync")
}

func runConfigureFull(cmd *cobra.Command, args []string) error {
	fmt.Println()
	fmt.Println("════════════════════════════════════════")
	fmt.Println("  DevLake — Full Configuration")
	fmt.Println("════════════════════════════════════════")

	// ── Select connections ──
	available := AvailableConnections()
	var defs []*ConnectionDef
	if fullPlugin != "" {
		for _, d := range available {
			if d.Plugin == fullPlugin {
				defs = append(defs, d)
				break
			}
		}
		if len(defs) == 0 {
			slugs := availablePluginSlugs()
			return fmt.Errorf("unknown plugin %q — choose: %s", fullPlugin, strings.Join(slugs, ", "))
		}
	} else {
		var labels []string
		for _, d := range available {
			labels = append(labels, d.DisplayName)
		}
		fmt.Println()
		selectedLabels := prompt.SelectMulti("Which connections to configure?", labels)
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

	results, client, statePath, state, err := runConnectionsInternal(defs, fullOrg, fullEnterprise, fullToken, fullEnvFile, fullSkipClean)
	if err != nil {
		return fmt.Errorf("phase 1 (connections) failed: %w", err)
	}
	if len(results) == 0 {
		return fmt.Errorf("no connections were created — cannot continue")
	}
	fmt.Println("\n   ✅ Phase 1 complete.")

	// Resolve org/enterprise from results if not set via flags
	org := fullOrg
	if org == "" {
		for _, r := range results {
			if r.Organization != "" {
				org = r.Organization
				break
			}
		}
	}
	enterprise := fullEnterprise
	if enterprise == "" {
		for _, r := range results {
			if r.Enterprise != "" {
				enterprise = r.Enterprise
				break
			}
		}
	}

	// ── Phase 2: Scope Connections (call inner functions directly) ──
	fmt.Println("\n╔══════════════════════════════════════╗")
	fmt.Println("║  PHASE 2: Configure Scopes           ║")
	fmt.Println("╚══════════════════════════════════════╝")

	for _, r := range results {
		fmt.Printf("\n📡 Configuring scopes for %s (connection %d)...\n",
			pluginDisplayName(r.Plugin), r.ConnectionID)

		switch r.Plugin {
		case "github":
			scopeOpts := &ScopeOpts{
				Repos:         fullRepos,
				ReposFile:     fullReposFile,
				DeployPattern: fullDeploy,
				ProdPattern:   fullProd,
				IncidentLabel: fullIncident,
			}
			_, err := scopeGitHub(client, r.ConnectionID, org, scopeOpts)
			if err != nil {
				fmt.Printf("   ⚠️  GitHub scope setup: %v\n", err)
			}

		case "gh-copilot":
			_, err := scopeCopilot(client, r.ConnectionID, org, enterprise)
			if err != nil {
				fmt.Printf("   ⚠️  Copilot scope setup: %v\n", err)
			}

		default:
			fmt.Printf("   ⚠️  Scope configuration for %q is not yet supported\n", r.Plugin)
		}
	}
	fmt.Println("\n   ✅ Phase 2 complete.")

	// ── Phase 3: Create Project (call inner functions directly) ──
	fmt.Println("\n╔══════════════════════════════════════╗")
	fmt.Println("║  PHASE 3: Project Setup              ║")
	fmt.Println("╚══════════════════════════════════════╝")

	projectName := fullProject
	if projectName == "" {
		projectName = org
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
		ac, err := listConnectionScopes(client, choice, org, enterprise)
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

	cron := fullCron
	if cron == "" {
		cron = "0 0 * * *"
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
		Cron:        cron,
		TimeAfter:   fullTimeAfter,
		SkipSync:    fullSkipSync,
		Wait:        true,
		Timeout:     5 * time.Minute,
	})
	if err != nil {
		return fmt.Errorf("phase 3 (project setup) failed: %w", err)
	}

	fmt.Println("\n════════════════════════════════════════")
	fmt.Println("  ✅ Full configuration complete!")
	fmt.Println("════════════════════════════════════════")
	fmt.Println()
	return nil
}

// runConnectionsInternal creates connections for the given defs, resolving
// tokens and org/enterprise per-plugin. Returns (results, client, statePath, state, error).
func runConnectionsInternal(defs []*ConnectionDef, org, enterprise, tokenVal, envFile string, skipClean bool) ([]ConnSetupResult, *devlake.Client, string, *devlake.State, error) {
	fmt.Println("\n🔍 Discovering DevLake instance...")
	disc, err := devlake.Discover(cfgURL)
	if err != nil {
		return nil, nil, "", nil, err
	}
	fmt.Printf("   Found DevLake at %s (via %s)\n", disc.URL, disc.Source)

	client := devlake.NewClient(disc.URL)

	var results []ConnSetupResult
	var cleanupEnvFile string

	for _, def := range defs {
		fmt.Printf("\n📡 Setting up %s connection...\n", def.DisplayName)

		// Resolve token per-plugin
		fmt.Printf("\n🔑 Resolving %s token...\n", def.DisplayName)
		tokResult, err := token.Resolve(token.ResolveOpts{
			FlagValue:   tokenVal,
			EnvFilePath: envFile,
			EnvFileKeys: def.EnvFileKeys,
			EnvVarNames: def.EnvVarNames,
			DisplayName: def.DisplayName,
			ScopeHint:   def.ScopeHint,
		})
		if err != nil {
			fmt.Printf("   ⚠️  Could not resolve token for %s: %v\n", def.DisplayName, err)
			continue
		}
		fmt.Printf("   Token loaded from: %s\n", tokResult.Source)
		if tokResult.EnvFilePath != "" {
			cleanupEnvFile = tokResult.EnvFilePath
		}

		// Resolve org per-plugin if needed
		pluginOrg := org
		if def.NeedsOrg && pluginOrg == "" {
			orgPrompt := def.OrgPrompt
			if orgPrompt == "" {
				orgPrompt = "Organization slug"
			}
			pluginOrg = prompt.ReadLine(orgPrompt)
			if pluginOrg == "" {
				fmt.Printf("   ⚠️  Organization is required for %s, skipping\n", def.DisplayName)
				continue
			}
		}

		// Resolve enterprise per-plugin if needed
		pluginEnterprise := enterprise
		if def.NeedsEnterprise && pluginEnterprise == "" {
			entPrompt := def.EnterprisePrompt
			if entPrompt == "" {
				entPrompt = "Enterprise slug (optional, press Enter to skip)"
			}
			pluginEnterprise = prompt.ReadLine(entPrompt)
		}

		params := ConnectionParams{
			Token:      tokResult.Token,
			Org:        pluginOrg,
			Enterprise: pluginEnterprise,
		}
		r, err := buildAndCreateConnection(client, def, params, pluginOrg, false)
		if err != nil {
			fmt.Printf("   ⚠️  Could not create %s connection: %v\n", def.DisplayName, err)
			continue
		}
		results = append(results, *r)
	}

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

	if !skipClean && cleanupEnvFile != "" {
		fmt.Printf("\n🧹 Cleaning up %s...\n", cleanupEnvFile)
		if err := os.Remove(cleanupEnvFile); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "⚠️  Could not delete env file: %v\n", err)
		} else {
			fmt.Println("   ✅ Env file deleted")
		}
	}

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

	return results, client, statePath, state, nil
}
