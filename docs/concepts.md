# DevLake Concepts

[← Back to README](../README.md)

Understanding these four concepts makes every other command in this CLI make sense.

## Connection

An authenticated link to a data source. Each DevLake plugin gets its own connection, authenticated with its own PAT.

- **GitHub connection** — grants access to repos, PRs, workflows, deployments, and issues.
- **Copilot connection** — grants access to GitHub Copilot usage metrics, seat assignments, and acceptance rates.

You can have multiple connections of the same type (e.g., two GitHub connections pointing to different orgs).

Create connections with [`configure connection`](configure-connection.md).

## Scope

Defines *what* to collect from a connection:

- **GitHub scope** — a specific repository (`owner/repo`). Also includes a **scope config**: regex patterns that tell DevLake what counts as a deployment, a production environment, and an incident.
- **Copilot scope** — an organization (or org + enterprise pair) to pull Copilot metrics for.

A connection can have many scopes. Add scopes with [`configure scope`](configure-scope.md).

## Scope Config (DORA Patterns)

Attached to each GitHub scope. Three regex patterns:

| Pattern | Default | Matches |
|---------|---------|---------|
| `deploymentPattern` | `(?i)deploy` | CI/CD workflow names that represent deployments |
| `productionPattern` | `(?i)prod` | Environment names that represent production |
| `incidentLabel` | `incident` | GitHub issue labels that mark incidents |

DevLake uses these to calculate:
- **Deployment Frequency** — how often workflows matching `deploymentPattern` run against environments matching `productionPattern`
- **Change Failure Rate** — what fraction of deployments are followed by incidents (issues with the incident label)
- **Failed Deployment Recovery Time** — time from deployment to incident closure

## Project

Groups one or more connections + scopes into a single analytics view. Enables DORA metrics for the included scopes. Think of one project per team or business unit.

Create a project with [`configure project`](configure-project.md).

## Blueprint

The sync schedule attached to a project. A cron expression (default: `0 0 * * *` — daily at midnight) that tells DevLake when to re-collect data from all scopes in the project.

Created automatically when you run `configure project`. You can customize the schedule with `--cron`.

## How They Fit Together

```
Connection (GitHub — my-org)
  └── Scope (my-org/repo1, DORA patterns)
  └── Scope (my-org/repo2, DORA patterns)

Connection (Copilot — my-org)
  └── Scope (my-org / my-enterprise)

Project (my-team)
  ├── includes: GitHub connection scopes
  ├── includes: Copilot connection scope
  └── Blueprint: sync daily at midnight
```

## Config UI

All of the above can also be configured through the DevLake Config UI at `:4000`. The CLI automates the REST API that the Config UI calls — they're equivalent paths to the same configuration.

## Related

- [configure-connection.md](configure-connection.md)
- [configure-scope.md](configure-scope.md)
- [configure-project.md](configure-project.md)
- [configure-full.md](configure-full.md)
