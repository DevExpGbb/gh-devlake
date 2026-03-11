# stop

Gracefully stops running DevLake services without removing containers, volumes, or state.

## Usage

```bash
gh devlake stop [flags]
```

Auto-detects deployment type from state files in the current directory.

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--service <name>` | *(all)* | Stop only a specific service (e.g., `grafana`) |
| `--local` | `false` | Force local (Docker Compose) stop mode |
| `--azure` | `false` | Force Azure stop mode |
| `--state-file <path>` | *(auto-detected)* | Path to state file |

## Auto-Detection

Without `--local` or `--azure`, the command checks:
1. `--state-file` path (if provided)
2. `.devlake-azure.json` → Azure mode
3. `.devlake-local.json` → Local mode
4. `docker-compose.yml` in current directory → Local mode

If no deployment is detected, an error is returned — use `gh devlake deploy` to create a new deployment.

## Local Deployments (Docker Compose)

Runs `docker compose stop` from the deployment directory. This is non-destructive:
- Containers are stopped but not removed
- Volumes and data are preserved
- A quick restart is possible with `gh devlake start`

```bash
gh devlake stop
gh devlake stop --service grafana
```

What it does:
1. Checks Docker availability
2. Runs `docker compose stop` (with optional service filter)
3. Confirms containers are stopped

> **Data preserved:** Unlike `gh devlake cleanup`, `stop` does not remove containers or volumes. Your data (database, Grafana dashboards) remains intact.

## Azure Deployments (Container Instances)

Reads container names and resource group from `.devlake-azure.json` and stops running resources.

```bash
gh devlake stop --azure
gh devlake stop --azure --service backend
```

What it does:
1. Reads resource group and container names from `.devlake-azure.json`
2. Checks Azure CLI login
3. Stops each Container Instance via `az container stop`
4. Stops the MySQL flexible server (when stopping all services)

## JSON Output

```bash
gh devlake stop --json
```

Returns:
```json
{"status": "stopped", "mode": "local"}
```

## Examples

```bash
# Auto-detect and stop all services
gh devlake stop

# Stop only Grafana (local)
gh devlake stop --service grafana

# Force Azure mode
gh devlake stop --azure

# Use a specific state file
gh devlake stop --state-file /path/to/.devlake-azure.json
```

## Mental Model

| Command | Effect |
|---------|--------|
| `gh devlake start` | Bring stopped services back up (idempotent) |
| `gh devlake stop` | Pause services (non-destructive, data preserved) |
| `gh devlake cleanup` | Permanent teardown (removes containers, volumes, files) |

## Related

- [start.md](start.md) — bring services back up after stop
- [status.md](status.md) — check service health
- [cleanup.md](cleanup.md) — permanent teardown
- [day-2.md](day-2.md) — day-2 operations overview
