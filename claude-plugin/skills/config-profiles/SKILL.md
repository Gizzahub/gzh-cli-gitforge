---
name: config-profiles
description: |
  Guide for managing gz-git configuration profiles and multi-environment settings.
  Use when:
  - Setting up work/personal environment profiles
  - Switching between GitLab/GitHub environments
  - Understanding 5-layer config precedence
  - Creating project-specific or global configs
  - Managing hierarchical workspace configurations
allowed-tools: Bash, Read, Write, Edit, Grep
---

# Configuration Profiles with gz-git

This skill covers profile-based configuration management for multi-environment workflows.

## Core Concept: 5-Layer Precedence

gz-git resolves configuration from 5 layers (highest → lowest):

```
┌─────────────────────────────────────────┐
│ 1. Command flags (--provider gitlab)    │  ← Highest priority
├─────────────────────────────────────────┤
│ 2. Project config (.gz-git.yaml)        │
├─────────────────────────────────────────┤
│ 3. Active profile (~/.config/gz-git/    │
│    profiles/{active}.yaml)              │
├─────────────────────────────────────────┤
│ 4. Global config (~/.config/gz-git/     │
│    config.yaml)                         │
├─────────────────────────────────────────┤
│ 5. Built-in defaults                    │  ← Lowest priority
└─────────────────────────────────────────┘
```

**Rule**: Higher layers override lower layers. Unset values fall through.

---

## File Locations

```
~/.config/gz-git/
├── config.yaml              # Global config
├── profiles/
│   ├── default.yaml        # Default profile
│   ├── work.yaml           # User profiles
│   └── personal.yaml
└── state/
    └── active-profile.txt  # Currently active profile name

# Project configs (auto-detected, walks up directory tree)
~/myproject/.gz-git.yaml
~/mydevbox/.gz-git.yaml
```

---

## Profile Commands

### Create Profile

```bash
# Interactive creation (recommended)
gz-git config profile create work

# Create with all settings
gz-git config profile create work \
  --provider gitlab \
  --base-url https://gitlab.company.com \
  --token ${WORK_GITLAB_TOKEN} \
  --clone-proto ssh \
  --ssh-port 2224 \
  --parallel 10 \
  --include-subgroups \
  --subgroup-mode flat
```

### List Profiles

```bash
gz-git config profile list

# Output:
# Profiles:
#   default
#   work (active)
#   personal
```

### Switch Profile

```bash
# Set active profile
gz-git config profile use work

# One-time override (doesn't change active)
gz-git sync from-forge --profile personal --org my-repos
```

### Show Profile

```bash
# Show specific profile
gz-git config profile show work

# Show active profile
gz-git config profile show
```

### Delete Profile

```bash
gz-git config profile delete old-profile
```

---

## Config Commands

### Show Configuration

```bash
# Show project config (if exists)
gz-git config show

# Show effective config with sources
gz-git config show --effective

# Output:
# provider: gitlab (source: profile:work)
# baseURL: https://gitlab.company.com (source: profile:work)
# parallel: 5 (source: project)
# cloneProto: ssh (source: default)
```

### Config Hierarchy

```bash
# Show config hierarchy tree
gz-git config hierarchy

# Output:
# Config Hierarchy:
# ├── Flag overrides: (none)
# ├── Project: ~/myproject/.gz-git.yaml
# │   └── profile: work
# ├── Active Profile: ~/.config/gz-git/profiles/work.yaml
# │   ├── provider: gitlab
# │   └── baseURL: https://gitlab.company.com
# └── Global: ~/.config/gz-git/config.yaml
#     └── activeProfile: work
```

---

## Profile File Format

```yaml
# ~/.config/gz-git/profiles/work.yaml
name: work

# Forge provider settings
provider: gitlab
baseURL: https://gitlab.company.com
token: ${WORK_GITLAB_TOKEN}        # Environment variable!

# Clone settings
cloneProto: ssh
sshPort: 2224                       # Custom SSH port

# Bulk operation settings
parallel: 10
includeSubgroups: true
subgroupMode: flat                  # flat | nested

# Command-specific overrides
sync:
  strategy: reset
  maxRetries: 3
  timeout: 60s

branch:
  defaultBranch: main
  protectedBranches:
    - main
    - develop
    - release/*

fetch:
  allRemotes: true
  prune: true

pull:
  rebase: true
  ffOnly: false

push:
  setUpstream: true
```

---

## Project Config Format

Project config (`.gz-git.yaml`) in a project directory:

```yaml
# ~/myproject/.gz-git.yaml

# Use work profile for this project
profile: work

# Override sync settings for this project
sync:
  strategy: pull      # Override profile's reset → pull
  parallel: 3         # Lower parallelism

# Project metadata
metadata:
  team: backend
  repository: https://gitlab.company.com/backend/myproject
```

---

## Global Config Format

```yaml
# ~/.config/gz-git/config.yaml

# Default active profile
activeProfile: work

# Global defaults (apply to all profiles)
defaults:
  parallel: 5
  cloneProto: ssh
  format: default

# Named environments (for token management)
environments:
  work:
    gitlabToken: ${WORK_GITLAB_TOKEN}
  personal:
    githubToken: ${PERSONAL_GITHUB_TOKEN}
```

---

## Environment Variable Expansion

Config files support `${VAR_NAME}` syntax for sensitive values:

```yaml
# Good: Use environment variables
token: ${GITLAB_TOKEN}
baseURL: ${GITLAB_URL}

# Bad: Plain text tokens (security risk!)
token: glpat-xxxxxxxxxxxx
```

**Security Notes**:
- Profile files: 0600 permissions (user read/write only)
- Config directory: 0700 permissions (user access only)
- Only `${VAR}` expansion (no shell command execution)

---

## Hierarchical Config (Advanced)

For complex workstation/workspace/project structures:

```yaml
# ~/.gz-git.yaml (workstation level)
profile: polypia
parallel: 10

# Inline profiles (no external file needed!)
profiles:
  polypia:
    provider: gitlab
    baseURL: https://gitlab.polypia.net
    token: ${GITLAB_POLYPIA_TOKEN}
    cloneProto: ssh

  github-personal:
    provider: github
    token: ${GITHUB_TOKEN}

# Named workspaces
workspaces:
  devbox:
    path: ~/mydevbox
    type: config              # Has nested .gz-git.yaml
    source:
      provider: gitlab
      org: devbox
      includeSubgroups: true
      subgroupMode: flat
    sync:
      strategy: pull

  personal:
    path: ~/personal
    type: git                 # Plain git repos
    profile: github-personal
```

### Workspace Types

| Type     | Description                          |
| -------- | ------------------------------------ |
| `forge`  | Sync from GitLab/GitHub/Gitea API    |
| `git`    | Plain git repository (leaf node)     |
| `config` | Has nested config file (recursive)   |

### Parent Config Reference

```yaml
# ~/mydevbox/.gz-git.yaml
parent: ~/devenv/workstation/.gz-git.yaml  # Explicit parent

# Profile lookup: current → parent → global
profile: polypia  # Found in parent's inline profiles
```

---

## Common Workflows

### Initial Setup (New Machine)

```bash
# 1. Create work profile
gz-git config profile create work \
  --provider gitlab \
  --base-url https://gitlab.company.com \
  --token ${WORK_GITLAB_TOKEN} \
  --clone-proto ssh

# 2. Create personal profile
gz-git config profile create personal \
  --provider github \
  --token ${GITHUB_TOKEN}

# 3. Set default
gz-git config profile use work

# 4. Verify
gz-git config profile list
gz-git config show --effective
```

### Multi-Environment Switching

```bash
# Morning: Work mode
gz-git config profile use work
gz-git sync from-forge --org backend

# Evening: Personal mode
gz-git config profile use personal
gz-git sync from-forge --org my-repos

# One-time override
gz-git sync from-forge --profile work --org urgent-project
```

### Project-Specific Override

```bash
# In project directory
cd ~/myproject

# Create project config
cat > .gz-git.yaml << 'EOF'
profile: work
sync:
  strategy: pull    # Keep local changes
  parallel: 3       # Lower parallelism
metadata:
  team: backend
EOF

# Now gz-git uses project settings automatically
gz-git workspace sync
```

---

## Troubleshooting

### "profile not found"

```bash
# Check available profiles
gz-git config profile list

# Create if missing
gz-git config profile create myprofile
```

### "token not set"

```bash
# Check environment variable
echo $GITLAB_TOKEN

# Export if missing
export GITLAB_TOKEN="glpat-xxxx"

# Or use --token flag
gz-git sync from-forge --token "glpat-xxxx" --org myorg
```

### "which config is being used?"

```bash
# Show effective config with sources
gz-git config show --effective

# Show full hierarchy
gz-git config hierarchy
```

### Permission errors on config files

```bash
# Fix permissions
chmod 700 ~/.config/gz-git
chmod 600 ~/.config/gz-git/profiles/*
chmod 600 ~/.config/gz-git/config.yaml
```

---

## Quick Reference

| Task                        | Command                                |
| --------------------------- | -------------------------------------- |
| Create profile              | `gz-git config profile create NAME`    |
| List profiles               | `gz-git config profile list`           |
| Switch profile              | `gz-git config profile use NAME`       |
| Show profile                | `gz-git config profile show [NAME]`    |
| Delete profile              | `gz-git config profile delete NAME`    |
| Show effective config       | `gz-git config show --effective`       |
| Show hierarchy              | `gz-git config hierarchy`              |
| One-time override           | `--profile NAME` flag on any command   |

---

## workspace vs config profile

| Feature           | `workspace` commands    | `config profile`         |
| ----------------- | ----------------------- | ------------------------ |
| Config format     | `repositories` array    | `workspaces` map         |
| Hierarchy         | Single level            | Recursive nesting        |
| Forge sync        | Manual (add URLs)       | API-based (org sync)     |
| Profile support   | No                      | Yes                      |
| Use case          | Simple local repo list  | Multi-env, complex setup |
