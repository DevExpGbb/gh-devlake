# deploy

Deploy DevLake locally via Docker Compose or to Azure via Bicep.

## deploy local

Downloads the official Apache DevLake Docker Compose files, generates an `ENCRYPTION_SECRET`, and prepares the directory for `docker compose up`.

### Usage

```bash
gh devlake deploy local [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--dir` | `.` | Target directory for Docker Compose files |
| `--version` | `latest` | DevLake release version (e.g., `v1.0.2`) |

### What It Does

1. Fetches the latest release tag from GitHub (or uses `--version`)
2. Downloads `docker-compose.yml` and `env.example` from the Apache DevLake release
3. Renames `env.example` → `.env`
4. Generates and injects a cryptographic `ENCRYPTION_SECRET` into `.env`
5. Checks that Docker is available

### After Running

```bash
cd <dir>
docker compose up -d
```

Wait ~2–3 minutes for all services to start.

### Service Endpoints (local)

| Service | URL | Default Credentials |
|---------|-----|---------------------|
| Backend API | http://localhost:8080 | — |
| Config UI | http://localhost:4000 | — |
| Grafana | http://localhost:3002 | admin / admin |

### Examples

```bash
# Deploy to current directory (latest version)
gh devlake deploy local

# Deploy a specific version to ./devlake
gh devlake deploy local --version v1.0.2 --dir ./devlake

# Then start the services
cd devlake
docker compose up -d
```

### Notes

- If `.env` already exists in the target directory, it is backed up to `.env.bak` before being replaced.
- `docker compose up` is NOT run automatically — this lets you inspect or customize `.env` first.
- To tear down: `gh devlake cleanup --local` or `docker compose down` from the target directory.

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

1. Checks Azure CLI login (runs `az login` if needed)
2. Creates the resource group (saves partial state immediately for safe cleanup)
3. Generates MySQL password and encryption secret via Key Vault
4. Optionally builds Docker images and pushes to Azure Container Registry (when `--official` is not set)
5. Deploys infrastructure via Bicep templates (Container Instances + MySQL + Key Vault)
6. Waits for the backend to respond, then triggers DB migration
7. Saves `.devlake-azure.json` state file with endpoints, resource names, and subscription info

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

### Tear Down

```bash
gh devlake cleanup --azure
```

See [cleanup.md](cleanup.md).

---

## Related

- [init.md](init.md) — guided wizard that includes deployment as Phase 1
- [cleanup.md](cleanup.md)
- [status.md](status.md)
