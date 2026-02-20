# configure full

Run connections + scopes + project in one command (3 phases).

This is the recommended path for getting fully configured in one shot. It combines [`configure connection`](configure-connection.md), [`configure scope`](configure-scope.md), and [`configure project`](configure-project.md).

## Usage

```bash
gh devlake configure full [flags]
```

## Phases

```
Phase 1: Configure Connections
  → Creates connections for selected plugins

Phase 2: Configure Scopes
  → Adds repo/org scopes using the connections from Phase 1

Phase 3: Project Setup
  → Creates a project, configures the blueprint, triggers first sync
```

Connection IDs from Phase 1 are automatically wired into Phases 2 and 3 — no manual ID passing required.

## Flags

Accepts all flags from `configure connection`, `configure scope`, and `configure project`:

| Flag | Default | Description |
|------|---------|-------------|
| `--org` | *(prompt)* | GitHub organization slug |
| `--enterprise` | | GitHub enterprise slug |
| `--plugin` | *(interactive multi-select)* | Limit to one plugin (`github`, `gh-copilot`). Skips the picker. |
| `--token` | | GitHub PAT (skips token prompt/file lookup) |
| `--env-file` | `.devlake.env` | Path to env file containing PAT |
| `--skip-cleanup` | `false` | Don't delete `.devlake.env` after setup |
| `--repos` | | Comma-separated repos (`owner/repo`) |
| `--repos-file` | | Path to file with repos (one per line) |
| `--project-name` | *(org name)* | DevLake project name |
| `--deployment-pattern` | `(?i)deploy` | Regex for deployment workflows |
| `--production-pattern` | `(?i)prod` | Regex for production environments |
| `--incident-label` | `incident` | Issue label for incidents |
| `--time-after` | *(6 months ago)* | Only collect data after this date |
| `--cron` | `0 0 * * *` | Blueprint sync schedule |
| `--skip-sync` | `false` | Skip the first data sync |

## Plugin Selection

Without `--plugin`, the CLI shows a multi-select list of available plugins (GitHub, GitHub Copilot). You choose which ones to configure.

With `--plugin github` or `--plugin gh-copilot`, the entire run is scoped to that one plugin (no picker shown).

## Examples

```bash
# Configure GitHub + Copilot with interactive pickers for repos
gh devlake configure full --org my-org

# Specify repos — skips interactive selection
gh devlake configure full --org my-org --repos my-org/api,my-org/frontend

# GitHub only
gh devlake configure full --org my-org --plugin github --repos my-org/api

# Enterprise Copilot
gh devlake configure full --org my-org --enterprise my-enterprise

# Load repos from file, skip first sync
gh devlake configure full --org my-org --repos-file repos.txt --skip-sync

# Custom DORA patterns
gh devlake configure full --org my-org --repos my-org/api \
    --deployment-pattern "(?i)(deploy|release)" \
    --production-pattern "(?i)(prod|live)"
```

## Notes

- `configure full` is equivalent to running `configure connection`, `configure scope`, and `configure project` in sequence with the same flags.
- PAT scopes are displayed as a reminder at the start of Phase 1.
- If a connection creation fails for one plugin, the run continues for remaining plugins.
- Phase 3 uses the connections from Phase 1 — it does not try to discover pre-existing connections.

## Related

- [configure-connection.md](configure-connection.md)
- [configure-scope.md](configure-scope.md)
- [configure-project.md](configure-project.md)
- [init.md](init.md) — includes deployment as a first phase
