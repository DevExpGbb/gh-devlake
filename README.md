# gh-devlake

A [GitHub CLI extension](https://cli.github.com/manual/gh_extension) that deploys, configures, and monitors [Apache DevLake](https://github.com/apache/incubator-devlake) from the terminal.

Deploy DevLake locally or on Azure, create connections to GitHub and Copilot, configure DORA project scopes, and trigger data syncs — all without touching the Config UI.

DevLake is an open-source dev data platform that normalizes data from DevOps tools so you can compute consistent engineering metrics like DORA. 

This CLI makes that setup fast and repeatable from the terminal (instead of clicking through the Config UI) — especially when you want to re-run the same configuration across teams.

> **Blog post:** [Beyond Copilot Dashboards: Measuring What AI Actually Changes](<!-- TODO: replace with actual blog URL -->) — why DORA + Copilot correlation matters and what this tool enables.

> [!NOTE]
> **GitHub Copilot plugin:** The Copilot metrics plugin is currently available in the [DevExpGBB/incubator-devlake](https://github.com/DevExpGBB/incubator-devlake) fork while the upstream PR ([apache/incubator-devlake#8728](https://github.com/apache/incubator-devlake/pull/8728)) is under review. The `deploy local` and `deploy azure` commands handle custom images automatically — no manual image builds needed. Once the PR merges, the official Apache images will include it.

<!-- SCREENSHOT: Grafana DORA dashboard + Copilot Adoption dashboard side-by-side — the "payoff" -->

---

## Prerequisites

| Requirement | When needed |
|-------------|-------------|
| [GitHub CLI](https://cli.github.com/) (`gh`) | Always — this is a `gh` extension |
| [Docker](https://docs.docker.com/get-docker/) | `deploy local`, `cleanup --local` |
| [Azure CLI](https://learn.microsoft.com/cli/azure/) (`az`) | `deploy azure` only |
| GitHub PAT | Required when creating connections (see Supported Plugins) |

<details>
<summary><strong>Building from source</strong></summary>

Requires [Go 1.22+](https://go.dev/).

```bash
git clone https://github.com/DevExpGBB/gh-devlake.git
cd gh-devlake
go build -o gh-devlake.exe .   # Windows
go build -o gh-devlake .       # Linux/macOS
gh extension install .
```

</details>

---

## Quick Start

```bash
gh extension install DevExpGBB/gh-devlake

# Option 1: Fully guided wizard (deploy → connect → scope → project)
gh devlake init

# Option 2: Step-by-step
gh devlake deploy local --dir ./devlake
cd devlake && docker compose up -d
# wait ~2 minutes, then:
gh devlake configure full
```

After setup, open Grafana at **http://localhost:3002** (admin / admin). DORA and Copilot dashboards will populate after the first sync completes.

| Service | URL |
|---------|-----|
| Grafana | http://localhost:3002 (admin/admin) |
| Config UI | http://localhost:4000 |
| Backend API | http://localhost:8080 |

---

## How DevLake Works

Four concepts to understand — then every command makes sense:

| Concept | What It Is |
|---------|-----------|
| **Connection** | An authenticated link to a data source (GitHub, Copilot). Each gets its own PAT. |
| **Scope** | *What* to collect — specific repos for GitHub, an org/enterprise for Copilot. |
| **Project** | Groups connections + scopes into a single view with DORA metrics enabled. |
| **Blueprint** | The sync schedule (cron). Created automatically with the project. |

<!-- SCREENSHOT: Config UI project page showing connections → scopes → project hierarchy -->

For a deeper explanation with diagrams, see [DevLake Concepts](docs/concepts.md).

---

## Getting Started Step by Step

### Step 1: Deploy

```bash
gh devlake deploy local --dir ./devlake
cd devlake && docker compose up -d
```

Downloads Docker Compose files, generates secrets, and prepares the stack. Give it ~2 minutes after `docker compose up`. See [docs/deploy.md](docs/deploy.md) for flags and details.

<details>
<summary><strong>Deploying to Azure instead</strong></summary>

```bash
gh devlake deploy azure --resource-group devlake-rg --location eastus --official
```

Creates Container Instances, MySQL Flexible Server, and Key Vault via Bicep (~$30–50/month with `--official`). Omit flags to be prompted interactively.

See [docs/deploy.md](docs/deploy.md) for all Azure options, custom image builds, and tear-down.

</details>

### Step 2: Create Connections

The CLI will prompt you for your PAT. You can also pass `--token`, use an `--env-file`, or set `GITHUB_TOKEN` in your environment. See [Token Handling](docs/token-handling.md) for the full resolution chain.

```bash
# GitHub (repos, PRs, workflows, deployments)
gh devlake configure connection --plugin github --org my-org

# Copilot (usage metrics, seats, acceptance rates)
gh devlake configure connection --plugin gh-copilot --org my-org
```

The CLI tests each connection before saving. On success:

```
   🔑 Testing connection...
   ✅ Connection test passed
   ✅ Created GitHub connection (ID=1)
```

See [docs/configure-connection.md](docs/configure-connection.md) for all flags.

### Step 3: Add Scopes

Tell DevLake which repos or orgs to collect from:

```bash
# GitHub — pick repos interactively, or pass --repos explicitly
gh devlake configure scope --plugin github --org my-org

# Copilot — org-level metrics
gh devlake configure scope --plugin gh-copilot --org my-org
```

DORA patterns (deployment workflow, production environment, incident label) use sensible defaults. See [docs/configure-scope.md](docs/configure-scope.md) for overrides.

### Step 4: Create a Project and Sync

```bash
gh devlake configure project
```

Discovers your connections and scopes, creates a DevLake project with DORA metrics enabled, sets up a daily sync blueprint, and triggers the first data collection.

See [docs/configure-project.md](docs/configure-project.md) for flags (`--project-name`, `--cron`, `--time-after`, `--skip-sync`).

<!-- SCREENSHOT: Terminal output of a successful configure project → sync completion -->

> **Shortcut:** `gh devlake configure full` chains Steps 2–4 interactively. See [docs/configure-full.md](docs/configure-full.md).

---

## Day-2 Operations

<details>
<summary>Status checks, token rotation, adding repos, and tear-down</summary>

```bash
gh devlake status                                                              # health + summary
gh devlake configure connection list                                           # list connections
gh devlake configure connection update --plugin github --id 1 --token ghp_new # rotate token
gh devlake configure scope --plugin github --org my-org                        # add more repos
gh devlake cleanup --local                                                     # tear down Docker
```

For the full guide, see [Day-2 Operations](docs/day-2.md).

</details>

---

## Supported Plugins

| Plugin | Status | What It Collects | Required PAT scopes |
|--------|--------|------------------|---------------------|
| GitHub | ✅ Available | Repos, PRs, issues, workflows, deployments (DORA) | `repo`, `read:org`, `read:user` |
| GitHub Copilot | ✅ Available | Usage metrics, seats, acceptance rates | `manage_billing:copilot`, `read:org` (+ `read:enterprise` for enterprise metrics) |
| Azure DevOps | 🔜 Coming soon | Repos, pipelines, deployments (DORA) | (TBD) |
| GitLab | 🔜 Coming soon | Repos, MRs, pipelines, deployments (DORA) | (TBD) |

See [Token Handling](docs/token-handling.md) for env key names and multi-plugin `.devlake.env` examples.

---

## Command Reference

| Command | Description | Docs |
|---------|-------------|------|
| `gh devlake init` | Guided 4-phase setup wizard | [init.md](docs/init.md) |
| `gh devlake status` | Health check and connection summary | [status.md](docs/status.md) |
| `gh devlake deploy local` | Local Docker Compose deploy | [deploy.md](docs/deploy.md) |
| `gh devlake deploy azure` | Azure Container Instance deploy | [deploy.md](docs/deploy.md) |
| `gh devlake configure connection` | Create a plugin connection | [configure-connection.md](docs/configure-connection.md) |
| `gh devlake configure connection list` | List all connections | [configure-connection.md](docs/configure-connection.md) |
| `gh devlake configure connection test` | Test a saved connection | [configure-connection.md](docs/configure-connection.md) |
| `gh devlake configure connection update` | Rotate token or update settings | [configure-connection.md](docs/configure-connection.md) |
| `gh devlake configure connection delete` | Remove a connection | [configure-connection.md](docs/configure-connection.md) |
| `gh devlake configure scope` | Add repo/org scopes to a connection | [configure-scope.md](docs/configure-scope.md) |
| `gh devlake configure project` | Create project + blueprint + first sync | [configure-project.md](docs/configure-project.md) |
| `gh devlake configure full` | Connections + scopes + project in one step | [configure-full.md](docs/configure-full.md) |
| `gh devlake cleanup` | Tear down local or Azure resources | [cleanup.md](docs/cleanup.md) |

Additional references: [Token Handling](docs/token-handling.md) · [State Files](docs/state-files.md) · [DevLake Concepts](docs/concepts.md) · [Day-2 Operations](docs/day-2.md)

---

## Development

```bash
go build -o gh-devlake.exe .     # Build (Windows)
go build -o gh-devlake .         # Build (Linux/macOS)
go test ./... -v                  # Run tests
gh extension install .            # Install locally for testing
```

## License

MIT — see [LICENSE](LICENSE).
