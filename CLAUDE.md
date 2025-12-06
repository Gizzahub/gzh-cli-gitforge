# CLAUDE.md

This file provides LLM-optimized guidance for Claude Code when working with this repository.

---

## Quick Start (30s scan)

**Binary**: `gz-gitforge`
**Module**: `github.com/gizzahub/gzh-cli-gitforge`
**Go Version**: 1.24+
**Architecture**: Multi-forge CLI (GitHub, GitLab, Gitea)

Unified interface for Git forge operations across multiple platforms.

---

## Top 10 Commands

| Command | Purpose | When to Use |
|---------|---------|-------------|
| `make quality` | fmt + lint + test | Pre-commit (CRITICAL) |
| `make dev-fast` | format + unit tests | Quick dev cycle |
| `make build` | Build binary | After changes |
| `make test` | All tests | Validation |
| `make test-coverage` | Coverage report | Check coverage |
| `make fmt` | Format code | Fix formatting |
| `make lint` | Run linters | Fix lint issues |
| `make pr-check` | Pre-PR verification | Before PR |
| `make install` | Install binary | Local testing |
| `make clean` | Clean artifacts | Fresh start |

---

## Absolute Rules (DO/DON'T)

### DO
- ✅ Use `gzh-cli-core` for common utilities (logger, testutil, errors)
- ✅ Read `cmd/AGENTS_COMMON.md` before ANY modification
- ✅ Run `make quality` before every commit
- ✅ Handle rate limits for all forges
- ✅ Provide forge name in error context
- ✅ Test coverage: 80%+ for core logic

### DON'T
- ❌ Skip reading AGENTS.md files
- ❌ Ignore API rate limits
- ❌ Log credentials or tokens
- ❌ Create forge-specific code in common packages
- ❌ Commit without quality checks

---

## Directory Structure

```
.
├── cmd/gitforge/           # CLI commands
│   ├── AGENTS.md           # Module guide (READ THIS!)
│   └── *.go                # Subcommands (repos, prs, issues)
├── internal/               # Private packages
│   ├── config/             # Configuration
│   ├── ratelimit/          # Rate limiting
│   └── httpclient/         # HTTP client wrapper
├── pkg/                    # Public packages
│   ├── forge/              # Unified forge interface
│   ├── github/             # GitHub API client
│   ├── gitlab/             # GitLab API client
│   └── gitea/              # Gitea API client
└── docs/.claude-context/   # Context docs
```

---

## Context Documentation

| Guide | Purpose |
|-------|---------|
| [Common Tasks](docs/.claude-context/common-tasks.md) | Adding commands, testing APIs |
| [Forge Guide](docs/.claude-context/forge-guide.md) | Platform-specific guidelines |

**CRITICAL**: Read before modifying:
- `cmd/AGENTS_COMMON.md` - Project-wide conventions
- `cmd/gitforge/AGENTS.md` - CLI-specific rules

---

## Common Mistakes (Top 3)

1. **Ignoring API rate limits**
   - ⚠️ Will cause 429 errors
   - ✅ Use `internal/ratelimit` package

2. **Logging credentials**
   - ⚠️ Security risk
   - ✅ Strip credentials before logging

3. **Not reading AGENTS.md**
   - ⚠️ Will miss critical module rules
   - ✅ Always check module-specific guides

---

## Shared Library (gzh-cli-core)

**IMPORTANT**: Use for common utilities. DO NOT duplicate.

```go
import (
    "github.com/gizzahub/gzh-cli-core/logger"
    "github.com/gizzahub/gzh-cli-core/errors"
    "github.com/gizzahub/gzh-cli-core/testutil"
    "github.com/gizzahub/gzh-cli-core/config"
)
```

---

## Forge-Specific APIs

| Forge | Client Library | Rate Limit |
|-------|----------------|------------|
| GitHub | `google/go-github/v66` | 5000/hour (authenticated) |
| GitLab | `xanzy/go-gitlab` | Varies by plan |
| Gitea | Similar to GitHub | Self-hosted |

See [Forge Guide](docs/.claude-context/forge-guide.md) for details.

---

## Git Commit Format

```
{type}({scope}): {description}

{body}

Model: claude-{model}
Co-Authored-By: Claude <noreply@anthropic.com>
```

**Types**: feat, fix, docs, refactor, test, chore
**Scope**: REQUIRED (cmd, internal, pkg/github, pkg/gitlab, pkg/gitea)

---

## FAQ

**Q: Where to add new forge commands?**
A: `cmd/gitforge/` - create new command file

**Q: Where to add forge client logic?**
A: `pkg/{forge}/` - GitHub, GitLab, Gitea

**Q: How to handle rate limits?**
A: Use `internal/ratelimit/` package

---

**Last Updated**: 2025-12-06
**Previous**: 250 lines → **Current**: 137 lines (45% reduction)
