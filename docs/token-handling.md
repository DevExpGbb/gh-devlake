# Token Handling

How the CLI resolves, uses, and secures Personal Access Tokens (PATs) for each plugin connection.

Supported today: **GitHub**, **GitHub Copilot**. Coming soon: **Azure DevOps**, **GitLab**.

## Token Resolution Order

For each plugin connection, the CLI resolves the PAT using this priority chain — first match wins:

| Priority | Source | Example |
|----------|--------|---------|
| 1 | `--token` flag | `--token ghp_abc123` |
| 2 | `--env-file` file (default: `.devlake.env`) | File containing `GITHUB_TOKEN=ghp_abc123` |
| 3 | Shell environment variable | Plugin-specific key (see below) |
| 4 | Interactive masked prompt | CLI prompts at the terminal (TTY required) |

If none of these produce a token, the command fails.

## Using a `.devlake.env` File

Create a file with one or more tokens:

```
GITHUB_TOKEN=ghp_your_token_here
GITLAB_TOKEN=glpat_your_token_here
AZURE_DEVOPS_PAT=ado_pat_here
```

The CLI checks **plugin-specific** keys (first match wins):

| Plugin | `.devlake.env` keys (in order) | Environment variables (in order) |
|--------|-------------------------------|---------------------------------|
| GitHub | `GITHUB_PAT`, `GITHUB_TOKEN`, `GH_TOKEN` | `GITHUB_PAT`, `GITHUB_TOKEN`, `GH_TOKEN` |
| GitHub Copilot | `GITHUB_PAT`, `GITHUB_TOKEN`, `GH_TOKEN` | `GITHUB_PAT`, `GITHUB_TOKEN`, `GH_TOKEN` |
| GitLab (coming soon) | `GITLAB_TOKEN` | `GITLAB_TOKEN` |
| Azure DevOps (coming soon) | `AZURE_DEVOPS_PAT` | `AZURE_DEVOPS_PAT` |

By default, the CLI looks for `.devlake.env` in the current directory. Override the path with `--env-file`:

```bash
gh devlake configure connection --plugin github --org my-org --env-file ./tokens/my.env
```

## Environment Variables

If no `--token` flag or env file is found, the CLI checks your shell environment using the plugin-specific key names in the table above.

## Interactive Prompt

As a final fallback, the CLI prompts you to paste the token at the terminal. Input is masked (hidden). This requires a TTY — it won't work in piped/non-interactive shells.

## Required PAT Scopes

| Plugin | Required Scopes |
|--------|----------------|
| GitHub | `repo`, `read:org`, `read:user` |
| GitHub Copilot | `manage_billing:copilot`, `read:org` |
| GitHub Copilot (enterprise) | + `read:enterprise` |

GitLab and Azure DevOps scopes will be documented when those plugins ship.

The CLI displays required scopes as a reminder before prompting for the token.

## Security

- **Never in history or logs.** The token is never echoed to the terminal, written to command history, or included in log output.
- **Encrypted at rest.** After a successful connection, the token is stored encrypted in DevLake's MySQL database using the `ENCRYPTION_SECRET` generated during deployment.
- **`.gitignore` by default.** The `.devlake.env` file is listed in the default `.gitignore` created by `deploy local`.

## Cleanup Behavior

| Command | `.devlake.env` after success |
|---------|------------------------------|
| `configure connection` | **Deleted** automatically (use `--skip-cleanup` to keep it) |
| `configure full` | **Deleted** automatically (use `--skip-cleanup` to keep it) |
| `init` | **Deleted** automatically (use `--skip-cleanup` to keep it) |

Cleanup only happens when the token was loaded from an env file (i.e., the command actually used `--env-file`). If you provided the token via `--token`, shell env vars, or the interactive prompt, there's no file to delete.

## Related

- [configure-connection.md](configure-connection.md) — all connection flags and examples
- [configure-full.md](configure-full.md) — multi-phase interactive setup
- [concepts.md](concepts.md) — what a connection is
