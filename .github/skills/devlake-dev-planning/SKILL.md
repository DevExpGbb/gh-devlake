---
name: devlake-dev-planning
description: gh-devlake roadmap, milestones, versioning, and release priorities. Use when asking about what's planned, what version something targets, or what issues exist for a milestone.
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

- **PATCH** bumps for bug fixes, internal refactors, docs, and housekeeping.
- **MINOR** bumps for any new user-facing feature: new commands, new flags, new plugins — any additive capability.
- **MAJOR (1.0)** — Reserved for production-ready stability declaration.

### Milestone Plan

- **0.3.x** — Current development line. CRUD subcommands, restructuring, and lifecycle commands.
- **0.4.0** — Multi-tool expansion + plugin UX. GitLab (#13), Azure DevOps (#14), dynamic flag validation (#59).
- **0.5.0** — AI-powered operations. Query engine (#62), Copilot SDK insights (#63), AI diagnose (#64), installable agent skill (#61).

> **Note:** Always query GitHub milestones for the latest status — this section is a snapshot.

## Current Release Plan

> **Note:** Always query GitHub milestones, current and upcoming releases, and issues for the latest status — this table is a snapshot.

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

## Release Procedure

When creating a new release:

1. **Create the release** with `gh release create <tag> --repo DevExpGBB/gh-devlake --title "<title>" --notes "<notes>"`
2. **Wait for the release workflow** — the `.github/workflows/release.yml` triggers on `v*` tag push and uses `cli/gh-extension-precompile@v2` to build cross-platform binaries. This takes ~60-90 seconds.
3. **Verify assets were uploaded** — poll until all 12 platform binaries appear:
   ```
   gh release view <tag> --repo DevExpGBB/gh-devlake --json assets --jq '[.assets[].name] | length'
   ```
   Expected: 12 assets (darwin-amd64, darwin-arm64, freebsd-386, freebsd-amd64, freebsd-arm64, linux-386, linux-amd64, linux-arm, linux-arm64, windows-386.exe, windows-amd64.exe, windows-arm64.exe)
4. **If assets are missing**, check the workflow run:
   ```
   gh run list --repo DevExpGBB/gh-devlake --workflow release.yml --limit 1
   ```
   If failed, re-trigger by deleting and re-creating the tag, or manually re-run the workflow.

> **Never mark a release as complete until all 12 assets are verified.**

## Key Design Decisions

1. **One plugin per invocation** in flag mode. Interactive mode walks through plugins sequentially.
2. **`--plugin` flag** replaces `--skip-copilot`/`--skip-github` (positive selection, not negative exclusion).
3. **Singular command names** (`connection`, `scope`, `project`) — not plurals.
4. **Delete/update/test are subcommands**, not flags — each is a distinct action with distinct UX.
5. **Plugin-specific fields** (org, enterprise, repos) are validated per-plugin, not shared across all.
