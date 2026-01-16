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
│   ├── sync/               # Sync config/types
│   ├── reposync/           # Repo sync planner/executor
│   └── provider/           # Forge providers (github/gitlab/gitea)
└── docs/.claude-context/   # Context docs
```

______________________________________________________________________

## Core Design: Bulk-First

**gz-git은 기본적으로 bulk 모드로 동작합니다.** 모든 주요 명령어는 디렉토리를 스캔하여
여러 repository를 동시에 처리합니다.

### 기본 동작

```go
// pkg/repository/types.go
DefaultBulkMaxDepth = 1    // 현재 디렉토리 + 1레벨 하위
DefaultBulkParallel = 5    // 5개 병렬 처리
```

| 명령어 | 기본 동작 |
|--------|-----------|
| `gz-git status` | 현재 디렉토리 + 1레벨 스캔, 5개 병렬 |
| `gz-git fetch` | 현재 디렉토리 + 1레벨 스캔, 5개 병렬 |
| `gz-git pull` | 현재 디렉토리 + 1레벨 스캔, 5개 병렬 |
| `gz-git push` | 현재 디렉토리 + 1레벨 스캔, 5개 병렬 |
| `gz-git switch` | 현재 디렉토리 + 1레벨 스캔, 5개 병렬 |

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
-j, --parallel     병렬 처리 수 (기본: 5)
-n, --dry-run      실행하지 않고 미리보기
--include          포함 패턴 (regex)
--exclude          제외 패턴 (regex)
-f, --format       출력 형식 (default, compact, json, llm)
```

### 주요 명령어

| Command | Description |
|---------|-------------|
| `clone` | 여러 repo를 병렬로 clone (`--url`, `--file`) |
| `status` | 모든 repo 상태 확인 (dirty, ahead/behind) |
| `fetch` | 모든 repo에서 fetch |
| `pull` | 모든 repo에서 pull (rebase/merge 지원) |
| `push` | 모든 repo에서 push (**refspec 지원**: `develop:master`) |
| `switch` | 모든 repo 브랜치 전환 |
| `commit` | 모든 dirty repo에 커밋 |
| `diff` | 모든 repo diff 보기 |
| `update` | 모든 repo를 안전하게 업데이트 (pull --rebase) |
| `cleanup branch` | merged/stale/gone 브랜치 정리 (dry-run 기본) |
| `sync forge` | **GitHub/GitLab/Gitea org 전체 동기화** (아래 참조) |
| `stash` | 모든 repo에서 stash 작업 |
| `tag` | 모든 repo에서 tag 작업 |

### Sync Forge (Org 전체 동기화)

**GitLab/GitHub/Gitea organization 전체를 로컬에 동기화**합니다.

```bash
# GitLab (기본: SSH clone, GitLab 설정 포트 자동 사용)
gz-git sync forge \
  --provider gitlab \
  --org mygroup \
  --target ~/repos \
  --base-url https://gitlab.com \
  --token $GITLAB_TOKEN

# Self-hosted GitLab (SSH 포트 자동 감지! --ssh-port 불필요)
gz-git sync forge \
  --provider gitlab \
  --org devbox \
  --target ~/.mydevbox \
  --base-url https://gitlab.polypia.net \
  --token $GITLAB_TOKEN

# HTTPS clone (SSH 대신)
gz-git sync forge \
  --provider gitlab \
  --org mygroup \
  --target ~/repos \
  --base-url https://gitlab.com \
  --token $GITLAB_TOKEN \
  --clone-proto https

# SSH 포트 강제 지정 (API 응답 무시, 거의 불필요)
gz-git sync forge \
  --provider gitlab \
  --org devbox \
  --target ~/.mydevbox \
  --base-url https://gitlab.polypia.net \
  --token $GITLAB_TOKEN \
  --ssh-port 2224
```

**주요 옵션**:
- `--base-url`: API endpoint (http/https)
- `--clone-proto`: Clone 프로토콜 (`ssh` 또는 `https`, 기본: `ssh`)
- `--ssh-port`: SSH 포트 강제 지정 (**선택**, GitLab API 자동 제공)
- `--dry-run`: 미리보기
- `--include-archived`: Archived repo 포함
- `--include-forks`: Fork repo 포함

**💡 SSH 포트 자동 감지**: GitLab API는 `ssh_url_to_repo` 필드에 올바른 SSH URL(포트 포함)을 제공합니다. `--ssh-port`는 특별한 경우에만 사용하세요.

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

**Last Updated**: 2026-01-01
**Previous**: 153 lines → **Current**: ~190 lines (added bulk ops docs)
