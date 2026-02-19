# gh-devlake

A [GitHub CLI extension](https://cli.github.com/manual/gh_extension) that deploys, configures, and monitors [Apache DevLake](https://devlake.apache.org/) instances from the command line.

One CLI to go from zero to DORA dashboards: deploy DevLake (locally or on Azure), create GitHub + Copilot connections, add repository scopes, build a DORA project, and trigger data syncs — all without touching the Config UI.

## Quick Start

```bash
# Install the extension
gh extension install DevExpGBB/gh-devlake

# Deploy DevLake locally (downloads official Docker Compose files)
gh devlake deploy local --dir ./my-devlake
cd my-devlake && docker compose up -d

# Wait ~2 minutes, then configure everything in one shot
gh devlake configure full --org my-org --repos my-org/repo1,my-org/repo2

# Check health and connections
gh devlake status
```

## Installation

### From GitHub Release (recommended)

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

| Plugin | Required Scopes | Notes |
|--------|----------------|-------|
| GitHub | `repo`, `read:org`, `read:user` | `repo` covers `repo:status` and `repo_deployment` |
| GitHub Copilot | `manage_billing:copilot`, `read:org` | Add `read:enterprise` for enterprise-level metrics |

## How It Works

The extension manages three phases of DevLake setup:

```
Phase 1 — Deploy          Phase 2 — Connections       Phase 3 — Scopes & Project
─────────────────          ─────────────────────       ──────────────────────────
deploy local               configure connection        configure scope
deploy azure

                            ─── or combine ───
cleanup                     configure full  (Phase 2 + 3 in one step)
```

**Auto-discovery** finds your DevLake instance automatically:
1. `--url` flag (explicit)
2. State files (`.devlake-azure.json`, `.devlake-local.json`) in the current directory
3. Well-known localhost ports (`8080`, `8085`)

**State files** track deployment info, connection IDs, and project configuration so commands can chain together without re-specifying IDs.

---

## Commands

### `gh devlake status`

Check DevLake health and list configured connections.

```bash
gh devlake status
gh devlake status --url http://localhost:8085
```

Output:
```
═══════════════════════════════════════════
  DevLake Status
═══════════════════════════════════════════

  Deployment  [.devlake-local.json]
  ──────────────────────────────────────────
  Method:    local
  Deployed:  2026-02-18 12:00 UTC

  Services
  ──────────────────────────────────────────
  Backend    ✅  http://localhost:8080
  Grafana    ✅  http://localhost:3002

  Connections
  ──────────────────────────────────────────
  GitHub              ID=1    "GitHub - my-org"
  GitHub Copilot      ID=1    "Copilot - my-org"  [org: my-org]

  Project
  ──────────────────────────────────────────
  Name:       my-org
  Blueprint:  1
  Repos:      my-org/app1, my-org/app2

═══════════════════════════════════════════
```

---

### `gh devlake deploy local`

Download official Apache DevLake Docker Compose files, generate an encryption secret, and prepare for `docker compose up`.

```bash
gh devlake deploy local
gh devlake deploy local --version v1.0.2 --dir ./devlake
```

| Flag | Default | Description |
|------|---------|-------------|
| `--dir` | `.` | Target directory for files |
| `--version` | `latest` | DevLake release version (e.g., `v1.0.2`) |

**What it does:**
1. Fetches the latest release tag from GitHub (or uses `--version`)
2. Downloads `docker-compose.yml` and `env.example`
3. Renames `env.example` → `.env`
4. Generates and injects a cryptographic `ENCRYPTION_SECRET`
5. Checks that Docker is available

**After running:** `cd <dir> && docker compose up -d`, then wait ~2 minutes.

**Endpoints:**
- Config UI: http://localhost:4000
- Grafana: http://localhost:3002 (admin/admin)
- Backend API: http://localhost:8080

---

### `gh devlake deploy azure`

Deploy DevLake to Azure using Bicep templates (Container Instances + MySQL Flexible Server + Key Vault).

```bash
# Official Apache images (no ACR, ~$30-50/month)
gh devlake deploy azure --resource-group devlake-rg --location eastus --official

# Custom images built from source (with ACR, ~$50-75/month)
gh devlake deploy azure --resource-group devlake-rg --location eastus

# Build from a remote fork
gh devlake deploy azure --resource-group devlake-rg --location eastus \
    --repo-url https://github.com/my-fork/incubator-devlake
```

| Flag | Default | Description |
|------|---------|-------------|
| `--resource-group` | *(required)* | Azure Resource Group name |
| `--location` | *(required)* | Azure region (e.g., `eastus`) |
| `--base-name` | `devlake` | Base name for Azure resources |
| `--official` | `false` | Use official Docker Hub images (no ACR) |
| `--skip-image-build` | `false` | Skip Docker image build step |
| `--repo-url` | | Clone a remote repo for building |

**What it does:**
1. Checks Azure CLI login (prompts `az login` if needed)
2. Creates the resource group
3. Generates MySQL password and encryption secret
4. Optionally builds + pushes Docker images to ACR
5. Deploys infrastructure via Bicep
6. Waits for the backend to respond, triggers DB migration
7. Saves `.devlake-azure.json` state file

---

### `gh devlake configure connection`

Create a plugin connection in DevLake using a PAT. If `--plugin` is not specified, prompts interactively.

```bash
gh devlake configure connection --plugin github --org my-org
gh devlake configure connection --plugin gh-copilot --org my-org
gh devlake configure connection --org my-org --name "My GitHub" --proxy http://proxy:8080
gh devlake configure connection --org my-org --endpoint https://github.example.com/api/v3/
```

| Flag | Default | Description |
|------|---------|-------------|
| `--plugin` | *(interactive)* | Plugin to configure (`github`, `gh-copilot`) |
| `--org` | *(required for Copilot)* | GitHub organization slug |
| `--enterprise` | | GitHub enterprise slug (for Copilot enterprise metrics) |
| `--name` | `Plugin - org` | Connection display name |
| `--endpoint` | `https://api.github.com/` | API endpoint (for GitHub Enterprise Server) |
| `--proxy` | | HTTP proxy URL |
| `--token` | | GitHub PAT (highest priority) |
| `--env-file` | `.devlake.env` | Path to env file containing `GITHUB_PAT` |
| `--skip-cleanup` | `false` | Don't delete `.devlake.env` after setup |

**Token resolution order:**
1. `--token` flag
2. `.devlake.env` file (`GITHUB_PAT=` or `GITHUB_TOKEN=` or `GH_TOKEN=`)
3. `$GITHUB_TOKEN` / `$GH_TOKEN` environment variables
4. Interactive masked prompt (terminal only)

**What it does:**
1. Auto-discovers DevLake instance
2. Resolves the GitHub PAT (displays required scopes if prompting interactively)
3. Prompts for connection name (Enter accepts default), proxy (Enter skips)
4. For GitHub: offers Cloud vs Enterprise Server endpoint choice
5. Tests the connection payload (GitHub only)
6. Creates the plugin connection
7. Saves connection ID to the state file
8. Deletes `.devlake.env` (tokens now stored encrypted in DevLake)

After creating connections, run `configure scope` to create a project and start data collection.

---

### `gh devlake configure scope`

Add repository scopes, create a DORA project with a blueprint, and trigger the first data sync.

```bash
# Specify repos directly
gh devlake configure scope --org my-org --repos my-org/app1,my-org/app2

# Load repos from a file (one owner/repo per line)
gh devlake configure scope --org my-org --repos-file repos.txt

# Interactive selection via gh CLI
gh devlake configure scope --org my-org

# Custom DORA patterns
gh devlake configure scope --org my-org --repos my-org/app1 \
    --deployment-pattern "(?i)(deploy|release)" \
    --production-pattern "(?i)(prod|production)" \
    --incident-label "bug/incident"
```

| Flag | Default | Description |
|------|---------|-------------|
| `--org` | | GitHub organization slug |
| `--repos` | | Comma-separated repos (`owner/repo`) |
| `--repos-file` | | Path to file with repos (one per line) |
| `--project-name` | *(org name)* | DevLake project name |
| `--deployment-pattern` | `(?i)deploy` | Regex matching deployment CI/CD workflows |
| `--production-pattern` | `(?i)prod` | Regex matching production environments |
| `--incident-label` | `incident` | Issue label identifying incidents |
| `--cron` | `0 0 * * *` | Blueprint sync schedule (daily midnight) |
| `--time-after` | *(6 months ago)* | Only collect data after this date |
| `--skip-copilot` | `false` | Skip adding Copilot org scope |
| `--skip-sync` | `false` | Skip triggering the first data sync |
| `--wait` | `true` | Wait for pipeline to complete |
| `--timeout` | `5m` | Max time to wait for pipeline |
| `--github-connection-id` | *(auto)* | Override auto-detected GitHub connection ID |
| `--copilot-connection-id` | *(auto)* | Override auto-detected Copilot connection ID |

**What it does:**
1. Discovers DevLake and resolves connection IDs (from state file or API)
2. Resolves repos (from flag, file, or interactive `gh repo list` selection)
3. Looks up repo details via `gh api repos/<owner>/<repo>`
4. Creates a DORA scope config (deployment/production patterns, incident label)
5. Adds repo scopes to the GitHub connection
6. Adds Copilot org scope (unless `--skip-copilot`)
7. Creates a DevLake project with DORA metrics enabled
8. Configures the project's blueprint with connection scopes
9. Triggers the first data sync and monitors the pipeline

---

### `gh devlake configure full`

Combine connections + scopes configuration in one step (Phase 2 + Phase 3).

```bash
gh devlake configure full --org my-org --repos my-org/app1,my-org/app2
gh devlake configure full --org my-org --repos-file repos.txt
gh devlake configure full --org my-org --enterprise my-ent --skip-sync
```

Accepts all flags from both `configure connection` and `configure scope`. Runs Phase 2 first, then Phase 3 — wiring connection IDs automatically between the two phases.

---

### `gh devlake cleanup`

Tear down DevLake resources. Auto-detects deployment type from state files.

```bash
gh devlake cleanup                  # auto-detect mode
gh devlake cleanup --local          # stop Docker Compose containers
gh devlake cleanup --azure --force  # delete Azure resource group (no prompt)
```

| Flag | Default | Description |
|------|---------|-------------|
| `--azure` | `false` | Force Azure cleanup mode |
| `--local` | `false` | Force local cleanup mode |
| `--force` | `false` | Skip confirmation prompt |
| `--keep-resource-group` | `false` | Delete resources but keep the Azure RG |
| `--state-file` | *(auto)* | Path to state file |

---

## Secure Token Handling

The extension **never** stores your PAT in command history or logs:

1. Create a `.devlake.env` file:
   ```
   GITHUB_TOKEN=ghp_your_token_here
   ```
2. Run the configure command — the token is read from the file:
   ```bash
   gh devlake configure connection --org my-org
   ```
3. After success, `.devlake.env` is **automatically deleted** (use `--skip-cleanup` to keep it). Tokens are now stored encrypted in DevLake's database.

The `.devlake.env` file is in `.gitignore` by default.

---

## State Files

| File | Created By | Purpose |
|------|-----------|---------|
| `.devlake-azure.json` | `deploy azure` | Azure resources, endpoints, suffix |
| `.devlake-local.json` | `configure connection` | Endpoints, connection IDs (local deploys) |
| `.devlake.env` | User (Phase 2) | **Ephemeral** — PATs for connection creation, auto-deleted |

State files enable command chaining — `configure scope` reads connection IDs saved by `configure connection`, so you don't need to pass `--github-connection-id`.

---

## End-to-End Example

```bash
# 1. Install
gh extension install DevExpGBB/gh-devlake

# 2. Deploy DevLake locally
gh devlake deploy local --dir ./devlake
cd devlake && docker compose up -d
# Wait ~2 minutes for services to start

# 3. Check health
gh devlake status

# 4. Create a secrets file (paste your PAT, save it)
echo "GITHUB_TOKEN=" > .devlake.env
# Edit .devlake.env and add your PAT after the =

# 5. Configure everything
gh devlake configure full --org my-org --repos my-org/app1,my-org/app2

# 6. Open Grafana dashboards
# http://localhost:3002 (admin/admin)
# Navigate to DORA dashboard

# 7. When done, tear down
gh devlake cleanup --local
```

---

## Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--url` | *(auto-discovered)* | DevLake API base URL |
| `--help` | | Show help for any command |

Use `--url` to target a specific DevLake instance. If omitted, the extension auto-discovers from state files or well-known localhost ports.

---

## Development

```bash
go build -o gh-devlake.exe .     # Build (Windows)
go build -o gh-devlake .         # Build (Linux/macOS)
go test ./... -v                  # Run tests
gh extension install .            # Install locally for testing
```

## Project Structure

```
├── main.go                          # Entry point
├── cmd/                             # Cobra command tree
│   ├── root.go                      # Root command + global flags
│   ├── status.go                    # status — health + connections
│   ├── deploy.go                    # deploy parent + cleanup wiring
│   ├── deploy_local.go              # deploy local (Docker Compose)
│   ├── deploy_azure.go              # deploy azure (Bicep)
│   ├── cleanup.go                   # cleanup (Azure + local)
│   ├── configure.go                 # configure parent command
│   ├── configure_connections.go     # configure connection (single plugin)
│   ├── configure_scopes.go          # configure scope + project + sync
│   ├── configure_full.go            # configure full (connections + scopes)
│   ├── connection_types.go          # plugin registry & connection builder
├── internal/
│   ├── azure/                       # Azure CLI wrapper + embedded Bicep
│   ├── devlake/                     # DevLake REST API client + discovery
│   ├── docker/                      # Docker CLI wrapper
│   ├── download/                    # HTTP download + GitHub releases
│   ├── envfile/                     # .devlake.env parser
│   ├── gh/                          # GitHub CLI wrapper (repo lookup)
│   ├── prompt/                      # Interactive terminal prompts
│   ├── repofile/                    # Repo list file parser (CSV/TXT)
│   ├── secrets/                     # Cryptographic secret generation
│   └── token/                       # PAT resolution chain
└── LICENSE
```

## License

MIT — see [LICENSE](LICENSE).
