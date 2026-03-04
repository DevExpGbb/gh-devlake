# configure scope

Manage scopes (repos, orgs) on existing DevLake connections.

Scopes define *what* data DevLake collects from a connection — specific repos for GitHub, or an org/enterprise for Copilot. This command only manages scopes; it does **not** create projects or trigger data syncs. After scoping, run [`configure project add`](configure-project.md) to create a project and start collection.

See [concepts.md](concepts.md) for what a scope is and how DORA patterns work.

## Subcommands

| Subcommand | Description |
|------------|-------------|
| [`configure scope add`](#configure-scope-add) | Add repo/org scopes to a connection |
| [`configure scope list`](#configure-scope-list) | List scopes on a connection |
| [`configure scope delete`](#configure-scope-delete) | Remove a scope from a connection |

Aliases: `scopes`

---

## configure scope add

Add repository or organization scopes to an existing DevLake connection.

### Usage

```bash
gh devlake configure scope add [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--plugin` | *(interactive or required)* | Plugin to configure (`github`, `gh-copilot`) |
| `--connection-id` | *(auto-detected)* | Override the connection ID to scope |
| `--org` | *(required)* | GitHub organization slug |
| `--enterprise` | | Enterprise slug (enables enterprise-level Copilot metrics) |
| `--repos` | | Comma-separated repos to add (`owner/repo,owner/repo2`) |
| `--repos-file` | | Path to a file with repos (one `owner/repo` per line) |
| `--deployment-pattern` | `(?i)deploy` | Regex matching CI/CD workflow names for deployments |
| `--production-pattern` | `(?i)prod` | Regex matching environment names for production |
| `--incident-label` | `incident` | GitHub issue label that marks incidents |

> **Note:** `--plugin` is required when using any other flag. Without flags, the CLI enters interactive mode and prompts for everything.

### Repo Resolution

When `--repos` and `--repos-file` are both omitted, the CLI uses the GitHub CLI to list up to 100 repos in `--org` for interactive multi-select.

If the GitHub CLI is unavailable or the list fails, you are prompted to enter repos manually.

### DORA Patterns

These patterns are attached to every GitHub repo scope as a **scope config**. They control how DevLake classifies CI/CD runs and incidents.

| Pattern | Default | Controls |
|---------|---------|---------|
| `--deployment-pattern` | `(?i)deploy` | Which workflow runs count as deployments |
| `--production-pattern` | `(?i)prod` | Which environments count as production |
| `--incident-label` | `incident` | Which issue labels mark incidents |

Example for a team using `release` workflows and `live` environments:

```bash
gh devlake configure scope add --plugin github --org my-org --repos my-org/api \
    --deployment-pattern "(?i)(deploy|release)" \
    --production-pattern "(?i)(prod|live)"
```

### Examples

```bash
# Add specific repos to GitHub connection
gh devlake configure scope add --plugin github --org my-org \
    --repos my-org/api,my-org/frontend

# Load repos from a file
gh devlake configure scope add --plugin github --org my-org \
    --repos-file repos.txt

# Interactive repo selection (omit --repos)
gh devlake configure scope add --plugin github --org my-org

# Add Copilot org scope
gh devlake configure scope add --plugin gh-copilot --org my-org

# Copilot with enterprise scope
gh devlake configure scope add --plugin gh-copilot --org my-org --enterprise my-enterprise

# Interactive (omit all flags)
gh devlake configure scope add
```

### What It Does (GitHub)

1. Resolves repos from `--repos`, `--repos-file`, or interactive selection
2. Fetches repo details via `gh api repos/<owner>/<repo>`
3. Creates or reuses a DORA scope config (deployment/production patterns, incident label)
4. Calls `PUT /plugins/github/connections/{id}/scopes` to add repos

### What It Does (Copilot)

1. Computes scope ID from org + enterprise: `enterprise/org`, `enterprise`, or `org`
2. Calls `PUT /plugins/gh-copilot/connections/{id}/scopes` to add the org/enterprise scope

---

## configure scope list

List all scopes configured on a DevLake plugin connection.

### Usage

```bash
gh devlake configure scope list [--plugin <plugin>] [--connection-id <id>]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--plugin` | *(interactive)* | Plugin to query (`github`, `gh-copilot`) |
| `--connection-id` | *(interactive)* | Connection ID to list scopes for |

**Flag mode:** both `--plugin` and `--connection-id` are required.

**Interactive mode:** Omit both flags — the CLI lists all connections across plugins and lets you pick one.

**JSON mode:** Pass the global `--json` flag to output a JSON array instead of a table. `--plugin` and `--connection-id` are required in JSON mode (interactive prompts are not supported).

### Output

```
Scope ID    Name              Full Name
──────────  ────────────────  ──────────────────────────────
12345678    api               my-org/api
87654321    frontend          my-org/frontend
```

### Examples

```bash
# Non-interactive
gh devlake configure scope list --plugin github --connection-id 1

# Interactive
gh devlake configure scope list

# JSON output (for scripting)
gh devlake configure scope list --plugin github --connection-id 1 --json
# → [{"id":"12345678","name":"api","fullName":"my-org/api"},{"id":"87654321","name":"frontend","fullName":"my-org/frontend"}]
```

---

## configure scope delete

Remove a scope from an existing DevLake plugin connection.

### Usage

```bash
gh devlake configure scope delete [--plugin <plugin>] [--connection-id <id>] [--scope-id <scope-id>]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--plugin` | *(interactive)* | Plugin of the connection (`github`, `gh-copilot`) |
| `--connection-id` | *(interactive)* | Connection ID |
| `--scope-id` | *(interactive)* | Scope ID to delete |

**Flag mode:** all three flags are required.

**Interactive mode:** Omit flags — the CLI picks a connection, lists its scopes, lets you pick one, then prompts for confirmation.

### Examples

```bash
# Non-interactive
gh devlake configure scope delete --plugin github --connection-id 1 --scope-id 12345678

# Interactive
gh devlake configure scope delete
```

> **Warning:** Deleting a scope removes it from any blueprints that reference it. Projects that depended on this scope will stop collecting data for it.

---

## Next Step

After scoping, run:

```bash
gh devlake configure project add --org my-org
```

## Related

- [concepts.md](concepts.md)
- [configure-connection.md](configure-connection.md)
- [configure-project.md](configure-project.md)
- [configure-full.md](configure-full.md) — connections + scopes + project in one step
