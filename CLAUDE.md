# CLAUDE.md

This file provides LLM-optimized guidance for Claude Code when working with this repository.

---

## Project Context

**Binary**: `gz-gitforge`
**Module**: `github.com/gizzahub/gzh-cli-gitforge`
**Go Version**: 1.24+
**Architecture**: Standard CLI (Cobra-based)

### Core Principles

- **Interface-driven design**: Use Go interfaces for abstraction
- **Direct constructors**: No DI containers, simple factory pattern
- **Multi-forge support**: GitHub, GitLab, Gitea with unified interface
- **Rate limiting**: Respect API rate limits for all forges
- **Use shared library**: Common utilities from `gzh-cli-core`

---

## Shared Library (gzh-cli-core)

**IMPORTANT**: Use `gzh-cli-core` for common utilities. DO NOT create local duplicates.

| Package | Import | Purpose |
|---------|--------|---------|
| logger | `gzh-cli-core/logger` | Structured logging |
| testutil | `gzh-cli-core/testutil` | Test helpers (TempDir, Assert*, Capture) |
| errors | `gzh-cli-core/errors` | Error types and wrapping |
| config | `gzh-cli-core/config` | Config loading utilities |
| cli | `gzh-cli-core/cli` | CLI flags and output |
| version | `gzh-cli-core/version` | Version info |

```go
import (
    "github.com/gizzahub/gzh-cli-core/logger"
    "github.com/gizzahub/gzh-cli-core/errors"
    "github.com/gizzahub/gzh-cli-core/testutil"
)
```

---

## Module-Specific Guides (AGENTS.md)

**Read these before modifying code:**

| Guide | Location | Purpose |
|-------|----------|---------|
| Common Rules | `cmd/AGENTS_COMMON.md` | Project-wide conventions |
| CLI Module | `cmd/gitforge/AGENTS.md` | CLI-specific rules |

---

## Internal Packages

| Package | Purpose | Key Functions |
|---------|---------|---------------|
| `internal/config` | Configuration management | Forge credentials, settings |
| `internal/httpclient` | HTTP client wrapper | Rate-limited requests |
| `internal/ratelimit` | Rate limit handling | API quota management |

## Public Packages (pkg/)

| Package | Purpose |
|---------|---------|
| `pkg/github` | GitHub API operations |
| `pkg/gitlab` | GitLab API operations |
| `pkg/gitea` | Gitea API operations |
| `pkg/forge` | Unified forge interface |

---

## Development Workflow

### Before Code Modification

1. **Read AGENTS.md** for the module you're modifying
2. Check existing patterns in `internal/` and `pkg/`
3. Review CONTRIBUTING.md for guidelines

### Code Modification Process

```bash
# 1. Write code + tests
# 2. Quality checks (CRITICAL)
make quality    # runs fmt + lint + test

# Quick development cycle
make dev-fast   # format + unit tests only

# Pre-PR verification
make pr-check
```

---

## Essential Commands Reference

### Development Workflow

```bash
# One-time setup
make deps
make install-tools

# Before every commit (CRITICAL)
make quality

# Build & install
make build
make install

# Quick development
make dev-fast   # format + unit tests
make dev        # format + lint + test
```

### Testing

```bash
make test           # All tests
make test-unit      # Unit tests only
make test-coverage  # With coverage report
make bench          # Benchmarks
```

### Code Quality

```bash
make fmt            # Format code
make lint           # Run linters
make fmt-diff       # Format changed files only
make lint-diff      # Lint changed files only
```

---

## Project Structure

```
.
├── cmd/
│   └── gitforge/
│       ├── AGENTS.md           # Module-specific guide
│       ├── main.go             # Entry point
│       ├── root.go             # Root command
│       └── *.go                # Subcommands (repos, prs, issues)
├── internal/                    # Private packages
│   ├── config/                 # Configuration management
│   ├── errors/                 # Forge-specific errors
│   ├── httpclient/             # HTTP client wrapper
│   ├── logger/                 # Structured logging
│   ├── ratelimit/              # Rate limiting
│   └── testutil/               # Test utilities
├── pkg/                         # Public packages
│   ├── forge/                  # Unified forge interface
│   ├── github/                 # GitHub API client
│   ├── gitlab/                 # GitLab API client
│   └── gitea/                  # Gitea API client
├── docs/                        # Documentation
├── examples/                    # Usage examples
├── .make/                       # Modular Makefile
├── .golangci.yml               # Linter config
├── CLAUDE.md                   # This file
├── go.mod                      # Go module
├── Makefile                    # Build automation
└── README.md                   # Project documentation
```

---

## Important Rules

### Critical Requirements

- **Read AGENTS.md** before modifying any module
- Always run `make quality` before commit
- Test coverage: 80%+ for core logic
- **API safety**: Handle rate limits, validate inputs
- **Error context**: Always provide forge name in errors

### Code Style

- **Binary name**: `gz-gitforge`
- **Interface-driven**: Use interfaces for testability
- **Error handling**: Use structured errors with context
- **Forge abstraction**: Unified interface for all forges

### Commit Format

```
{type}({scope}): {description}

{body}

Model: claude-{model}
Co-Authored-By: Claude <noreply@anthropic.com>
```

**Types**: feat, fix, docs, refactor, test, chore
**Scope**: REQUIRED (e.g., cmd, internal, pkg/github, pkg/gitlab)

---

## Forge-Specific Guidelines

### GitHub API
- Use `github.com/google/go-github/v66`
- Respect rate limits (5000/hour authenticated)
- Handle secondary rate limits

### GitLab API
- Use `github.com/xanzy/go-gitlab`
- Rate limits vary by plan
- Handle pagination properly

### Gitea API
- Similar to GitHub API
- Self-hosted considerations
- Version compatibility

---

## FAQ

**Q: Where to add new forge commands?**
A: `cmd/gitforge/` - create new command file

**Q: Where to add forge client logic?**
A: `pkg/{forge}/` - GitHub, GitLab, Gitea

**Q: Where to add common forge operations?**
A: `pkg/forge/` - unified interface

**Q: How to handle rate limits?**
A: Use `internal/ratelimit/` package

**Q: How to add forge-specific errors?**
A: `internal/errors/errors.go` - categorize by forge

**Q: What files should AI not modify?**
A: See `.claudeignore` (build artifacts, generated code)

---

**Last Updated**: 2024-12-05
