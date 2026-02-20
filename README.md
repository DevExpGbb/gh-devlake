# gh-devlake

A [GitHub CLI extension](https://cli.github.com/manual/gh_extension) that deploys, configures, and monitors [Apache DevLake](https://devlake.apache.org/) from the terminal.

Deploy DevLake locally or on Azure, create GitHub and Copilot connections, configure DORA project scopes, and trigger data syncs — without touching the Config UI.

> **Blog post:** [Beyond Copilot Dashboards: Measuring What AI Actually Changes](https://github.com/DevExpGBB/gh-devlake) — why DORA + Copilot correlation matters and what this tool enables.

## Quick Start

```bash
gh extension install DevExpGBB/gh-devlake
gh devlake deploy local --dir ./devlake
cd devlake && docker compose up -d
# wait ~2 minutes, then:
gh devlake configure full --org my-org --repos my-org/repo1,my-org/repo2
```

Open Grafana at http://localhost:3002 (admin/admin). DORA and Copilot dashboards will be populated.

## How DevLake Configuration Works

Four concepts to understand before the commands make sense:

| Concept | What It Is |
|---------|-----------|
| **Connection** | An authenticated link to a data source. Each plugin (GitHub, Copilot) gets its own connection with its own PAT. |
| **Scope** | What to collect from a connection — specific repos for GitHub, an org/enterprise for Copilot. Includes DORA pattern config. |
| **Project** | Groups connections and scopes into a single view with DORA metrics enabled. |
| **Blueprint** | The sync schedule (cron) that re-collects data on a recurring basis. Created automatically when you set up a project. |

You can configure these through the Config UI at `:4000`, or through the DevLake REST API. This CLI automates the API path.

## Installation

```bash
gh extension install DevExpGBB/gh-devlake
```

### From Source

```bash
git clone https://github.com/DevExpGBB/gh-devlake.git
cd gh-devlake
go build -o gh-devlake.exe .   # Windows
go build -o gh-devlake .       # Linux/macOS
gh extension install .
```

### Prerequisites

| Requirement | When needed |
|-------------|-------------|
| [GitHub CLI](https://cli.github.com/) (`gh`) | Always — this is a `gh` extension |
| [Go 1.22+](https://go.dev/) | Building from source only |
| [Docker](https://docs.docker.com/get-docker/) | `deploy local`, `cleanup --local` |
| [Azure CLI](https://learn.microsoft.com/cli/azure/) (`az`) | `deploy azure`, `cleanup --azure` |
| GitHub PAT | `configure connection` / `configure full` |

**Required PAT scopes:**

| Plugin | Required Scopes |
|--------|----------------|
| GitHub | `repo`, `read:org`, `read:user` |
| GitHub Copilot | `manage_billing:copilot`, `read:org` (add `read:enterprise` for enterprise-level metrics) |

---

## Deploy

### Local Docker (recommended to start)

```bash
gh devlake deploy local --dir ./devlake
cd devlake && docker compose up -d
```

Downloads the official Apache DevLake Docker Compose files, generates an `ENCRYPTION_SECRET`, and writes `.env`. After `docker compose up`, give it ~2 minutes.

| Service | URL |
|---------|-----|
| Backend API | http://localhost:8080 |
| Config UI | http://localhost:4000 |
| Grafana | http://localhost:3002 (admin/admin) |

Flags: `--dir` (default: `.`), `--version` (default: `latest`). See [docs/deploy.md](docs/deploy.md).

### Azure (for persistent deployments)

```bash
gh devlake deploy azure --resource-group devlake-rg --location eastus --official
```

Creates Azure Container Instances, MySQL Flexible Server, and Key Vault via Bicep. `--official` uses Docker Hub images (~$30–50/month). Omit `--resource-group` or `--location` to be prompted interactively.

```bash
# Custom images from a fork
gh devlake deploy azure --resource-group devlake-rg --location eastus \
    --repo-url https://github.com/my-fork/incubator-devlake
```

> Tear down with `gh devlake cleanup --azure`.

See [docs/deploy.md](docs/deploy.md) for all flags.

### Init Wizard

```bash
gh devlake init
```

Fully guided 4-phase wizard: deploy → connections → scopes → project. No flags required. Supports both local and Azure targets.

---

## Configure

### Step 1: Create Connections

Create a token file (or use `--token` directly, or let the CLI prompt you):

```bash
echo "GITHUB_TOKEN=ghp_your_pat_here" > .devlake.env
```

**GitHub connection** (repos, PRs, deployments, workflows):

```bash
gh devlake configure connection --plugin github --org my-org
```

**Copilot connection** (usage metrics, seats, acceptance rates):

```bash
gh devlake configure connection --plugin gh-copilot --org my-org
# For enterprise-level metrics:
gh devlake configure connection --plugin gh-copilot --org my-org --enterprise my-enterprise
```

The CLI tests each connection before saving. On success, `.devlake.env` is deleted — tokens are stored encrypted in DevLake.

```
   🔑 Testing connection...
   ✅ Connection test passed
   ✅ Created GitHub connection (ID=1)
```

Omit `--plugin` to select interactively. See [docs/configure-connection.md](docs/configure-connection.md) for all flags and token resolution order.

### Step 2: Add Scopes

Tell DevLake which repos or orgs to collect from:

```bash
# GitHub — specific repos
gh devlake configure scope --plugin github --org my-org --repos my-org/repo1,my-org/repo2

# GitHub — interactive repo selection (omit --repos)
gh devlake configure scope --plugin github --org my-org

# Copilot — org scope
gh devlake configure scope --plugin gh-copilot --org my-org
```

DORA pattern defaults (override with flags):

| Pattern | Default | Flag |
|---------|---------|------|
| Deployment workflow | `(?i)deploy` | `--deployment-pattern` |
| Production environment | `(?i)prod` | `--production-pattern` |
| Incident label | `incident` | `--incident-label` |

`configure scope` only manages scopes — it does not create projects or trigger syncs.

See [docs/configure-scope.md](docs/configure-scope.md) for all flags.

### Step 3: Create a Project and Sync

```bash
gh devlake configure project --org my-org
```

Discovers existing connections and scopes, lets you include them interactively, creates a DevLake project with DORA metrics enabled, configures a daily sync blueprint, and triggers the first data collection.

```
   [10s] Status: TASK_RUNNING | Tasks: 2/8
   [30s] Status: TASK_RUNNING | Tasks: 5/8
   [60s] Status: TASK_COMPLETED | Tasks: 8/8

   ✅ Data sync completed!
```

Key flags: `--project-name`, `--cron` (default: `0 0 * * *`), `--time-after` (default: 6 months ago), `--skip-sync`, `--timeout`. See [docs/configure-project.md](docs/configure-project.md).

### Or: Do It All at Once

`configure full` chains connections → scopes → project in 3 phases:

```bash
gh devlake configure full --org my-org --repos my-org/repo1,my-org/repo2
```

Use `--plugin` to limit to a single plugin. Accepts all flags from `configure connection`, `configure scope`, and `configure project`. See [docs/configure-full.md](docs/configure-full.md).

---

## Day-2 Operations

### Status

```bash
gh devlake status
```

Shows deployment info, service health (Backend/Grafana/Config UI), connections, and project configuration.

### Manage Connections

```bash
gh devlake configure connection list                                           # list all
gh devlake configure connection test --plugin github --id 1                   # test a saved connection
gh devlake configure connection update --plugin github --id 1 --token ghp_new # rotate token
gh devlake configure connection delete --plugin gh-copilot --id 2             # remove a connection
```

All connection subcommands support interactive mode — omit flags to be prompted. See [docs/configure-connection.md](docs/configure-connection.md).

### Add More Repos

Re-run `configure scope` — existing connections and projects are preserved.

### Tear Down

```bash
gh devlake cleanup --local           # stop Docker Compose containers
gh devlake cleanup --azure --force   # delete Azure resource group (no prompt)
```

See [docs/cleanup.md](docs/cleanup.md).

---

## Command Reference

| Command | Description | Reference |
|---------|-------------|-----------|
| `gh devlake init` | Guided 4-phase setup wizard | [docs/init.md](docs/init.md) |
| `gh devlake status` | Health check and connection summary | [docs/status.md](docs/status.md) |
| `gh devlake deploy local` | Local Docker Compose deploy | [docs/deploy.md](docs/deploy.md) |
| `gh devlake deploy azure` | Azure Container Instance deploy | [docs/deploy.md](docs/deploy.md) |
| `gh devlake configure connection` | Create a plugin connection | [docs/configure-connection.md](docs/configure-connection.md) |
| `gh devlake configure connection list` | List all connections | [docs/configure-connection.md](docs/configure-connection.md) |
| `gh devlake configure connection test` | Test a saved connection | [docs/configure-connection.md](docs/configure-connection.md) |
| `gh devlake configure connection update` | Rotate token or update settings | [docs/configure-connection.md](docs/configure-connection.md) |
| `gh devlake configure connection delete` | Remove a connection | [docs/configure-connection.md](docs/configure-connection.md) |
| `gh devlake configure scope` | Add repo/org scopes to a connection | [docs/configure-scope.md](docs/configure-scope.md) |
| `gh devlake configure project` | Create project + blueprint + first sync | [docs/configure-project.md](docs/configure-project.md) |
| `gh devlake configure full` | Connections + scopes + project in one step | [docs/configure-full.md](docs/configure-full.md) |
| `gh devlake cleanup` | Tear down local or Azure resources | [docs/cleanup.md](docs/cleanup.md) |

---

## Secure Token Handling

The extension **never** stores your PAT in command history or logs:

1. Create a `.devlake.env` file:
   ```
   GITHUB_TOKEN=ghp_your_token_here
   ```
2. Run the configure command — the token is read from the file.
3. After success, `.devlake.env` is **automatically deleted** (use `--skip-cleanup` to keep it). Tokens are stored encrypted in DevLake's database.

The `.devlake.env` file is in `.gitignore` by default.

---

## State Files

| File | Created By | Purpose |
|------|-----------|---------|
| `.devlake-azure.json` | `deploy azure` | Azure resources, endpoints |
| `.devlake-local.json` | `configure connection` | Endpoints, connection IDs (local deploys) |
| `.devlake.env` | User | **Ephemeral** — PATs for connection creation, auto-deleted after use |

State files enable command chaining. `configure scope` and `configure project` read connection IDs saved by `configure connection`, so you don't need to pass `--connection-id` manually.

---

## Development

```bash
go build -o gh-devlake.exe .     # Build (Windows)
go build -o gh-devlake .         # Build (Linux/macOS)
go test ./... -v                  # Run tests
gh extension install .            # Install locally for testing
```

### Project Structure

```
├── main.go
├── cmd/                             # Cobra command tree
│   ├── root.go                      # Root command + global flags
│   ├── status.go                    # status
│   ├── init.go                      # init wizard
│   ├── deploy.go / deploy_local.go / deploy_azure.go
│   ├── cleanup.go
│   ├── configure.go                 # configure parent
│   ├── configure_connections.go     # configure connection + subcommands
│   ├── configure_scopes.go          # configure scope
│   ├── configure_projects.go        # configure project
│   ├── configure_full.go            # configure full
│   └── connection_types.go          # plugin registry
└── internal/
    ├── azure/                       # Azure CLI wrapper + Bicep templates
    ├── devlake/                     # REST API client + auto-discovery
    ├── docker/                      # Docker CLI wrapper
    ├── download/                    # HTTP downloads + GitHub release tags
    ├── envfile/                     # .devlake.env parser
    ├── gh/                          # GitHub CLI wrapper
    ├── prompt/                      # Interactive terminal prompts
    ├── repofile/                    # Repo list file parser
    ├── secrets/                     # Cryptographic secret generation
    └── token/                       # PAT resolution chain
```

## License

MIT — see [LICENSE](LICENSE).
