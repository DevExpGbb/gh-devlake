---
name: devlake-dev-integration
description: DevLake plugin registry and REST API integration patterns — ConnectionDef wiring, token resolution, scope handling, and typed API helpers. Use when adding plugins, modifying connections/scopes/tokens, or implementing DevLake API calls.
---

# Plugin Registry & API Integration

This skill covers how gh-devlake wires plugins into the CLI (via `ConnectionDef`) and how it communicates with the DevLake backend REST API.

## Adding a New Plugin

1. Add a `ConnectionDef` entry to `connectionRegistry` in `cmd/connection_types.go`
2. Set `Available: true` when ready (false = "coming soon")
3. Add a scope function (e.g. `scopeGitLab`) in `cmd/configure_scopes.go`
4. Add a `case` in the scope dispatch switch in `configure_scopes.go` (this is the only file with a scope dispatch switch)

No other registration needed — token resolution, connection creation, help text, and menu labels all derive from the `ConnectionDef` fields.

See [ConnectionDef field reference](references/connection-def-fields.md) for the full struct definition and field-by-field documentation.

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

Both `BuildCreateRequest` and `BuildTestRequest` accept a `ConnectionParams` struct:

```go
type ConnectionParams struct {
    Token      string
    Org        string
    Enterprise string
    Name       string // override default connection name
    Proxy      string // HTTP proxy URL
    Endpoint   string // override default endpoint (e.g. GitHub Enterprise Server)
}
```

Fields are mapped declaratively from `ConnectionDef`:

- `RateLimitPerHour` → `req.RateLimitPerHour` (default 4500 if 0)
- `EnableGraphql` → `req.EnableGraphql`
- `NeedsOrg` or `NeedsOrgOrEnt` → conditionally includes `req.Organization`
- `NeedsEnterprise` or `NeedsOrgOrEnt` → conditionally includes `req.Enterprise`

`NeedsOrgOrEnt` means the plugin requires at least one of org or enterprise (e.g. `gh-copilot`), but not necessarily both. This is distinct from `NeedsOrg`/`NeedsEnterprise` which each require their specific field.

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
  2. Prompt for org if def.NeedsOrg or def.NeedsOrgOrEnt (using def.OrgPrompt)
  3. Prompt for enterprise if def.NeedsEnterprise or def.NeedsOrgOrEnt (using def.EnterprisePrompt)
  4. Build ConnectionParams, test & create connection
```

This supports mixed-plugin flows where different plugins need different tokens (e.g. GitHub + GitLab).

## Scope ID Resolution in Projects

`listConnectionScopes()` resolves scope IDs differently per plugin:
- GitHub: uses `githubId` (int) from scope response
- Copilot: uses `id` (string) from scope response

This is pragmatic — DevLake's scope API returns plugin-specific ID fields.

## REST API Patterns

All API calls use generic helpers in `internal/devlake/client.go`. See [API endpoint reference](references/api-endpoints.md) for the full endpoint table and helper signatures.

Usage pattern:

```go
result, err := doPost[Connection](c, "/plugins/github/connections", req)
```

All helpers: marshal request → send → check status → unmarshal response into `*T`.

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

## Cross-Repo References

For deeper API understanding, read these from related repos using MCP tools:

- **apache/incubator-devlake**: `backend/server/api/` (routes), `backend/core/plugin/` (interfaces), `backend/plugins/github/api/` (reference impl)
- **DevExpGBB/incubator-devlake**: `backend/plugins/gh-copilot/` (custom Copilot plugin with `listGhCopilotRemoteScopes`)
- **eldrick-test-org/devlake-demo**: `scripts/` (PowerShell API call examples), `README.md` (payload examples)

## Common Pitfalls

- Forgetting to set `RateLimitPerHour` → defaults to 4500, which may be wrong for some plugins
- Not setting `TokenPrompt` → generic "PAT" label in prompts
- Copilot scope ID must match backend's `listGhCopilotRemoteScopes` logic exactly
- On Windows, always build with `-o gh-devlake.exe` — PowerShell resolves `.exe` preferentially
