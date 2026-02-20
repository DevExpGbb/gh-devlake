# configure full

Run connections + scopes + project in one interactive session.

This is the recommended path for getting fully configured without the deploy phase. It combines [`configure connection`](configure-connection.md), [`configure scope`](configure-scope.md), and [`configure project`](configure-project.md) — interactively, in sequence.

For scripted/CI automation, chain the individual commands instead:

```bash
gh devlake configure connection --plugin github --org my-org --token $PAT
gh devlake configure scope --plugin github --org my-org --repos owner/repo1
gh devlake configure project --project-name my-project
```

## Usage

```bash
gh devlake configure full [flags]
```

## Phases

```
Phase 1: Configure Connections
  → Multi-select picker for plugins; creates connections

Phase 2: Configure Scopes
  → Interactive scope setup per connection (repos for GitHub, org for Copilot)

Phase 3: Project Setup
  → Prompts for project name; creates project, blueprint, and triggers first sync
```

Connection IDs from Phase 1 are automatically wired into Phases 2 and 3 — no manual ID passing required.

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--token` | | Personal access token (seeds token resolution; may still prompt per plugin) |
| `--env-file` | `.devlake.env` | Path to env file containing PAT |
| `--skip-cleanup` | `false` | Don't delete `.devlake.env` after setup |

All other configuration (org, repos, DORA patterns, project name) is gathered interactively.

## Plugin Selection

A multi-select list of all available plugins is always shown at the start. Select one or more to configure.

## Examples

```bash
# Full interactive configuration
gh devlake configure full

# Seed token from env file (avoids the token prompt)
gh devlake configure full --env-file .devlake.env

# Pass token directly
gh devlake configure full --token $GITHUB_TOKEN
```

## Notes

- `configure full` is equivalent to running `configure connection`, `configure scope`, and `configure project` in sequence.
- If a connection creation fails for one plugin, the run continues for remaining plugins.
- Phase 3 uses the connections from Phase 1 — it does not try to discover pre-existing connections.
- For fine-grained control or CI automation, use the individual commands directly.
- If the token was loaded from an env file, `.devlake.env` is deleted at the end by default (use `--skip-cleanup` to keep it).

## Related

- [configure-connection.md](configure-connection.md)
- [configure-scope.md](configure-scope.md)
- [configure-project.md](configure-project.md)
- [init.md](init.md) — includes deployment as a first phase
