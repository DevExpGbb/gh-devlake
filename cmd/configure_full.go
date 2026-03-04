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
  gh devlake configure connection add --plugin github --org my-org
  gh devlake configure scope --plugin github --org my-org --repos owner/repo1
  gh devlake configure project add --project-name my-project`,
	RunE: runConfigureFull,
}

func init() {
	configureFullCmd.Flags().StringVar(&fullToken, "token", "", "Personal access token (seeds token resolution; may still prompt per plugin)")
	configureFullCmd.Flags().StringVar(&fullEnvFile, "env-file", ".devlake.env", "Path to env file containing PAT")
	configureFullCmd.Flags().BoolVar(&fullSkipClean, "skip-cleanup", false, "Do not delete .devlake.env after setup")
}

func runConfigureFull(cmd *cobra.Command, args []string) error {
	printBanner("DevLake — Full Configuration")

	if err := configureAllPhases(ConfigureAllOpts{
		Token:     fullToken,
		EnvFile:   fullEnvFile,
		SkipClean: fullSkipClean,
		ReAddLoop: false,
	}); err != nil {
		return err
	}

	printBanner("✅ Full configuration complete!")
	fmt.Println()
	return nil
}

// runConnectionsInternal creates connections for the given defs, resolving
// tokens and org/enterprise per-plugin. Returns (results, client, statePath, state, error).
func runConnectionsInternal(defs []*ConnectionDef, org, enterprise, tokenVal, envFile string, skipClean bool) ([]ConnSetupResult, *devlake.Client, string, *devlake.State, error) {
	client, disc, err := discoverClient(cfgURL)
	if err != nil {
		return nil, nil, "", nil, err
	}

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
		if def.NeedsOrgOrEnt && pluginOrg == "" {
			orgPrompt := def.OrgPrompt
			if orgPrompt == "" {
				orgPrompt = "Organization slug (optional if enterprise provided)"
			}
			pluginOrg = prompt.ReadLine(orgPrompt)
		}
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
		if def.NeedsOrgOrEnt && pluginEnterprise == "" {
			entPrompt := def.EnterprisePrompt
			if entPrompt == "" {
				entPrompt = "Enterprise slug (optional if org provided)"
			}
			pluginEnterprise = prompt.ReadLine(entPrompt)
			if pluginOrg == "" && pluginEnterprise == "" {
				fmt.Printf("   ⚠️  Either organization or enterprise is required for %s, skipping\n", def.DisplayName)
				continue
			}
		}
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
		r, err := buildAndCreateConnection(client, def, params, pluginOrg, true)
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

	if len(results) == 0 {
		fmt.Println("\n" + strings.Repeat("─", 40))
		fmt.Println("⚠️  No connections were created.")
		fmt.Println(strings.Repeat("─", 40))
		fmt.Println()
		return results, client, statePath, state, fmt.Errorf("no connections were created")
	}

	fmt.Println("\n" + strings.Repeat("─", 40))
	fmt.Println("✅ Connections configured successfully!")
	for _, r := range results {
		name := r.Plugin
		if def := FindConnectionDef(r.Plugin); def != nil {
			name = def.DisplayName
		}
		fmt.Printf("   %-18s  ID=%d  %q\n", name, r.ConnectionID, r.Name)
	}
	fmt.Println(strings.Repeat("─", 40))
	fmt.Println()

	return results, client, statePath, state, nil
}
