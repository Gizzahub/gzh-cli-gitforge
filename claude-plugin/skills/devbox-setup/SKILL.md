---
name: devbox-setup
description: |
  Guide for simplifying DevBox/Monorepo multi-repository clone setup using gz-git.
  Use when:
  - Creating or refactoring Makefile prepare targets for multi-repo projects
  - Migrating from shell for-loops to gz-git clone bulk operations
  - Setting up new DevBox with multiple git repositories
  - Choosing between Makefile, Workspace CLI, or Forge Sync approaches
allowed-tools: Bash, Read, Write, Edit
---

# DevBox Multi-Repo Setup with gz-git

This skill guides you to simplify multi-repository clone operations using `gz-git`.

## Overview: Three Approaches

| Approach           | Complexity | Best For                               |
| ------------------ | ---------- | -------------------------------------- |
| Makefile + clone   | Simple     | Quick setup, CI/CD, fixed repo list    |
| Workspace CLI      | Medium     | YAML config, team sharing, local mgmt  |
| Forge Sync         | Advanced   | Org-wide sync, profiles, auto-discover |

### Quick Decision Guide

```
Do you need to sync an entire GitHub/GitLab organization?
├─ Yes → Approach 3: Forge Sync
└─ No
   └─ Do you need YAML config for team sharing?
      ├─ Yes → Approach 2: Workspace CLI
      └─ No → Approach 1: Makefile + clone
```

---

## Approach 1: Makefile + gz-git clone

Best for: Fixed repository lists, CI/CD pipelines, simple setups.

### Problem: Complex Shell Loops

```makefile
# BAD: Complex, sequential, error-prone (30+ lines)
prepare:
	@for lib in $(ALL_LIBS); do \
		if [ -d "$$lib/.git" ]; then \
			# complex validation logic...
		fi; \
	done
```

### Solution: gz-git clone

```makefile
# GOOD: Simple, parallel, reliable (3 lines)
prepare:
	@echo "Syncing repositories..."
	@$(GZ) clone --update -b $(branch_name) $(GZ_CLONE_URL_ARGS)
	@echo "Done."
```

### Minimal Template

```makefile
GZ ?= gz-git
branch_name ?= master

REPOS := repo1 repo2 repo3
GIT_BASE := git@github.com:myorg
REPO_URLS := $(addsuffix .git,$(addprefix $(GIT_BASE)/,$(REPOS)))
GZ_CLONE_URL_ARGS = $(foreach u,$(REPO_URLS),--url $(u))

.PHONY: prepare

prepare:
	@echo "Cloning/updating repositories (branch: $(branch_name))..."
	@$(GZ) clone --update -b $(branch_name) $(GZ_CLONE_URL_ARGS)
```

### Full Featured Template

```makefile
GZ ?= gz-git
branch_name ?= master

GIT_BASE_SSH := git@github.com:myorg
GIT_BASE_HTTPS := https://github.com/myorg

REPOS := project-core project-api project-web project-cli

REPO_URLS_SSH := $(addsuffix .git,$(addprefix $(GIT_BASE_SSH)/,$(REPOS)))
REPO_URLS_HTTPS := $(addsuffix .git,$(addprefix $(GIT_BASE_HTTPS)/,$(REPOS)))

GZ_CLONE_URLS ?= $(REPO_URLS_SSH)
GZ_CLONE_URL_ARGS = $(foreach u,$(GZ_CLONE_URLS),--url $(u))
GZ_PARALLEL ?= 10

.PHONY: prepare prepare-https prepare-dry status

prepare:
	@echo "Syncing repositories via SSH..."
	@$(GZ) clone --update -b $(branch_name) -j $(GZ_PARALLEL) $(GZ_CLONE_URL_ARGS)
	@$(MAKE) status

prepare-https:
	@$(MAKE) prepare GZ_CLONE_URLS="$(REPO_URLS_HTTPS)"

prepare-dry:
	@$(GZ) clone --update -b $(branch_name) --dry-run $(GZ_CLONE_URL_ARGS)

status:
	@$(GZ) status -d 1 .
```

### File-Based URL List

For large projects:

```makefile
prepare:
	@$(GZ) clone --update -b $(branch_name) --file repos.txt
```

**repos.txt**:
```
git@github.com:myorg/repo1.git
git@github.com:myorg/repo2.git
```

---

## Approach 2: Workspace CLI

Best for: Team-shared YAML config, local workspace management, declarative setup.

### Quick Start

```bash
# Option A: Scan existing repos → generate config
gz-git workspace scan ~/mydevbox -o .gz-git.yaml

# Option B: Create empty config
gz-git workspace init

# Sync based on config
gz-git workspace sync

# Check status
gz-git workspace status
```

### Config Format (.gz-git.yaml)

```yaml
# .gz-git.yaml (Workspace CLI format uses `repositories` array)
repositories:
  - name: gzh-cli-core
    url: git@github.com:gizzahub/gzh-cli-core.git
    branch: master
  - name: gzh-cli-gitforge
    url: git@github.com:gizzahub/gzh-cli-gitforge.git
    branch: develop
  - name: gzh-cli-quality
    url: git@github.com:gizzahub/gzh-cli-quality.git
    branch: master
```

### Key Commands

| Command                        | Purpose                        |
| ------------------------------ | ------------------------------ |
| `workspace init`               | Create empty .gz-git.yaml      |
| `workspace scan <dir>`         | Scan repos → generate config   |
| `workspace sync`               | Clone/update from config       |
| `workspace status`             | Health check                   |
| `workspace add <url>`          | Add repo to config             |
| `workspace validate`           | Validate config syntax         |

### Makefile Integration

```makefile
GZ ?= gz-git
CONFIG ?= .gz-git.yaml

.PHONY: prepare scan status

prepare:
	@$(GZ) workspace sync -c $(CONFIG)

scan:
	@$(GZ) workspace scan . -o $(CONFIG) --depth 2

status:
	@$(GZ) workspace status -c $(CONFIG)
```

See: `skill:workspace-management` for detailed workspace operations.

---

## Approach 3: Forge Sync + Profiles

Best for: Org-wide synchronization, multiple environments, automatic discovery.

### Quick Start

```bash
# 1. Create profile (one-time setup)
gz-git config profile create work \
  --provider gitlab \
  --base-url https://gitlab.company.com \
  --token ${GITLAB_TOKEN} \
  --clone-proto ssh \
  --ssh-port 2224

# 2. Activate profile
gz-git config profile use work

# 3. Sync entire organization
gz-git sync from-forge --org mygroup --target ~/repos
```

### Profile Config (~/.config/gz-git/profiles/work.yaml)

```yaml
name: work
provider: gitlab
baseURL: https://gitlab.company.com
token: ${WORK_GITLAB_TOKEN}
cloneProto: ssh
sshPort: 2224
parallel: 10
includeSubgroups: true
subgroupMode: flat

sync:
  strategy: reset
  maxRetries: 3
```

### Multi-Environment Setup

```bash
# Work environment
gz-git config profile create work \
  --provider gitlab \
  --base-url https://gitlab.company.com

# Personal environment
gz-git config profile create personal \
  --provider github

# Switch contexts
gz-git config profile use work
gz-git sync from-forge --org backend

gz-git config profile use personal
gz-git sync from-forge --org my-projects
```

### Generate Config from Forge

```bash
# Fetch org repos → generate workspace config
gz-git sync config generate \
  --provider gitlab \
  --org devbox \
  --token $TOKEN \
  -o .gz-git.yaml

# Then use workspace CLI
gz-git workspace sync
```

### Makefile Integration

```makefile
GZ ?= gz-git
PROFILE ?= work
ORG ?= devbox

.PHONY: prepare prepare-forge status

# Profile-based sync
prepare:
	@$(GZ) config profile use $(PROFILE)
	@$(GZ) sync from-forge --org $(ORG) --target .

# Generate config and sync
prepare-forge:
	@$(GZ) sync config generate --org $(ORG) -o .gz-git.yaml
	@$(GZ) workspace sync

status:
	@$(GZ) sync status --target . --depth 2
```

See: `skill:forge-sync`, `skill:config-profiles` for detailed guides.

---

## Choosing the Right Approach

| Scenario                                  | Recommended Approach         |
| ----------------------------------------- | ---------------------------- |
| Fixed 5-10 repos, CI/CD                   | Makefile + clone             |
| Team shares repo list                     | Workspace CLI                |
| Sync entire GitHub/GitLab org             | Forge Sync                   |
| Multiple environments (work/personal)     | Forge Sync + Profiles        |
| Mixed: some manual + some from org        | Forge Sync → config generate → Workspace |

### Combination Pattern

```bash
# Start with Forge Sync to get org repos
gz-git sync config generate --org devbox -o .gz-git.yaml

# Manually add extra repos to config
gz-git workspace add https://github.com/other/repo.git

# Use Workspace CLI for daily operations
gz-git workspace sync
gz-git workspace status
```

---

## Migration Paths

### Shell Loops → Makefile + clone

1. Extract repository list from loop
2. Define `GIT_BASE` and `REPOS` variables
3. Build URL arguments with `$(foreach)`
4. Replace loop with single `gz-git clone` call
5. Test with `--dry-run`

### Makefile → Workspace CLI

1. Run `gz-git workspace scan .` to generate config
2. Review and edit `.gz-git.yaml`
3. Replace Makefile target with `gz-git workspace sync`
4. Share config with team

### Workspace CLI → Forge Sync

1. Create profile: `gz-git config profile create`
2. Test: `gz-git sync from-forge --org myorg --dry-run`
3. Generate config: `gz-git sync config generate -o .gz-git.yaml`
4. Verify: `gz-git workspace sync`

---

## Performance Comparison

| Method         | 8 repos | 20 repos | Notes       |
| -------------- | ------- | -------- | ----------- |
| Shell for-loop | ~40s    | ~100s    | Sequential  |
| gz-git (j=5)   | ~10s    | ~25s     | 5 parallel  |
| gz-git (j=10)  | ~8s     | ~15s     | 10 parallel |

---

## Troubleshooting

### "gz-git: command not found"

```bash
go install github.com/gizzahub/gzh-cli-gitforge/cmd/gz-git@latest
```

### SSH authentication issues

```bash
ssh -T git@github.com          # Test connection
make prepare-https             # Use HTTPS instead
```

### Branch doesn't exist

```bash
gz-git clone --update -b main $(GZ_CLONE_URL_ARGS)
```

### Profile not found

```bash
gz-git config profile list     # Check available profiles
gz-git config init             # Initialize config directory
```

---

## Related Skills

| Skill                  | Purpose                              |
| ---------------------- | ------------------------------------ |
| `gz-git`               | Core CLI reference                   |
| `workspace-management` | Workspace CLI detailed guide         |
| `config-profiles`      | Profile creation and management      |
| `forge-sync`           | GitHub/GitLab/Gitea sync guide       |
| `sync-troubleshooting` | Sync diagnostics and error handling  |
