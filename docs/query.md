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

**Status:** 🚧 Not yet implemented

**Reason:** DevLake does not currently expose a metrics API endpoint. DORA metrics are calculated in Grafana dashboards using SQL queries against the domain layer tables, but these calculations are not available via the REST API.

**Workaround:** View DORA metrics in your Grafana dashboards:
```bash
gh devlake status  # Shows Grafana URL
```

Then navigate to the DORA dashboards in Grafana.

**Future implementation requires:**
1. Upstream DevLake metrics API endpoint
2. OR direct database query support (requires DB credentials in state files)
3. OR Grafana API integration to fetch dashboard data

---

### copilot

Query GitHub Copilot usage metrics.

```bash
gh devlake query copilot --project <name> [flags]
```

**Status:** 🚧 Not yet implemented

**Reason:** DevLake does not currently expose a metrics API endpoint. Copilot metrics are stored in the `gh-copilot` plugin's database tables and visualized in Grafana dashboards, but not accessible via the REST API.

**Workaround:** View Copilot metrics in your Grafana dashboards:
```bash
gh devlake status  # Shows Grafana URL
```

Then navigate to the Copilot dashboards in Grafana.

**Future implementation requires:**
1. Upstream DevLake metrics API endpoint for Copilot plugin
2. OR direct database query support (requires DB credentials in state files)
3. OR Grafana API integration to fetch dashboard data

---

## Global Flags

These flags are inherited from the root command:

- `--url <url>` - DevLake API base URL (auto-discovered if omitted)
- `--json` - Output as JSON (suppresses banners and interactive prompts)

## Architecture Notes

The `query` command is designed to be extensible:

- **Current:** The `pipelines` subcommand uses the existing `/pipelines` REST API endpoint
- **Future:** The `dora` and `copilot` subcommands are placeholders awaiting upstream API support

When DevLake exposes metrics APIs, the existing command structure will remain the same — only the implementation will change from returning an error to fetching actual metrics.

## See Also

- `gh devlake status` - Check DevLake deployment and connection status
- `gh devlake configure project list` - List all projects
