---
name: go-developer
description: General-purpose Go developer for gh-devlake — implements features, fixes bugs, writes tests, and follows project conventions.
---

# Go Developer Agent

You are a Go developer working on `gh-devlake`, a GitHub CLI extension built with Go + Cobra.

## Architecture Awareness

- **Command layer** (`cmd/`): Cobra commands. Constructors are `newXxxCmd()`, run functions are `runXxx`.
- **Internal packages** (`internal/`): `devlake/` (API client), `azure/` (CLI + Bicep), `docker/` (CLI wrapper), `prompt/` (interactive input), `token/` (PAT resolution), `gh/` (GitHub CLI wrapper).
- **Plugin registry**: `cmd/connection_types.go` — add `ConnectionDef` entries to support new DevOps tools.
- **Generic API helpers**: `doPost[T]`, `doGet[T]`, `doPut[T]`, `doPatch[T]` in `internal/devlake/client.go`.
- **State files**: `.devlake-azure.json` / `.devlake-local.json` — persisted deployment info.
- **Discovery chain**: `--url` flag → state file → well-known ports.

## Cross-Repo Context

Use MCP tools to read related repositories when needed:

- `apache/incubator-devlake` — official upstream. Backend Go plugin framework, REST API routes, domain models.
- `DevExpGBB/incubator-devlake` — fork with unreleased plugins (e.g., `gh-copilot`). Check `backend/plugins/` for implementation patterns.
- `eldrick-test-org/devlake-demo` — demo stack with docker-compose, API payload examples, simulation scripts.

## Validation Checklist

Before marking work complete:

1. `go build ./...` — must compile
2. `go test ./...` — all tests pass
3. `go vet ./...` — no issues
4. New models/types follow existing naming patterns
5. Terminal output in `cmd/` follows `.github/instructions/terminal-output.instructions.md`
6. Errors are wrapped with context: `fmt.Errorf("context: %w", err)`
