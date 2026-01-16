---
name: devbox-setup
description: |
  Guide for simplifying DevBox/Monorepo multi-repository clone setup using gz-git.
  Use when:
  - Creating or refactoring Makefile prepare targets for multi-repo projects
  - Migrating from shell for-loops to gz-git clone bulk operations
  - Setting up new DevBox with multiple git repositories
allowed-tools: Bash, Read, Write, Edit
---

# DevBox Multi-Repo Setup with gz-git

This skill guides you to simplify multi-repository clone operations using `gz-git clone`.

## Problem: Complex Shell Loops

Traditional Makefile `prepare` targets often look like this:

```makefile
# BAD: Complex, sequential, error-prone (30+ lines)
prepare:
	@for lib in $(ALL_LIBS); do \
		if [ -d "$$lib/.git" ]; then \
			# complex validation logic...
		fi; \
		# branch handling...
		# clone or update...
		# fallback logic...
	done
```

**Issues**:
- Sequential execution (slow)
- Complex error handling
- Branch logic duplication
- Hard to maintain

## Solution: gz-git clone

```makefile
# GOOD: Simple, parallel, reliable (3 lines)
prepare:
	@echo "Syncing repositories..."
	@$(GZ) clone --update -b $(branch_name) $(GZ_CLONE_URL_ARGS)
	@echo "Done."
```

**Benefits**:
- Parallel execution (5x faster by default)
- Built-in error handling
- Consistent branch management
- Single line of logic

## Makefile Template

### Minimal Setup

```makefile
# Variables
GZ ?= gz-git
branch_name ?= master

# Repository list
REPOS := repo1 repo2 repo3
GIT_BASE := git@github.com:myorg
REPO_URLS := $(addsuffix .git,$(addprefix $(GIT_BASE)/,$(REPOS)))
GZ_CLONE_URL_ARGS = $(foreach u,$(REPO_URLS),--url $(u))

.PHONY: prepare

prepare:
	@echo "Cloning/updating repositories (branch: $(branch_name))..."
	@$(GZ) clone --update -b $(branch_name) $(GZ_CLONE_URL_ARGS)
	@echo "Repository sync complete."
```

### Full Featured Setup

```makefile
# ==============================================================================
# Variables
# ==============================================================================

GZ ?= gz-git
branch_name ?= master

# Repository configuration
GIT_BASE_SSH := git@github.com:myorg
GIT_BASE_HTTPS := https://github.com/myorg

REPOS := project-core project-api project-web project-cli

# Build URL arguments
REPO_URLS_SSH := $(addsuffix .git,$(addprefix $(GIT_BASE_SSH)/,$(REPOS)))
REPO_URLS_HTTPS := $(addsuffix .git,$(addprefix $(GIT_BASE_HTTPS)/,$(REPOS)))

# Default to SSH
GZ_CLONE_URLS ?= $(REPO_URLS_SSH)
GZ_CLONE_URL_ARGS = $(foreach u,$(GZ_CLONE_URLS),--url $(u))

# Parallelism (default: 5)
GZ_PARALLEL ?= 5

.PHONY: prepare prepare-https prepare-dry status

# ==============================================================================
# Repository Management
# ==============================================================================

# Clone/update via SSH (default)
prepare:
	@echo "Syncing repositories via SSH (branch: $(branch_name))..."
	@$(GZ) clone --update -b $(branch_name) -j $(GZ_PARALLEL) $(GZ_CLONE_URL_ARGS)
	@echo "Repository sync complete."
	@$(MAKE) status

# Clone/update via HTTPS
prepare-https:
	@$(MAKE) prepare GZ_CLONE_URLS="$(REPO_URLS_HTTPS)"

# Dry-run to preview actions
prepare-dry:
	@echo "Preview (dry-run):"
	@$(GZ) clone --update -b $(branch_name) --dry-run $(GZ_CLONE_URL_ARGS)

# Check repository status
status:
	@echo ""
	@echo "Repository Status"
	@echo "================="
	@$(GZ) status -d 1 .
```

## gz-git clone Key Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--url URL` | Repository URL (repeatable) | - |
| `--file FILE` | File containing URLs (one per line) | - |
| `--update` | Update if already cloned | false |
| `-b, --branch` | Target branch | default branch |
| `-j, --parallel` | Parallel workers | 5 |
| `--dry-run` | Preview without executing | false |
| `-d, --scan-depth` | Directory scan depth | 1 |

## Migration Checklist

When converting existing Makefile prepare targets:

1. **Identify repository list**
   ```makefile
   # Extract from existing loop
   REPOS := repo1 repo2 repo3
   ```

2. **Define Git base URL**
   ```makefile
   GIT_BASE := git@github.com:myorg
   # or for GitLab:
   GIT_BASE := ssh://git@gitlab.example.com:2224/mygroup
   ```

3. **Build URL arguments**
   ```makefile
   REPO_URLS := $(addsuffix .git,$(addprefix $(GIT_BASE)/,$(REPOS)))
   GZ_CLONE_URL_ARGS = $(foreach u,$(REPO_URLS),--url $(u))
   ```

4. **Replace for-loop with gz-git**
   ```makefile
   prepare:
   	@$(GZ) clone --update -b $(branch_name) $(GZ_CLONE_URL_ARGS)
   ```

5. **Remove fallback logic** - gz-git handles errors internally

6. **Test with dry-run first**
   ```bash
   make prepare-dry
   ```

## Alternative: File-Based URL List

For large projects, use a file instead of Makefile variables:

```makefile
# repos.txt contains one URL per line
prepare:
	@$(GZ) clone --update -b $(branch_name) --file repos.txt
```

**repos.txt**:
```
git@github.com:myorg/repo1.git
git@github.com:myorg/repo2.git
git@github.com:myorg/repo3.git
```

## Real-World Example

From agent-mesh-devbox:

```makefile
GZ ?= gz-git
branch_name ?= master
AGENT_MESH_GIT_BASE ?= ssh://git@gitlab.polypia.net:2224/agent-mesh
AGENT_MESH_REPOS := agent-mesh-cli agent-mesh-connectors \
	agent-mesh-engine agent-mesh-flow-dsl agent-mesh-ops agent-mesh-prompts \
	agent-mesh-saas-backend agent-mesh-webui
AGENT_MESH_REPO_URLS := $(addsuffix .git,$(addprefix $(AGENT_MESH_GIT_BASE)/,$(AGENT_MESH_REPOS)))
GZ_CLONE_URL_ARGS = $(foreach u,$(AGENT_MESH_REPO_URLS),--url $(u))

prepare:
	@echo "Syncing Agent Mesh repositories (branch: $(branch_name))..."
	@$(GZ) clone --update -b $(branch_name) $(GZ_CLONE_URL_ARGS)
	@echo "Repository sync complete."
```

**Result**: 8 repositories cloned/updated in parallel with 3 lines of logic.

## Performance Comparison

| Method | 8 repos | 20 repos | Notes |
|--------|---------|----------|-------|
| Shell for-loop | ~40s | ~100s | Sequential |
| gz-git (j=5) | ~10s | ~25s | 5 parallel |
| gz-git (j=10) | ~8s | ~15s | 10 parallel |

## Troubleshooting

### "gz-git: command not found"
```bash
# Install gz-git
go install github.com/gizzahub/gzh-cli-gitforge/cmd/gz-git@latest
```

### SSH authentication issues
```bash
# Test SSH connection first
ssh -T git@github.com

# Or use HTTPS
make prepare-https
```

### Branch doesn't exist
```bash
# gz-git will report which repos lack the branch
# Use default branch or check branch existence first
gz-git clone --update -b main $(GZ_CLONE_URL_ARGS)
```
