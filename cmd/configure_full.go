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
	fullToken     string
	fullEnvFile   string
	fullSkipClean bool
)

var configureFullCmd = &cobra.Command{
	Use:   "full",
	Short: "Run connections + scopes + project in one step",
	Long: `Runs connections, scopes, and project setup in one interactive session.
Equivalent to 'gh devlake init' but skips the deploy phase.

For scripted/CI use, chain individual commands instead:
  gh devlake configure connection --plugin github --org my-org
  gh devlake configure scope --plugin github --org my-org --repos owner/repo1
  gh devlake configure project --project-name my-project`,
	RunE: runConfigureFull,
}

func init() {
	configureFullCmd.Flags().StringVar(&fullToken, "token", "", "Personal access token (seeds token resolution; may still prompt per plugin)")
	configureFullCmd.Flags().StringVar(&fullEnvFile, "env-file", ".devlake.env", "Path to env file containing PAT")
	configureFullCmd.Flags().BoolVar(&fullSkipClean, "skip-cleanup", false, "Do not delete .devlake.env after setup")
}

func runConfigureFull(cmd *cobra.Command, args []string) error {
	fmt.Println()
	fmt.Println("════════════════════════════════════════")
	fmt.Println("  DevLake — Full Configuration")
	fmt.Println("════════════════════════════════════════")

	// ── Select connections ──
	available := AvailableConnections()
	var labels []string
	for _, d := range available {
		labels = append(labels, d.DisplayName)
	}
	fmt.Println()
	selectedLabels := prompt.SelectMulti("Which connections to configure?", labels)
	var defs []*ConnectionDef
	for _, label := range selectedLabels {
		for _, d := range available {
			if d.DisplayName == label {
				defs = append(defs, d)
				break
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

	results, client, statePath, state, err := runConnectionsInternal(defs, "", "", fullToken, fullEnvFile, fullSkipClean)
	if err != nil {
		return fmt.Errorf("phase 1 (connections) failed: %w", err)
	}
	if len(results) == 0 {
		return fmt.Errorf("no connections were created — cannot continue")
	}
	fmt.Println("\n   ✅ Phase 1 complete.")

	// Derive org from connection results for project name default
	org := ""
	for _, r := range results {
		if r.Organization != "" {
			org = r.Organization
			break
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
	fmt.Println("\n   ✅ Phase 2 complete.")

	// ── Phase 3: Create Project (call inner functions directly) ──
	fmt.Println("\n╔══════════════════════════════════════╗")
	fmt.Println("║  PHASE 3: Project Setup              ║")
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
		ac, err := listConnectionScopes(client, choice)
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
