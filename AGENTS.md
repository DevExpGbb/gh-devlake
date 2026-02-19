# gh-devlake — AI Coding Agent Instructions

## Project Overview

`gh-devlake` is a GitHub CLI extension (`gh extension`) that automates the full lifecycle of Apache DevLake — deploy, configure, and monitor — from a single terminal. Built with Go + Cobra, output uses emoji and Unicode box-drawing (no ANSI color codes).

## Architecture

```
cmd/                 # Cobra commands — all user-facing terminal output lives here
internal/
  azure/             # Azure CLI wrapper + Bicep templates
  devlake/           # REST API client, auto-discovery, state file management
  docker/            # Docker CLI wrapper (build, compose up/down)
  download/          # HTTP file downloads + GitHub release tag fetch
  envfile/           # .devlake.env parser
  gh/                # GitHub CLI wrapper (repo list, repo details)
  prompt/            # Interactive terminal prompts (confirm, select, masked input)
  repofile/          # CSV/TXT repo list parser
  secrets/           # Cryptographic secret generation
  token/             # PAT resolution chain (flag → envfile → env → prompt)
```

### Key Patterns
- **Command constructors**: `newDeployLocalCmd()` returns `*cobra.Command`
- **Run functions**: `runStatus`, `runDeployLocal` — `run<CommandName>`
- **Plugin registry**: `connection_types.go` — ordered list of plugin definitions
- **State files**: `.devlake-azure.json` / `.devlake-local.json` persist deployment info
- **Discovery chain**: explicit `--url` → state file → well-known ports
- **Generic API helpers**: `doPost[T]`, `doGet[T]`, `doPut[T]`, `doPatch[T]` in `internal/devlake/client.go`

### Plugin System
Plugins are defined via `ConnectionDef` structs in `cmd/connection_types.go`. Each entry declares the plugin slug, endpoint, required fields (`NeedsOrg`, `NeedsEnterprise`), and PAT scopes. To add a new DevOps tool, add a `ConnectionDef` to `connectionRegistry` — no other registration needed.

**One plugin per invocation.** Flag-based commands target a single `--plugin`. Interactive mode walks through plugins sequentially. This keeps plugin-specific fields (org, enterprise, repos, tokens) self-contained.

### Copilot Scope ID Convention
The `gh-copilot` plugin computes scope IDs as: enterprise + org → `"enterprise/org"`, enterprise only → `"enterprise"`, org only → `"org"`. See `copilotScopeID()` in `cmd/configure_scopes.go`. The scope ID must match the plugin's `listGhCopilotRemoteScopes` logic exactly or blueprint references will break.

## Roadmap
See `.github/skills/gh-devlake-roadmap/SKILL.md` for version plan, milestones, and design decisions. Project board: https://github.com/orgs/DevExpGbb/projects/21

## Terminal Output & UX

**The terminal IS the UI.** See `.github/instructions/terminal-output.instructions.md` for the full formatting rules (auto-applied to `cmd/**/*.go`).

Quick reference — standard emoji vocabulary:

| Emoji | Meaning | Example |
|-------|---------|---------|
| 🔍 | Discovery / search | `Discovering DevLake instance...` |
| 🔑 | Authentication / secrets | `Resolving PAT...` |
| 📡 | Connection | `Creating GitHub connection...` |
| 🏗️ | Building / creating | `Creating DevLake project...` |
| 🚀 | Deploy / launch | `Deploying infrastructure...` |
| 🐳 | Docker operations | `Starting containers...` |
| 📦 | Resources / packages | `Creating Resource Group...` |
| 📝 | Writing / scopes | `Adding repository scopes...` |
| ⏳ | Waiting / polling | `Waiting for DevLake to be ready...` |
| ✅ | Success | Inline after steps, completion banners |
| ❌ | Failure | Status ping failure |
| ⚠️ | Warning (non-fatal) | `Could not reach endpoint` |
| 💾 | Save / persist | `State saved to ...` |
| 🧹 | Cleanup | `Cleaning up resources...` |

## Related Repositories

These repos provide essential context. Use MCP tools (`mcp_github_get_file_contents`, `mcp_github_search_code`) to read them.

| Repository | Purpose |
|------------|---------|
| `apache/incubator-devlake` | Official upstream — backend Go code, plugin framework, domain models, REST API routes |
| `DevExpGBB/incubator-devlake` | Fork with unreleased custom plugins (gh-copilot). Rolls up to apache upstream |
| `eldrick-test-org/devlake-demo` | Demo stack — docker-compose, simulation scripts, API payload examples |

## Code Conventions

### Error Handling
- All command `RunE` functions return `error`
- Wrap errors: `fmt.Errorf("context: %w", err)`
- Non-fatal errors → print `⚠️` warning and continue
- State file write failures → warning, not fatal

### Naming
- **Package vars** for flags: `camelCase` (`deployLocalDir`, `azureRG`)
- **Command constructors**: `newXxxCmd()` returning `*cobra.Command`
- **Run functions**: `runXxx` (`runStatus`, `runDeployLocal`)
- **Packages**: short lowercase (`devlake`, `azure`, `docker`, `prompt`)

### Imports
Standard Go convention — stdlib, then external, then internal, separated by blank lines:
```go
import (
    "fmt"
    "os"

    "github.com/spf13/cobra"

    "github.com/DevExpGBB/gh-devlake/internal/devlake"
)
```

### Flags
- Global flags on `rootCmd.PersistentFlags()`
- Command-specific flags as package-level vars with `cmd.Flags().StringVar()`
- Required flags validated in `RunE` (not via `MarkFlagRequired`)

## Testing

- Unit tests: `*_test.go` alongside source
- Run: `go test ./...`
- Suffix generation test: `internal/azure/suffix_test.go`

## Build & Run

```bash
go build -o gh-devlake .          # Build the extension
gh extension install .             # Install locally for testing
gh devlake status                  # Run via gh CLI
gh devlake configure connection    # Create a plugin connection
gh devlake configure scope         # Configure collection scopes
gh devlake configure project       # Create a project and start data collection
```
