# gh devlake query

Query DevLake's aggregated data and metrics.

## Usage

```bash
gh devlake query <subcommand> [flags]
```

## Subcommands

### pipelines

Query recent pipeline runs.

```bash
gh devlake query pipelines [flags]
```

**Flags:**
- `--project <name>` - Filter by project name
- `--status <status>` - Filter by status (`TASK_CREATED`, `TASK_RUNNING`, `TASK_COMPLETED`, `TASK_FAILED`)
- `--limit <n>` - Maximum number of pipelines to return (default: 20)
- `--format <format>` - Output format: `json` or `table` (default: `json`)

**Examples:**

```bash
# List recent pipelines as JSON
gh devlake query pipelines

# List pipelines for a specific project
gh devlake query pipelines --project my-team

# List only completed pipelines
gh devlake query pipelines --status TASK_COMPLETED --limit 10

# Display as table
gh devlake query pipelines --format table
```

**Output (JSON):**

```json
[
  {
    "id": 123,
    "status": "TASK_COMPLETED",
    "blueprintId": 1,
    "createdAt": "2026-03-12T10:00:00Z",
    "beganAt": "2026-03-12T10:00:05Z",
    "finishedAt": "2026-03-12T10:15:30Z",
    "finishedTasks": 12,
    "totalTasks": 12
  }
]
```

**Output (Table):**

```
════════════════════════════════════════
  DevLake — Pipeline Query
════════════════════════════════════════

  Found 3 pipeline(s)
  ────────────────────────────────────────────────────────────────────────────────
  ID      STATUS           TASKS       FINISHED AT
  ────────────────────────────────────────────────────────────────────────────────
  123     TASK_COMPLETED   12/12       2026-03-12T10:15:30Z
  122     TASK_COMPLETED   12/12       2026-03-12T09:15:30Z
  121     TASK_RUNNING     8/12        (running)
```

---

### dora

Query DORA (DevOps Research and Assessment) metrics.

```bash
gh devlake query dora --project <name> [flags]
```

**Status:** ⚠️ Partial implementation (limited by available API data)

**What's available:**
- Project metadata (name, description, blueprint info)
- Clear explanation of limitations in the response

**What's not available:**
Full DORA metric calculations (deployment frequency, lead time for changes, change failure rate, mean time to restore) require SQL queries against DevLake's domain layer tables. DevLake does not expose database credentials or a metrics API endpoint.

**Current output (JSON):**

```json
{
  "project": "my-team",
  "timeframe": "30d",
  "availableData": {
    "project": { "name": "my-team", "blueprint": {...} }
  },
  "limitations": "Full DORA metrics require SQL against domain tables..."
}
```

**Workaround for full metrics:** View DORA metrics in your Grafana dashboards:
```bash
gh devlake status  # Shows Grafana URL
```

Then navigate to the DORA dashboards in Grafana.

**Full implementation requires:**
1. Upstream DevLake metrics API endpoint
2. OR direct database query support (requires DB credentials)
3. OR Grafana API integration to fetch dashboard data

---

### copilot

Query GitHub Copilot usage metrics.

```bash
gh devlake query copilot --project <name> [flags]
```

**Status:** ⚠️ Partial implementation (limited by available API data)

**What's available:**
- Project metadata (name, description, blueprint info)
- GitHub Copilot connection information
- Clear explanation of limitations in the response

**What's not available:**
Copilot usage metrics (total seats, active users, acceptance rates, language breakdowns, editor usage) are stored in `_tool_gh_copilot_*` database tables and visualized in Grafana dashboards, but DevLake does not expose a metrics API endpoint.

**Current output (JSON):**

```json
{
  "project": "my-team",
  "timeframe": "30d",
  "availableData": {
    "project": { "name": "my-team", "blueprint": {...} },
    "connections": [...]
  },
  "limitations": "Copilot metrics in _tool_gh_copilot_* tables require metrics API..."
}
```

**Workaround for full metrics:** View Copilot metrics in your Grafana dashboards:
```bash
gh devlake status  # Shows Grafana URL
```

Then navigate to the Copilot dashboards in Grafana.

**Full implementation requires:**
1. Upstream DevLake metrics API endpoint for Copilot plugin
2. OR direct database query support (requires DB credentials)
3. OR Grafana API integration to fetch dashboard data

---

## Global Flags

These flags are inherited from the root command:

- `--url <url>` - DevLake API base URL (auto-discovered if omitted)
- `--json` - Output as JSON (suppresses banners and interactive prompts)

## Architecture Notes

The `query` command uses the `internal/query/` package for extensible API-backed queries:

- **Pipelines:** Fully functional - queries the `/pipelines` REST API endpoint with filtering and formatting
- **DORA:** Partial - returns project metadata from REST API; full metric calculations require SQL against domain tables
- **Copilot:** Partial - returns project and connection metadata from REST API; usage metrics are in database tables not exposed via API

All queries use the query engine abstraction (`internal/query/engine.go`) with registered query definitions. When DevLake exposes metrics APIs in the future, only the query execution functions need to change - the command structure and engine remain the same.

## See Also

- `gh devlake status` - Check DevLake deployment and connection status
- `gh devlake configure project list` - List all projects
