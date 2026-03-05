# gh-devlake — Repository Custom Instructions

These instructions apply to all GitHub Copilot interactions in this repository: Chat, Coding Agent, and Code Review Agent.

## Project Context

`gh-devlake` is a GitHub CLI extension (`gh extension`) built with Go + Cobra that automates deployment, configuration, and monitoring of Apache DevLake.

- **Entry point**: `main.go` → `cmd/root.go`
- **Architecture reference**: `AGENTS.md` (read this first for full project context)
- **Command tree**: `init` cascades → `deploy` + `configure full` → `connection` + `scope` + `project`

## Go Conventions

### Naming

- **Command constructors**: `newXxxCmd()` returning `*cobra.Command`
- **Run functions**: `runXxx` (e.g., `runStatus`, `runDeployLocal`)
- **Package vars** for flags: `camelCase` (`deployLocalDir`, `azureRG`)
- **Packages**: short lowercase (`devlake`, `azure`, `docker`, `prompt`)

### Error Handling

- All command `RunE` functions return `error`
- Wrap errors with context: `fmt.Errorf("context: %w", err)`
- Non-fatal errors → print `⚠️` warning and continue
- State file write failures → warning, not fatal

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

## Plugin Registry

- All plugin definitions live in `cmd/connection_types.go` via `ConnectionDef` structs
- **Never hardcode plugin names** (`"github"`, `"gh-copilot"`) in switch/case branches outside `connectionRegistry`
- Adding a new plugin = adding a `ConnectionDef` entry — token resolution, connection creation, and menu labels derive from fields automatically
- Use `doGet[T]`, `doPost[T]`, `doPut[T]`, `doPatch[T]` generic helpers for API calls — do not write raw HTTP

## Terminal Output

All terminal output must follow `.github/instructions/terminal-output.instructions.md`:

- Blank line before every emoji-prefixed step
- Sub-items (3-space indent) stay tight under their parent
- Phase banners get blank lines on both sides
- Standard emoji vocabulary defined in `AGENTS.md`

## Validation Requirements

Before completing any task:

1. `go build ./...` — must compile
2. `go test ./...` — all tests pass
3. `go vet ./...` — no issues

## Code Review Guidelines

When performing a code review on this repository:

- Check that error wrapping follows `fmt.Errorf("context: %w", err)` pattern
- Verify Cobra command constructors use `newXxxCmd()` naming
- Confirm imports follow stdlib → external → internal ordering
- Flag any hardcoded plugin names outside `connectionRegistry`
- Verify `doGet[T]`/`doPost[T]` generics are used instead of raw HTTP calls
- Check that new plugin-specific flags are registered in `ConnectionFlags`/`ScopeFlags` on the `ConnectionDef` for dynamic validation
- Check terminal output follows spacing rules from `terminal-output.instructions.md`
- Verify new commands have test files alongside them
- Ensure README command reference table is updated for new/renamed commands
