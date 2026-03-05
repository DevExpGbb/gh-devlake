# Changelog

All notable changes to `gh-devlake` will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added
- CONTRIBUTING.md with development guidelines
- CHANGELOG.md for tracking version history
- TROUBLESHOOTING.md for common issues and solutions
- ARCHITECTURE.md for technical documentation

---

## [0.3.0] - TBD

### Added
- Initial public release
- GitHub plugin support for DORA metrics
- GitHub Copilot plugin for usage metrics
- Local deployment via Docker Compose
- Azure deployment via Container Instances
- Interactive wizard (`init` command)
- Connection management (add, list, test, update, delete)
- Scope management (add, list, delete)
- Project management (add, list, delete)
- Status command for health checks
- Cleanup command for teardown
- PAT resolution chain (flag → envfile → env → prompt)
- State file management for deployment tracking
- JSON output mode for scripting
- Comprehensive documentation in `docs/`

### Technical Details
- Built with Go 1.22+ and Cobra CLI framework
- REST API client with generic helpers (`doGet[T]`, `doPost[T]`, etc.)
- Plugin registry system for extensibility
- Auto-discovery of DevLake instances
- Azure Bicep templates for infrastructure as code
- Terminal UI with emoji vocabulary and Unicode box-drawing

---

## Release Notes Template

When creating a new release, use this template:

```markdown
## [X.Y.Z] - YYYY-MM-DD

### Added
- New features

### Changed
- Changes to existing functionality

### Deprecated
- Features that will be removed in future releases

### Removed
- Features that were removed

### Fixed
- Bug fixes

### Security
- Security-related changes
```

---

[Unreleased]: https://github.com/DevExpGBB/gh-devlake/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/DevExpGBB/gh-devlake/releases/tag/v0.3.0
