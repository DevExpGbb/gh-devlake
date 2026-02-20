# init

Guided 4-phase setup wizard. Takes you from zero to a fully configured DevLake instance interactively — no flags required.

## Usage

```bash
gh devlake init [flags]
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--token` | | GitHub PAT (skips interactive token prompt) |
| `--env-file` | `.devlake.env` | Path to env file containing PAT |

## Phases

```
╔══════════════════════════════════════╗
║  PHASE 1: Deploy DevLake             ║
╚══════════════════════════════════════╝

╔══════════════════════════════════════╗
║  PHASE 2: Configure Connections      ║
╚══════════════════════════════════════╝

╔══════════════════════════════════════╗
║  PHASE 3: Configure Scopes           ║
╚══════════════════════════════════════╝

╔══════════════════════════════════════╗
║  PHASE 4: Project Setup              ║
╚══════════════════════════════════════╝
```

### Phase 1: Deploy

Prompts you to choose a deployment target:
- **local** — runs `deploy local` in the current directory, then starts Docker containers and waits for DevLake to be ready
- **azure** — runs `deploy azure` interactively (prompts for official vs. custom images, resource group, location)

### Phase 2: Configure Connections

Presents a multi-select list of available plugins. For each selected plugin:
- Resolves the PAT (from `--token`, `--env-file`, environment, or interactive prompt)
- Prompts for organization (and enterprise if needed)
- Creates the connection and tests it

### Phase 3: Configure Scopes

For each connection created in Phase 2:
- **GitHub**: shows current DORA pattern defaults and asks if you want to customize them. Then resolves repos interactively.
- **Copilot**: adds the org/enterprise scope automatically.

### Phase 4: Project Setup

- Prompts for a project name (defaults to org name)
- Lists the scopes from Phase 3 and includes them in the project
- Creates the project with DORA metrics enabled
- Configures a daily sync blueprint
- Triggers the first data collection and waits for completion

## Example

```bash
gh devlake init
```

Fully interactive — no other flags needed for a standard setup.

## When to Use `init` vs `configure full`

| | `init` | `configure full` |
|--|--------|-----------------|
| Deploy included | ✅ Phase 1 | ❌ DevLake must already be running |
| Flag-driven | Mostly interactive | Fully scriptable |
| Customization | Prompted at each step | Set via flags |
| Best for | First time, exploratory | Repeatable / CI use |

## Notes

- `init` calls the same underlying functions as `deploy local/azure`, `configure connection`, `configure scope`, and `configure project` — it's orchestration, not a separate implementation.
- DORA patterns can be customized interactively in Phase 3. If you accept defaults and want to change them later, re-run `configure scope` with the pattern flags.
- After `init` completes, run `gh devlake status` to confirm all services are healthy.

## Related

- [deploy.md](deploy.md)
- [configure-connection.md](configure-connection.md)
- [configure-scope.md](configure-scope.md)
- [configure-project.md](configure-project.md)
- [configure-full.md](configure-full.md) — same phases but flag-driven, no deploy
