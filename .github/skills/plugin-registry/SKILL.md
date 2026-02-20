---
name: plugin-registry
description: How the ConnectionDef plugin registry works — adding plugins, token resolution, scope handling, and the tool-agnostic design. Use when adding a new DevOps tool plugin, modifying ConnectionDef fields, changing how connections/scopes/tokens are resolved, or debugging plugin-specific behavior.
---

# Plugin Registry — ConnectionDef as Single Source of Truth

## ConnectionDef Fields

Each plugin is a `ConnectionDef` struct in `cmd/connection_types.go`:

```go
type ConnectionDef struct {
    Plugin           string   // DevLake plugin slug (e.g. "github", "gh-copilot")
    DisplayName      string   // User-facing name (e.g. "GitHub Copilot")
    Available        bool     // false = "coming soon" in menus
    Endpoint         string   // Default API endpoint
    NeedsOrg         bool     // Prompt for org during connection creation
    NeedsEnterprise  bool     // Prompt for enterprise during connection creation
    SupportsTest     bool     // Test connection before creating
    RateLimitPerHour int      // API rate limit (0 = default 4500)
    EnableGraphql    bool     // Send enableGraphql=true in payloads
    RequiredScopes   []string // PAT scopes for documentation
    ScopeHint        string   // Displayed in token prompt
    TokenPrompt      string   // Label for masked token prompt
    OrgPrompt        string   // Label for org prompt (empty = not prompted)
    EnterprisePrompt string   // Label for enterprise prompt (empty = not prompted)
    EnvVarNames      []string // Environment variables for token resolution
    EnvFileKeys      []string // .devlake.env keys for token resolution
}
```

## Adding a New Plugin

1. Add a `ConnectionDef` entry to `connectionRegistry` in `cmd/connection_types.go`
2. Set `Available: true` when ready (false = "coming soon")
3. Add a scope function (e.g. `scopeGitLab`) in `cmd/configure_scopes.go`
4. Add a `case` in the scope dispatch switch in `init.go`, `configure_full.go`, and `configure_scopes.go`

No other registration needed — token resolution, connection creation, help text, and menu labels all derive from the `ConnectionDef` fields.

## Token Resolution Chain

Token resolution is fully data-driven via `token.ResolveOpts`:

```go
token.Resolve(token.ResolveOpts{
    FlagValue:   flagToken,
    EnvFilePath: envFile,
    EnvFileKeys: def.EnvFileKeys,   // e.g. ["GITHUB_PAT", "GITHUB_TOKEN"]
    EnvVarNames: def.EnvVarNames,   // e.g. ["GITHUB_TOKEN", "GH_TOKEN"]
    DisplayName: def.DisplayName,   // e.g. "GitHub Copilot"
    ScopeHint:   def.ScopeHint,     // e.g. "manage_billing:copilot, read:org"
})
```

Priority: `--token` flag → `.devlake.env` file → environment variable → interactive masked prompt.

There are NO switch/case statements on plugin names in the token package. All lookup keys come from the caller (ConnectionDef fields).

## Build/Test Request Construction

`BuildCreateRequest` and `BuildTestRequest` on `ConnectionDef` read fields directly:

- `RateLimitPerHour` → `req.RateLimitPerHour` (default 4500 if 0)
- `EnableGraphql` → `req.EnableGraphql`
- `NeedsOrg` → conditionally includes `req.Organization`
- `NeedsEnterprise` → conditionally includes `req.Enterprise`

No `if d.Plugin == "github"` branches — behavior is declarative.

The `ConnectionTestRequest` includes a `Name` field (required by some plugins like gh-copilot for validation).

## Scope Handling

Each plugin has its own scope function:

| Plugin | Scope Function | Data Scope |
|--------|---------------|------------|
| `github` | `scopeGitHub()` | Repos (via `gh repo list` or `--repos` flag) |
| `gh-copilot` | `scopeCopilot()` | Org/enterprise (computed as scope ID) |

### Copilot Scope ID Convention

```go
func copilotScopeID(org, enterprise string) string
// enterprise + org → "enterprise/org"
// enterprise only → "enterprise"
// org only        → "org"
```

This must match `listGhCopilotRemoteScopes` in the DevLake backend exactly.

### Scope Config

Each plugin has different scope config shapes:
- **GitHub**: `deploymentPattern`, `productionPattern`, `issueTypeIncident` (DORA patterns)
- **Copilot**: `baselinePeriodDays`, `implementationDate`

`ensureScopeConfig(client, plugin, connID, opts)` accepts the plugin string as a parameter.

## Per-Plugin Resolution in Orchestrators

`runConnectionsInternal` (used by `configure full` and `init`) resolves per-plugin:

```
for each selected ConnectionDef:
  1. Resolve token using def.EnvVarNames, def.TokenPrompt
  2. Prompt for org if def.NeedsOrg (using def.OrgPrompt)
  3. Prompt for enterprise if def.NeedsEnterprise (using def.EnterprisePrompt)
  4. Test & create connection
```

This supports mixed-plugin flows where different plugins need different tokens (e.g. GitHub + GitLab).

## Scope ID Resolution in Projects

`listConnectionScopes()` resolves scope IDs differently per plugin:
- GitHub: uses `githubId` (int) from scope response
- Copilot: uses `id` (string) from scope response

This is pragmatic — DevLake's scope API returns plugin-specific ID fields.

## State Files

Connection results are saved to state files (`.devlake-azure.json` or `.devlake-local.json`):

```go
StateConnection{
    Plugin:       "gh-copilot",
    ConnectionID: 1,
    Name:         "GitHub Copilot - my-org",
    Organization: "my-org",
    Enterprise:   "my-enterprise",
}
```

State enables command chaining — `configure scope` and `configure project` read connection IDs from state.

## Common Pitfalls

- Forgetting to set `RateLimitPerHour` → defaults to 4500, which may be wrong for some plugins
- Not setting `TokenPrompt` → generic "PAT" label in prompts
- Copilot scope ID must match backend's `listGhCopilotRemoteScopes` logic exactly
- On Windows, always build with `-o gh-devlake.exe` — PowerShell resolves `.exe` preferentially
