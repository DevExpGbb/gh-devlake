---
name: gh-devlake-roadmap
description: Look up the gh-devlake CLI roadmap, milestones, issues, version plans, and release priorities. Use when the user asks about roadmap, priorities, what's planned, what version something is in, or what issues exist for a milestone.
---

# gh-devlake Roadmap Lookup

## Repository

- **Owner:** DevExpGBB
- **Repo:** gh-devlake
- **Project Board:** https://github.com/orgs/DevExpGbb/projects/21 (Project #21)
- **Milestones URL:** https://github.com/DevExpGBB/gh-devlake/milestones

## How to Look Up Roadmap Information

Use the GitHub MCP tools to query issues and milestones from `DevExpGBB/gh-devlake`.

### List milestones and their issues

Use `mcp_github_list_issues` with the repo `DevExpGBB/gh-devlake` to get issues.
Filter by milestone name to see what's in each release.

To find all milestones, use the `gh` CLI:
```
gh api repos/DevExpGBB/gh-devlake/milestones --jq '.[] | "\(.title): \(.description) (\(.open_issues) open, \(.closed_issues) closed)"'
```

To find issues for a specific milestone:
```
gh issue list --repo DevExpGBB/gh-devlake --milestone "v0.3.4" --json number,title,state,labels
```

### Issue labels

| Label | Meaning |
|-------|---------|
| `enhancement` | New feature or request |
| `bug` | Something isn't working |
| `refactor` | Code restructure, no behavior change |
| `documentation` | Docs, skills, instructions |

## Versioning Scheme

Semantic versioning: `MAJOR.MINOR.PATCH`

- **0.3.x** — Current development line. Incremental features, restructuring, and lifecycle commands.
  - PATCH bumps for features that don't change the CLI's plugin surface area (same set of supported DevOps tools).
- **0.4.x** — Multi-tool expansion. New plugin types (GitLab, Azure DevOps) that expand the CLI's supported tool surface.
- **MINOR bumps** only when genuinely new categories of capability arrive (new DevOps tool plugins, new token resolution chains).
- **MAJOR bump (1.0)** — Reserved for production-ready stability declaration.

## Current Release Plan

| Version | Theme | Status |
|---------|-------|--------|
| v0.3.3 | Enterprise Support | In progress — scope ID fix, connection testing, rate limit, enterprise threading |
| v0.3.4 | CLI Restructure | Planned — singular commands, --plugin flag, list command, CLI versioning |
| v0.3.5 | Connection Lifecycle | Planned — delete and test commands |
| v0.3.6 | Connection Update + Skill | Planned — update command, this roadmap skill |
| v0.4.0 | Multi-Tool Expansion | Future — GitLab, Azure DevOps, per-plugin token chains |

## CLI Command Architecture (Option A)

Connection lifecycle commands live under `configure connection`:
```
gh devlake configure connection create  --plugin gh-copilot ...
gh devlake configure connection delete  --plugin gh-copilot --id 2
gh devlake configure connection update  --plugin gh-copilot --id 2 --token ghp_new
gh devlake configure connection list
gh devlake configure connection test    --plugin gh-copilot --id 2
```

Each command operates on one plugin at a time. Interactive mode prompts for plugin selection.

## Key Design Decisions

1. **One plugin per invocation** in flag mode. Interactive mode walks through plugins sequentially.
2. **`--plugin` flag** replaces `--skip-copilot`/`--skip-github` (positive selection, not negative exclusion).
3. **Singular command names** (`connection`, `scope`, `project`) — not plurals.
4. **Delete/update/test are subcommands**, not flags — each is a distinct action with distinct UX.
5. **Plugin-specific fields** (org, enterprise, repos) are validated per-plugin, not shared across all.
