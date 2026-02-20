# configure project

Create a DevLake project, configure its blueprint, and trigger the first data sync.

A **project** groups existing connection scopes into a single analytics view with DORA metrics enabled. A **blueprint** is the sync schedule attached to the project. See [concepts.md](concepts.md).

**Prerequisites:** Run [`configure scope`](configure-scope.md) first to add scopes to your connections.

## Usage

```bash
gh devlake configure project [flags]
```

Aliases: `projects`

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--org` | *(state file or prompt)* | Organization slug |
| `--enterprise` | *(state file or prompt)* | Enterprise slug |
| `--plugin` | *(all connections)* | Limit to one plugin (`github`, `gh-copilot`) |
| `--project-name` | *(org name)* | DevLake project name |
| `--cron` | `0 0 * * *` | Blueprint sync schedule (cron expression) |
| `--time-after` | *(6 months ago)* | Only collect data after this date (`YYYY-MM-DD`) |
| `--skip-sync` | `false` | Create the project + blueprint but don't trigger the first sync |
| `--wait` | `true` | Wait for the first pipeline to complete |
| `--timeout` | `5m` | Max time to wait for pipeline completion |

## What It Does

1. Discovers your DevLake instance
2. Lists all connections (from state file + API)
3. Prompts you to select which connections to include in the project
4. For each selected connection, lists its existing scopes
5. Creates the DevLake project with DORA metrics enabled
6. Patches the blueprint with the selected connection scopes, cron schedule, and `time-after`
7. Triggers the first data sync (unless `--skip-sync`)
8. Monitors pipeline progress until completion or `--timeout`
9. Updates the state file with project + blueprint info

## Pipeline Output

```
   [10s] Status: TASK_RUNNING | Tasks: 2/8
   [30s] Status: TASK_RUNNING | Tasks: 5/8
   [60s] Status: TASK_COMPLETED | Tasks: 8/8

   ✅ Data sync completed!
```

The first sync may take 5–30 minutes depending on data volume and how far back `--time-after` reaches.

## Cron Schedule Reference

| Schedule | Cron Expression |
|----------|-----------------|
| Daily at midnight (default) | `0 0 * * *` |
| Every 6 hours | `0 */6 * * *` |
| Hourly | `0 * * * *` |
| Weekly on Sunday | `0 0 * * 0` |

## Examples

```bash
# Create a project for my-org (interactive scope selection)
gh devlake configure project --org my-org

# Custom project name and sync from 1 year ago
gh devlake configure project --org my-org \
    --project-name my-team \
    --time-after 2025-01-01

# Create project without triggering sync yet
gh devlake configure project --org my-org --skip-sync

# Longer timeout for large repos
gh devlake configure project --org my-org --timeout 30m

# Limit to GitHub connection only
gh devlake configure project --org my-org --plugin github
```

## Notes

- If a project with `--project-name` already exists, the command reuses its blueprint ID rather than creating a duplicate.
- If `--time-after` is omitted, defaults to 6 months before today.
- `--wait false` returns immediately after triggering the sync. Check pipeline status at `GET /pipelines/{id}` or via [`status`](status.md).
- Org and enterprise are read from the state file if not passed as flags.

## Related

- [concepts.md](concepts.md)
- [configure-scope.md](configure-scope.md) — add scopes before creating a project
- [configure-full.md](configure-full.md) — connections + scopes + project in one step
- [status.md](status.md)
