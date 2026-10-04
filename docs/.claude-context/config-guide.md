# Configuration Guide

Detailed configuration examples for gz-git profiles, workspaces, and hierarchical configs.

______________________________________________________________________

## Config File Locations

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

______________________________________________________________________

## Profile Management Commands

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

# Show project config (default)
gz-git config show

# Show effective config with precedence sources
gz-git config show --effective

# Show config hierarchy tree
gz-git config hierarchy
```

______________________________________________________________________

## Profile Example (work.yaml)

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

______________________________________________________________________

## Project Config Example (.gz-git.yaml)

```yaml
profile: work  # Use work profile for this project

# Project-specific overrides
sync:
  strategy: pull  # Override profile's reset
  parallel: 3     # Lower parallelism
branch:
  defaultBranch: main
  protectedBranches: [main, develop]  # protected from deletion
  naming:
    device: wip/{device}/{task}       # override one template, keep the rest

push:
  policy:
    protected: [main]      # refuse to push to these at all
    forceMode: lease-only  # lease-only | allow | deny
    foreignWork: block     # block | allow

metadata:
  team: backend
  repository: https://gitlab.company.com/backend/myproject
```

______________________________________________________________________

## Usage Example

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

______________________________________________________________________

## Hierarchical Configuration

### Unified Config Structure

All levels use the same `.gz-git.yaml` format:

```yaml
# ~/.gz-git.yaml (workstation level)
parallel: 10
cloneProto: ssh

workspaces:
  mydevbox:
    path: ~/mydevbox
    type: config              # Has config file (recursive)
    profile: opensource
    parallel: 10

  mywork:
    path: ~/mywork
    type: config
    profile: work

  single-repo:
    path: ~/single-repo
    type: git                 # Plain git repo (no config)
```

```yaml
# ~/mydevbox/.gz-git.yaml (workspace level - same structure!)
profile: opensource

sync:
  strategy: reset  # reset, pull, rebase, fetch, skip, clone
  parallel: 10

workspaces:
  gzh-cli:
    path: gzh-cli
    type: git               # Plain repo

  gzh-cli-gitforge:
    path: gzh-cli-gitforge
    type: config           # Has config file
    sync:
      strategy: pull       # Inline override
```

### Workspace Types

- **`type: config`**: Directory with config file (enables recursive nesting)
- **`type: git`**: Plain Git repository (leaf node, no config)

### Discovery Modes

```bash
# Explicit: Only use workspaces defined in config
gz-git status --discovery-mode explicit

# Auto: Scan directories, ignore explicit workspaces
gz-git status --discovery-mode auto

# Hybrid: Use workspaces if defined, otherwise scan (DEFAULT)
gz-git status --discovery-mode hybrid
```

### API (pkg/config)

```go
// Load config recursively
config, err := config.LoadConfigRecursive(
    "/home/user/mydevbox",
    ".gz-git.yaml",
)

// Apply discovery mode
err = config.LoadWorkspaces(
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

______________________________________________________________________

## Config Systems: Two Formats

gz-git supports two config formats (both work with `workspace sync`):

### Simple Format (`repositories` array)

```yaml
# .gz-git.yaml - Simple format
strategy: pull
parallel: 4
repositories:
  # name omitted - auto-extracted from URL (proxynd-core)
  - url: ssh://git@gitlab.polypia.net:2224/scripton-open/proxynd/proxynd-core.git
    branch: develop
  # name specified - custom directory name
  - name: enterprise
    url: ssh://git@gitlab.polypia.net:2224/scripton-open/proxynd/proxynd-enterprise.git
    branch: develop
  # path for subdirectory clone
  - url: https://github.com/discourse/discourse.git
    path: subdir/discourse
```

| Field    | Required | Description                      |
| -------- | -------- | -------------------------------- |
| `url`    | Yes      | Git clone URL (HTTPS, SSH, git@) |
| `name`   | No       | Directory name (auto from URL)   |
| `path`   | No       | Target path (defaults to name)   |
| `branch` | No       | Branch to checkout               |

#### Flat 로더의 입력 검증 계약

workspace와 reposync의 flat 로더는 TASK-265부터 같은 스키마로 YAML을 해석한다.
이 공통 입력 검증을 유지한다. 각 로더가 실행에 적용하는 필드와 입력 타입 검증은 별개다.
workspace에서 실행에 적용하지 않는 `resume`, `dryRun`, repo의 `strictBranchCheckout`도
boolean 타입이어야 하며 repo의 `provider`는 문자열이어야 한다. `notabool`이나
`provider: [oops]`는 거부한다. 이전 workspace 로더가 이 값을 무시하던 관용성은
공통 스키마 도입으로 변경되었으므로 올바른 타입으로 바꾸거나 사용하지 않는 키를 제거한다.

공통 flat 스키마에 없는 키는 무시한다. `sshPort: [oops]`, `cloneProto: [ssh]`도
flat 문서에서는 로드되지만 실행에 적용되지 않는다. flat URL의 SSH 포트는 위 예시처럼
URL에 지정한다. 계층형 설정의 키를 flat 설정에 옮겨도 적용된다고 가정하지 않는다.
이 두 키를 과거 workspace 로더가 타입 검사하던 동작은 유지하지 않는다.

검증 순서는 YAML 해석 → 각 repository의 URL 존재 → 로더별 strategy 등 의미 검증이다.
따라서 `strategy: bogus`와 URL 누락이 동시에 있으면 `repository[0]: missing URL`을
먼저 보고한다. YAML 문법·타입 오류 접두사는 `parse flat repositories YAML:`이다.
이는 예전 workspace의 `parse YAML:` 접두사를 대체한다.

reposync의 parent 병합에서 child의 `parallel: 0`은 parent의 parallel 값을 유지한다.
반면 child의 명시적인 `maxRetries: 0`은 parent 값을 0으로 덮어쓴다.

### Hierarchical Format (`workspaces` map)

```yaml
# .gz-git.yaml - Hierarchical format
parallel: 10
cloneProto: ssh

profiles:
  polypia:
    provider: gitlab
    baseURL: https://gitlab.polypia.net
    token: ${GITLAB_TOKEN}
    sshPort: 2224

workspaces:
  mydevbox:
    path: ~/mydevbox
    profile: polypia
    source:
      provider: gitlab
      org: devbox
      includeSubgroups: true
    sync:
      strategy: pull
```

### Format Selection

| Scenario          | Recommended Format          |
| ----------------- | --------------------------- |
| Simple repo list  | Simple (`repositories`)     |
| Forge org sync    | Hierarchical (`workspaces`) |
| Multiple profiles | Hierarchical (`workspaces`) |
| Quick setup       | Simple (`repositories`)     |

### Child Config Generation Mode

```yaml
childConfigMode: repositories  # Default - flat array format
# childConfigMode: workspaces  # Map-based format for nested management
# childConfigMode: none        # Directory only, no config generation
```

### Format Detection (Content-Based)

1. **Explicit `kind:` field** (highest priority)
1. **Content inspection**: `workspaces`/`profiles` → hierarchical; `repositories` → simple
1. **Default**: Falls back to `repositories` format

______________________________________________________________________

## Push Policy

`push.policy` restricts what `push` and `handoff end` may write. It is distinct
from `branch.protectedBranches`, which guards deletion.

```yaml
push:
  policy:
    protected: [main, master, "release/*"]
    forceMode: lease-only
    foreignWork: block
```

| Key           | Meaning                                                                                                                                                                       |
| ------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `protected`   | Branch names and trailing-`*` patterns that may not be pushed to. Empty by default. The **destination** decides, so `--refspec develop:main` is refused.                      |
| `forceMode`   | `lease-only` (default) allows `--force`, which pushes with `--force-with-lease`, and refuses a `+` refspec, which has no lease. `allow` permits both. `deny` permits neither. |
| `foreignWork` | `block` (default) refuses a force push that would discard remote commits signed by a different device or agent. `allow` permits it.                                           |

`--force-mode` and `--foreign-work` override their keys for one invocation. A
refused repository is reported as `blocked`, the rest of the batch still runs,
and the command exits non-zero.

`lease-only` applies even with no config file: without it a `+` refspec would be
a silent way around `--force-with-lease`.

### Why `foreignWork` is not redundant with `--force-with-lease`

A lease compares the remote against the ref this machine last fetched, so it
protects only until the next fetch — and a multi-device workflow fetches on
arrival. After that the lease is satisfied and a force push silently drops
whatever the other machine wrote.

The rule reads the [identity trailers](#identity) on the commits that would be
discarded, and refuses only on positive evidence of another writer:

```
push blocked by policy (foreign-work): force push would discard 2 commit(s)
from another machine or agent: a1b2c3d4 chore(wip): handoff checkpoint
(dave-laptop/hermes-02), and 1 more
```

Two limits follow from that:

- Only commits signed by `handoff end` can be attributed. A commit made by hand
  elsewhere carries no trailer and is never counted as foreign — the rule finds
  real conflicts in a workflow that checkpoints through this tool, and finds
  nothing in one that does not.
- A machine that names no identity skips the check entirely; it cannot tell its
  own commits from anyone else's.

`gz-git handoff start` reads the same trailers in the other direction and names
the branches whose remote advanced under someone else. That one only reports:
rebasing onto another writer's commits loses nothing. It is the signal that a
branch has two writers and should be split.

______________________________________________________________________

## Identity

`identity` names the machine and the agent behind an automated commit. `handoff end` writes them as git trailers on the checkpoint commit:

```
chore(wip): handoff checkpoint

Device: dave-office
Agent: hermes-01
```

```yaml
# ~/.config/gz-git/config.yaml — global, not a project's .gz-git.yaml
identity:
  device: dave-office
  agent: hermes-01
```

| Key      | Meaning                                                                                |
| -------- | -------------------------------------------------------------------------------------- |
| `device` | This machine. Defaults to the hostname, so a checkpoint is signed even with no config. |
| `agent`  | The automation driving this machine. Empty means a person is, so nothing is recorded.  |

`GZ_GIT_DEVICE` and `GZ_GIT_AGENT` override the config — an agent process knows
its own name at launch, while a config file is written once and shared by every
run on the machine.

Keep this out of a project's `.gz-git.yaml`: that file is committed, and every
machine that clones it would then report the same device name. Global config and
profiles are machine-local, which is what the value describes.

`gz-git handoff end --no-trailers` omits them for one run.

The trailers are what the [`foreignWork` push rule](#push-policy) and `handoff start`'s shared-branch note read back, so a workspace that never sets an
identity gets a working checkpoint but no cross-machine safety from either.

[`branch naming`](#branch-naming) reads the same two values, but off the
configuration rather than off a commit — it puts them into the branch name in
the first place.

______________________________________________________________________

## Branch Naming

`branch.naming` holds the templates `gz-git branch name <task>` builds from, one
per role a branch plays when a task has more than one writer.

```yaml
branch:
  naming:
    work: feat/{task}              # a task with one writer
    device: feat/{task}/{device}   # one machine's slice of a shared task
    agent: agent/{task}/{agent}    # one agent's, kept out of a person's
```

Those are the defaults; every key is optional and layers merge one key at a
time, so overriding `device` in a project leaves `work` and `agent` alone. (The
push policy is replaced whole instead — a narrower policy must not be widened by
a broader layer. Templates carry no such risk.)

```bash
gz-git branch name task-001-product-unit                 # feat/task-001-product-unit
gz-git branch name task-001-product-unit --kind device   # feat/task-001-product-unit/dave-office
gz-git branch name task-001-product-unit --kind agent    # agent/task-001-product-unit/hermes-01

# Compose: create it across every repository the task spans
gz-git switch "$(gz-git branch name task-001 --kind device)" --create
```

The command prints a name and creates nothing — creation stays with
`gz-git switch --create` for bulk work and plain git for one repository. What it
adds is the part plain git cannot work out: the writer segment, taken from the
resolved [identity](#identity).

| Placeholder | Source                               |
| ----------- | ------------------------------------ |
| `{task}`    | The command's argument               |
| `{device}`  | `identity.device` or `GZ_GIT_DEVICE` |
| `{agent}`   | `identity.agent` or `GZ_GIT_AGENT`   |

Two rules follow from that:

- Every substituted value is slugified — lowercased, with runs of anything
  outside `[a-z0-9]` collapsed to a dash. The default device name is the
  hostname, and `Daves-MacBook.local` is not a legal branch name, so without
  this a template would work on one machine and fail on the next.
- A `device` or `agent` branch is refused when its segment is unnamed. The
  result would be the shared branch again under a longer name, which is the
  collision that splitting the branch exists to avoid.

A misspelled placeholder survives substitution intact and is reported rather
than baked into a branch name.

This is what `handoff start`'s shared-branch note points at: when the remote
advanced under another writer, `--kind device` or `--kind agent` gives each of
them a branch of their own.

______________________________________________________________________

## Security Notes

- Profile files: 0600 permissions (user read/write only)
- Config directory: 0700 permissions (user access only)
- Use environment variables for tokens: `token: ${GITLAB_TOKEN}`
- No shell command execution (only `${VAR}` expansion)

______________________________________________________________________

**Last Updated**: 2026-01-23
