# Architecture

Technical architecture documentation for `gh-devlake`.

---

## Table of Contents

- [Overview](#overview)
- [Directory Structure](#directory-structure)
- [Command Architecture](#command-architecture)
- [Plugin System](#plugin-system)
- [API Client Layer](#api-client-layer)
- [State Management](#state-management)
- [Token Resolution](#token-resolution)
- [Discovery Chain](#discovery-chain)
- [Terminal Output System](#terminal-output-system)
- [Testing Strategy](#testing-strategy)
- [Design Decisions](#design-decisions)

---

## Overview

`gh-devlake` is a GitHub CLI extension built with Go and the Cobra CLI framework. It orchestrates the deployment and configuration of Apache DevLake through a series of CLI commands that interact with:

- **DevLake REST API** — for connection, scope, and project management
- **Docker CLI** — for local deployment via Docker Compose
- **Azure CLI** — for Azure deployment via Bicep templates
- **GitHub CLI** — for repository and organization metadata

### Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                     gh-devlake CLI                          │
│                  (Cobra Commands)                           │
└─────────────────────────────────────────────────────────────┘
                            │
          ┌─────────────────┼─────────────────┐
          │                 │                 │
          ▼                 ▼                 ▼
   ┌──────────┐      ┌──────────┐     ┌──────────┐
   │  Docker  │      │  Azure   │     │   gh     │
   │   CLI    │      │   CLI    │     │   CLI    │
   └──────────┘      └──────────┘     └──────────┘
          │                 │                 │
          ▼                 ▼                 │
   ┌──────────┐      ┌──────────┐            │
   │  Docker  │      │  Azure   │            │
   │ Compose  │      │ Resources│            │
   └──────────┘      └──────────┘            │
          │                 │                 │
          └─────────────────┴─────────────────┘
                            │
                            ▼
                  ┌──────────────────┐
                  │  DevLake REST    │
                  │      API         │
                  └──────────────────┘
                            │
                            ▼
                  ┌──────────────────┐
                  │  MySQL Database  │
                  └──────────────────┘
```

---

## Directory Structure

```
gh-devlake/
├── cmd/                         # Cobra commands (user-facing)
│   ├── root.go                  # Root command + global flags
│   ├── init.go                  # Interactive wizard
│   ├── deploy_local.go          # Docker Compose deployment
│   ├── deploy_azure.go          # Azure deployment
│   ├── configure_connection_*.go # Connection CRUD
│   ├── configure_scope_*.go     # Scope CRUD
│   ├── configure_project_*.go   # Project CRUD
│   ├── configure_full.go        # Multi-phase orchestrator
│   ├── status.go                # Status command
│   ├── cleanup.go               # Teardown command
│   ├── connection_types.go      # Plugin registry
│   └── helpers.go               # Shared utilities
│
├── internal/                    # Internal packages
│   ├── devlake/                 # DevLake REST API client
│   │   ├── client.go            # Generic HTTP helpers
│   │   ├── types.go             # API types
│   │   ├── discovery.go         # Instance discovery
│   │   ├── state.go             # State file management
│   │   └── *_test.go            # Tests
│   │
│   ├── azure/                   # Azure CLI wrapper
│   │   ├── deploy.go            # Bicep deployment
│   │   ├── templates/           # Bicep templates
│   │   └── suffix.go            # Unique suffix generation
│   │
│   ├── docker/                  # Docker CLI wrapper
│   │   ├── build.go             # Image building
│   │   ├── compose.go           # docker-compose operations
│   │   └── templates/           # docker-compose.yml
│   │
│   ├── gh/                      # GitHub CLI wrapper
│   │   └── gh.go                # Repo list, repo details
│   │
│   ├── prompt/                  # Terminal prompts
│   │   └── prompt.go            # Select, confirm, input
│   │
│   ├── token/                   # Token resolution
│   │   └── token.go             # PAT resolution chain
│   │
│   ├── envfile/                 # .devlake.env parser
│   ├── repofile/                # CSV/TXT repo list parser
│   ├── secrets/                 # Secret generation
│   └── download/                # HTTP downloads
│
├── docs/                        # User documentation
├── .github/                     # CI, agents, skills
├── AGENTS.md                    # AI agent instructions
├── README.md                    # Main documentation
├── CONTRIBUTING.md              # Contribution guide
├── CHANGELOG.md                 # Version history
└── main.go                      # Entry point
```

---

## Command Architecture

### Cobra Command Tree

Commands are organized hierarchically using Cobra:

```
rootCmd (gh devlake)
├── initCmd
├── deployCmd
│   ├── deployLocalCmd
│   └── deployAzureCmd
├── configureCmd
│   ├── configureFullCmd
│   ├── connectionCmd
│   │   ├── connectionAddCmd
│   │   ├── connectionListCmd
│   │   ├── connectionTestCmd
│   │   ├── connectionUpdateCmd
│   │   └── connectionDeleteCmd
│   ├── scopeCmd
│   │   ├── scopeAddCmd
│   │   ├── scopeListCmd
│   │   └── scopeDeleteCmd
│   └── projectCmd
│       ├── projectAddCmd
│       ├── projectListCmd
│       └── projectDeleteCmd
├── statusCmd
└── cleanupCmd
```

### Command Patterns

All commands follow these conventions:

1. **Constructor**: `newXxxCmd()` returns `*cobra.Command`
2. **Run function**: `runXxx(cmd *cobra.Command, args []string) error`
3. **Flags**: Package-level vars, registered in constructor
4. **Validation**: In `RunE`, before business logic
5. **Error handling**: Return `error`, don't call `os.Exit`

Example:

```go
var deployLocalDir string  // Package-level flag var

func newDeployLocalCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "local",
        Short: "Deploy DevLake locally via Docker Compose",
        RunE:  runDeployLocal,
    }
    cmd.Flags().StringVar(&deployLocalDir, "dir", "./devlake", "...")
    return cmd
}

func runDeployLocal(cmd *cobra.Command, args []string) error {
    // Validation
    if deployLocalDir == "" {
        return fmt.Errorf("--dir cannot be empty")
    }

    // Business logic
    fmt.Println("\n🚀 Deploying DevLake locally...")
    // ...

    return nil
}
```

---

## Plugin System

### ConnectionDef Registry

Plugins are defined declaratively in `cmd/connection_types.go`:

```go
type ConnectionDef struct {
    Plugin           string      // Slug: "github", "gh-copilot"
    DisplayName      string      // Human name: "GitHub"
    Available        bool        // false = "coming soon"
    Endpoint         string      // REST endpoint prefix
    RateLimitPerHour int         // API rate limit
    TokenPrompt      string      // Prompt for PAT input
    ScopeHint        string      // Hint for scope selection
    EnvVarNames      []string    // Token env vars
    EnvFileKeys      []string    // Token env file keys
    ScopeFunc        func(...)   // Scope handler (optional)
}

var connectionRegistry = []ConnectionDef{
    {
        Plugin:           "github",
        DisplayName:      "GitHub",
        Available:        true,
        Endpoint:         "/plugins/github/connections",
        RateLimitPerHour: 5000,
        TokenPrompt:      "GitHub PAT (requires repo, read:org, read:user)",
        EnvVarNames:      []string{"GITHUB_TOKEN", "GH_TOKEN"},
        EnvFileKeys:      []string{"GITHUB_TOKEN"},
        ScopeFunc:        addGitHubScopes,
    },
    // ... more plugins
}
```

### Plugin Resolution

Commands use `FindConnectionDef(plugin string)` to look up plugin metadata:

```go
def, err := FindConnectionDef(plugin)
if err != nil {
    return fmt.Errorf("plugin not found: %w", err)
}
if !def.Available {
    return fmt.Errorf("%s connections are coming soon", def.DisplayName)
}
```

### Adding a New Plugin

To add support for a new DevOps tool:

1. Add a `ConnectionDef` entry to `connectionRegistry`
2. Set `Available: true` when ready
3. Implement scope handler if needed (or use generic GitHub-like flow)
4. Add token env var/file keys
5. Update `docs/token-handling.md` with new plugin
6. Update `README.md` Supported Plugins table

**No hardcoded plugin names in switch/case statements** — all behavior derives from `ConnectionDef` fields.

---

## API Client Layer

### Generic HTTP Helpers

The `internal/devlake` package provides typed HTTP helpers:

```go
func doGet[T any](client *Client, path string) (*T, error)
func doPost[T any](client *Client, path string, body any) (*T, error)
func doPut[T any](client *Client, path string, body any) (*T, error)
func doPatch[T any](client *Client, path string, body any) (*T, error)
```

Example usage:

```go
type Connection struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
}

conn, err := doPost[Connection](
    client,
    "/plugins/github/connections",
    ConnectionCreateRequest{
        Name:  "GitHub - my-org",
        Token: "ghp_...",
    },
)
```

### Client Methods

Higher-level methods wrap the generic helpers:

```go
func (c *Client) ListConnections(plugin string) ([]Connection, error)
func (c *Client) CreateConnection(plugin string, req ConnectionCreateRequest) (*Connection, error)
func (c *Client) UpdateConnection(plugin string, id int, req ConnectionUpdateRequest) error
func (c *Client) DeleteConnection(plugin string, id int) error
func (c *Client) TestConnection(plugin string, req ConnectionTestRequest) error
```

### Error Handling

API errors are wrapped with context:

```go
conn, err := client.CreateConnection(plugin, req)
if err != nil {
    return fmt.Errorf("creating connection: %w", err)
}
```

---

## State Management

### State Files

`gh-devlake` persists deployment metadata in JSON state files:

| File | Created By | Contents |
|------|-----------|----------|
| `.devlake-local.json` | `deploy local` | DevLake URL, connection IDs, project name |
| `.devlake-azure.json` | `deploy azure` | Resource group, endpoints, subscription, connection IDs |

### State Structure

```go
type State struct {
    DevLakeURL      string         `json:"devlakeURL,omitempty"`
    ConnectionIDs   map[string]int `json:"connectionIDs,omitempty"`
    ProjectName     string         `json:"projectName,omitempty"`
    BlueprintID     int            `json:"blueprintID,omitempty"`
}
```

### Merge Behavior

`SaveState()` preserves unknown fields by merging:

1. Read existing JSON into `map[string]interface{}`
2. Marshal new `State` struct
3. Unmarshal into same map (overlaying new fields)
4. Write merged JSON back to file

This allows Azure-specific fields (resourceGroup, location) and local-specific fields (directory) to coexist without clobbering each other.

---

## Token Resolution

### Resolution Chain

PATs are resolved in this order (see `internal/token/token.go`):

1. **Flag**: `--token ghp_...`
2. **Env file**: `--env-file .devlake.env` → `GITHUB_TOKEN=ghp_...`
3. **Environment variable**: `GITHUB_TOKEN` or `GH_TOKEN`
4. **Interactive prompt**: Masked input if terminal is interactive

### Per-Plugin Resolution

Each plugin defines its own env var names and file keys:

```go
{
    Plugin:      "github",
    EnvVarNames: []string{"GITHUB_TOKEN", "GH_TOKEN"},
    EnvFileKeys: []string{"GITHUB_TOKEN"},
}

{
    Plugin:      "gh-copilot",
    EnvVarNames: []string{"GITHUB_COPILOT_TOKEN", "GH_TOKEN"},
    EnvFileKeys: []string{"GITHUB_COPILOT_TOKEN"},
}
```

Orchestrators (`init`, `configure full`) resolve tokens independently for each plugin in sequence.

---

## Discovery Chain

### Auto-Discovery

DevLake instance URL is discovered in this order (see `internal/devlake/discovery.go`):

1. **Explicit flag**: `--url http://my-devlake.com`
2. **State files**: `.devlake-azure.json` → `.devlake-local.json`
3. **Well-known ports**: `http://localhost:8080`, `http://localhost:8085`

### Ping Check

Each candidate URL is validated with `GET /ping` (5-second timeout, expects HTTP 200).

### State File Priority

Azure state file is checked first because Azure deployments are more likely to have custom URLs. Local deployments default to `localhost:8080`.

---

## Terminal Output System

### Design Principle

**The terminal IS the UI.** All `fmt.Print` calls are UX decisions.

### Spacing Rules

See `.github/instructions/terminal-output.instructions.md` for full rules. Key points:

- Blank line before every emoji-prefixed step: `fmt.Println("\n🔍 Step...")`
- Sub-items use 3-space indent: `fmt.Printf("   Detail: %s\n", x)`
- Headers use Unicode `═` at 40 characters:
  ```go
  fmt.Println("\n" + strings.Repeat("═", 40))
  fmt.Println("  Title")
  fmt.Println(strings.Repeat("═", 40))
  ```

### Emoji Vocabulary

Standardized emoji for common operations:

| Emoji | Meaning | Example |
|-------|---------|---------|
| 🔍 | Discovery | `Discovering DevLake instance...` |
| 🔑 | Auth | `Resolving PAT...` |
| 📡 | Connection | `Creating GitHub connection...` |
| 🏗️ | Building | `Creating DevLake project...` |
| 🚀 | Deploy | `Deploying infrastructure...` |
| ✅ | Success | `✅ Created!` |
| ⚠️ | Warning | `⚠️ Could not reach endpoint` |

### No ANSI Color Codes

Output uses only emoji and Unicode box-drawing. No ANSI escape codes for colors. This ensures compatibility with all terminals and logging systems.

---

## Testing Strategy

### Test Coverage

Current coverage: ~80% in `internal/devlake` package

Test files:
- `internal/devlake/client_test.go` — REST API client methods
- `internal/devlake/state_test.go` — State file merge behavior
- `internal/devlake/discovery_test.go` — Discovery chain logic
- `internal/docker/docker_test.go` — Docker CLI wrapper
- `internal/gh/gh_test.go` — GitHub CLI wrapper

### Testing Patterns

#### Table-Driven Tests

```go
tests := []struct {
    name       string
    input      string
    want       string
    wantErr    bool
}{
    {"valid", "input", "output", false},
    {"invalid", "bad", "", true},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        got, err := fn(tt.input)
        if (err != nil) != tt.wantErr {
            t.Errorf("unexpected error: %v", err)
        }
        if got != tt.want {
            t.Errorf("got %v, want %v", got, tt.want)
        }
    })
}
```

#### HTTP Mocking

```go
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(Connection{ID: 1, Name: "test"})
}))
defer server.Close()

client := devlake.NewClient(server.URL)
conn, err := client.CreateConnection("github", req)
```

#### CLI Mocking

Docker and GitHub CLI calls are mocked using the TestHelperProcess pattern:

```go
var execCommand = exec.Command  // Package-level var

func TestDockerBuild(t *testing.T) {
    execCommand = fakeExecCommand
    defer func() { execCommand = exec.Command }()
    // ... test
}

func fakeExecCommand(command string, args ...string) *exec.Cmd {
    // Return a cmd that calls TestHelperProcess
}
```

---

## Design Decisions

### One Plugin Per Invocation

Flag-based commands target a single `--plugin`. Interactive mode walks through plugins sequentially.

**Rationale**: Simplifies flag validation, token resolution, and error handling. Avoids flag name conflicts across plugins.

### Declarative Plugin System

Plugin behavior comes from `ConnectionDef` fields, not switch/case branches.

**Rationale**: Enables adding new plugins without modifying orchestrator logic. All token resolution, org prompts, and connection creation derive from registry entries.

### Interactive vs. Flag-Driven

`init` and `configure full` are interactive-only. Individual commands support both modes.

**Rationale**: Interactive orchestrators provide best UX for first-time setup. Flag-driven commands enable CI/CD automation.

### State File Merge

`SaveState()` preserves unknown JSON fields.

**Rationale**: Allows Azure-specific and local-specific fields to coexist without clobbering. Supports future extensibility without breaking changes.

### Explicit URL Over Auto-Discovery

`--url` flag always takes precedence over state files and localhost.

**Rationale**: Enables targeting specific instances when multiple DevLake deployments exist (dev, staging, prod).

### No Embedded Config UI

The CLI does not embed a web server or Config UI.

**Rationale**: DevLake already has a Config UI. The CLI is terminal-first, optimized for automation and repeatability.

---

## Related Documentation

- [AGENTS.md](AGENTS.md) — Quick reference for AI agents
- [CONTRIBUTING.md](CONTRIBUTING.md) — Development workflow
- [.github/copilot-instructions.md](.github/copilot-instructions.md) — Code conventions
- [.github/skills/devlake-dev-integration/SKILL.md](.github/skills/devlake-dev-integration/SKILL.md) — Plugin patterns
- [docs/](docs/) — User-facing documentation
