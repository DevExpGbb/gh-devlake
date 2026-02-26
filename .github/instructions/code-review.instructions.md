---
description: "Go-specific review rules for gh-devlake CLI code. Applied automatically when reviewing Go files in cmd/ and internal/."
applyTo: "cmd/**/*.go,internal/**/*.go"
---

# Go Code Review Rules

These rules supplement the general guidance in `.github/copilot-instructions.md` with Go-specific checks for the `gh-devlake` codebase.

## Cobra Command Patterns

- Command constructors must be named `newXxxCmd()` and return `*cobra.Command`
- Run functions must be named `runXxx` (e.g., `runStatus`, `runDeployLocal`)
- Parent commands that only group subcommands should NOT have a `RunE` — they print help when invoked without a subcommand
- Subcommands register via `parentCmd.AddCommand(newXxxCmd())` in an `init()` function or constructor

## Plugin Registry

- Flag any `case "github":` or `case "gh-copilot":` in switch statements — scope dispatch and connection creation should use `ConnectionDef` fields from `connectionRegistry`, not hardcoded plugin names
- New `ConnectionDef` entries must have all required fields: `Plugin`, `DisplayName`, `Available`, `TokenPrompt`, `ScopeHint`, `EnvVarNames`, `EnvFileKeys`
- `ScopeFunc` (when present) should be set on the registry entry, not dispatched via switch

## API Calls

- All DevLake REST API calls must use `doGet[T]`, `doPost[T]`, `doPut[T]`, or `doPatch[T]` generic helpers from `internal/devlake/client.go`
- Do not use `http.NewRequest` directly — wrap in the typed helpers
- New client methods should follow the established signature pattern: `func (c *Client) XxxMethod(params...) (*ResultType, error)`

## Error Handling

- `RunE` functions must return `error`, not call `os.Exit`
- Wrap errors with context: `fmt.Errorf("creating connection: %w", err)`
- Non-fatal issues print `⚠️` warnings and continue — do not return an error for recoverable situations
- State file write failures are warnings, not fatal errors

## Terminal Output

- Every emoji-prefixed step must have `\n` before it
- Sub-items use 3-space indent, stay tight under their parent (no blank lines between them)
- Phase banners get blank lines on both sides
- Reference `.github/instructions/terminal-output.instructions.md` for the full rules
