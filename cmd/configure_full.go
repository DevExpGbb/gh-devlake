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
  gh devlake configure full --org my-org --repos owner/repo1,owner/repo2`,
RunE: runConfigureFull,
}

func init() {
configureFullCmd.Flags().StringVar(&fullOrg, "org", "", "GitHub organization name")
configureFullCmd.Flags().StringVar(&fullEnterprise, "enterprise", "", "GitHub enterprise slug")
configureFullCmd.Flags().StringVar(&fullToken, "token", "", "GitHub PAT")
configureFullCmd.Flags().StringVar(&fullEnvFile, "env-file", ".devlake.env", "Path to env file containing GITHUB_PAT")
configureFullCmd.Flags().BoolVar(&fullSkipClean, "skip-cleanup", false, "Do not delete .devlake.env after setup")
configureFullCmd.Flags().StringVar(&fullPlugin, "plugin", "", "Limit to one plugin (github, gh-copilot)")
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
fmt.Println("\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550")
fmt.Println("  DevLake \u2014 Full Configuration")
fmt.Println("\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550")

// Select connections
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
return fmt.Errorf("unknown plugin %q \u2014 choose: github, gh-copilot", fullPlugin)
}
} else {
var labels []string
for _, d := range available {
labels = append(labels, d.DisplayName)
}
fmt.Println()
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

// Phase 1: Configure Connections
fmt.Println("\n\u2554\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2557")
fmt.Println("\u2551  PHASE 1: Configure Connections      \u2551")
fmt.Println("\u255a\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u255d")

results, client, statePath, state, err := runConnectionsInternal(defs, fullOrg, fullEnterprise, fullToken, fullEnvFile, fullSkipClean)
if err != nil {
return fmt.Errorf("phase 1 (connections) failed: %w", err)
}
fmt.Println("\n   \u2705 Phase 1 complete.")

// Phase 2: Scope connections
fmt.Println("\n\u2554\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2557")
fmt.Println("\u2551  PHASE 2: Configure Scopes            \u2551")
fmt.Println("\u255a\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u255d")

scopeOpts := &ScopeOpts{
Org:           fullOrg,
Enterprise:    fullEnterprise,
Repos:         fullRepos,
ReposFile:     fullReposFile,
DeployPattern: fullDeploy,
ProdPattern:   fullProd,
IncidentLabel: fullIncident,
}

// Resolve org from results if not set
if scopeOpts.Org == "" {
for _, r := range results {
if r.Organization != "" {
scopeOpts.Org = r.Organization
break
}
}
}
if scopeOpts.Enterprise == "" {
for _, r := range results {
if r.Enterprise != "" {
scopeOpts.Enterprise = r.Enterprise
break
}
}
}

for _, r := range results {
switch r.Plugin {
case "github":
scopeOpts.GHConnID = r.ConnectionID
scopeOpts.Plugin = "github"
scopeOpts.SkipCopilot = true
scopeOpts.SkipGitHub = false
if err := runConfigureScopes(cmd, args, scopeOpts); err != nil {
fmt.Printf("   \u26a0\ufe0f  GitHub scope setup: %v\n", err)
}
case "gh-copilot":
scopeOpts.CopilotConnID = r.ConnectionID
scopeOpts.Plugin = "gh-copilot"
scopeOpts.SkipGitHub = true
scopeOpts.SkipCopilot = false
if err := runConfigureScopes(cmd, args, scopeOpts); err != nil {
fmt.Printf("   \u26a0\ufe0f  Copilot scope setup: %v\n", err)
}
}
}
fmt.Println("\n   \u2705 Phase 2 complete.")

// Phase 3: Create project
fmt.Println("\n\u2554\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2557")
fmt.Println("\u2551  PHASE 3: Project Setup               \u2551")
fmt.Println("\u255a\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u255d")

projectOpts := &ProjectOpts{
Org:         scopeOpts.Org,
Enterprise:  scopeOpts.Enterprise,
ProjectName: fullProject,
TimeAfter:   fullTimeAfter,
Cron:        fullCron,
SkipSync:    fullSkipSync,
Wait:        true,
Timeout:     5 * time.Minute,
Client:      client,
StatePath:   statePath,
State:       state,
}

if err := runConfigureProjects(cmd, args, projectOpts); err != nil {
return fmt.Errorf("phase 3 (project setup) failed: %w", err)
}

fmt.Println("\n\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550")
fmt.Println("  \u2705 Full configuration complete!")
fmt.Println("\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550")
fmt.Println()
return nil
}

// runConnectionsInternal creates connections for the given defs using a shared token.
// Returns (results, client, statePath, state, error).
func runConnectionsInternal(defs []*ConnectionDef, org, enterprise, tokenVal, envFile string, skipClean bool) ([]ConnSetupResult, *devlake.Client, string, *devlake.State, error) {
fmt.Println("\n\U0001f50d Discovering DevLake instance...")
disc, err := devlake.Discover(cfgURL)
if err != nil {
return nil, nil, "", nil, err
}
fmt.Printf("   Found DevLake at %s (via %s)\n", disc.URL, disc.Source)

client := devlake.NewClient(disc.URL)

fmt.Println("\n\U0001f511 Resolving GitHub PAT...")
scopeHint := aggregateScopeHints(defs)
tokResult, err := token.Resolve(defs[0].Plugin, tokenVal, envFile, scopeHint)
if err != nil {
return nil, nil, "", nil, err
}
fmt.Printf("   Token loaded from: %s\n", tokResult.Source)

for _, def := range defs {
if def.NeedsOrg && org == "" {
org = prompt.ReadLine("GitHub organization slug")
break
}
}

var results []ConnSetupResult
for _, def := range defs {
fmt.Printf("\n\U0001f4e1 Creating %s connection...\n", def.DisplayName)
params := ConnectionParams{
Token:      tokResult.Token,
Org:        org,
Enterprise: enterprise,
}
r, err := buildAndCreateConnection(client, def, params, org, false)
if err != nil {
fmt.Printf("   \u26a0\ufe0f  Could not create %s connection: %v\n", def.DisplayName, err)
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
fmt.Fprintf(os.Stderr, "\u26a0\ufe0f  Could not update state file: %v\n", err)
} else {
fmt.Printf("\n\U0001f4be State saved to %s\n", statePath)
}

if !skipClean && tokResult.EnvFilePath != "" {
fmt.Printf("\n\U0001f9f9 Cleaning up %s...\n", tokResult.EnvFilePath)
if err := os.Remove(tokResult.EnvFilePath); err != nil && !os.IsNotExist(err) {
fmt.Fprintf(os.Stderr, "\u26a0\ufe0f  Could not delete env file: %v\n", err)
} else {
fmt.Println("   \u2705 Env file deleted")
}
}

fmt.Println("\n" + strings.Repeat("\u2500", 50))
fmt.Println("\u2705 Connections configured successfully!")
for _, r := range results {
name := r.Plugin
if def := FindConnectionDef(r.Plugin); def != nil {
name = def.DisplayName
}
fmt.Printf("   %-18s  ID=%d  %q\n", name, r.ConnectionID, r.Name)
}
fmt.Println(strings.Repeat("\u2500", 50))

return results, client, statePath, state, nil
}