# Architecture Overview

[← Back to README](../README.md)

This extension is a Go + Cobra CLI that wraps the DevLake REST API and related tooling (Docker, Azure CLI, GitHub CLI). Understanding where behaviors live makes it easier to add commands or debug flows.

## Command Layout

- **Entry point**: `main.go` wires the Cobra tree via `cmd/root.go`.
- **Command files**: `cmd/` holds all user-facing commands. Constructors follow the `newXxxCmd()` pattern and delegate to `runXxx` functions for behavior.
- **Command tree**: `init` drives the guided setup; `deploy`, `configure`, `status`, and `cleanup` map to single-purpose flows. See the full tree in [AGENTS.md](../AGENTS.md).

## Key Packages

| Package | Role |
|---------|------|
| `internal/devlake` | REST client (`doGet`, `doPost`, `doPut`, `doPatch`), state files, instance discovery (`/ping` checks). |
| `internal/docker` | Docker CLI wrapper for local deploys. |
| `internal/azure` | Azure CLI wrapper + Bicep templates for cloud deploys. |
| `internal/gh` | GitHub CLI wrapper (repo listing, repo details). |
| `internal/prompt` | Interactive prompts used by setup and configure flows. |

## Plugin Registry

Supported DevLake plugins are declared in `cmd/connection_types.go` as `ConnectionDef` entries. The registry drives:

- Token and username resolution (flag → env file → env vars → prompt).
- Scope handling (per-plugin scope prompts via `ScopeFunc`).
- Connection creation and testing against the DevLake API.

Avoid hardcoding plugin slugs elsewhere—dispatch comes from the registry.

## State & Discovery

Deploy commands persist connection details to `.devlake-local.json` or `.devlake-azure.json`. Discovery prefers `--url`, then state files, then well-known localhost ports (8080/8085) using `/ping` health checks.

## Terminal Output

All terminal UX is in `cmd/` and follows `.github/instructions/terminal-output.instructions.md`: blank lines before emoji-prefixed steps, 40-character `═` banners, and tight 3-space indents for sub-items. This keeps interactive flows readable in the terminal.

## Where to Start When Adding Features

1) Add or update a `ConnectionDef` if a new DevOps tool is involved.  
2) Create a new `cmd` file with a `newXxxCmd()` constructor and `runXxx` executor.  
3) Use the generic DevLake client helpers for REST calls instead of raw `http`.  
4) Extend docs in `docs/` and the README command table alongside the new command.
