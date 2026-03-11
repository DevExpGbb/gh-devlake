# Day-2 Operations

After your initial setup is running, here's how to monitor, manage, and evolve your DevLake instance.

## Status Check

```bash
gh devlake status
```

Shows deployment info, service health (Backend / Grafana / Config UI), active connections, and project configuration. See [status.md](status.md) for full output reference.

## Restarting Services

If services are stopped, crashed, or exited (e.g. after a machine reboot):

```bash
gh devlake start
```

Runs `docker compose up -d` for local deployments, or starts stopped Azure Container Instances. See [start.md](start.md) for all flags.

```bash
# Start only a specific service
gh devlake start --service config-ui

# Start without waiting for health check
gh devlake start --no-wait
```

## Managing Connections

### List connections

```bash
gh devlake configure connection list
```

### Test a saved connection

```bash
gh devlake configure connection test --plugin github --id 1
```

### Rotate a token

```bash
gh devlake configure connection update --plugin github --id 1 --token ghp_new_token
```

### Delete a connection

```bash
gh devlake configure connection delete --plugin gh-copilot --id 2
```

All subcommands support interactive mode — omit flags to be prompted. See [configure-connection.md](configure-connection.md) for all flags.

## Adding More Repos

Re-run `configure scope` to add new repositories to an existing connection:

```bash
gh devlake configure scope --plugin github --org my-org --repos my-org/new-repo
```

Existing connections, scopes, and projects are preserved. See [configure-scope.md](configure-scope.md).

## Re-triggering a Sync

Projects sync automatically on the blueprint schedule (default: daily at midnight). To trigger an immediate sync, use the DevLake API:

```bash
curl -X POST http://localhost:8080/blueprints/<blueprint-id>/trigger
```

You can find the blueprint ID via `gh devlake status` or the Config UI.

## Tear Down

### Local

```bash
gh devlake cleanup --local
```

Stops Docker Compose containers and removes the local state file.

### Azure

```bash
gh devlake cleanup --azure              # with confirmation prompt
gh devlake cleanup --azure --force      # skip confirmation
```

Deletes the Azure resource group and all resources within it. See [cleanup.md](cleanup.md) for all flags.

## Related

- [status.md](status.md) — full output reference
- [start.md](start.md) — restart stopped services
- [configure-connection.md](configure-connection.md) — connection CRUD
- [configure-scope.md](configure-scope.md) — scope management
- [cleanup.md](cleanup.md) — tear down
- [state-files.md](state-files.md) — how state files work
