package cmd

import (
"fmt"
"strconv"
"strings"

"github.com/DevExpGBB/gh-devlake/internal/devlake"
"github.com/DevExpGBB/gh-devlake/internal/gh"
"github.com/DevExpGBB/gh-devlake/internal/prompt"
"github.com/DevExpGBB/gh-devlake/internal/repofile"
"github.com/spf13/cobra"
)

// ScopeOpts holds all options for scope and project commands, replacing
// the former package-level scope* variables.
type ScopeOpts struct {
Org           string
Enterprise    string
Plugin        string
Repos         string
ReposFile     string
ConnectionID  int
ProjectName   string
DeployPattern string
ProdPattern   string
IncidentLabel string
TimeAfter     string
Cron          string
SkipSync      bool
Wait          bool
Timeout       string
}

func newConfigureScopesCmd() *cobra.Command {
var opts ScopeOpts
cmd := &cobra.Command{
Use:     "scope",
Aliases: []string{"scopes"},
Short:   "Add scopes (repos, orgs) to existing connections",
Long: `Adds repository scopes and scope-configs to existing DevLake connections.

This command only manages scopes on connections -- it does not create projects
or trigger data syncs. To create a project after scoping, run:
  gh devlake configure project

Example:
  gh devlake configure scope --plugin github --org my-org --repos owner/repo1,owner/repo2
  gh devlake configure scope --plugin gh-copilot --org my-org --enterprise my-ent`,
RunE: func(cmd *cobra.Command, args []string) error {
return runConfigureScopes(cmd, args, &opts)
},
}

cmd.Flags().StringVar(&opts.Org, "org", "", "Organization slug")
cmd.Flags().StringVar(&opts.Enterprise, "enterprise", "", "Enterprise slug (enables enterprise-level metrics)")
cmd.Flags().StringVar(&opts.Plugin, "plugin", "", fmt.Sprintf("Plugin to configure (%s)", strings.Join(availablePluginSlugs(), ", ")))
cmd.Flags().StringVar(&opts.Repos, "repos", "", "Comma-separated repos (owner/repo)")
cmd.Flags().StringVar(&opts.ReposFile, "repos-file", "", "Path to file with repos (one per line)")
cmd.Flags().IntVar(&opts.ConnectionID, "connection-id", 0, "Connection ID (auto-detected if omitted)")
cmd.Flags().StringVar(&opts.DeployPattern, "deployment-pattern", "(?i)deploy", "Regex to match deployment workflows")
cmd.Flags().StringVar(&opts.ProdPattern, "production-pattern", "(?i)prod", "Regex to match production environment")
cmd.Flags().StringVar(&opts.IncidentLabel, "incident-label", "incident", "Issue label for incidents")

return cmd
}

func init() {}

// scopeGitHubResult holds the outputs from scoping a GitHub connection.
type scopeGitHubResult struct {
Connection  devlake.BlueprintConnection
Repos       []string
RepoDetails []*gh.RepoDetails
}

// scopeGitHub resolves repos, creates scope config, and PUTs repo scopes
// for a GitHub connection. Returns the BlueprintConnection entry and repo list.
func scopeGitHub(client *devlake.Client, connID int, org string, opts *ScopeOpts) (*scopeGitHubResult, error) {
fmt.Println("\n\U0001f4e6 Resolving repositories...")
repos, err := resolveRepos(org, opts)
if err != nil {
return nil, err
}
if len(repos) == 0 {
return nil, fmt.Errorf("at least one repository is required")
}
fmt.Printf("   Repos to configure: %s\n", strings.Join(repos, ", "))

fmt.Println("\n\U0001f50e Looking up repo details...")
var repoDetails []*gh.RepoDetails
for _, repo := range repos {
detail, err := gh.GetRepoDetails(repo)
if err != nil {
fmt.Printf("   \u26a0\ufe0f  Could not fetch details for %q: %v\n", repo, err)
continue
}
repoDetails = append(repoDetails, detail)
fmt.Printf("   %s (ID: %d)\n", detail.FullName, detail.ID)
}
if len(repoDetails) == 0 {
return nil, fmt.Errorf("could not resolve any repository details \u2014 verify repos exist and gh CLI is authenticated")
}

fmt.Println("\n\u2699\ufe0f  Creating DORA scope config...")
scopeConfigID, err := ensureScopeConfig(client, "github", connID, opts)
if err != nil {
fmt.Printf("   \u26a0\ufe0f  Could not create scope config: %v\n", err)
} else {
fmt.Printf("   Scope config ID: %d\n", scopeConfigID)
}

fmt.Println("\n\U0001f4dd Adding repository scopes...")
err = putGitHubScopes(client, connID, scopeConfigID, repoDetails)
if err != nil {
return nil, fmt.Errorf("failed to add repo scopes: %w", err)
}
fmt.Printf("   \u2705 Added %d repo scope(s)\n", len(repoDetails))

var ghScopes []devlake.BlueprintScope
for _, d := range repoDetails {
ghScopes = append(ghScopes, devlake.BlueprintScope{
ScopeID:   strconv.Itoa(d.ID),
ScopeName: d.FullName,
})
}

return &scopeGitHubResult{
Connection: devlake.BlueprintConnection{
PluginName:   "github",
ConnectionID: connID,
Scopes:       ghScopes,
},
Repos:       repos,
RepoDetails: repoDetails,
}, nil
}

// scopeCopilot PUTs the org/enterprise scope for a Copilot connection.
func scopeCopilot(client *devlake.Client, connID int, org, enterprise string) (*devlake.BlueprintConnection, error) {
fmt.Println("\n\U0001f4dd Adding Copilot scope...")
scopeID := copilotScopeID(org, enterprise)
err := putCopilotScope(client, connID, org, enterprise)
if err != nil {
return nil, fmt.Errorf("could not add Copilot scope: %w", err)
}
fmt.Printf("   \u2705 Copilot scope added: %s\n", scopeID)

return &devlake.BlueprintConnection{
PluginName:   "gh-copilot",
ConnectionID: connID,
Scopes: []devlake.BlueprintScope{
{ScopeID: scopeID, ScopeName: scopeID},
},
}, nil
}

func runConfigureScopes(cmd *cobra.Command, args []string, opts *ScopeOpts) error {
fmt.Println()
fmt.Println("\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550")
fmt.Println("  DevLake \u2014 Configure Scopes")
fmt.Println("\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550")

// Determine which plugin to scope
var selectedPlugin string
if opts.Plugin != "" {
def := FindConnectionDef(opts.Plugin)
if def == nil || !def.Available {
slugs := availablePluginSlugs()
return fmt.Errorf("unknown plugin %q \u2014 choose: %s", opts.Plugin, strings.Join(slugs, ", "))
}
selectedPlugin = opts.Plugin
} else {
flagMode := cmd.Flags().Changed("org") ||
cmd.Flags().Changed("repos") ||
cmd.Flags().Changed("repos-file") ||
cmd.Flags().Changed("connection-id")
if flagMode {
slugs := availablePluginSlugs()
return fmt.Errorf("--plugin is required when using flags (choose: %s)", strings.Join(slugs, ", "))
}
available := AvailableConnections()
var labels []string
for _, d := range available {
labels = append(labels, d.DisplayName)
}
fmt.Println()
chosen := prompt.Select("Which plugin to configure?", labels)
if chosen == "" {
return fmt.Errorf("plugin selection is required")
}
for _, d := range available {
if d.DisplayName == chosen {
selectedPlugin = d.Plugin
break
}
}
if selectedPlugin == "" {
return fmt.Errorf("plugin selection is required")
}
}

fmt.Println("\n\U0001f50d Discovering DevLake instance...")
disc, err := devlake.Discover(cfgURL)
if err != nil {
return err
}
fmt.Printf("   Found DevLake at %s (via %s)\n", disc.URL, disc.Source)

client := devlake.NewClient(disc.URL)
_, state := devlake.FindStateFile(disc.URL, disc.GrafanaURL)

fmt.Println("\n\U0001f517 Resolving connection...")
connID, err := resolveConnectionID(client, state, selectedPlugin, opts.ConnectionID)
if err != nil {
return fmt.Errorf("no %s connection found \u2014 run 'configure connection' first: %w", pluginDisplayName(selectedPlugin), err)
}
fmt.Printf("   %s connection ID: %d\n", pluginDisplayName(selectedPlugin), connID)

org := resolveOrg(state, opts.Org)
if org == "" {
return fmt.Errorf("organization is required (use --org)")
}
fmt.Printf("   Organization: %s\n", org)

enterprise := resolveEnterprise(state, opts.Enterprise)
if enterprise != "" {
fmt.Printf("   Enterprise: %s\n", enterprise)
}

// Dispatch to plugin-specific scope handler
switch selectedPlugin {
case "github":
_, err = scopeGitHub(client, connID, org, opts)
case "gh-copilot":
_, err = scopeCopilot(client, connID, org, enterprise)
default:
return fmt.Errorf("scope configuration for %q is not yet supported", selectedPlugin)
}
if err != nil {
return err
}

fmt.Println("\n" + strings.Repeat("\u2500", 50))
fmt.Printf("\u2705 %s scopes configured successfully!\n", pluginDisplayName(selectedPlugin))
fmt.Printf("   Connection %d: scopes added\n", connID)
fmt.Println(strings.Repeat("\u2500", 50))
fmt.Println("\nNext step:")
fmt.Println("  Run 'gh devlake configure project' to create a project and start data collection.")

return nil
}

// resolveConnectionID finds a connection ID from flag, state, or API.
func resolveConnectionID(client *devlake.Client, state *devlake.State, plugin string, flagValue int) (int, error) {
if flagValue > 0 {
return flagValue, nil
}
if state != nil {
for _, c := range state.Connections {
if c.Plugin == plugin {
return c.ConnectionID, nil
}
}
}
conns, err := client.ListConnections(plugin)
if err != nil {
return 0, fmt.Errorf("could not list %s connections: %w", plugin, err)
}
if len(conns) == 1 {
return conns[0].ID, nil
}
if len(conns) > 1 {
fmt.Printf("   Multiple %s connections found:\n", plugin)
for _, c := range conns {
fmt.Printf("     ID=%d  Name=%q\n", c.ID, c.Name)
}
input := prompt.ReadLine(fmt.Sprintf("   Enter the %s connection ID to use", plugin))
id, err := strconv.Atoi(input)
if err != nil {
return 0, fmt.Errorf("invalid connection ID: %s", input)
}
return id, nil
}
return 0, fmt.Errorf("no %s connections found \u2014 run 'configure connection' first", plugin)
}

// resolveOrg determines the organization from flag, state, or prompt.
func resolveOrg(state *devlake.State, flagValue string) string {
if flagValue != "" {
return flagValue
}
if state != nil {
for _, c := range state.Connections {
if c.Organization != "" {
return c.Organization
}
}
}
return prompt.ReadLine("Enter organization slug")
}

// resolveEnterprise determines the enterprise slug from flag, state, or API.
func resolveEnterprise(state *devlake.State, flagValue string) string {
if flagValue != "" {
return flagValue
}
if state != nil {
for _, c := range state.Connections {
if c.Enterprise != "" {
return c.Enterprise
}
}
}
return ""
}

// resolveRepos determines repos from flags, file, or interactive gh CLI selection.
func resolveRepos(org string, opts *ScopeOpts) ([]string, error) {
if opts.Repos != "" {
var repos []string
for _, r := range strings.Split(opts.Repos, ",") {
r = strings.TrimSpace(r)
if r != "" {
repos = append(repos, r)
}
}
return repos, nil
}
if opts.ReposFile != "" {
repos, err := repofile.Parse(opts.ReposFile)
if err != nil {
return nil, fmt.Errorf("failed to read repos file: %w", err)
}
fmt.Printf("   Loaded %d repo(s) from file\n", len(repos))
return repos, nil
}
if gh.IsAvailable() {
fmt.Printf("   Listing repos in %q via gh CLI...\n", org)
available, err := gh.ListRepos(org, 30)
if err != nil {
fmt.Printf("   \u26a0\ufe0f  Could not list repos: %v\n", err)
} else if len(available) == 0 {
fmt.Println("   \u26a0\ufe0f  No repos found \u2014 verify the org name and PAT scopes (read:org)")
} else {
selected := prompt.SelectMulti(fmt.Sprintf("Available repos in %s (up to 30)", org), available)
return selected, nil
}
}
input := prompt.ReadLine("Enter repos (comma-separated, e.g. org/repo1,org/repo2)")
var repos []string
for _, r := range strings.Split(input, ",") {
r = strings.TrimSpace(r)
if r != "" {
repos = append(repos, r)
}
}
return repos, nil
}

// ensureScopeConfig creates a DORA scope config or returns an existing one.
func ensureScopeConfig(client *devlake.Client, plugin string, connID int, opts *ScopeOpts) (int, error) {
cfg := &devlake.ScopeConfig{
Name:              "dora-config",
ConnectionID:      connID,
DeploymentPattern: opts.DeployPattern,
ProductionPattern: opts.ProdPattern,
IssueTypeIncident: opts.IncidentLabel,
Refdiff: &devlake.RefdiffConfig{
TagsPattern: ".*",
TagsLimit:   10,
TagsOrder:   "reverse semver",
},
}
result, err := client.CreateScopeConfig(plugin, connID, cfg)
if err == nil {
return result.ID, nil
}
existing, listErr := client.ListScopeConfigs(plugin, connID)
if listErr != nil {
return 0, fmt.Errorf("create failed: %w; list failed: %v", err, listErr)
}
if len(existing) > 0 {
return existing[0].ID, nil
}
return 0, err
}

// putGitHubScopes adds repo scopes to the GitHub connection.
func putGitHubScopes(client *devlake.Client, connID, scopeConfigID int, details []*gh.RepoDetails) error {
var data []any
for _, d := range details {
entry := devlake.GitHubRepoScope{
GithubID:     d.ID,
ConnectionID: connID,
Name:         d.Name,
FullName:     d.FullName,
HTMLURL:      d.HTMLURL,
CloneURL:     d.CloneURL,
}
if scopeConfigID > 0 {
entry.ScopeConfigID = scopeConfigID
}
data = append(data, entry)
}
return client.PutScopes("github", connID, &devlake.ScopeBatchRequest{Data: data})
}

// putCopilotScope adds the organization/enterprise scope to the Copilot connection.
func putCopilotScope(client *devlake.Client, connID int, org, enterprise string) error {
scopeID := copilotScopeID(org, enterprise)
data := []any{
devlake.CopilotScope{
ID:           scopeID,
ConnectionID: connID,
Organization: org,
Enterprise:   enterprise,
Name:         scopeID,
FullName:     scopeID,
},
}
return client.PutScopes("gh-copilot", connID, &devlake.ScopeBatchRequest{Data: data})
}

// copilotScopeID computes the scope ID matching the plugin's convention.
func copilotScopeID(org, enterprise string) string {
org = strings.TrimSpace(org)
enterprise = strings.TrimSpace(enterprise)
if enterprise != "" {
if org != "" {
return enterprise + "/" + org
}
return enterprise
}
return org
}