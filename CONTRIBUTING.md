# Contributing to gh-devlake

Thank you for your interest in contributing to `gh-devlake`! This guide will help you get started with development, testing, and submitting your changes.

---

## Table of Contents

- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Code Conventions](#code-conventions)
- [Testing](#testing)
- [Documentation](#documentation)
- [Submitting Changes](#submitting-changes)
- [Release Process](#release-process)

---

## Getting Started

### Prerequisites

- [Go 1.22+](https://go.dev/)
- [GitHub CLI](https://cli.github.com/) (`gh`)
- [Docker](https://docs.docker.com/get-docker/) (for local testing)
- [Azure CLI](https://learn.microsoft.com/cli/azure/) (`az`) (for Azure testing)

### Clone and Build

```bash
git clone https://github.com/DevExpGBB/gh-devlake.git
cd gh-devlake
go build -o gh-devlake.exe .   # Windows
go build -o gh-devlake .       # Linux/macOS
gh extension install .
```

> **Windows**: Always build with `-o gh-devlake.exe`. PowerShell resolves `.exe` preferentially — a stale `.exe` will shadow a freshly built binary without the extension.

### Repository Structure

See [AGENTS.md](AGENTS.md) for the complete architecture overview. Key directories:

```
cmd/                 # Cobra commands — all user-facing terminal output
internal/
  azure/             # Azure CLI wrapper + Bicep templates
  devlake/           # REST API client, auto-discovery, state file management
  docker/            # Docker CLI wrapper
  gh/                # GitHub CLI wrapper
  prompt/            # Interactive terminal prompts
  token/             # PAT resolution chain
docs/                # User-facing documentation
.github/
  agents/            # AI agent definitions
  skills/            # AI coding skills with references
  instructions/      # Code style and review guidelines
```

---

## Development Workflow

### 1. Create a Branch

```bash
git checkout -b feature/my-feature
# or
git checkout -b fix/my-bugfix
```

### 2. Make Changes

Follow the [Code Conventions](#code-conventions) below. Before making changes:

1. Read existing code in the area you're modifying
2. Check [AGENTS.md](AGENTS.md) for architecture patterns
3. Review `.github/copilot-instructions.md` for style guidelines
4. Look at similar commands for consistency

### 3. Test Your Changes

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test ./... -cover

# Run tests verbosely
go test ./... -v

# Test a specific package
go test ./internal/devlake/...
```

### 4. Build and Manual Test

```bash
go build -o gh-devlake.exe .     # Windows
go build -o gh-devlake .         # Linux/macOS
gh extension install .            # Reinstall with your changes

# Test your command
gh devlake <your-command>
```

### 5. Validation Checklist

Before committing, ensure:

```bash
go build ./...    # Must compile
go test ./...     # All tests pass
go vet ./...      # No issues
```

---

## Code Conventions

### Go Naming

- **Command constructors**: `newXxxCmd()` returning `*cobra.Command`
- **Run functions**: `runXxx` (e.g., `runStatus`, `runDeployLocal`)
- **Package vars** for flags: `camelCase` (`deployLocalDir`, `azureRG`)
- **Packages**: short lowercase (`devlake`, `azure`, `docker`, `prompt`)

### Error Handling

- All command `RunE` functions return `error`
- Wrap errors with context: `fmt.Errorf("context: %w", err)`
- Non-fatal errors → print `⚠️` warning and continue
- State file write failures → warning, not fatal

### Imports

Standard Go convention — stdlib, then external, then internal, separated by blank lines:

```go
import (
    "fmt"
    "os"

    "github.com/spf13/cobra"

    "github.com/DevExpGBB/gh-devlake/internal/devlake"
)
```

### Flags

- Global flags on `rootCmd.PersistentFlags()`
- Command-specific flags as package-level vars with `cmd.Flags().StringVar()`
- Required flags validated in `RunE` (not via `MarkFlagRequired`)

### Terminal Output

See `.github/instructions/terminal-output.instructions.md` for detailed rules. Key points:

- **Blank line before every emoji-prefixed step**
- **Unicode `═` for headers** (40 characters wide)
- **3-space indent for sub-items**
- **Standard emoji vocabulary** (🔍 discovery, 🔑 auth, 📡 connection, etc.)
- **No ANSI color codes** — only emoji and Unicode box-drawing

Example:

```go
fmt.Println("\n🔍 Discovering DevLake instance...")
fmt.Printf("   URL: %s\n", url)
fmt.Println("   ✅ Found!")
```

### Plugin Registry

- **Never hardcode plugin names** (`"github"`, `"gh-copilot"`) in switch/case branches outside `connectionRegistry`
- All plugin definitions live in `cmd/connection_types.go` via `ConnectionDef` structs
- Adding a new plugin = adding a `ConnectionDef` entry
- Use `doGet[T]`, `doPost[T]`, `doPut[T]`, `doPatch[T]` generic helpers for API calls

---

## Testing

### Test Files

- Unit tests: `*_test.go` alongside source
- Follow table-driven test pattern where applicable
- Use `httptest.NewServer` to mock REST API responses
- See `internal/devlake/client_test.go` for examples

### Test Coverage

Current coverage: ~80% in `internal/devlake` package

To check coverage:

```bash
go test ./internal/devlake/... -cover
```

### Manual Testing

Before submitting a PR:

1. **Build and install**: `go build && gh extension install .`
2. **Test happy path**: Run your command with valid inputs
3. **Test error paths**: Try invalid flags, missing prereqs, network errors
4. **Check terminal output**: Verify emoji, spacing, and formatting
5. **Verify state files**: Check `.devlake-*.json` if applicable

---

## Documentation

### When to Update Docs

Update documentation when:

- Adding a new command or subcommand
- Changing command flags or behavior
- Adding a new plugin
- Changing terminal output format
- Modifying state file structure

### Documentation Files to Update

| Change | Files to Update |
|--------|----------------|
| New command | `docs/<command>.md` + `README.md` (Command Reference table) |
| New flag | Command's `docs/<command>.md` file (Flags table) |
| New plugin | `README.md` (Supported Plugins table) + `docs/token-handling.md` |
| Terminal output | Update examples in relevant `docs/<command>.md` |
| Architecture | `AGENTS.md` + `.github/copilot-instructions.md` |

### Documentation Style

Follow the pattern in existing docs:

1. **Title** - Command name as heading
2. **Brief description** - One-liner explaining the command
3. **Usage block** - Exact `gh devlake` syntax
4. **Flags table** - With columns: Flag | Default | Description
5. **What It Does** - Numbered steps explaining behavior
6. **Examples** - Multiple realistic use cases
7. **Output** - Example terminal output
8. **Notes** - Important caveats or warnings
9. **Related** - Links to related documentation files

---

## Submitting Changes

### Pull Request Checklist

Before submitting a PR:

- [ ] Code compiles: `go build ./...`
- [ ] Tests pass: `go test ./...`
- [ ] No vet issues: `go vet ./...`
- [ ] Documentation updated (if applicable)
- [ ] Terminal output follows spacing rules
- [ ] Commit messages are descriptive
- [ ] PR description explains the change

### Commit Message Format

Use conventional commit style:

```
<type>: <description>

<optional body>
```

Types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`

Examples:

```
feat: add gh-copilot plugin support

docs: update token handling guide with enterprise scopes

fix: handle rate limit errors in connection test
```

### PR Description

Include:

- **What**: Summary of changes
- **Why**: Problem being solved or feature being added
- **How**: Approach taken (if non-obvious)
- **Testing**: How you verified the changes
- **Related**: Link to any related issues

---

## Release Process

Releases are managed via the GitHub Foreman agent. See `.github/skills/devlake-dev-planning/SKILL.md` for the full process.

### Version Scheme

Semantic versioning: `MAJOR.MINOR.PATCH`

- **MAJOR**: Breaking changes
- **MINOR**: New features (backward compatible)
- **PATCH**: Bug fixes

### Creating a Release

Releases are created by maintainers using the Foreman workflow:

1. Tag the commit: `git tag v0.3.0`
2. Push tag: `git push origin v0.3.0`
3. Foreman builds binaries for all platforms
4. GitHub release is created with assets
5. Extension catalog is updated

---

## Getting Help

- **Documentation**: Check [README.md](README.md) and [docs/](docs/)
- **Architecture**: See [AGENTS.md](AGENTS.md)
- **Issues**: Browse existing issues or create a new one
- **Discussions**: Ask questions in GitHub Discussions

---

## Code of Conduct

This project follows the [Contributor Covenant](https://www.contributor-covenant.org/) code of conduct. Be respectful and inclusive in all interactions.

---

## License

By contributing, you agree that your contributions will be licensed under the same [MIT License](LICENSE) that covers this project.
