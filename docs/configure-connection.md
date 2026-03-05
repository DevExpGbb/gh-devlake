# configure connection

Create, list, test, update, and delete DevLake plugin connections.

A **connection** is an authenticated link to a data source. Each plugin gets its own connection with its own PAT. See [concepts.md](concepts.md) for background.

---

## configure connection (create)

Create a plugin connection in DevLake using a PAT.

### Usage

```bash
gh devlake configure connection [flags]
```

Aliases: `connections`

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--plugin` | *(interactive)* | Plugin to configure (`github`, `gh-copilot`, `jenkins`) |
| `--org` | *(required for Copilot)* | GitHub organization slug |
| `--enterprise` | | GitHub enterprise slug (for enterprise-level Copilot metrics) |
| `--name` | `Plugin - org` | Connection display name |
| `--endpoint` | `https://api.github.com/` (GitHub/Copilot) | API endpoint (override for GitHub Enterprise Server; Jenkins has no default and must supply a URL) |
| `--proxy` | | HTTP proxy URL |
| `--token` | | GitHub PAT (highest priority source). For BasicAuth plugins (Jenkins, Bitbucket, Jira), this is the password. |
| `--username` | | Username for BasicAuth plugins (Jenkins, Bitbucket, Jira). Not used by GitHub or Copilot. |
| `--env-file` | `.devlake.env` | Path to env file containing PAT |
| `--skip-cleanup` | `false` | Don't delete `.devlake.env` after setup |

### Required PAT Scopes

| Plugin | Required Scopes |
|--------|----------------|
| `github` | `repo`, `read:org`, `read:user` |
| `gh-copilot` | `manage_billing:copilot`, `read:org` |
| `gh-copilot` (enterprise metrics) | + `read:enterprise` |
| `jenkins` | Username + API token/password (BasicAuth) |

### Token Resolution Order

For each plugin, the CLI resolves the PAT in this order (see [token-handling.md](token-handling.md) for the full guide):

1. `--token` flag
2. `.devlake.env` file — checked for plugin-specific keys:
   - GitHub / Copilot: `GITHUB_PAT`, `GITHUB_TOKEN`, or `GH_TOKEN`
3. Plugin-specific environment variable (same key names, from shell environment)
4. Interactive masked prompt (terminal fallback)

### Username Resolution (BasicAuth plugins)

For plugins that use BasicAuth (e.g. Jenkins, Bitbucket, Jira), the `--username` flag is resolved in this order:

1. `--username` flag
2. `.devlake.env` file — checked for plugin-specific keys (e.g. `JENKINS_USERNAME`)
3. Plugin-specific environment variable (from shell environment)
4. Interactive prompt (terminal fallback)

When `--username` is provided for a plugin that uses AccessToken auth (like GitHub or Copilot), the CLI prints a warning that the flag is not applicable.

### What It Does

1. Auto-discovers DevLake instance (state file → localhost ports → `--url`)
2. Resolves the PAT using the resolution chain above
3. Displays required PAT scopes as a reminder (regardless of token source)
4. Prompts for connection name and proxy (Enter accepts defaults / skips)
5. For GitHub: offers Cloud vs. Enterprise Server endpoint choice
6. Tests the connection before saving
7. Calls `POST /plugins/{plugin}/connections`
8. Saves the connection ID to the state file
9. Deletes `.devlake.env` (unless `--skip-cleanup`)

### Examples

```bash
# GitHub connection (interactive token prompt if .devlake.env not present)
gh devlake configure connection --plugin github --org my-org

# Copilot connection
gh devlake configure connection --plugin gh-copilot --org my-org

# Jenkins connection (endpoint required)
gh devlake configure connection --plugin jenkins --endpoint https://jenkins.example.com --username admin

# Enterprise Copilot metrics
gh devlake configure connection --plugin gh-copilot --org my-org --enterprise my-enterprise

# GitHub Enterprise Server
gh devlake configure connection --plugin github --org my-org \
    --endpoint https://github.example.com/api/v3/

# With proxy
gh devlake configure connection --plugin github --org my-org --proxy http://proxy:8080

# BasicAuth plugin (e.g. Jenkins)
gh devlake configure connection --plugin jenkins --username admin --token mypassword

# Interactive (no --plugin — prompts for everything)
gh devlake configure connection
```

### Output

```
   🔑 Testing connection...
   ✅ Connection test passed
   ✅ Created GitHub connection (ID=1)
```

---

## configure connection list

List all plugin connections in DevLake.

### Usage

```bash
gh devlake configure connection list [--plugin <plugin>]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--plugin` | *(all plugins)* | Filter output to one plugin (`github`, `gh-copilot`, `jenkins`) |

### Output

```
Plugin       ID  Name                         Organization  Enterprise
──────────   ──  ──────────────────────────   ────────────  ──────────
github        1  GitHub - my-org              my-org
gh-copilot    2  GitHub Copilot - my-org      my-org        avocado-corp
```

---

## configure connection test

Test an existing DevLake connection by plugin and ID.

### Usage

```bash
gh devlake configure connection test [--plugin <plugin>] [--id <id>]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--plugin` | *(interactive)* | Plugin to test (`github`, `gh-copilot`, `jenkins`) |
| `--id` | `0` | Connection ID to test |

Both flags are required for non-interactive mode. If either is omitted, the CLI prompts interactively.

### Output

```
✅ Connection test passed
```

```
❌ Connection test failed: <error message>
   💡 Ensure your PAT has these scopes: repo, read:org, read:user
```

---

## configure connection update

Update an existing connection in-place. Use for token rotation, endpoint changes, or org/enterprise updates. Preserves scope configs and blueprint associations.

### Usage

```bash
gh devlake configure connection update [--plugin <plugin>] [--id <id>] [update flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--plugin` | *(interactive)* | Plugin slug (`github`, `gh-copilot`, `jenkins`) |
| `--id` | *(interactive)* | Connection ID to update |
| `--token` | | New PAT for token rotation |
| `--org` | | New organization slug |
| `--enterprise` | | New enterprise slug |
| `--name` | | New connection display name |
| `--endpoint` | | New API endpoint URL |
| `--proxy` | | New HTTP proxy URL |

**Flag mode:** `--plugin` and `--id` are both required. Only fields you specify are changed.

**Interactive mode:** Omit `--plugin` or `--id`. The CLI lists existing connections, lets you pick one, then shows current values as defaults (press Enter to keep any field unchanged).

### Examples

```bash
# Token rotation
gh devlake configure connection update --plugin github --id 1 --token ghp_newtoken

# Change org
gh devlake configure connection update --plugin gh-copilot --id 2 --org new-org

# Interactive
gh devlake configure connection update
```

---

## configure connection delete

Delete a plugin connection from DevLake.

### Usage

```bash
gh devlake configure connection delete [--plugin <plugin>] [--id <id>]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--plugin` | *(interactive)* | Plugin of the connection to delete |
| `--id` | *(interactive)* | ID of the connection to delete |
| `--force` | `false` | Skip confirmation prompt |

**Flag mode:** both `--plugin` and `--id` are required.

**Interactive mode:** Lists all connections across plugins, prompts to select one, then prompts for confirmation.

### Example

```bash
# Non-interactive
gh devlake configure connection delete --plugin github --id 3

# Skip confirmation (useful in CI/CD)
gh devlake configure connection delete --plugin github --id 3 --force

# Interactive
gh devlake configure connection delete
```

> **Warning:** Deleting a connection removes its associated scopes from any blueprints that reference them. Projects that depended on this connection will stop collecting data for those scopes.

---

## Related

- [concepts.md](concepts.md) — what a connection is
- [token-handling.md](token-handling.md) — PAT resolution, `.devlake.env`, security notes
- [configure-scope.md](configure-scope.md) — add scopes to a connection
- [configure-project.md](configure-project.md) — create a project that uses connections
- [configure-full.md](configure-full.md) — create connections + scopes + project in one step
