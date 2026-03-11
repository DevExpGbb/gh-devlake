# start

Brings up stopped or exited DevLake services for an existing deployment.

## Usage

```bash
gh devlake start [flags]
```

Auto-detects deployment type from state files in the current directory.

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--service <name>` | *(all)* | Start only a specific service (e.g., `config-ui`) |
| `--no-wait` | `false` | Skip health polling after start |
| `--local` | `false` | Force local (Docker Compose) start mode |
| `--azure` | `false` | Force Azure start mode |
| `--state-file <path>` | *(auto-detected)* | Path to state file |

## Auto-Detection

Without `--local` or `--azure`, the command checks:
1. `--state-file` path (if provided)
2. `.devlake-azure.json` → Azure mode
3. `.devlake-local.json` → Local mode
4. `docker-compose.yml` in current directory → Local mode

If no deployment is detected, an error is returned — use `gh devlake deploy` to create a new deployment.

## Local Deployments (Docker Compose)

Runs `docker compose up -d` from the current directory. This is idempotent:
- Running containers are unaffected
- Stopped containers are started
- Crashed or exited containers are restarted

```bash
gh devlake start
gh devlake start --service config-ui
gh devlake start --no-wait
```

What it does:
1. Checks Docker availability
2. Runs `docker compose up -d` (with optional service filter)
3. Polls the backend `/ping` endpoint until healthy (up to 60s)
4. Prints service URLs

> **Shorter health timeout:** `start` uses a 60-second health timeout (vs 6 minutes for `deploy`) because databases and volumes are already initialized.

## Azure Deployments (Container Instances)

Reads container names and resource group from `.devlake-azure.json` and starts any stopped resources.

```bash
gh devlake start --azure
gh devlake start --azure --service backend
```

What it does:
1. Reads resource group and container names from `.devlake-azure.json`
2. Checks Azure CLI login
3. Starts the MySQL flexible server (if present)
4. Starts each Container Instance via `az container start`
5. Polls the backend endpoint until healthy (up to 60s)
6. Prints endpoints

## JSON Output

```bash
gh devlake start --json
```

Returns:
```json
{"status": "started", "mode": "local"}
```

## Examples

```bash
# Auto-detect and start all services
gh devlake start

# Start only config-ui (local)
gh devlake start --service config-ui

# Start without waiting for health check
gh devlake start --no-wait

# Force Azure mode
gh devlake start --azure

# Use a specific state file
gh devlake start --state-file /path/to/.devlake-azure.json
```

## Motivating Scenario

After a machine reboot, `gh devlake status` shows `❌` for one or more services. Instead of manually finding the docker-compose directory and running raw Docker commands, run:

```bash
gh devlake start
```

## Related

- [status.md](status.md) — check service health
- [cleanup.md](cleanup.md) — tear down all resources
- [deploy.md](deploy.md) — initial deployment
- [day-2.md](day-2.md) — day-2 operations overview
