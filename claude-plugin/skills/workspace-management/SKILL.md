---
name: workspace-management
description: |
  Guide for managing local workspaces with gz-git workspace CLI.
  Use when:
  - Creating or managing .gz-git.yaml config files
  - Scanning local directories to generate repo config
  - Syncing multiple repos from config file
  - Checking workspace health and status
  - Adding repos to existing workspace config
allowed-tools: Bash, Read, Write, Edit, Grep
---

# Workspace Management with gz-git

This skill covers local config-based multi-repository management using `gz-git workspace` commands.

## Overview

**Workspace CLI** manages local git repositories through a simple YAML config file (`.gz-git.yaml`).

| Command    | Purpose                               |
| ---------- | ------------------------------------- |
| `init`     | Create empty config file              |
| `scan`     | Scan directory → generate config      |
| `sync`     | Clone/update repos from config        |
| `status`   | Check workspace health                |
| `add`      | Add repo to config                    |
| `validate` | Validate config file                  |

**Key Difference**: Workspace CLI uses `repositories` array for simple local management.
For hierarchical multi-level config or Forge API sync, use `gz-git config` or `gz-git sync from-forge`.

---

## Config File Format (.gz-git.yaml)

```yaml
# .gz-git.yaml
version: 1
kind: workspace

metadata:
  name: my-workspace
  description: My development workspace

# Sync settings (optional)
strategy: reset      # reset | pull | skip
parallel: 10         # concurrent workers
maxRetries: 3        # retry failed ops
cleanupOrphans: false

# Repository list
repositories:
  - name: project-core
    url: git@github.com:myorg/project-core.git
    path: project-core           # optional: defaults to name
    strategy: pull               # optional: per-repo override

  - name: project-api
    url: git@github.com:myorg/project-api.git
    description: Backend API     # optional: human-readable
    enabled: true                # optional: set false to skip

  - name: project-web
    url: git@github.com:myorg/project-web.git
    additionalRemotes:           # optional: extra remotes
      upstream: git@github.com:upstream/project-web.git
```

### Repository Fields

| Field               | Required | Default | Description                    |
| ------------------- | -------- | ------- | ------------------------------ |
| `name`              | Yes      | -       | Repository identifier          |
| `url`               | Yes      | -       | Primary clone URL              |
| `path`              | No       | `name`  | Local directory path           |
| `description`       | No       | -       | Human-readable description     |
| `strategy`          | No       | global  | Sync strategy override         |
| `enabled`           | No       | `true`  | Include in sync                |
| `additionalRemotes` | No       | -       | Extra git remotes (name: url)  |
| `assumePresent`     | No       | `false` | Skip clone check (trust local) |

### Sync Strategies

| Strategy | Behavior                                      |
| -------- | --------------------------------------------- |
| `reset`  | Hard reset to remote (discard local changes)  |
| `pull`   | Pull with merge (preserve local changes)      |
| `skip`   | Skip sync (useful for submodules)             |

---

## Commands

### 1. `workspace init` - Create Empty Config

```bash
# Create .gz-git.yaml in current directory
gz-git workspace init

# Custom filename
gz-git workspace init -c myworkspace.yaml

# With metadata
gz-git workspace init --name "My Devbox" --description "Development workspace"
```

**Output**: Empty config file ready for manual editing or `workspace add`.

---

### 2. `workspace scan` - Scan Directory → Config

```bash
# Scan current directory (depth=1)
gz-git workspace scan

# Scan specific directory with depth
gz-git workspace scan ~/mydevbox --depth 2

# Output to specific file
gz-git workspace scan ~/mydevbox -o .gz-git.yaml

# Exclude patterns
gz-git workspace scan ~/mydevbox --exclude "vendor,tmp,node_modules"

# Include only matching patterns
gz-git workspace scan ~/mydevbox --include "gzh-cli-*"
```

**Workflow**: Scan first, then edit generated config to customize.

---

### 3. `workspace sync` - Clone/Update from Config

```bash
# Sync using .gz-git.yaml in current directory
gz-git workspace sync

# Sync with custom config
gz-git workspace sync -c myworkspace.yaml

# Dry-run (preview actions)
gz-git workspace sync --dry-run

# Override parallel workers
gz-git workspace sync -j 5

# Force strategy for all repos
gz-git workspace sync --strategy reset
```

**Actions**:
- **Missing repos**: Clone from URL
- **Existing repos**: Pull/reset based on strategy
- **Orphans**: Optionally remove (if `cleanupOrphans: true`)

---

### 4. `workspace status` - Check Health

```bash
# Check workspace health
gz-git workspace status

# Verbose output (show branches, remotes)
gz-git workspace status --verbose

# JSON output for scripting
gz-git workspace status --format json
```

**Output**:
```
Workspace Status
================
✓ project-core     (main)     up-to-date
⚠ project-api      (develop)  3↓ behind
✗ project-web      (feature)  dirty + 2↑ ahead

Summary: 1 healthy, 1 warning, 1 dirty (3 total)
```

---

### 5. `workspace add` - Add Repo to Config

```bash
# Add by URL
gz-git workspace add git@github.com:myorg/new-repo.git

# Add with custom name and path
gz-git workspace add git@github.com:myorg/new-repo.git \
  --name my-repo \
  --path libs/my-repo

# Add current directory's repo to config
gz-git workspace add --from-current

# Add to specific config file
gz-git workspace add git@github.com:myorg/repo.git -c myworkspace.yaml
```

---

### 6. `workspace validate` - Validate Config

```bash
# Validate .gz-git.yaml
gz-git workspace validate

# Validate specific file
gz-git workspace validate -c myworkspace.yaml

# Check URLs are reachable (slow)
gz-git workspace validate --check-urls
```

**Checks**:
- YAML syntax
- Required fields (name, url)
- Duplicate names/paths
- URL format validity

---

## Common Workflows

### New Project Setup

```bash
# 1. Create config
gz-git workspace init --name "My Project"

# 2. Add repositories
gz-git workspace add git@github.com:myorg/core.git
gz-git workspace add git@github.com:myorg/api.git
gz-git workspace add git@github.com:myorg/web.git

# 3. Clone all
gz-git workspace sync
```

### Existing Directory → Config

```bash
# 1. Scan existing repos
gz-git workspace scan ~/mydevbox -o .gz-git.yaml

# 2. Review and edit
$EDITOR .gz-git.yaml

# 3. Validate
gz-git workspace validate

# 4. Future syncs
gz-git workspace sync
```

### Daily Development

```bash
# Morning: sync all repos
gz-git workspace sync

# Check status
gz-git workspace status

# Add new dependency
gz-git workspace add git@github.com:vendor/lib.git --path vendor/lib
gz-git workspace sync
```

---

## Config Examples

### Minimal

```yaml
repositories:
  - name: my-project
    url: git@github.com:user/my-project.git
```

### With Metadata

```yaml
version: 1
kind: workspace

metadata:
  name: backend-services
  description: Backend microservices workspace
  team: backend

strategy: pull
parallel: 5

repositories:
  - name: auth-service
    url: git@github.com:myorg/auth-service.git
    description: Authentication and authorization

  - name: api-gateway
    url: git@github.com:myorg/api-gateway.git
    description: API gateway and routing
```

### Multi-Remote Setup

```yaml
repositories:
  - name: forked-lib
    url: git@github.com:myorg/forked-lib.git
    additionalRemotes:
      upstream: git@github.com:original/lib.git
      backup: git@gitlab.com:myorg/forked-lib.git
```

### Mixed Strategies

```yaml
strategy: reset  # global default

repositories:
  - name: core
    url: git@github.com:myorg/core.git
    # uses global reset

  - name: local-changes
    url: git@github.com:myorg/local-changes.git
    strategy: pull  # preserve local changes

  - name: submodule
    url: git@github.com:vendor/lib.git
    strategy: skip  # managed separately
```

---

## Troubleshooting

### "config file not found"

```bash
# Check current directory
ls -la .gz-git.yaml

# Create if missing
gz-git workspace init

# Or specify path
gz-git workspace sync -c path/to/.gz-git.yaml
```

### "repository already exists"

Workspace sync detects existing repos. Use strategy to control behavior:
- `reset`: Discard local, match remote
- `pull`: Merge remote changes
- `skip`: Leave unchanged

### "authentication failed"

```bash
# Test SSH connection
ssh -T git@github.com

# Or use HTTPS URLs in config
url: https://github.com/myorg/repo.git
```

### "duplicate name/path"

Each repository must have unique `name` and `path`. Check config for duplicates:

```bash
grep -E "^\s+name:" .gz-git.yaml | sort | uniq -d
grep -E "^\s+path:" .gz-git.yaml | sort | uniq -d
```

---

## Workspace vs Sync vs Config

| Command          | Purpose                          | Config Format      |
| ---------------- | -------------------------------- | ------------------ |
| `workspace`      | Local config management          | `repositories` 배열 |
| `sync from-forge`| Forge API → local clone          | CLI flags          |
| `config profile` | Profile-based settings           | `workspaces` map   |

**Rule of Thumb**:
- Simple local repo list → `workspace`
- Forge org sync → `sync from-forge`
- Multi-environment profiles → `config profile`
