# CLAUDE.md

This file provides LLM-optimized guidance for Claude Code when working with this repository.

______________________________________________________________________

## Quick Start (30s scan)

**Binary**: `gz-git`
**Module**: `github.com/gizzahub/gzh-cli-gitforge`
**Go Version**: 1.25.1+
**Architecture**: Safe Git operations CLI

Interface-driven design with strict input sanitization for security.

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

- ✅ Use `gzh-cli-core` for common utilities
- ✅ Read `cmd/AGENTS_COMMON.md` before ANY modification
- ✅ Run `make quality` before every commit
- ✅ **ALWAYS sanitize git inputs** (prevent command injection)
- ✅ Test coverage: 80%+ for core logic
- ✅ Use git-specific test helpers from `internal/testutil`

### DON'T

- ❌ Use shell execution (`sh -c`) - command injection risk
- ❌ Concatenate user input into commands
- ❌ Skip input validation
- ❌ Log credentials or sensitive data
- ❌ Commit without security tests

______________________________________________________________________

## Directory Structure

```
.
├── cmd/gz-git/            # CLI commands
│   ├── AGENTS.md           # Module guide (READ THIS!)
│   └── *.go                # Subcommands
├── internal/               # Private packages
│   ├── gitcmd/             # Git command executor
│   ├── parser/             # Output parsing
│   └── testutil/           # Git test helpers
├── pkg/                    # Public packages
│   ├── repository/         # Repository abstraction + bulk ops
│   ├── branch/             # Branch utilities + cleanup services
│   ├── history/            # History analysis
│   ├── merge/              # Merge conflict detection
│   ├── stash/              # Stash management
│   ├── tag/                # Tag management + semver
│   ├── watch/              # Repo monitoring
│   ├── scanner/            # Local git repo scanner (NEW!)
│   ├── reposync/           # Repo sync planner/executor
│   ├── reposynccli/        # Sync CLI commands (from-forge, config generate)
│   ├── workspacecli/       # Workspace CLI commands (init, scan, sync, status, add, validate)
│   ├── config/             # Configuration management (profiles, precedence) **NEW!**
│   └── provider/           # Forge providers (github/gitlab/gitea)
└── docs/.claude-context/   # Context docs
```

______________________________________________________________________

## Configuration Profiles (NEW!)

**gz-git** supports configuration profiles to eliminate repetitive flags and enable context switching.

### 5-Layer Precedence (Highest to Lowest)

```
1. Command flags (--provider gitlab)
2. Project config (.gz-git.yaml in current dir or parent)
3. Active profile (~/.config/gz-git/profiles/{active}.yaml)
4. Global config (~/.config/gz-git/config.yaml)
5. Built-in defaults
```

### Config File Locations

```
~/.config/gz-git/
├── config.yaml              # Global config
├── profiles/
│   ├── default.yaml        # Default profile
│   ├── work.yaml           # User profiles
│   └── personal.yaml
└── state/
    └── active-profile.txt  # Currently active profile

# Project config (auto-detected)
~/myproject/.gz-git.yaml
```

### Profile Management Commands

```bash
# Initialize config directory
gz-git config init

# Create profile (interactive)
gz-git config profile create work

# Create profile (with flags)
gz-git config profile create work \
  --provider gitlab \
  --base-url https://gitlab.company.com \
  --token ${WORK_TOKEN} \
  --clone-proto ssh \
  --ssh-port 2224

# List profiles
gz-git config profile list

# Set active profile
gz-git config profile use work

# Show profile details
gz-git config profile show work

# Delete profile
gz-git config profile delete work

# Show effective config (with precedence sources)
gz-git config show

# Get specific value
gz-git config get provider

# Set global default
gz-git config set defaults.parallel 10
```

### Profile Example (work.yaml)

```yaml
name: work
provider: gitlab
baseURL: https://gitlab.company.com
token: ${WORK_GITLAB_TOKEN}  # Environment variable
cloneProto: ssh
sshPort: 2224
parallel: 10
includeSubgroups: true
subgroupMode: flat

# Command-specific overrides
sync:
  strategy: reset
  maxRetries: 3
```

### Project Config Example (.gz-git.yaml)

```yaml
profile: work  # Use work profile for this project

# Project-specific overrides
sync:
  strategy: pull  # Override profile's reset
  parallel: 3     # Lower parallelism
branch:
  defaultBranch: main
  protectedBranches: [main, develop]

metadata:
  team: backend
  repository: https://gitlab.company.com/backend/myproject
```

### Usage Example

```bash
# One-time setup
gz-git config profile create work \
  --provider gitlab \
  --base-url https://gitlab.company.com \
  --token ${WORK_TOKEN}
gz-git config profile use work

# Now all commands use work profile automatically
gz-git sync from-forge --org backend  # Uses gitlab, token, etc.
gz-git status                         # Uses work profile defaults

# Switch context
gz-git config profile use personal
gz-git sync from-forge --org my-projects  # Now uses personal profile

# One-off override
gz-git sync from-forge --profile work --org backend  # Temporarily use work
```

### Environment Variable Expansion

Config files support `${VAR_NAME}` syntax for sensitive values:

```yaml
token: ${GITLAB_TOKEN}      # Recommended
baseURL: ${GITLAB_URL}
```

**Security Notes:**

- Profile files: 0600 permissions (user read/write only)
- Config directory: 0700 permissions (user access only)
- Use environment variables for tokens, not plain text
- No shell command execution (only `${VAR}` expansion)

### Recursive Hierarchical Configuration (NEW!)

**gz-git** now supports recursive hierarchical configuration for managing complex workstation/workspace/project structures.

**Key Concept**: One unified `Config` type that nests recursively at all levels (workstation → workspace → project → submodule, etc.)

#### Unified Config Structure

All levels use the same `.gz-git.yaml` format (or custom filename):

```yaml
# ~/.gz-git.yaml (workstation level)
parallel: 10
cloneProto: ssh

children:
  - path: ~/mydevbox
    type: config              # Has config file (recursive)
    profile: opensource
    parallel: 10

  - path: ~/mywork
    type: config
    configFile: .work-config.yaml  # Custom filename!
    profile: work

  - path: ~/single-repo
    type: git                 # Plain git repo (no config)
```

```yaml
# ~/mydevbox/.gz-git.yaml (workspace level - same structure!)
profile: opensource

sync:
  strategy: reset
  parallel: 10

children:
  - path: gzh-cli
    type: git               # Plain repo

  - path: gzh-cli-gitforge
    type: config           # Has config file
    sync:
      strategy: pull       # Inline override
```

```yaml
# ~/mydevbox/gzh-cli-gitforge/.gz-git.yaml (project level - same structure!)
sync:
  strategy: pull

children:
  - path: vendor/lib
    type: git
    sync:
      strategy: skip       # Submodule skip sync
```

#### Child Types

- **`type: config`**: Directory with config file (enables recursive nesting)
- **`type: git`**: Plain Git repository (leaf node, no config)

#### Discovery Modes

```bash
# Explicit: Only use children defined in config
gz-git status --discovery-mode explicit

# Auto: Scan directories, ignore explicit children
gz-git status --discovery-mode auto

# Hybrid: Use children if defined, otherwise scan (DEFAULT)
gz-git status --discovery-mode hybrid
```

#### Precedence (Recursive)

```
1. Command flags
2. Current directory config (.gz-git.yaml)
3. Parent directory config (../.gz-git.yaml)
4. Grandparent directory config (../../.gz-git.yaml)
   ... (recursively up to root)
N. Active profile
N+1. Global config
N+2. Built-in defaults
```

**Simple Rule**: Child overrides parent

#### API (pkg/config)

```go
// Load config recursively
config, err := config.LoadConfigRecursive(
    "/home/user/mydevbox",
    ".gz-git.yaml",
)

// Apply discovery mode
err = config.LoadChildren(
    "/home/user/mydevbox",
    config,
    config.HybridMode,
)

// Find nearest config file
configDir, err := config.FindConfigRecursive(
    "/home/user/mydevbox/project",
    ".gz-git.yaml",
)
```

#### Benefits

- ✅ **Single Type**: One `Config` type for all levels
- ✅ **Single Filename**: `.gz-git.yaml` everywhere (customizable)
- ✅ **Infinite Nesting**: Unlimited hierarchy depth
- ✅ **Inline Overrides**: Children can override parent settings
- ✅ **Custom Filenames**: `configFile: .custom.yaml`

______________________________________________________________________

## Core Design: Bulk-First

**gz-git은 기본적으로 bulk 모드로 동작합니다.** 모든 주요 명령어는 디렉토리를 스캔하여
여러 repository를 동시에 처리합니다.

### 기본 동작

```go
// pkg/repository/types.go
DefaultBulkMaxDepth = 1    // 현재 디렉토리 + 1레벨 하위
DefaultBulkParallel = 10    // 10개 병렬 처리
```

| 명령어          | 기본 동작                             |
| --------------- | ------------------------------------- |
| `gz-git status` | 현재 디렉토리 + 1레벨 스캔, 10개 병렬 |
| `gz-git fetch`  | 현재 디렉토리 + 1레벨 스캔, 10개 병렬 |
| `gz-git pull`   | 현재 디렉토리 + 1레벨 스캔, 10개 병렬 |
| `gz-git push`   | 현재 디렉토리 + 1레벨 스캔, 10개 병렬 |
| `gz-git switch` | 현재 디렉토리 + 1레벨 스캔, 10개 병렬 |

### 스캔 깊이 (--scan-depth, -d)

```
depth=0: 현재 디렉토리만 (단일 repo처럼 동작)
depth=1: 현재 + 1레벨 (기본값) - ~/projects/repo1, ~/projects/repo2
depth=2: 현재 + 2레벨 - ~/projects/org/repo1, ~/projects/org/repo2
```

### 단일 Repository 작업

경로를 직접 지정하면 해당 repo만 처리:

```bash
gz-git info /path/to/single/repo
gz-git watch /path/to/single/repo
gz-git status /path/to/single/repo
gz-git fetch /path/to/single/repo
```

### 공통 플래그

```
-d, --scan-depth   스캔 깊이 (기본: 1)
-j, --parallel     병렬 처리 수 (기본: 10)
-n, --dry-run      실행하지 않고 미리보기
--include          포함 패턴 (regex)
--exclude          제외 패턴 (regex)
-f, --format       출력 형식 (default, compact, json, llm)
```

### 주요 명령어

| Command                | Description                                                               |
| ---------------------- | ------------------------------------------------------------------------- |
| `clone`                | 여러 repo를 병렬로 clone (`--url`, `--file`)                              |
| `status`               | **종합 health check** (fetch + divergence + 추천) - 모든 remote fetch     |
| `fetch`                | 모든 repo에서 fetch - **기본적으로 모든 remote** (`--all-remotes` 기본값) |
| `pull`                 | 모든 repo에서 pull (rebase/merge 지원)                                    |
| `push`                 | 모든 repo에서 push (**refspec 지원**: `develop:master`)                   |
| `switch`               | 모든 repo 브랜치 전환                                                     |
| `commit`               | 모든 dirty repo에 커밋                                                    |
| `diff`                 | 모든 repo diff 보기                                                       |
| `update`               | 모든 repo를 안전하게 업데이트 (pull --rebase)                             |
| `cleanup branch`       | merged/stale/gone 브랜치 정리 (dry-run 기본)                              |
| `sync from-forge`      | **GitHub/GitLab/Gitea org 전체 동기화**                                   |
| `sync config generate` | **Forge API → config 생성**                                               |
| `sync status`          | **Repository health 진단 (fetch, divergence, conflicts)**                 |
| `workspace init`       | **빈 config 파일 생성** (.gz-git.yaml)                                    |
| `workspace scan`       | **로컬 디렉토리 스캔 → config 생성**                                      |
| `workspace sync`       | **Config 기반 repo clone/update**                                         |
| `workspace status`     | **Workspace health check**                                                |
| `workspace add`        | **Config에 repo 추가**                                                    |
| `workspace validate`   | **Config 파일 검증**                                                      |
| `stash`                | 모든 repo에서 stash 작업                                                  |
| `tag`                  | 모든 repo에서 tag 작업                                                    |
| `config`               | **프로파일 및 설정 관리 (NEW!)**                                          |
| `config profile`       | 프로파일 생성/수정/삭제/전환                                              |
| `config show`          | 현재 설정 보기 (precedence 포함)                                          |

### Workspace 명령어 (Local Config Management)

**gz-git workspace**는 로컬 config 파일 기반 워크스페이스 관리를 제공합니다:

```bash
# 워크스페이스 초기화
gz-git workspace init                    # .gz-git.yaml 생성
gz-git workspace init -c myworkspace.yaml

# 디렉토리 스캔 → config 생성
gz-git workspace scan ~/mydevbox
gz-git workspace scan ~/mydevbox --depth 3 --exclude "vendor,tmp"

# Config 기반 clone/update
gz-git workspace sync
gz-git workspace sync -c myworkspace.yaml --dry-run

# 워크스페이스 health check
gz-git workspace status
gz-git workspace status --verbose

# Repo 추가
gz-git workspace add https://github.com/user/repo.git
gz-git workspace add --from-current

# Config 검증
gz-git workspace validate
```

______________________________________________________________________

### Sync 명령어 (Forge Synchronization)

**gz-git sync**는 Git Forge (GitHub/GitLab/Gitea) API를 통한 동기화를 제공합니다.

로컬 config 기반 작업은 `gz-git workspace` 명령어를 사용하세요.

#### **`sync from-forge`** - Git Forge에서 직접 동기화

```bash
# GitLab (기본: SSH clone, SSH 포트 자동 감지)
gz-git sync from-forge \
  --provider gitlab \
  --org mygroup \
  --target ~/repos \
  --base-url https://gitlab.com \
  --token $GITLAB_TOKEN

# Self-hosted GitLab with subgroups (flat mode)
gz-git sync from-forge \
  --provider gitlab \
  --org parent-group \
  --target ~/repos \
  --base-url https://gitlab.polypia.net \
  --token $GITLAB_TOKEN \
  --include-subgroups \
  --subgroup-mode flat
```

**주요 옵션**:

- `--base-url`: API endpoint (http/https)
- `--clone-proto`: Clone 프로토콜 (`ssh` | `https`, 기본: `ssh`)
- `--ssh-port`: SSH 포트 강제 지정 (GitLab은 API 자동 제공)
- `--include-subgroups`: GitLab 하위 그룹 포함
- `--subgroup-mode`: `flat` (dash-separated) | `nested` (directories)

#### **`sync config generate`** - Forge API에서 config 생성

```bash
gz-git sync config generate \
  --provider gitlab \
  --org devbox \
  --target ~/repos \
  --token $GITLAB_TOKEN \
  -o .gz-git.yaml

# 생성된 config로 workspace 사용
gz-git workspace sync
```

#### **`sync status`** - Repository Health 진단

**진단 기능**:

- ✅ **모든 remote fetch** (timeout 지원, 기본 30초)
- ✅ **네트워크 문제 감지** (timeout, unreachable, auth failed)
- ✅ **local/remote HEAD 비교** (ahead/behind/diverged)
- ✅ **충돌 가능성 탐지** (dirty + behind, merge conflicts)
- ✅ **실행 가능한 권장사항 제공** (다음 명령어 안내)

```bash
# Config 기반 health check
gz-git sync status -c sync.yaml

# 디렉토리 스캔 + health check
gz-git sync status --target ~/repos --depth 2

# 빠른 체크 (remote fetch 생략, 기존 데이터 사용)
gz-git sync status -c sync.yaml --skip-fetch

# Custom timeout (느린 네트워크)
gz-git sync status -c sync.yaml --timeout 60s

# 상세 출력 (branch, divergence, working tree)
gz-git sync status -c sync.yaml --verbose
```

**출력 예시**:

```
Checking repository health...

✓ gzh-cli (master)              healthy     up-to-date
⚠ gzh-cli-gitforge (develop)   warning     3↓ 2↑ diverged
  → Diverged: 2 ahead, 3 behind. Use 'git pull --rebase' or 'git merge' to reconcile
✗ gzh-cli-quality (main)        error       dirty + 5↓ behind
  → Commit or stash 3 modified files, then pull 5 commits from upstream
⊘ gzh-cli-template (master)     timeout     fetch failed (30s timeout)
  → Check network connection and verify remote URL is accessible

Summary: 1 healthy, 1 warning, 1 error, 1 unreachable (4 total)
Total time: 32.5s
```

**Health Status**:

- `✓ healthy` - 최신 상태, clean working tree
- `⚠ warning` - diverged, behind, ahead (해결 가능)
- `✗ error` - conflicts, dirty + behind (수동 개입 필요)
- `⊘ unreachable` - network timeout, auth failed

**Divergence Types**:

- `up-to-date` - local == remote
- `N↓ behind` - fast-forward 가능
- `N↑ ahead` - push 가능
- `N↑ N↓ diverged` - merge/rebase 필요
- `conflict` - merge conflict 존재
- `no-upstream` - upstream branch 미설정

### Push with Refspec (브랜치 매핑)

**Refspec**을 사용하면 로컬 브랜치를 다른 이름의 원격 브랜치로 push할 수 있습니다:

```bash
# develop 브랜치를 master로 push (모든 하위 repo)
gz-git push --refspec develop:master

# force push (주의!)
gz-git push --refspec +develop:master

# 여러 원격지에 동시 push
gz-git push --refspec develop:master --remote origin --remote backup

# dry-run으로 먼저 확인
gz-git push --refspec develop:master --dry-run
```

**Refspec 검증** (자동으로 수행):

- ✅ **형식 검증**: Git 브랜치명 규칙 준수 체크 (명령어 실행 전)
- ✅ **소스 브랜치 확인**: 로컬에 소스 브랜치 존재 여부 확인 (원격 체크 전)
- ✅ **커밋 수 계산**: 실제 push될 커밋 수를 정확히 계산
- ✅ **원격 브랜치 확인**: 원격 브랜치 존재 여부 체크

**에러 메시지 예시**:

```bash
# 소스 브랜치 없음
✗ agent-mesh-cli (master)  failed  10ms
  ⚠  refspec source branch 'develop' not found in repository (current branch: master)

# 잘못된 형식
Error: invalid refspec: refspec contains invalid character: ":"
```

**유효한 형식**:

- `branch` - 같은 이름으로 push
- `local:remote` - 로컬 브랜치를 원격 브랜치로
- `+local:remote` - force push (--force-with-lease 사용)
- `refs/heads/main:refs/heads/master` - 전체 ref 경로

**Invalid 형식** (자동으로 에러 발생):

- `develop::master` - 이중 콜론
- `branch name` - 공백 포함
- `-invalid` - 하이픈으로 시작
- `branch.` - 점으로 종료
- `branch..name` - 연속 점

______________________________________________________________________

## Context Documentation

| Guide                                                    | Purpose                            |
| -------------------------------------------------------- | ---------------------------------- |
| [Common Tasks](docs/.claude-context/common-tasks.md)     | Adding commands, testing git ops   |
| [Security Guide](docs/.claude-context/security-guide.md) | Input sanitization, safe execution |

**CRITICAL**: Read before modifying:

- `cmd/AGENTS_COMMON.md` - Project-wide conventions
- `cmd/gz-git/AGENTS.md` - CLI-specific rules
- [Security Guide](docs/.claude-context/security-guide.md) - Security requirements

______________________________________________________________________

## Common Mistakes (Top 3)

1. **Not sanitizing git inputs**

   - ⚠️ Command injection vulnerability
   - ✅ Always validate inputs, use `internal/gitcmd`

1. **Using shell execution**

   - ⚠️ Security risk (`sh -c`)
   - ✅ Use `exec.Command("git", args...)` with separate args

1. **Logging credentials**

   - ⚠️ URLs may contain credentials
   - ✅ Strip credentials before logging

______________________________________________________________________

## Shared Library (gzh-cli-core)

**IMPORTANT**: Use for common utilities. DO NOT duplicate.

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

### Safe Command Execution

```go
// ✅ SAFE - Arguments passed separately
cmd := exec.Command("git", "clone", url)

// ❌ DANGEROUS - Shell execution
cmd := exec.Command("sh", "-c", "git clone " + url)
```

### Input Validation

```go
// Always validate before executing
if !isValidBranchName(branch) {
    return errors.New("invalid branch name")
}
```

See [Security Guide](docs/.claude-context/security-guide.md) for details.

______________________________________________________________________

## Git Commit Format

```
{type}({scope}): {description}

{body}

Model: claude-{model}
Co-Authored-By: Claude <noreply@anthropic.com>
```

**Types**: feat, fix, docs, refactor, test, chore
**Scope**: REQUIRED (cmd, internal, pkg/branch, pkg/commit)

______________________________________________________________________

## FAQ

**Q: Where to add new git commands?**
A: `cmd/gz-git/` - create new command file

**Q: Where to add git execution logic?**
A: `internal/gitcmd/` - safe command execution

**Q: Where to add output parsing?**
A: `internal/parser/` - git output parsing

______________________________________________________________________

## Future Development

**Phase 8: Advanced Features** (PLANNED)

- [Phase 8 Overview](docs/design/PHASE8_OVERVIEW.md) - Complete feature roadmap
- [Config Profiles](docs/design/CONFIG_PROFILES.md) - Per-project and global settings (P2)
- [Advanced TUI](docs/design/ADVANCED_TUI.md) - Interactive terminal UI (P1)
- [Interactive Mode](docs/design/INTERACTIVE_MODE.md) - Guided workflows and wizards (P2)

See [Roadmap](docs/00-product/06-roadmap.md) for full development plan.

______________________________________________________________________

**Last Updated**: 2026-01-16
**Previous**: 153 lines → **Current**: ~470 lines (added bulk ops, sync redesign, Phase 8)
