# configure scope

Add repository or organization scopes to an existing DevLake connection.

This command manages scopes only — it does **not** create projects or trigger data syncs. After scoping, run [`configure project`](configure-project.md) to create a project and start collection.

See [concepts.md](concepts.md) for what a scope is and how DORA patterns work.

## Usage

```bash
gh devlake configure scope [flags]
```

Aliases: `scopes`

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--plugin` | *(interactive or required)* | Plugin to configure (`github`, `gh-copilot`) |
| `--org` | *(required)* | GitHub organization slug |
| `--enterprise` | | Enterprise slug (enables enterprise-level Copilot metrics) |
| `--repos` | | Comma-separated repos to add (`owner/repo,owner/repo2`) |
| `--repos-file` | | Path to a file with repos (one `owner/repo` per line) |
| `--connection-id` | *(auto-detected)* | Override the connection ID to scope |
| `--deployment-pattern` | `(?i)deploy` | Regex matching CI/CD workflow names for deployments |
| `--production-pattern` | `(?i)prod` | Regex matching environment names for production |
| `--incident-label` | `incident` | GitHub issue label that marks incidents |

> **Note:** `--plugin` is required when using any other flag. Without flags, the CLI enters interactive mode and prompts for everything.

## Repo Resolution

When `--repos` and `--repos-file` are both omitted, the CLI uses the GitHub CLI to list up to 30 repos in `--org` for interactive multi-select.

If the GitHub CLI is unavailable or the list fails, you are prompted to enter repos manually.

## DORA Patterns

These patterns are attached to every GitHub repo scope as a **scope config**. They control how DevLake classifies CI/CD runs and incidents.

| Pattern | Default | Controls |
|---------|---------|---------|
| `--deployment-pattern` | `(?i)deploy` | Which workflow runs count as deployments |
| `--production-pattern` | `(?i)prod` | Which environments count as production |
| `--incident-label` | `incident` | Which issue labels mark incidents |

Example for a team using `release` workflows and `live` environments:

```bash
gh devlake configure scope --plugin github --org my-org --repos my-org/api \
    --deployment-pattern "(?i)(deploy|release)" \
    --production-pattern "(?i)(prod|live)"
```

## Connection Resolution

The CLI auto-detects the connection ID from the state file. If multiple connections of the same plugin exist, you are prompted to choose. Override with `--connection-id`.

## Examples

```bash
# Add specific repos to GitHub connection
gh devlake configure scope --plugin github --org my-org \
    --repos my-org/api,my-org/frontend

# Load repos from a file
gh devlake configure scope --plugin github --org my-org \
    --repos-file repos.txt

# Interactive repo selection (omit --repos)
gh devlake configure scope --plugin github --org my-org

# Add Copilot org scope
gh devlake configure scope --plugin gh-copilot --org my-org

# Copilot with enterprise scope
gh devlake configure scope --plugin gh-copilot --org my-org --enterprise my-enterprise

# Interactive (omit all flags)
gh devlake configure scope
```

## What It Does (GitHub)

1. Resolves repos from `--repos`, `--repos-file`, or interactive selection
2. Fetches repo details via `gh api repos/<owner>/<repo>`
3. Creates or reuses a DORA scope config (deployment/production patterns, incident label)
4. Calls `PUT /plugins/github/connections/{id}/scopes` to add repos

## What It Does (Copilot)

1. Computes scope ID from org + enterprise: `enterprise/org`, `enterprise`, or `org`
2. Calls `PUT /plugins/gh-copilot/connections/{id}/scopes` to add the org/enterprise scope

## Next Step

After scoping, run:

```bash
gh devlake configure project --org my-org
```

## Related

- [concepts.md](concepts.md)
- [configure-connection.md](configure-connection.md)
- [configure-project.md](configure-project.md)
- [configure-full.md](configure-full.md) — connections + scopes + project in one step
