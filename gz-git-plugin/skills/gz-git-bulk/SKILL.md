---
name: gz-git-bulk
description: |
  gz-git bulk operations for multi-repository workflows. Use when:
  - Managing multiple Git repositories in a directory
  - Cloning from GitHub/GitLab organizations
  - Bulk fetch/pull/push across repos
  - Parallel repository operations
allowed-tools: Bash, Read, Grep, Glob
---

# gz-git Bulk Operations Reference

gz-git provides bulk operations for managing multiple repositories efficiently.

## Quick Detection

```bash
# Check if directory contains multiple repos
find . -maxdepth 2 -name ".git" -type d | head -5
```

## Bulk Mode Overview

When a **directory** is passed instead of running in a single repo, gz-git operates in **bulk mode**:
- Scans for Git repositories in the directory
- Executes operations in parallel
- Reports aggregated results

## Common Bulk Flags

All bulk operations support these flags:

```
-d, --scan-depth INT   Directory scan depth (default: 1)
-j, --parallel INT     Parallel workers (default: 5)
-n, --dry-run          Simulation mode (show what would happen)
--include REGEX        Include repos matching pattern
--exclude REGEX        Exclude repos matching pattern
```

## Bulk Clone Operations

| Command | Description |
|---------|-------------|
| `gz-git clone --org ORG` | Clone all repos from org |
| `gz-git clone --manifest FILE` | Clone from manifest |

```bash
# Clone from GitHub organization
gz-git clone --org mycompany --provider github
gz-git clone --org mycompany --provider github --include "^api-"
gz-git clone --org mycompany --provider github --exclude "archived"

# Clone from GitLab group
gz-git clone --org mygroup --provider gitlab

# Clone from manifest file
gz-git clone --manifest repos.yml
gz-git clone --manifest repos.yml --parallel 10

# Dry run (see what would be cloned)
gz-git clone --org mycompany --dry-run
```

**Clone Flags:**
- `--org, -o` : Organization/group name
- `--provider, -p` : git forge provider (github, gitlab)
- `--manifest, -m` : Manifest file path
- `--branch, -b` : Default branch to checkout
- `--depth INT` : Shallow clone depth
- `--ssh` : Use SSH URLs (default: HTTPS)

## Bulk Fetch/Pull/Push

| Command | Description |
|---------|-------------|
| `gz-git fetch [DIR]` | Fetch all repos in directory |
| `gz-git pull [DIR]` | Pull all repos in directory |
| `gz-git push [DIR]` | Push all repos in directory |

```bash
# Fetch all repos in current directory
gz-git fetch .
gz-git fetch . --parallel 10

# Fetch repos in specific directory
gz-git fetch ~/projects

# With filters
gz-git fetch . --include "^gzh-cli"
gz-git fetch . --exclude "archived"

# Pull all repos
gz-git pull .
gz-git pull . --rebase                   # Pull with rebase
gz-git pull . --ff-only                  # Fast-forward only

# Push all repos
gz-git push .
gz-git push . --dry-run                  # Check first
gz-git push . --force-with-lease         # Force push safely
```

## Bulk Status

```bash
# Status of all repos
gz-git status .
gz-git status . --short                  # One-line per repo
gz-git status . --dirty-only             # Only show dirty repos
gz-git status . --behind                 # Show repos behind remote

# Deep scan
gz-git status . --scan-depth 2           # Scan 2 levels deep
```

**Status Output Columns:**
- Repository name
- Current branch
- Dirty state (modified files)
- Ahead/behind remote count

## Bulk Switch

```bash
# Switch all repos to a branch
gz-git switch main .
gz-git switch develop .

# Create branch if not exists
gz-git switch feature/new . --create

# With filters
gz-git switch main . --include "^frontend"
```

## Bulk Diff

```bash
# Diff all repos
gz-git diff .
gz-git diff . --stat                     # Stats only
gz-git diff . --name-only                # File names only

# Staged changes
gz-git diff . --staged
```

## Bulk Commit

```bash
# Commit in all dirty repos
gz-git commit . auto
gz-git commit . auto --type chore
gz-git commit . auto --scope deps
```

## Bulk Branch Cleanup

```bash
# Clean merged branches in all repos
gz-git branch cleanup .
gz-git branch cleanup . --gone           # Clean gone branches
gz-git branch cleanup . --dry-run        # Preview cleanup
```

## Bulk Stash

```bash
# Stash in all dirty repos
gz-git stash save . "WIP: bulk save"
gz-git stash list .
gz-git stash pop .
```

## Bulk Tag

```bash
# List tags across repos
gz-git tag list .

# Create same tag in all repos
gz-git tag create v1.0.0 .
gz-git tag create v1.0.0 . -m "Release"

# Push tags
gz-git tag push .
```

## Manifest File Format

```yaml
# repos.yml
repositories:
  - url: https://github.com/org/repo1
    path: repo1
    branch: main
  - url: https://github.com/org/repo2
    path: services/repo2
    branch: develop
  - url: git@github.com:org/repo3.git
    path: repo3
```

## Common Workflows

### Multi-Repo Project Setup
```bash
# Clone all organization repos
gz-git clone --org mycompany --provider github ~/work/mycompany

# Check status
gz-git status ~/work/mycompany
```

### Daily Sync
```bash
# Fetch and pull all repos
gz-git fetch .
gz-git pull .

# Check for dirty repos
gz-git status . --dirty-only
```

### Bulk Update
```bash
# Switch all to main and pull
gz-git switch main .
gz-git pull .

# Check what's behind
gz-git status . --behind
```

### Branch Cleanup
```bash
# Preview cleanup
gz-git branch cleanup . --dry-run --gone

# Execute cleanup
gz-git branch cleanup . --gone
```

### Release Tagging
```bash
# Tag all repos
gz-git tag create v2.0.0 . -m "Q1 2025 Release"
gz-git tag push .
```

## Output Format

Bulk operations output uses a consistent format:

```
[repo-name] status: message
[repo-name] OK: operation successful
[repo-name] SKIP: reason
[repo-name] ERROR: error message
```

**Summary at end:**
```
Summary: 10 repos, 8 OK, 1 SKIP, 1 ERROR
```

## Performance Tips

1. **Use `--parallel`** for large repository counts
2. **Use `--include/--exclude`** to filter relevant repos
3. **Use `--dry-run`** before destructive operations
4. **Use `--scan-depth 1`** (default) for flat structures
