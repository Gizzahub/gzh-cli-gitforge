---
name: gz-git
description: |
  gz-git CLI for safe Git operations. Use when:
  - Managing Git repositories (single or multiple)
  - Bulk fetch/pull/push/status across repos
  - Cloning from GitHub/GitLab organizations
  - Branch, tag, stash, commit automation
  gz-git operates in BULK MODE by default (scans directories for repos).
allowed-tools: Bash, Read, Grep, Glob
---

# gz-git CLI Reference

gz-git provides advanced Git operations with input sanitization and safe command execution.

## Core Concept: Bulk-First Design

**gz-git operates in BULK MODE by default.** All major commands automatically scan directories and process multiple repositories in parallel.

### Default Behavior

| Setting | Default | Description |
|---------|---------|-------------|
| **Scan Depth** | `1` | Current directory + 1 level deep |
| **Parallel** | `5` | Process 5 repos concurrently |

```bash
# These commands scan ALL repos in current directory
gz-git status          # Status of all repos (depth=1)
gz-git fetch           # Fetch all repos
gz-git pull            # Pull all repos
gz-git push            # Push all repos
```

### Understanding Scan Depth

```
depth=0: Current directory only (single repo behavior)
depth=1: Current + immediate children (DEFAULT)
         ~/projects/repo1, ~/projects/repo2
depth=2: Current + 2 levels deep
         ~/projects/org/repo1, ~/projects/team/repo2
```

### Single Repository Operations

Provide the path directly to target a specific repo:

```bash
gz-git status /path/to/repo      # Single repo status
gz-git fetch /path/to/repo       # Fetch single repo
```

______________________________________________________________________

## Common Flags (All Bulk Commands)

```bash
-d, --scan-depth INT   # Directory depth (default: 1)
-j, --parallel INT     # Parallel workers (default: 5)
-n, --dry-run          # Preview without executing
--include REGEX        # Include repos matching pattern
--exclude REGEX        # Exclude repos matching pattern
-f, --format FORMAT    # Output: default, compact, json, llm
```

______________________________________________________________________

## Command Reference

### Status

```bash
gz-git status                     # All repos in current dir
gz-git status -d 2 ~/projects     # 2 levels deep
gz-git status --dirty-only        # Only show dirty repos
gz-git status --behind            # Show repos behind remote
gz-git status --format compact    # One-line per repo
gz-git status /path/to/repo       # Single repo
```

### Fetch / Pull / Push

```bash
# Fetch
gz-git fetch                      # All repos (default)
gz-git fetch -d 2 ~/projects      # 2 levels deep
gz-git fetch --all                # Fetch all remotes
gz-git fetch --prune              # Prune deleted branches
gz-git fetch -t                   # Fetch tags

# Pull
gz-git pull                       # All repos
gz-git pull -s rebase             # Pull with rebase
gz-git pull -s ff-only            # Fast-forward only
gz-git pull --stash               # Auto-stash changes

# Push
gz-git push                       # All repos
gz-git push -n                    # Dry run
gz-git push --force-with-lease    # Safe force push
gz-git push --set-upstream        # Set upstream
gz-git push --tags                # Push tags
```

### Switch (Branch)

```bash
gz-git switch main                # Switch all repos to main
gz-git switch develop -d 2        # 2 levels deep
gz-git switch feature/x -n        # Dry run
gz-git switch main --force        # Force switch (DANGEROUS)
```

### Clone

```bash
# Single repository
gz-git clone https://github.com/user/repo.git
gz-git clone -b develop https://github.com/user/repo.git
gz-git clone --depth 1 https://github.com/user/repo.git

# Bulk clone from organization
gz-git clone --org mycompany --provider github
gz-git clone --org mycompany --provider gitlab
gz-git clone --org mycompany --include "^api-"
gz-git clone --org mycompany --exclude "archived"

# Clone from manifest file
gz-git clone --manifest repos.yml
gz-git clone --manifest repos.yml -j 10
```

### Commit

```bash
gz-git commit auto                # Auto-generate message (all repos)
gz-git commit auto --type feat    # Specify commit type
gz-git commit -m "chore: update"  # Common message
gz-git commit -e                  # Edit in $EDITOR

# Per-repository messages
gz-git commit \
  --messages "frontend:feat(ui): add button" \
  --messages "backend:fix(api): null check" \
  --yes

gz-git commit --messages-file /tmp/messages.json --yes
```

### Branch Management

```bash
gz-git branch list                # All branches (all repos)
gz-git branch list --remote       # Remote branches
gz-git branch create feature/new  # Create branch
gz-git branch delete old-feature  # Delete branch
gz-git branch cleanup             # Remove merged branches
gz-git branch cleanup --gone      # Remove gone branches
```

### Stash

```bash
gz-git stash save "WIP"           # Stash all dirty repos
gz-git stash list                 # List stashes
gz-git stash pop                  # Pop stashes
gz-git stash apply                # Apply (keep in list)
gz-git stash drop                 # Remove stash
gz-git stash clear                # Clear all
```

### Tag

```bash
gz-git tag list                   # List tags (all repos)
gz-git tag list --semver          # Sorted by semver
gz-git tag create v1.0.0          # Create tag
gz-git tag create v1.0.0 -m "Release"  # Annotated
gz-git tag delete v1.0.0          # Delete local
gz-git tag delete v1.0.0 --remote # Delete remote too
gz-git tag push                   # Push all tags
gz-git tag push v1.0.0            # Push specific tag
```

### Diff

```bash
gz-git diff                       # Diff all repos
gz-git diff --stat                # Stats only
gz-git diff --staged              # Staged changes
gz-git diff --name-only           # File names only
```

### History & Info

```bash
gz-git history stats              # Commit statistics
gz-git history stats --since "1 month"
gz-git history contributors       # Contributor list
gz-git history contributors --top 10
gz-git history file src/main.go   # File history
gz-git info                       # Repository info
```

### Watch & Update

```bash
gz-git watch                      # Monitor all repos
gz-git watch --interval 5s        # Custom interval
gz-git update                     # Pull and update deps
```

### Merge

```bash
gz-git merge detect feature/x main  # Detect conflicts
gz-git merge do feature/x           # Merge into current
gz-git merge do feature/x --no-ff   # No fast-forward
gz-git merge do feature/x --squash  # Squash merge
```

______________________________________________________________________

## Common Workflows

### Daily Sync (Multi-Repo)

```bash
gz-git fetch                 # Get updates
gz-git status --dirty-only   # Check dirty repos
gz-git pull                  # Pull all
```

### Feature Development

```bash
gz-git switch main
gz-git pull
gz-git branch create feature/x --switch
# ... work ...
gz-git commit auto --type feat
gz-git push
```

### Organization Clone

```bash
# Clone all org repos
gz-git clone --org mycompany --provider github ~/work

# Daily sync
cd ~/work
gz-git fetch
gz-git status
```

### Branch Cleanup

```bash
# Preview
gz-git branch cleanup --gone -n

# Execute
gz-git branch cleanup --gone
```

### Release Tagging

```bash
gz-git tag create v1.0.0 -m "Release 1.0.0"
gz-git tag push
```

______________________________________________________________________

## Output Format

Bulk operations show:

```
[repo-name] OK: operation successful
[repo-name] SKIP: reason
[repo-name] ERROR: error message

Summary: 10 repos, 8 OK, 1 SKIP, 1 ERROR
```

## Global Flags

```
-v, --verbose     Verbose output
-q, --quiet       Quiet mode (errors only)
--version         Show version
--help            Show help
```
