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
Short:   "Guided setup wizard \u2014 deploy and configure DevLake in one step",
Long: `Walks you through deploying and configuring DevLake from scratch.

The wizard will:
  1. Ask where to deploy (local Docker or Azure)
  2. Deploy DevLake and wait for it to be ready
  3. Create GitHub and Copilot connections
  4. Configure repository scopes and create a project
  5. Trigger the first data sync

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
fmt.Println("\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550")
fmt.Println("  DevLake \u2014 Setup Wizard")
fmt.Println("  Deploy \u2192 Connect \u2192 Configure")
fmt.Println("\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550")

// Phase 1: Choose deployment target
targets := []string{"local - Docker Compose on this machine", "azure - Azure Container Apps"}
choice := prompt.Select("Where would you like to deploy DevLake?", targets)
if choice == "" {
return fmt.Errorf("deployment target is required")
}
target := strings.SplitN(choice, " ", 2)[0]
fmt.Printf("\n   Selected: %s\n", target)

fmt.Println("\n\u2554\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2557")
fmt.Println("\u2551  PHASE 1: Deploy DevLake             \u2551")
fmt.Println("\u255a\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u255d")

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

// Verify DevLake is reachable
fmt.Println("\n\U0001f50d Verifying DevLake is reachable...")
disc, err := devlake.Discover(cfgURL)
if err != nil {
return fmt.Errorf("cannot reach DevLake after deploy: %w", err)
}
fmt.Printf("   \u2705 DevLake at %s (via %s)\n", disc.URL, disc.Source)

// Phase 2: Configure connections
fmt.Println("\n\u2554\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2557")
fmt.Println("\u2551  PHASE 2: Configure Connections      \u2551")
fmt.Println("\u255a\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u255d")

if initOrg == "" {
initOrg = prompt.ReadLine("GitHub organization slug")
if initOrg == "" {
return fmt.Errorf("--org is required")
}
}

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
selectedDefs = available
}

results, client, statePath, state, err := runConnectionsInternal(selectedDefs, initOrg, initEnterprise, initToken, initEnvFile, true)
if err != nil {
return fmt.Errorf("connection setup failed: %w", err)
}
fmt.Println("\n   \u2705 Connections configured.")

// Phase 3: Configure scopes
fmt.Println("\n\u2554\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2557")
fmt.Println("\u2551  PHASE 3: Configure Scopes           \u2551")
fmt.Println("\u255a\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u255d")

deployPattern := "(?i)deploy"
prodPattern := "(?i)prod"
incidentLabel := "incident"

fmt.Println("\n   Default DORA patterns:")
fmt.Printf("     Deployment: %s\n", deployPattern)
fmt.Printf("     Production: %s\n", prodPattern)
fmt.Printf("     Incidents:  label=%s\n", incidentLabel)
if !prompt.Confirm("Use these defaults?") {
deployPattern = prompt.ReadLine("Deployment workflow regex")
if deployPattern == "" {
deployPattern = "(?i)deploy"
}
prodPattern = prompt.ReadLine("Production environment regex")
if prodPattern == "" {
prodPattern = "(?i)prod"
}
incidentLabel = prompt.ReadLine("Incident issue label")
if incidentLabel == "" {
incidentLabel = "incident"
}
}

scopeOpts := &ScopeOpts{
Org:           initOrg,
Enterprise:    initEnterprise,
Repos:         initRepos,
ReposFile:     initReposFile,
DeployPattern: deployPattern,
ProdPattern:   prodPattern,
IncidentLabel: incidentLabel,
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

// Phase 4: Create project
fmt.Println("\n\u2554\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2557")
fmt.Println("\u2551  PHASE 4: Project Setup              \u2551")
fmt.Println("\u255a\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u255d")

projectOpts := &ProjectOpts{
Org:        initOrg,
Enterprise: initEnterprise,
Wait:       true,
Timeout:    5 * time.Minute,
Client:     client,
StatePath:  statePath,
State:      state,
}
if err := runConfigureProjects(cmd, args, projectOpts); err != nil {
return fmt.Errorf("project setup failed: %w", err)
}

// Summary
fmt.Println("\n\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550")
fmt.Println("  \u2705 DevLake is ready!")
fmt.Println("\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550")
fmt.Println()

disc2, _ := devlake.Discover(cfgURL)
if disc2 != nil {
fmt.Printf("  Backend:  %s\n", disc2.URL)
if disc2.GrafanaURL != "" {
fmt.Printf("  Grafana:  %s\n", disc2.GrafanaURL)
}
}
fmt.Printf("  Org:      %s\n", initOrg)
fmt.Println("\nNext steps:")
fmt.Println("  \u2022 Open Grafana and explore the DORA dashboard")
fmt.Println("  \u2022 Run 'gh devlake status' to check health")
fmt.Println("  \u2022 Run 'gh devlake cleanup' when finished")

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

fmt.Println("\n\U0001f504 Triggering database migration...")
client := devlake.NewClient(backendURL)
if err := client.TriggerMigration(); err != nil {
fmt.Printf("   \u26a0\ufe0f  Migration may need manual trigger: %v\n", err)
} else {
fmt.Println("   \u2705 Migration triggered")
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