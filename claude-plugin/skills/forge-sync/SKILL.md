---
name: forge-sync
description: |
  Guide for syncing repositories from Git forges (GitHub, GitLab, Gitea).
  Use when:
  - Cloning entire GitHub/GitLab/Gitea organization or group
  - Setting up new development environment from forge
  - Generating config from forge API
  - Checking repository health and sync status
  - Managing self-hosted GitLab with custom SSH ports
allowed-tools: Bash, Read, Write, Edit, Grep
---

# Forge Sync with gz-git

This skill covers synchronization from Git forges (GitHub, GitLab, Gitea) using the `gz-git sync` commands.

## Overview

| Command              | Purpose                                    |
| -------------------- | ------------------------------------------ |
| `sync from-forge`    | Clone/update repos directly from forge API |
| `sync config generate` | Generate config file from forge API      |
| `sync status`        | Check repository health and divergence     |
| `sync setup`         | Interactive setup wizard                   |

**Key Difference**:
- `sync from-forge`: One-off direct sync (no config file)
- `sync config generate` + `workspace sync`: Config-based workflow

---

## Supported Providers

| Provider | API URL                    | Features                        |
| -------- | -------------------------- | ------------------------------- |
| GitHub   | api.github.com             | Orgs, users, SSH/HTTPS          |
| GitLab   | gitlab.com/api/v4          | Groups, subgroups, SSH port     |
| Gitea    | (custom)/api/v1            | Orgs, SSH/HTTPS                 |

---

## Quick Start

### GitHub Organization

```bash
# Clone entire org (SSH, default)
gz-git sync from-forge \
  --provider github \
  --org myorg \
  --target ~/repos \
  --token $GITHUB_TOKEN

# Include forks and archived repos
gz-git sync from-forge \
  --provider github \
  --org myorg \
  --target ~/repos \
  --token $GITHUB_TOKEN \
  --include-forks \
  --include-archived
```

### GitLab Group

```bash
# Basic GitLab sync
gz-git sync from-forge \
  --provider gitlab \
  --org mygroup \
  --target ~/repos \
  --token $GITLAB_TOKEN

# With subgroups (flat naming: parent-child-repo)
gz-git sync from-forge \
  --provider gitlab \
  --org parent-group \
  --target ~/repos \
  --token $GITLAB_TOKEN \
  --include-subgroups \
  --subgroup-mode flat

# With subgroups (nested directories: parent/child/repo)
gz-git sync from-forge \
  --provider gitlab \
  --org parent-group \
  --target ~/repos \
  --token $GITLAB_TOKEN \
  --include-subgroups \
  --subgroup-mode nested
```

### Self-Hosted GitLab

```bash
# Custom base URL + SSH port
gz-git sync from-forge \
  --provider gitlab \
  --org mygroup \
  --target ~/repos \
  --base-url https://gitlab.company.com \
  --token $GITLAB_TOKEN \
  --clone-proto ssh \
  --ssh-port 2224
```

### Gitea

```bash
gz-git sync from-forge \
  --provider gitea \
  --org myorg \
  --target ~/repos \
  --base-url https://gitea.company.com \
  --token $GITEA_TOKEN
```

---

## from-forge Flags Reference

### Required Flags

| Flag         | Description                          |
| ------------ | ------------------------------------ |
| `--provider` | `github`, `gitlab`, or `gitea`       |
| `--org`      | Organization or group name           |
| `--target`   | Local directory for cloned repos     |
| `--token`    | API token for authentication         |

### Optional Flags

| Flag                  | Default   | Description                           |
| --------------------- | --------- | ------------------------------------- |
| `--base-url`          | (default) | API endpoint for self-hosted          |
| `--clone-proto`       | `ssh`     | Clone protocol: `ssh` or `https`      |
| `--ssh-port`          | `22`      | Custom SSH port                       |
| `--ssh-key`           | -         | SSH private key file path             |
| `--include-subgroups` | `false`   | Include GitLab subgroups              |
| `--subgroup-mode`     | `flat`    | `flat` or `nested` directory layout   |
| `--include-forks`     | `false`   | Include forked repos                  |
| `--include-archived`  | `false`   | Include archived repos                |
| `--include-private`   | `true`    | Include private repos                 |
| `--strategy`          | `reset`   | Sync strategy: `reset`, `pull`, `fetch` |
| `--parallel`          | `4`       | Parallel workers                      |
| `--max-retries`       | `3`       | Retry attempts on failure             |
| `--cleanup-orphans`   | `false`   | Delete repos not in org               |
| `--dry-run`           | `false`   | Preview without executing             |
| `--user`              | `false`   | Treat `--org` as username             |

---

## Sync Strategies

| Strategy | Behavior                                         |
| -------- | ------------------------------------------------ |
| `reset`  | Hard reset to remote HEAD (discard local)        |
| `pull`   | Pull with merge (preserve local changes)         |
| `fetch`  | Fetch only (no checkout, update refs)            |

```bash
# Safe sync (preserve local changes)
gz-git sync from-forge --strategy pull ...

# Force sync (discard local changes)
gz-git sync from-forge --strategy reset ...

# Fetch only (manual merge later)
gz-git sync from-forge --strategy fetch ...
```

---

## Subgroup Modes (GitLab Only)

### Flat Mode (Default)

Subgroup names joined with dash:

```
parent-group/
├── parent-repo/
├── child-group-repo1/      # child-group/repo1 → child-group-repo1
└── child-group-repo2/
```

```bash
gz-git sync from-forge --include-subgroups --subgroup-mode flat ...
```

### Nested Mode

Preserves directory hierarchy:

```
parent-group/
├── parent-repo/
└── child-group/
    ├── repo1/
    └── repo2/
```

```bash
gz-git sync from-forge --include-subgroups --subgroup-mode nested ...
```

---

## Config Generation

Generate `.gz-git.yaml` from forge API for `workspace sync`:

```bash
# Generate config from GitLab
gz-git sync config generate \
  --provider gitlab \
  --org devbox \
  --target ~/repos \
  --token $GITLAB_TOKEN \
  -o .gz-git.yaml

# With subgroups
gz-git sync config generate \
  --provider gitlab \
  --org parent-group \
  --target ~/repos \
  --token $GITLAB_TOKEN \
  --include-subgroups \
  --subgroup-mode flat \
  -o .gz-git.yaml

# Then use workspace commands
gz-git workspace sync
gz-git workspace status
```

### Generated Config Example

```yaml
version: 1
kind: workspace

metadata:
  name: devbox
  generatedFrom: gitlab:devbox

strategy: reset
parallel: 4

repositories:
  - name: project-core
    url: ssh://git@gitlab.company.com:2224/devbox/project-core.git
    path: project-core

  - name: project-api
    url: ssh://git@gitlab.company.com:2224/devbox/project-api.git
    path: project-api
```

---

## Sync Status

Check repository health and sync status:

```bash
# Basic status check
gz-git sync status --target ~/repos

# From config file
gz-git sync status -c .gz-git.yaml

# Skip remote fetch (faster, uses cached data)
gz-git sync status --skip-fetch

# Custom timeout (slow network)
gz-git sync status --timeout 60s

# Verbose output
gz-git sync status --verbose

# JSON output for scripting
gz-git sync status -f json
```

### Status Output

```
Checking repository health...

✓ project-core (main)           healthy     up-to-date
⚠ project-api (develop)         warning     3↓ 2↑ diverged
  → Diverged: 2 ahead, 3 behind. Use 'git pull --rebase'
✗ project-web (feature)         error       dirty + 5↓ behind
  → Commit or stash 3 modified files, then pull
⊘ project-old (master)          timeout     fetch failed (30s)
  → Check network connection

Summary: 1 healthy, 1 warning, 1 error, 1 timeout (4 total)
```

### Health Status Icons

| Icon | Status      | Meaning                          |
| ---- | ----------- | -------------------------------- |
| ✓    | healthy     | Up-to-date, clean                |
| ⚠    | warning     | Diverged, behind, or ahead       |
| ✗    | error       | Conflicts, dirty + behind        |
| ⊘    | unreachable | Network timeout, auth failed     |

---

## Using Profiles

Combine with config profiles to avoid repeating flags:

```bash
# One-time setup
gz-git config profile create work \
  --provider gitlab \
  --base-url https://gitlab.company.com \
  --token ${WORK_GITLAB_TOKEN} \
  --ssh-port 2224

gz-git config profile use work

# Now sync without flags
gz-git sync from-forge --org devbox --target ~/repos
```

---

## Common Workflows

### New Environment Setup

```bash
# 1. Sync from forge
gz-git sync from-forge \
  --provider gitlab \
  --org devbox \
  --target ~/mydevbox \
  --token $GITLAB_TOKEN \
  --include-subgroups

# 2. Generate config for future syncs
gz-git sync config generate \
  --provider gitlab \
  --org devbox \
  --target ~/mydevbox \
  --token $GITLAB_TOKEN \
  -o ~/mydevbox/.gz-git.yaml

# 3. Future syncs via workspace
cd ~/mydevbox
gz-git workspace sync
```

### Daily Sync

```bash
# Quick status check
gz-git sync status --target ~/repos

# Full sync
gz-git sync from-forge --org devbox --target ~/repos

# Or via workspace
gz-git workspace sync
```

### Dry Run Preview

```bash
# See what would happen
gz-git sync from-forge \
  --provider gitlab \
  --org devbox \
  --target ~/repos \
  --dry-run

# Output:
# [DRY-RUN] Would clone: project-new
# [DRY-RUN] Would update: project-core (3 commits behind)
# [DRY-RUN] Would skip: project-api (up-to-date)
```

---

## Token Setup

### GitHub

```bash
# Personal Access Token (classic)
# Scopes needed: repo, read:org
export GITHUB_TOKEN="ghp_xxxxxxxxxxxx"

# Fine-grained PAT
# Permissions: Repository access, Organization read
```

### GitLab

```bash
# Personal Access Token
# Scopes: api, read_repository
export GITLAB_TOKEN="glpat-xxxxxxxxxxxx"

# Group Access Token (for specific group)
# Scopes: api, read_repository
```

### Gitea

```bash
# Application Token
export GITEA_TOKEN="xxxxxxxxxxxxxxxx"
```

---

## Troubleshooting

### "authentication failed"

```bash
# Check token
echo $GITLAB_TOKEN

# Test API manually
curl -H "PRIVATE-TOKEN: $GITLAB_TOKEN" \
  "https://gitlab.company.com/api/v4/groups/mygroup"

# Check token scopes (needs: api, read_repository)
```

### "SSH connection refused"

```bash
# Check SSH port
ssh -T -p 2224 git@gitlab.company.com

# Use correct port flag
gz-git sync from-forge --ssh-port 2224 ...

# Or use HTTPS
gz-git sync from-forge --clone-proto https ...
```

### "subgroups not included"

```bash
# Add --include-subgroups flag
gz-git sync from-forge --include-subgroups ...

# Check token has access to subgroups
```

### "timeout during fetch"

```bash
# Increase timeout
gz-git sync status --timeout 120s

# Or skip fetch for quick check
gz-git sync status --skip-fetch
```

### "repos in wrong directory structure"

```bash
# Check subgroup-mode
# flat: parent-child-repo (default)
# nested: parent/child/repo

gz-git sync from-forge --subgroup-mode nested ...
```

---

## Quick Reference

| Task                    | Command                                           |
| ----------------------- | ------------------------------------------------- |
| Sync GitHub org         | `sync from-forge --provider github --org NAME`    |
| Sync GitLab group       | `sync from-forge --provider gitlab --org NAME`    |
| Include subgroups       | `--include-subgroups --subgroup-mode flat`        |
| Self-hosted GitLab      | `--base-url URL --ssh-port PORT`                  |
| Generate config         | `sync config generate -o .gz-git.yaml`            |
| Check status            | `sync status --target DIR`                        |
| Dry run                 | `--dry-run`                                       |
| Preserve local changes  | `--strategy pull`                                 |
| Use profile             | `--profile work`                                  |

---

## forge-sync vs workspace

| Feature          | `sync from-forge`          | `workspace sync`           |
| ---------------- | -------------------------- | -------------------------- |
| Source           | Forge API (live)           | Config file (static)       |
| New repos        | Auto-discovered            | Manual add to config       |
| Use case         | Initial sync, refresh      | Daily sync, stable list    |
| Flags            | All forge options          | Config-defined             |
| Profile support  | Yes                        | Yes                        |

**Recommended Workflow**:
1. `sync from-forge` for initial clone
2. `sync config generate` to create config
3. `workspace sync` for daily operations
