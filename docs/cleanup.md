# cleanup

Tear down DevLake resources — stops Docker containers (local) or deletes Azure resources.

## Usage

```bash
gh devlake cleanup [flags]
```

Auto-detects deployment type from state files in the current directory.

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--local` | `false` | Force local cleanup mode (Docker Compose down) |
| `--azure` | `false` | Force Azure cleanup mode (delete resource group) |
| `--force` | `false` | Skip confirmation prompt |
| `--keep-resource-group` | `false` | Delete Azure resources but keep the resource group |
| `--resource-group` | *(from state file)* | Override Azure resource group name |
| `--state-file` | *(auto-detected)* | Path to state file |

## Auto-Detection

Without `--local` or `--azure`, the command checks:
1. `--state-file` path (if provided)
2. `.devlake-azure.json` → Azure mode
3. `.devlake-local.json` → Local mode

If neither file is found, cleanup fails with an error — use `--azure` or `--local` to force the mode.

## Local Cleanup

Stops and removes Docker Compose containers. Equivalent to `docker compose down` from the deployment directory.

```bash
gh devlake cleanup --local
```

What it does:
1. Prompts for confirmation (skip with `--force`)
2. Runs `docker compose down` from the current directory
3. Removes `.devlake-local.json`

## Azure Cleanup

Deletes the Azure resource group and all resources within it. This includes Container Instances, MySQL, Key Vault, and (if applicable) Container Registry.

```bash
gh devlake cleanup --azure
gh devlake cleanup --azure --force          # no confirmation prompt
gh devlake cleanup --azure --keep-resource-group  # delete resources, keep the RG
```

What it does:
1. Reads resource group and resource names from `.devlake-azure.json` (or `--state-file`)
2. Prints a summary of resources to be deleted
3. Prompts for confirmation (skip with `--force`)
4. Checks Azure CLI login
5. Deletes the resource group (or individual resources if `--keep-resource-group`)
6. Removes `.devlake-azure.json`

> **Note:** Resource group deletion runs in the background in Azure. Use `az group show --name <rg>` to check completion status.

### Cleanup Without a State File

If `.devlake-azure.json` is missing, use `--resource-group` to specify the RG directly:

```bash
gh devlake cleanup --azure --resource-group devlake-rg --force
```

## Examples

```bash
# Auto-detect (uses state file)
gh devlake cleanup

# Local — stop Docker containers
gh devlake cleanup --local

# Azure — with confirmation prompt
gh devlake cleanup --azure

# Azure — no prompt
gh devlake cleanup --azure --force

# Azure — delete resources but keep resource group
gh devlake cleanup --azure --keep-resource-group

# Point at a non-default state file
gh devlake cleanup --state-file /path/to/.devlake-azure.json
```

## Related

- [deploy.md](deploy.md)
- [state-files.md](state-files.md) — what gets cleaned up
- [status.md](status.md)
