# Troubleshooting

Common issues and solutions for `gh-devlake`.

---

## Table of Contents

- [Installation Issues](#installation-issues)
- [Deployment Issues](#deployment-issues)
- [Connection Issues](#connection-issues)
- [Scope and Project Issues](#scope-and-project-issues)
- [Docker Issues](#docker-issues)
- [Azure Issues](#azure-issues)
- [API and Network Issues](#api-and-network-issues)
- [State File Issues](#state-file-issues)
- [Getting Help](#getting-help)

---

## Installation Issues

### `gh: Unknown command "devlake"`

**Cause**: The extension is not installed or not in the PATH.

**Solution**:

```bash
# Install from GitHub
gh extension install DevExpGBB/gh-devlake

# Or install locally if building from source
cd gh-devlake
gh extension install .
```

### `command not found: gh`

**Cause**: GitHub CLI is not installed.

**Solution**: Install GitHub CLI from https://cli.github.com/

---

## Deployment Issues

### `docker compose` fails with "Cannot connect to Docker daemon"

**Cause**: Docker is not running or not installed.

**Solution**:

```bash
# Check if Docker is running
docker ps

# Start Docker Desktop (Windows/macOS) or Docker daemon (Linux)
sudo systemctl start docker  # Linux
```

### DevLake containers fail to start after `docker compose up -d`

**Cause**: Port conflicts, insufficient resources, or image pull failures.

**Solution**:

```bash
# Check container status
docker compose ps

# Check logs for specific service
docker compose logs mysql
docker compose logs devlake

# Check if ports are already in use
lsof -i :8080  # Backend
lsof -i :3002  # Grafana
lsof -i :4000  # Config UI

# Restart with fresh state
docker compose down -v
docker compose up -d
```

### Azure deployment fails with "InvalidTemplateDeployment"

**Cause**: Invalid Bicep parameters, quota limits, or permissions issues.

**Solution**:

```bash
# Check Azure CLI login
az account show

# Check subscription and resource quotas
az vm list-usage --location eastus

# Retry with explicit parameters
gh devlake deploy azure \
  --resource-group devlake-rg \
  --location eastus \
  --subscription "My Subscription"
```

### DevLake not ready after 5 minutes

**Cause**: Slow container startup, database initialization, or resource constraints.

**Solution**:

```bash
# Check DevLake backend logs
docker compose logs -f devlake

# Check MySQL logs
docker compose logs -f mysql

# Manually ping the backend
curl http://localhost:8080/ping

# Wait longer — first startup can take 3-5 minutes for MySQL init
```

---

## Connection Issues

### "Connection test failed" when creating a GitHub connection

**Cause**: Invalid PAT, insufficient scopes, or network issues.

**Solution**:

```bash
# Verify PAT has required scopes
# For GitHub: repo, read:org, read:user
# For Copilot: manage_billing:copilot, read:org

# Test manually with curl
curl -H "Authorization: token ghp_YOUR_TOKEN" \
     https://api.github.com/user

# Check token expiration
gh auth status

# Create new token with correct scopes
gh auth login --scopes repo,read:org,read:user
```

### "Token not found" during interactive connection creation

**Cause**: PAT not provided via flag, env file, or environment variable.

**Solution**:

```bash
# Option 1: Pass via flag
gh devlake configure connection add \
  --plugin github \
  --org my-org \
  --token ghp_YOUR_TOKEN

# Option 2: Use .devlake.env file
echo "GITHUB_TOKEN=ghp_YOUR_TOKEN" > .devlake.env
gh devlake configure connection add \
  --plugin github \
  --org my-org \
  --env-file .devlake.env

# Option 3: Set environment variable
export GITHUB_TOKEN=ghp_YOUR_TOKEN
gh devlake configure connection add \
  --plugin github \
  --org my-org
```

### Rate limit errors when creating connections

**Cause**: GitHub API rate limits exceeded.

**Solution**:

```bash
# Check current rate limit
gh api /rate_limit

# Wait for rate limit reset or use a different token
# Authenticated requests have 5000/hour limit
```

### Connection list is empty after creating connections

**Cause**: Wrong DevLake instance URL or state file mismatch.

**Solution**:

```bash
# Check state file
cat .devlake-local.json
# or
cat .devlake-azure.json

# Verify DevLake URL
gh devlake status

# Explicitly set URL
gh devlake configure connection list \
  --url http://localhost:8080
```

---

## Scope and Project Issues

### "No repositories found" when adding GitHub scopes

**Cause**: Org name incorrect, PAT lacks `repo` scope, or no repos in org.

**Solution**:

```bash
# Verify org name
gh api /user/orgs | jq '.[].login'

# List repos manually
gh repo list my-org

# Check connection token scopes
gh devlake configure connection test \
  --plugin github \
  --id 1
```

### Project creation fails with "No scopes found"

**Cause**: No scopes added to any connection before creating project.

**Solution**:

```bash
# Add scopes first
gh devlake configure scope add \
  --plugin github \
  --org my-org

# Then create project
gh devlake configure project add
```

### Blueprint sync never completes

**Cause**: Large data volume, API rate limits, or DevLake errors.

**Solution**:

```bash
# Check pipeline status
gh devlake status

# Check DevLake logs
docker compose logs -f devlake

# Check Grafana for task status
# Open http://localhost:3002 and check Pipeline page

# Cancel and retry
# Go to http://localhost:4000, cancel the pipeline, and rerun
```

---

## Docker Issues

### "Image not found" when running `docker compose up`

**Cause**: Custom image build failed or Docker registry unreachable.

**Solution**:

```bash
# If using custom images (default for gh-copilot plugin)
cd devlake
docker compose build

# Check if images exist
docker images | grep devlake

# Pull official images (if using --official flag)
docker compose pull
```

### Containers keep restarting

**Cause**: Health check failures, missing environment variables, or database connection issues.

**Solution**:

```bash
# Check which container is restarting
docker compose ps

# Check logs for error messages
docker compose logs mysql
docker compose logs devlake

# Common fixes:
# 1. Wait for MySQL init (first startup takes 2-3 minutes)
# 2. Check .env file has correct values
# 3. Verify port 3306 is not blocked
```

### "Permission denied" errors in containers

**Cause**: Volume mount permission issues (Linux).

**Solution**:

```bash
# Fix ownership of volume directories
sudo chown -R 1000:1000 devlake/mysql-data
sudo chown -R 472:472 devlake/grafana-data

# Or run with --user flag
docker compose down
# Edit docker-compose.yml and add user: "1000:1000"
docker compose up -d
```

---

## Azure Issues

### "Insufficient permissions" during Azure deployment

**Cause**: Azure user lacks Contributor role on subscription or resource group.

**Solution**:

```bash
# Check current role assignments
az role assignment list --assignee $(az account show --query user.name -o tsv)

# Request Contributor access from subscription admin
# Or use a subscription where you have permissions
```

### Container Instances fail to start

**Cause**: Image pull failures, resource constraints, or network issues.

**Solution**:

```bash
# Check container instance status
az container show \
  --resource-group devlake-rg \
  --name devlake-backend \
  --query "{status:instanceView.state,events:instanceView.events}"

# Check logs
az container logs \
  --resource-group devlake-rg \
  --name devlake-backend

# Redeploy
gh devlake cleanup --azure
gh devlake deploy azure
```

### MySQL Flexible Server connection timeout

**Cause**: Firewall rules, network configuration, or server not ready.

**Solution**:

```bash
# Check server status
az mysql flexible-server show \
  --resource-group devlake-rg \
  --name devlake-mysql-<suffix>

# Check firewall rules
az mysql flexible-server firewall-rule list \
  --resource-group devlake-rg \
  --name devlake-mysql-<suffix>

# Add your IP if needed
az mysql flexible-server firewall-rule create \
  --resource-group devlake-rg \
  --name devlake-mysql-<suffix> \
  --rule-name allow-my-ip \
  --start-ip-address <your-ip> \
  --end-ip-address <your-ip>
```

---

## API and Network Issues

### "Could not discover DevLake instance"

**Cause**: DevLake not running, wrong URL, or network connectivity issues.

**Solution**:

```bash
# Check if DevLake is running locally
docker compose ps
curl http://localhost:8080/ping

# Check if running on Azure
az container list --resource-group devlake-rg

# Manually specify URL
gh devlake status --url http://localhost:8080

# Check state files
cat .devlake-local.json
cat .devlake-azure.json
```

### "API request failed" or "500 Internal Server Error"

**Cause**: DevLake backend errors, database issues, or invalid requests.

**Solution**:

```bash
# Check DevLake logs
docker compose logs -f devlake

# Check database connectivity
docker compose exec mysql mysql -u merico -pmerico -e "SHOW DATABASES;"

# Restart DevLake
docker compose restart devlake

# Check for known issues in DevLake repo
# https://github.com/apache/incubator-devlake/issues
```

### Timeout errors during connection creation

**Cause**: Slow network, GitHub API rate limits, or DevLake overload.

**Solution**:

```bash
# Retry the operation
gh devlake configure connection add --plugin github --org my-org

# Check GitHub API status
# https://www.githubstatus.com/

# Increase timeout (if supported in future versions)
# For now, wait and retry
```

---

## State File Issues

### State file not found

**Cause**: Wrong directory, state file deleted, or never created.

**Solution**:

```bash
# State files are created in the directory where you run deploy
ls -la .devlake-*.json

# If deploying locally, cd to the deploy directory
cd devlake
gh devlake status

# Or use explicit URL
gh devlake status --url http://localhost:8080
```

### State file has wrong URL

**Cause**: Manual edit, or deployment changed.

**Solution**:

```bash
# Check state file
cat .devlake-local.json

# Edit manually or delete and rediscover
rm .devlake-local.json
gh devlake status  # Will rediscover
```

### State file merge issues

**Cause**: Corrupted JSON or unexpected fields.

**Solution**:

```bash
# Validate JSON
cat .devlake-local.json | jq .

# If corrupted, delete and redeploy
rm .devlake-local.json
gh devlake deploy local --dir ./devlake
```

---

## Getting Help

If you're still stuck:

1. **Check documentation**: [README.md](README.md) and [docs/](docs/)
2. **Search existing issues**: https://github.com/DevExpGBB/gh-devlake/issues
3. **Create a new issue**: Include:
   - Command you ran
   - Full error message
   - Output of `gh devlake status`
   - Docker/Azure logs (if applicable)
   - OS and versions (`go version`, `gh --version`, `docker --version`)
4. **Join discussions**: https://github.com/DevExpGBB/gh-devlake/discussions

---

## Common Error Messages Reference

| Error Message | Likely Cause | Quick Fix |
|---------------|--------------|-----------|
| `Connection test failed` | Invalid PAT or scopes | Verify token scopes and permissions |
| `Could not discover DevLake` | DevLake not running | Start DevLake: `docker compose up -d` |
| `Port already in use` | Another service using 8080/3002/4000 | Stop conflicting service or change ports |
| `No scopes found` | Scopes not added before project | Run `configure scope add` first |
| `Resource group not found` | Wrong Azure subscription or RG name | Check `az group list` |
| `Invalid token` | Expired or malformed PAT | Generate new token with `gh auth login` |
| `Plugin not available` | Plugin set to `Available: false` | Wait for plugin support or use available plugins |
| `Database connection failed` | MySQL not ready or wrong credentials | Wait for MySQL init, check .env file |

---

## Debug Mode

For detailed troubleshooting, enable verbose output:

```bash
# Docker Compose logs
docker compose logs -f

# Azure Container logs
az container logs --resource-group devlake-rg --name devlake-backend --follow

# GitHub API debugging
GH_DEBUG=api gh devlake <command>

# Check DevLake API directly
curl -v http://localhost:8080/api/connections
```

---

See also:
- [State Files](docs/state-files.md) — Discovery chain and file format
- [Token Handling](docs/token-handling.md) — PAT resolution order
- [Deploy](docs/deploy.md) — Deployment options and flags
- [Day-2 Operations](docs/day-2.md) — Maintenance and updates
