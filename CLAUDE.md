# CLAUDE.md

LLM-optimized guidance for gzh-cli-gitforge.

______________________________________________________________________

## Quick Start (30s scan)

**Binary**: `gz-git`
**Module**: `github.com/gizzahub/gzh-cli-gitforge`
**Go Version**: 1.25.1+
**Architecture**: Safe Git operations CLI with interface-driven design

______________________________________________________________________

## Top 10 Commands

| Command              | Purpose             | When to Use           |
| -------------------- | ------------------- | --------------------- |
| `make quality`       | fmt + lint + test   | Pre-commit (CRITICAL) |
| `make dev-fast`      | format + unit tests | Quick dev cycle       |
| `make build`         | Build binary        | After changes         |
| `make test`          | All tests           | Validation            |
| `make test-coverage` | Coverage report     | Check coverage        |
| `make fmt`           | Format code         | Fix formatting        |
| `make lint`          | Run linters         | Fix lint issues       |
| `make pr-check`      | Pre-PR verification | Before PR             |
| `make install`       | Install binary      | Local testing         |
| `make clean`         | Clean artifacts     | Fresh start           |

______________________________________________________________________

## Absolute Rules (DO/DON'T)

### DO

- Use `gzh-cli-core` for common utilities
- Read `cmd/AGENTS_COMMON.md` before ANY modification
- Run `make quality` before every commit
- **ALWAYS sanitize git inputs** (prevent command injection)
- Test coverage: 80%+ for core logic
- Use git-specific test helpers from `internal/testutil`

### DON'T

- Use shell execution (`sh -c`) - command injection risk
- Concatenate user input into commands
- Skip input validation
- Log credentials or sensitive data
- Commit without security tests

______________________________________________________________________

## Directory Structure

```
.
├── cmd/gz-git/            # CLI commands (AGENTS.md inside)
├── internal/               # Private packages
│   ├── gitcmd/             # Git command executor
│   ├── parser/             # Output parsing
│   ├── config/             # Internal config utilities
│   └── testutil/           # Git test helpers
├── pkg/                    # Public packages
│   ├── repository/         # Repository abstraction + bulk ops
│   ├── config/             # Configuration management
│   ├── provider/           # Forge providers (github/gitlab/gitea)
│   ├── reposync/           # Repo sync planner/executor
│   ├── reposynccli/        # Sync CLI commands
│   ├── workspacecli/       # Workspace CLI commands
│   ├── scanner/            # Local git repo scanner
│   ├── branch/             # Branch utilities + cleanup
│   └── ...                 # history, merge, stash, tag, watch, tui, wizard
└── docs/.claude-context/   # Context docs
```

______________________________________________________________________

## Configuration System

**gz-git** supports profiles and hierarchical config for context switching.

### 5-Layer Precedence (Highest to Lowest)

```
1. Command flags (--provider gitlab)
2. Project config (.gz-git.yaml)
3. Active profile (~/.config/gz-git/profiles/{active}.yaml)
4. Global config (~/.config/gz-git/config.yaml)
5. Built-in defaults
```

### Quick Reference

```bash
gz-git config init                    # Initialize config directory
gz-git config profile create work     # Create profile
gz-git config profile use work        # Set active profile
gz-git config show --effective        # Show effective config
gz-git config hierarchy               # Show config hierarchy tree
```

### Two Config Formats (Both Supported)

| Format | Key | Use Case |
| ------ | --- | -------- |
| **Simple** | `repositories` array | Quick repo list management |
| **Hierarchical** | `workspaces` map | Complex org sync, profiles |

**Details**: See [config-guide.md](docs/.claude-context/config-guide.md)

### Branch Configuration

Configure default branch with fallback support:

```yaml
branch:
  # Single branch
  defaultBranch: develop

  # Fallback list (tries in order: develop → master → repo default)
  defaultBranch: develop,master

  # YAML list format (equivalent to above)
  defaultBranch:
    - develop
    - master

  # Protected branches
  protectedBranches: [main, master, release/*]
```

**Behavior**: When cloning/syncing, tries each branch in order until one exists.
If none exist, falls back to the repository's default branch.

______________________________________________________________________

## Core Design: Bulk-First

**All commands operate in bulk mode by default.** Scan directory to process multiple repos.

```go
// Defaults: pkg/repository/types.go
DefaultBulkMaxDepth = 1    // current + 1 level
DefaultBulkParallel = 10   // 10 parallel operations
```

### Common Flags

```
-d, --scan-depth   Scan depth (default: 1)
-j, --parallel     Parallel count (default: 10)
-n, --dry-run      Preview without executing
--include          Include pattern (regex)
--exclude          Exclude pattern (regex)
-f, --format       Output format (default, compact, json, llm)
--full             Output all fields in generated configs (sync/workspace)
```

**Compact Output**: Generated config files omit redundant `path` field when it
equals the repository name. Use `--full` to include all fields.

### Single Repo Operation

Specify path directly: `gz-git status /path/to/single/repo`

______________________________________________________________________

## Clone Config System (Code Structure)

`gz-git clone -c config.yaml` supports two formats with auto-detection.

| Format | Detection | Types Location |
| ------ | --------- | -------------- |
| **Flat** | `repositories` key at top level | `cmd/gz-git/cmd/clone.go` |
| **Named Groups** | `{group}: { target, repositories }` | `cmd/gz-git/cmd/clone.go` |

**Types** (`clone.go:392-425`):

| Type | Purpose |
| ---- | ------- |
| `CloneConfig` | 전체 config (strategy, parallel, Groups) |
| `CloneGroup` | Named group (target, branch, repositories, hooks) |
| `CloneRepoSpec` | 개별 repo (url, name, path, branch, hooks) |
| `CloneHooks` | Before/after hook commands |

**Parsing**: `parseCloneConfig()` - YAML 파싱, flat/grouped 자동 감지

**Usage docs**: [docs/usage/clone-command.md](docs/usage/clone-command.md)

______________________________________________________________________

## Main Commands

| Command                | Description                                   |
| ---------------------- | --------------------------------------------- |
| `clone`                | Parallel clone (--url, --file, -c config)     |
| `status`               | Health check (fetch + divergence + recommend) |
| `fetch`                | Fetch all repos (--all-remotes default)       |
| `pull`                 | Pull all repos (rebase/merge)                 |
| `push`                 | Push all repos (refspec: `develop:master`)    |
| `switch`               | Branch switch all repos                       |
| `commit`               | Commit all dirty repos                        |
| `update`               | Safe update (pull --rebase)                   |
| `cleanup branch`       | Clean merged/stale/gone branches              |
| `sync from-forge`      | Sync from GitHub/GitLab/Gitea org             |
| `sync config generate` | Generate config from Forge API                |
| `sync status`          | Repository health diagnosis                   |
| `sync setup`           | Interactive sync setup wizard                 |
| `workspace init`       | Scan directory → generate config              |
| `workspace sync`       | Clone/update from config                      |
| `workspace generate-config` | Generate config from Git forge             |
| `config profile`       | Profile management (create/use/list)          |
| `config hierarchy`     | Show config hierarchy tree                    |

______________________________________________________________________

## Workspace Commands

```bash
# Initialize (scan → config)
gz-git workspace init .                  # Scan current directory
gz-git workspace init ~/mydevbox -d 3    # Depth 3
gz-git workspace init . --template       # Empty template (no scan)

# Sync (config → clone/update)
gz-git workspace sync                    # Use .gz-git.yaml
gz-git workspace sync -c workstation.yaml --dry-run
gz-git workspace sync --full             # Output all fields in generated configs

# Status & Management
gz-git workspace status --verbose
gz-git workspace add https://github.com/user/repo.git
gz-git workspace validate
```

______________________________________________________________________

## Sync Commands (Forge API)

```bash
# Direct forge sync
gz-git sync from-forge \
  --provider gitlab \
  --org mygroup \
  --target ~/repos \
  --base-url https://gitlab.com \
  --token $GITLAB_TOKEN \
  --include-subgroups \
  --subgroup-mode flat

# Generate config from forge (compact output by default)
gz-git sync config generate \
  --provider gitlab \
  --org devbox \
  -o .gz-git.yaml

# Generate with all fields (name, path) even if redundant
gz-git sync config generate \
  --provider gitlab \
  --org devbox \
  -o .gz-git.yaml \
  --full

# Health diagnosis
gz-git sync status -c sync.yaml --verbose
gz-git sync status --target ~/repos --depth 2 --timeout 60s
```

### Health Status

| Symbol | Status | Meaning |
| ------ | ------ | ------- |
| `✓` | healthy | Up-to-date, clean |
| `⚠` | warning | Diverged/behind (resolvable) |
| `✗` | error | Conflicts, dirty + behind |
| `⊘` | unreachable | Network timeout |

______________________________________________________________________

## Push with Refspec

```bash
gz-git push --refspec develop:master          # Push develop to master
gz-git push --refspec +develop:master         # Force push
gz-git push --refspec develop:master --dry-run
```

**Valid formats**: `branch`, `local:remote`, `+local:remote`, `refs/heads/main:refs/heads/master`

______________________________________________________________________

## Context Documentation

| Guide | Purpose |
| ----- | ------- |
| [config-guide.md](docs/.claude-context/config-guide.md) | Profiles, hierarchical config, examples |
| [common-tasks.md](docs/.claude-context/common-tasks.md) | Adding commands, testing git ops |
| [security-guide.md](docs/.claude-context/security-guide.md) | Input sanitization, safe execution |

**CRITICAL**: Read before modifying:
- `cmd/AGENTS_COMMON.md` - Project-wide conventions
- `cmd/gz-git/AGENTS.md` - CLI-specific rules

______________________________________________________________________

## Common Mistakes (Top 3)

1. **Not sanitizing git inputs** → Command injection. Use `internal/gitcmd`
2. **Using shell execution** → Security risk. Use `exec.Command("git", args...)`
3. **Logging credentials** → URLs may contain creds. Strip before logging

______________________________________________________________________

## Shared Library (gzh-cli-core)

```go
import (
    "github.com/gizzahub/gzh-cli-core/logger"
    "github.com/gizzahub/gzh-cli-core/errors"
    "github.com/gizzahub/gzh-cli-core/testutil"
)
```

**Git-specific test helpers**:

```go
import "github.com/gizzahub/gzh-cli-gitforge/internal/testutil"

repo := testutil.TempGitRepo(t)
repoWithCommit := testutil.TempGitRepoWithCommit(t)
```

______________________________________________________________________

## Security (CRITICAL)

```go
// SAFE - Arguments passed separately
cmd := exec.Command("git", "clone", url)

// DANGEROUS - Shell execution
cmd := exec.Command("sh", "-c", "git clone " + url)  // NEVER
```

See [security-guide.md](docs/.claude-context/security-guide.md) for details.

______________________________________________________________________

## Git Commit Format

```
{type}({scope}): {description}

{body}

Model: claude-{model}
Co-Authored-By: Claude <noreply@anthropic.com>
```

**Types**: feat, fix, docs, refactor, test, chore
**Scope**: REQUIRED (cmd, internal, pkg/branch, pkg/config)

______________________________________________________________________

## FAQ

| Question | Answer |
| -------- | ------ |
| New git commands? | `cmd/gz-git/` |
| Git execution logic? | `internal/gitcmd/` |
| Output parsing? | `internal/parser/` |

______________________________________________________________________

## Future Development

**Phase 8: Advanced Features** (2/4 Complete)

- [x] Config Profiles
- [x] Workspace Config (recursive hierarchical)
- [ ] Advanced TUI (P1)
- [ ] Interactive Mode (P2)

See [Roadmap](docs/00-product/06-roadmap.md) for full plan.

______________________________________________________________________

**Last Updated**: 2026-01-23
**Lines**: ~350 (optimized from ~940)
