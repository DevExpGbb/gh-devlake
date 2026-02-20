# State Files

The CLI uses local JSON state files to pass context between commands — so you don't need to re-enter connection IDs, endpoints, or resource names at every step.

## File Reference

| File | Created By | Contents |
|------|-----------|----------|
| `.devlake-local.json` | `configure connection` | DevLake API URL, connection IDs, project name |
| `.devlake-azure.json` | `deploy azure` | Azure resource group, endpoints, subscription info, connection IDs |
| `.devlake.env` | User (manual) | PATs for plugin connection creation (can include multiple tools) — see [Token Handling](token-handling.md) |

Both `.devlake-local.json` and `.devlake-azure.json` are listed in the default `.gitignore`.

## How Command Chaining Works

State files are what make this sequence work without repeating IDs:

```bash
gh devlake configure connection --plugin github --org my-org
# → saves connection ID=1 to .devlake-local.json

gh devlake configure scope --plugin github --org my-org --repos my-org/api
# → reads connection ID=1 from state file, attaches scope to it

gh devlake configure project
# → reads all connection IDs from state file, creates project with them
```

Without state files, you'd need to pass `--connection-id 1`, `--url http://localhost:8080`, etc. to every command.

## Discovery Chain

The CLI finds the DevLake API endpoint using this priority:

| Priority | Source |
|----------|--------|
| 1 | `--url` flag (explicit) |
| 2 | State file in the current directory (`.devlake-azure.json` → `.devlake-local.json`) |
| 3 | Well-known local ports (`http://localhost:8080`) |

## Location

State files are written to the **current working directory** when the command runs. Run your commands from the same directory (typically the one where you ran `deploy local` or `deploy azure`), or use `--url` to bypass state-based discovery.

## Cleanup

- `gh devlake cleanup --local` deletes `.devlake-local.json`
- `gh devlake cleanup --azure` deletes `.devlake-azure.json`
- `.devlake.env` cleanup depends on the command — see [Token Handling](token-handling.md#cleanup-behavior)

## Related

- [deploy.md](deploy.md) — creates the initial state file
- [status.md](status.md) — reads state files for deployment info
- [cleanup.md](cleanup.md) — deletes state files
- [token-handling.md](token-handling.md) — `.devlake.env` lifecycle
