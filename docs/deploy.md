# deploy

Deploy DevLake locally via Docker Compose or to Azure via Bicep.

## deploy local

Sets up DevLake Docker Compose files and starts containers (by default). Supports official Apache releases, forked repositories, or custom compose configurations.

### Usage

```bash
gh devlake deploy local [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--dir` | `.` | Target directory for Docker Compose files |
| `--version` | `latest` | DevLake release version (e.g., `v1.0.2`) |
| `--source` | *(interactive if omitted)* | Image source: `official`, `fork`, or `custom` |
| `--repo-url` | | Repository URL to clone (for `fork` source) |
| `--start` | `true` | Start containers after setup |

### What It Does

Depending on `--source`:

**official** (default):
1. Fetches the latest release tag from GitHub (or uses `--version`)
2. Downloads `docker-compose.yml` and `env.example` from the Apache DevLake release
3. Renames `env.example` → `.env`
4. Generates and injects a cryptographic `ENCRYPTION_SECRET` into `.env`
5. Checks that Docker is available
6. Starts containers (unless `--start=false`)

**fork**:
1. Clones the repository specified by `--repo-url`
2. Builds DevLake images from source
3. Generates `.env` with `ENCRYPTION_SECRET`
4. Checks that Docker is available
5. Starts containers (unless `--start=false`)

**custom**:
1. Uses the existing `docker-compose.yml` in the target directory
2. Generates or updates `.env` with `ENCRYPTION_SECRET` if needed
3. Checks that Docker is available
4. Starts containers (unless `--start=false`)

   **Note**: The compose file must be named `docker-compose.yml`. If you only have `docker-compose-dev.yml`, either rename it or start containers manually with `docker compose -f docker-compose-dev.yml up -d`.

### After Running

Containers start automatically by default. Service endpoints are available immediately (wait ~2–3 minutes for all services to initialize).

If you staged files without starting containers (`--start=false`), manually start them with:

```bash
cd <dir>
docker compose up -d
```

### Service Endpoints (local)

| Service | URL | Default Credentials |
|---------|-----|---------------------|
| Backend API | http://localhost:8080 or http://localhost:8085 | — |
| Config UI | http://localhost:4000 or http://localhost:4004 | — |
| Grafana | http://localhost:3002 or http://localhost:3004 | admin / admin |

**Port Fallback**: When deploying with `--source official` or `--source fork`, the CLI automatically recovers from port conflicts by retrying with alternate ports (`8085/3004/4004`). As part of this recovery, it updates the port mappings in `docker-compose.yml` on disk (creating a backup at `docker-compose.yml.bak` first) so that future `docker compose up` runs use the fallback ports. Custom deployments (`--source custom`) are not modified automatically and require manual port conflict resolution.

To revert fallback changes, restore from the backup file (`docker-compose.yml.bak`), edit `docker-compose.yml` to change the mapped ports back to your preferred values (e.g., `8080/3002/4000`), or re-run the deployment in a clean directory.

### Examples

```bash
# Deploy to current directory (latest version)
gh devlake deploy local

# Deploy a specific version to ./devlake
gh devlake deploy local --version v1.0.2 --dir ./devlake

# Stage files without starting containers
gh devlake deploy local --start=false
```

### Notes

- If `.env` already exists in the target directory, it is backed up to `.env.bak` before being replaced.
- To tear down: `gh devlake cleanup --local` or `docker compose down` from the target directory.

#### Deployment Resilience

The CLI includes bounded recovery for common Docker errors:

- **Port conflicts**: When deploying with `--source official` or `--source fork`, the CLI detects port conflicts (patterns: `port is already allocated`, `bind for`, `ports are not available`, `address already in use`, `failed programming external connectivity`) and automatically retries with alternate ports (`8085/3004/4004`). Recovery is bounded to a single retry.
- **Custom deployments**: Port conflicts in `--source custom` deployments require manual resolution — the CLI will identify the conflicting container and suggest remediation commands.

---

## deploy azure

Provisions DevLake on Azure using Container Instances, Azure Database for MySQL (Flexible Server), and Key Vault.

### Usage

```bash
gh devlake deploy azure [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--resource-group` | *(prompt if omitted)* | Azure Resource Group name |
| `--location` | *(prompt if omitted)* | Azure region (e.g., `eastus`) |
| `--base-name` | `devlake` | Base name prefix for all Azure resources |
| `--official` | `false` | Use official Apache DevLake images from Docker Hub (no ACR required) |
| `--skip-image-build` | `false` | Skip building Docker images (use with existing ACR images) |
| `--repo-url` | | Clone a remote DevLake repository to build custom images from |

### What It Does

1. Checks Azure CLI login (runs `az login` if needed — **bounded recovery**)
2. Creates the resource group (saves partial state immediately for safe cleanup)
3. Generates MySQL password and encryption secret via Key Vault
4. Optionally builds Docker images and pushes to Azure Container Registry (when `--official` is not set)
5. Checks for stopped MySQL servers and starts them (**bounded recovery**)
6. Checks for soft-deleted Key Vaults and purges them before deployment (**bounded recovery**)
7. Deploys infrastructure via Bicep templates (Container Instances + MySQL + Key Vault)
8. Waits for the backend to respond, then triggers DB migration
9. Saves `.devlake-azure.json` state file with endpoints, resource names, and subscription info

### Cost Estimate

| Mode | Estimated Monthly Cost |
|------|------------------------|
| `--official` (no ACR) | ~$30–50/month |
| Custom images (with ACR) | ~$50–75/month |

### Examples

```bash
# Official Apache images (recommended for getting started)
gh devlake deploy azure --resource-group devlake-rg --location eastus --official

# Custom images (builds from sibling incubator-devlake repo)
gh devlake deploy azure --resource-group devlake-rg --location eastus

# Custom images from a remote fork
gh devlake deploy azure --resource-group devlake-rg --location eastus \
    --repo-url https://github.com/my-fork/incubator-devlake

# Interactive — will prompt for missing flags
gh devlake deploy azure
```

### Notes

- If `--resource-group` or `--location` are omitted, the CLI prompts interactively (with a region picker).
- A partial state file is written immediately after the Resource Group is created. This ensures `gh devlake cleanup --azure` can clean up even if the Bicep deployment fails mid-flight.
- Service endpoints are printed at the end of a successful deployment and saved to `.devlake-azure.json`.
- The Bicep templates are embedded in the binary — no external template files needed.

#### Azure Deployment Resilience

The CLI includes bounded recovery for known Azure failure modes:

- **Missing authentication**: Automatically runs `az login` when not logged in (single attempt).
- **Stopped MySQL servers**: Detects stopped MySQL Flexible Servers and starts them before deployment (single attempt, non-fatal).
- **Soft-deleted Key Vaults**: Detects and purges soft-deleted Key Vaults that conflict with the deployment (single attempt).
- **State checkpointing**: Partial state file is written immediately after Resource Group creation to enable cleanup even when deployment fails mid-flight.

All recovery actions are bounded to a single retry and report clear detection → repair → outcome messages.

### Tear Down

```bash
gh devlake cleanup --azure
```

See [cleanup.md](cleanup.md).

---

## Related

- [init.md](init.md) — guided wizard that includes deployment as Phase 1
- [state-files.md](state-files.md) — what `.devlake-local.json` and `.devlake-azure.json` contain
- [cleanup.md](cleanup.md)
- [status.md](status.md)
