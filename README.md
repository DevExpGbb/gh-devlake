# gh-devlake

A [GitHub CLI extension](https://cli.github.com/manual/gh_extension) for managing [Apache DevLake](https://devlake.apache.org/) deployments.

Deploy, configure, and monitor DevLake instances from the command line. Automates connection setup, scope configuration, and health monitoring.

## Installation

```bash
gh extension install DevExpGBB/gh-devlake
```

Or build from source:

```bash
git clone https://github.com/DevExpGBB/gh-devlake
cd gh-devlake
go build -o gh-devlake .
gh extension install .
```

## Commands

### `gh devlake status`

Check DevLake health and list configured connections.

```bash
gh devlake status
gh devlake status --url http://localhost:8085
```

### `gh devlake configure connections`

Create GitHub and Copilot connections in DevLake using a PAT.

```bash
gh devlake configure connections --org my-org
gh devlake configure connections --org my-org --enterprise my-enterprise
gh devlake configure connections --org my-org --env-file ./my-env
```

**Token resolution order:**
1. `--token` flag
2. `.devlake.env` file (`GITHUB_PAT=...`)
3. `$GITHUB_TOKEN` / `$GH_TOKEN` environment variables
4. Interactive masked prompt

### `gh devlake configure scopes`

Configure collection scopes (repos), create a DORA project, and trigger the first sync.

```bash
gh devlake configure scopes --org my-org --repos owner/repo1,owner/repo2
gh devlake configure scopes --org my-org --repos-file repos.txt
gh devlake configure scopes --org my-org  # interactive repo selection via gh CLI
```

**Key flags:** `--deployment-pattern`, `--production-pattern`, `--incident-label`, `--skip-copilot`, `--skip-sync`, `--time-after`, `--cron`

### `gh devlake configure full`

Run connections + scopes in one step.

```bash
gh devlake configure full --org my-org --repos owner/repo1,owner/repo2
gh devlake configure full --org my-org --repos-file repos.txt
```

### `gh devlake deploy local`

Download and set up official DevLake via Docker Compose.

```bash
gh devlake deploy local
gh devlake deploy local --version v1.0.2 --dir ./devlake
```

### `gh devlake deploy azure`

Deploy DevLake to Azure using Bicep templates (Container Instances + MySQL).

```bash
gh devlake deploy azure --resource-group devlake-rg --location eastus
gh devlake deploy azure --resource-group devlake-rg --location eastus --official
```

### `gh devlake cleanup`

Tear down DevLake resources (auto-detects local vs Azure).

```bash
gh devlake cleanup
gh devlake cleanup --azure --force
gh devlake cleanup --local
```

## Secure Token Handling

The extension **never** stores your PAT in command history or logs. Recommended workflow:

1. Create a `.devlake.env` file:
   ```
   GITHUB_PAT=ghp_your_token_here
   ```
2. Run the configure command:
   ```bash
   gh devlake configure connections --org my-org
   ```
3. The `.devlake.env` file is **automatically deleted** after successful setup (use `--skip-cleanup` to keep it).

The `.devlake.env` file is in `.gitignore` by default.

## DevLake Discovery

The extension auto-discovers running DevLake instances by checking:

1. `--url` flag (explicit)
2. State files (`.devlake-azure.json`, `.devlake-local.json`) in the current directory
3. Well-known localhost ports (`8080`, `8085`)

## Development

```bash
go build -o gh-devlake.exe .    # Build
go test ./... -v                 # Test
```

## Project Structure

```
├── main.go                          # Entry point
├── cmd/                             # Cobra command tree
│   ├── root.go                      # Root command + global flags
│   ├── configure.go                 # configure parent command
│   ├── configure_connections.go     # configure connections
│   ├── configure_scopes.go          # configure scopes + project + sync
│   ├── configure_stubs.go           # configure full (chains connections→scopes)
│   ├── deploy.go                    # deploy parent + cleanup wiring
│   ├── deploy_local.go              # deploy local (Docker Compose)
│   ├── deploy_azure.go              # deploy azure (Bicep)
│   ├── cleanup.go                   # cleanup (Azure + local)
│   └── status.go                    # status command
├── internal/
│   ├── azure/
│   │   ├── cli.go                   # Azure CLI wrapper
│   │   ├── suffix.go                # Deterministic resource suffix
│   │   ├── templates.go             # Embedded Bicep templates
│   │   └── templates/*.bicep        # Bicep IaC files
│   ├── devlake/
│   │   ├── client.go                # DevLake REST API client
│   │   ├── discovery.go             # Auto-discover DevLake instances
│   │   ├── state.go                 # State file management
│   │   └── types.go                 # API request/response types
│   ├── docker/
│   │   └── build.go                 # Docker CLI wrapper
│   ├── download/
│   │   └── download.go              # HTTP download + GitHub releases
│   ├── envfile/
│   │   └── envfile.go               # .devlake.env parser
│   ├── gh/
│   │   └── gh.go                    # GitHub CLI wrapper
│   ├── prompt/
│   │   └── prompt.go                # Interactive terminal prompts
│   ├── repofile/
│   │   └── parse.go                 # Repo list file parser
│   ├── secrets/
│   │   └── generate.go              # Cryptographic secret generation
│   └── token/
│       └── resolve.go               # PAT resolution chain
└── .gitignore
```

## License

MIT
