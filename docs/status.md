# status

Show a summary of your DevLake deployment, service health, connections, and project configuration.

## Usage

```bash
gh devlake status [--url <url>]
```

## Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--url` | *(auto-discovered)* | DevLake API base URL |

## Output Structure

```
════════════════════════════════════════
  DevLake Status
════════════════════════════════════════

  Deployment  [.devlake-local.json]
  ──────────────────────────────────────
  Method:    local
  Deployed:  2026-02-18 12:00 UTC

  Services
  ──────────────────────────────────────
  Backend    ✅  http://localhost:8080
  Grafana    ✅  http://localhost:3002
  Config UI  ✅  http://localhost:4000

  Connections
  ──────────────────────────────────────
  GitHub              ID=1    "GitHub - my-org"  [org: my-org]
  GitHub Copilot      ID=2    "Copilot - my-org" [org: my-org]

  Project
  ──────────────────────────────────────
  Name:       my-org
  Blueprint:  1
  Repos:      my-org/api, my-org/frontend
  Configured: 2026-02-18 12:05 UTC

════════════════════════════════════════
```

### Sections

**Deployment** — loaded from the state file (`.devlake-azure.json` or `.devlake-local.json`). Shows deployment method and timestamp.

**Services** — live health checks against each endpoint. Each service shows ✅, ❌, or ⚠️ with HTTP status code.

| Icon | Meaning |
|------|---------|
| ✅ | HTTP 2xx or 3xx — healthy |
| ⚠️ (code) | Unexpected HTTP status |
| ❌ | Connection refused or timeout |

Grafana is checked at `/api/health`. Backend and Config UI are checked at their root URL.

**Connections** — loaded from the state file. Shows plugin name, connection ID, display name, and org.

**Project** — loaded from the state file. Shows project name, blueprint ID, configured repos, and configuration timestamp.

## States

### No state file, DevLake reachable

If no state file is found but DevLake responds at a well-known port:

```
  ✅ DevLake reachable at http://localhost:8080
  Run 'gh devlake configure full' to set up connections.
```

### No state file, DevLake unreachable

```
  No state file found. Run 'gh devlake deploy' to get started.
```

### No connections yet

```
  Connections
  ──────────────────────────────────────
  (none — run 'gh devlake configure connection')
```

### No project yet

```
  Project
  ──────────────────────────────────────
  (none — run 'gh devlake configure project')
```

## Examples

```bash
# Auto-discover from state file or localhost
gh devlake status

# Target a specific instance
gh devlake status --url http://my-devlake.example.com
```

## Related

- [deploy.md](deploy.md)
- [configure-connection.md](configure-connection.md)
- [configure-project.md](configure-project.md)
- [cleanup.md](cleanup.md)
