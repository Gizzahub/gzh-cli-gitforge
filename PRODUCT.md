# Product Goals (No-PRD)

**Project**: gzh-cli-gitforge
**Doc Type**: Goals + Constraints + Quality Gates
**Status**: Active
**Last Updated**: 2025-12-31

______________________________________________________________________

## 1) Product Intent

gzh-cli-gitforge provides a Git-focused CLI and Go library that:

- speeds up common Git workflows,
- enforces consistent commit practices,
- enables parallel worktree workflows,
- and offers repo sync for Git forge providers.

This document replaces a full PRD. It defines goals, non-goals, guardrails,
and release quality gates.

**Detailed Documentation**: See [docs/00-product/](docs/00-product/) for comprehensive product documentation.

| Document                                             | Description                         |
| ---------------------------------------------------- | ----------------------------------- |
| [Vision](docs/00-product/01-vision.md)               | Why this project exists, anti-goals |
| [Principles](docs/00-product/02-principles.md)       | Core values and trade-offs          |
| [Problem Space](docs/00-product/03-problem-space.md) | Target users and pain points        |
| [Scope](docs/00-product/04-scope.md)                 | What's in/out of scope              |
| [Metrics](docs/00-product/05-metrics.md)             | Success criteria and measurement    |
| [Roadmap](docs/00-product/06-roadmap.md)             | Phases and milestones               |

______________________________________________________________________

## 2) Goals (Measurable Targets)

G1. **Reduce Git operation time by 30%**

- Target: common ops (status/commit/branch/merge) p95 < 100ms

G2. **Commit message consistency**

- Target: >= 90% of commits follow a configured template or conventional format

G3. **Parallel workflow adoption**

- Target: >= 50% of active users use worktrees or multi-repo bulk commands weekly

G4. **Library adoption**

- Target: gzh-cli uses this library for 100% of Git operations

G5. **Repo sync reliability**

- Target: >= 99% success rate for org/user sync runs in typical networks

______________________________________________________________________

## 3) Non-Goals (Explicitly Out of Scope)

- No GUI or web interface
- No self-hosted Git server or repository hosting
- No IDE plugins (VS Code/JetBrains/etc.)
- No Git hooks manager bundled by default
- No built-in CI/CD system (only CLI output for automation)
- No advanced TUI until explicitly approved

______________________________________________________________________

## 4) Guardrails and Technical Constraints

**Architecture**

- Library-first: core logic lives in `pkg/*` and is reusable
- CLI in `cmd/` is thin; it should delegate to library packages
- Git CLI is the source of truth (no go-git as the primary engine)
- All operations accept `context.Context` for cancellation/timeouts

**Dependency Boundaries**

- `pkg/` should avoid CLI framework dependencies
- Exception: `pkg/reposynccli` is allowed as a shared CLI adapter only

**Compatibility**

- Go 1.25+ (align with `go.mod`)
- Git 2.30+ on Linux/macOS/Windows (amd64/arm64)

**Safety**

- Destructive operations require explicit flags or dry-run
- Force push is blocked on protected branches unless explicitly overridden
- Inputs must be sanitized before Git CLI execution

**Documentation**

- Public APIs must be GoDoc documented
- CLI usage examples must match actual flags and behavior

______________________________________________________________________

## 5) Quality Gates (Release Readiness)

**Build and Lint**

- `make build` and `make quality` pass with no warnings

**Testing**

- Unit + integration + E2E test suites pass
- Coverage targets:
  - `internal/` >= 80%
  - `pkg/` >= 85%
  - `cmd/` >= 70% (or equivalent CLI integration coverage)

**Performance**

- 95% of common ops < 100ms
- 100% of common ops < 500ms

**Docs and Examples**

- CLI reference complete for all commands
- API reference covers all exported types/functions
- Migration notes for major version bumps

**Integration**

- gzh-cli integration complete and tested

______________________________________________________________________

## 6) Decision Rules

- New features must map to at least one goal or be explicitly approved
- Anything that violates guardrails requires a documented exception
- Release is blocked if quality gates are not met

______________________________________________________________________

**End of Document**
