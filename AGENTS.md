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

## Terminal Output & UX — CRITICAL

**The terminal IS the UI.** Every `fmt.Print` call is a UX decision. Readability, rhythm, and breathing room are non-negotiable.

### Line Spacing Rules

These rules are mandatory for all terminal output in `cmd/` files.

**Rule 1: Blank line before every step.**
Every step (emoji + action text) MUST be preceded by `\n`:
```go
// ✅ CORRECT — blank line before step
fmt.Println("\n🔍 Discovering DevLake instance...")

// ❌ WRONG — no breathing room, steps run together
fmt.Println("🔍 Discovering DevLake instance...")
```

**Rule 2: Blank line before AND after section separators.**
Separator lines (`───`, `═══`, phase banners) must have blank lines on both sides:
```go
// ✅ CORRECT
fmt.Println("\n" + strings.Repeat("─", 50))
fmt.Println("✅ Connection configured!")
fmt.Println(strings.Repeat("─", 50))
fmt.Println()

// ❌ WRONG — separator jammed against previous/next output
fmt.Println(strings.Repeat("─", 50))
fmt.Println("✅ Connection configured!")
fmt.Println(strings.Repeat("─", 50))
```

**Rule 3: Blank line after completion banners.**
After a success/summary block, always add a trailing blank line:
```go
fmt.Println("═══════════════════════════════════════")
fmt.Println("  ✅ Setup Complete!")
fmt.Println("═══════════════════════════════════════")
fmt.Println()  // ← always trail with blank line
```

**Rule 4: Sub-items stay tight under their parent.**
Detail lines (3-space indent) do NOT get blank lines between them:
```go
// ✅ CORRECT — grouped tight under parent step
fmt.Println("\n📡 Creating GitHub connection...")
fmt.Printf("   Endpoint: %s\n", endpoint)
fmt.Printf("   Token:    %s\n", masked)
fmt.Printf("   ✅ Created (ID=%d)\n", conn.ID)

// ❌ WRONG — unnecessary blank lines break grouping
fmt.Println("\n📡 Creating GitHub connection...")
fmt.Println()
fmt.Printf("   Endpoint: %s\n", endpoint)
```

**Rule 5: Phase banners get a blank line before and after.**
```go
fmt.Println()  // or use \n prefix on next line

fmt.Println("╔══════════════════════════════════════╗")
fmt.Println("║  PHASE 1: Configure Connections      ║")
fmt.Println("╚══════════════════════════════════════╝")

fmt.Println()  // blank line after banner before first step
```

### Header Standards

All headers use Unicode double-line `═` at **40 characters** width:

**Top-level command banner:**
```go
fmt.Println("\n════════════════════════════════════════")
fmt.Println("  DevLake — Command Title")
fmt.Println("════════════════════════════════════════")
```

**Phase banner (box-drawing):**
```go
fmt.Println("\n╔══════════════════════════════════════╗")
fmt.Println("║  PHASE N: Description                ║")
fmt.Println("╚══════════════════════════════════════╝")
```

**Section separator (thin line):**
```go
fmt.Println("\n" + strings.Repeat("─", 50))
```

**Completion banner:**
```go
fmt.Println("\n════════════════════════════════════════")
fmt.Println("  ✅ Action Complete!")
fmt.Println("════════════════════════════════════════")
fmt.Println()
```

### Emoji Usage

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

### Indentation

- **Steps**: No indent — start at column 0 with emoji
- **Sub-items under a step**: 3-space indent (`"   "`)
- **Content inside banners**: 2-space indent (`"  "`)
- **Bullet lists**: 2-space indent + `•` (`"  • item"`)
- **Numbered steps**: 2-space indent + number (`"  1. step"`)

### UX Principles

1. **Think end-to-end.** Before adding output, mentally walk through the entire command flow. Does the full terminal scroll look clean? Are steps visually distinct? Can a user scan from top to bottom and understand what happened?

2. **Breathing room over density.** When in doubt, add a blank line. Cramped output is harder to scan than output with generous spacing.

3. **Group related output, separate unrelated steps.** Sub-items under a step stay tight; different steps get blank lines between them.

4. **Every command tells a story.** The output should read as: banner → steps with progress → summary. The user should never wonder "is it still running?" or "did that work?"

5. **Consistency is UX.** If one command uses `═══` headers and another uses `===`, it feels broken even if it works. Match the established patterns exactly.

6. **Prompts go to stderr, output to stdout.** Keep stdout clean for piping. All `prompt.*` functions write to `os.Stderr`.

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
```
