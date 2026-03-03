# ConnectionDef Field Reference

Each plugin is a `ConnectionDef` struct in `cmd/connection_types.go`:

```go
type ConnectionDef struct {
    Plugin           string       // DevLake plugin slug (e.g. "github", "gh-copilot")
    DisplayName      string       // User-facing name (e.g. "GitHub Copilot")
    Available        bool         // false = "coming soon" in menus
    Endpoint         string       // Default API endpoint
    NeedsOrg         bool         // Prompt for org during connection creation
    NeedsEnterprise  bool         // Prompt for enterprise during connection creation
    NeedsOrgOrEnt    bool         // Requires at least one of org or enterprise
    SupportsTest     bool         // Test connection before creating
    RateLimitPerHour int          // API rate limit (0 = default 4500)
    EnableGraphql    bool         // Send enableGraphql=true in payloads
    RequiredScopes   []string     // PAT scopes for documentation
    ScopeHint        string       // Displayed in token prompt
    TokenPrompt      string       // Label for masked token prompt
    OrgPrompt        string       // Label for org prompt (empty = not prompted)
    EnterprisePrompt string       // Label for enterprise prompt (empty = not prompted)
    EnvVarNames      []string     // Environment variables for token resolution
    EnvFileKeys      []string     // .devlake.env keys for token resolution
    ScopeFunc        ScopeHandler // Function that configures scopes. nil = not yet supported
}
```

## Field Details

| Field | Required | Purpose |
|-------|----------|---------|
| `Plugin` | Yes | DevLake plugin slug used in API paths (e.g. `/plugins/{Plugin}/connections`) |
| `DisplayName` | Yes | Human-readable name shown in prompts and banners |
| `Available` | Yes | `true` = selectable; `false` = shown as "coming soon" |
| `Endpoint` | Yes | Default API base URL (e.g. `https://api.github.com`) |
| `NeedsOrg` | No | If true, always prompts user for organization |
| `NeedsEnterprise` | No | If true, always prompts user for enterprise slug |
| `NeedsOrgOrEnt` | No | If true, prompts for both org and enterprise but requires at least one. Used by `gh-copilot` |
| `SupportsTest` | No | If true, test-connection is called before creating |
| `RateLimitPerHour` | No | Sent in create/test payloads. 0 defaults to 4500 |
| `EnableGraphql` | No | Adds `enableGraphql: true` to connection payloads |
| `RequiredScopes` | No | PAT OAuth scopes listed in help text |
| `ScopeHint` | No | Shown next to masked token prompt (e.g. "manage_billing:copilot") |
| `TokenPrompt` | No | Label for the masked input. Empty = generic "PAT" |
| `OrgPrompt` | No | Label for org input. Empty = org not prompted |
| `EnterprisePrompt` | No | Label for enterprise input. Empty = not prompted |
| `EnvVarNames` | No | Environment variables checked for token (e.g. `["GITHUB_TOKEN"]`) |
| `EnvFileKeys` | No | Keys in `.devlake.env` checked for token (e.g. `["GITHUB_PAT"]`) |
| `ScopeFunc` | No | Function that configures scopes for this plugin's connections. `nil` = not yet supported |
