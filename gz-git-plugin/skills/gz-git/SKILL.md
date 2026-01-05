---
name: gz-git
description: |
  gz-git CLI for safe Git operations. Use when:
  - Managing single Git repositories
  - Commit automation with templates
  - Branch, tag, stash management
  - History analysis and diff viewing
allowed-tools: Bash, Read, Grep, Glob
---

# gz-git CLI Reference

gz-git provides advanced Git operations with input sanitization and safe command execution.

## Quick Detection

```bash
command -v gz-git && echo "gz-git available" || echo "gz-git not installed"
```

## Command Reference

### Core Operations

| Command | Description |
|---------|-------------|
| `gz-git status [DIR]` | Show repository status |
| `gz-git fetch [DIR]` | Fetch from remotes |
| `gz-git pull [DIR]` | Pull changes |
| `gz-git push [DIR]` | Push changes |
| `gz-git diff [DIR]` | Show differences |
| `gz-git clone URL [DIR]` | Clone repository |

```bash
gz-git status                  # Current repo status
gz-git fetch                   # Fetch all remotes
gz-git pull                    # Pull with rebase
gz-git push                    # Push current branch
gz-git diff                    # Show unstaged changes
gz-git diff --staged           # Show staged changes
gz-git clone https://github.com/org/repo
```

### Commit Operations

| Command | Description |
|---------|-------------|
| `gz-git commit auto` | Auto-generate commit message |
| `gz-git commit template` | Use commit template |
| `gz-git commit amend` | Amend last commit |
| `gz-git commit fixup HASH` | Create fixup commit |

```bash
gz-git commit auto                        # AI-generated message
gz-git commit auto --type feat            # Specify type
gz-git commit template --scope api        # Use template
gz-git commit amend                       # Amend last commit
gz-git commit fixup abc1234               # Fixup specific commit
```

**Commit Flags:**
- `--type, -t` : Commit type (feat, fix, docs, refactor, test, chore)
- `--scope, -s` : Commit scope
- `--message, -m` : Custom message
- `--no-verify` : Skip hooks

### Branch Management

| Command | Description |
|---------|-------------|
| `gz-git branch list` | List branches |
| `gz-git branch create NAME` | Create branch |
| `gz-git branch delete NAME` | Delete branch |
| `gz-git branch cleanup` | Remove merged/gone branches |
| `gz-git switch BRANCH` | Switch to branch |

```bash
gz-git branch list                        # All branches
gz-git branch list --remote               # Remote branches
gz-git branch create feature/new          # Create and stay
gz-git branch create feature/new --switch # Create and switch
gz-git branch delete old-feature          # Delete local
gz-git branch delete old-feature --remote # Delete remote too
gz-git branch cleanup                     # Clean merged branches
gz-git branch cleanup --gone              # Clean gone branches
gz-git switch main                        # Switch branch
gz-git switch feature/x --create          # Create if not exists
```

### Stash Management

| Command | Description |
|---------|-------------|
| `gz-git stash save` | Save changes to stash |
| `gz-git stash list` | List stashes |
| `gz-git stash pop` | Apply and remove top stash |
| `gz-git stash apply` | Apply stash (keep in list) |
| `gz-git stash drop` | Remove stash |
| `gz-git stash clear` | Remove all stashes |

```bash
gz-git stash save "WIP: feature work"     # Save with message
gz-git stash save --include-untracked     # Include untracked files
gz-git stash list                         # Show all stashes
gz-git stash pop                          # Apply and remove
gz-git stash apply stash@{2}              # Apply specific stash
gz-git stash drop stash@{0}               # Remove specific stash
gz-git stash clear                        # Remove all
```

### Tag Management

| Command | Description |
|---------|-------------|
| `gz-git tag list` | List tags |
| `gz-git tag create NAME` | Create tag |
| `gz-git tag delete NAME` | Delete tag |
| `gz-git tag push` | Push tags to remote |

```bash
gz-git tag list                           # All tags
gz-git tag list --semver                  # Semantic version sorted
gz-git tag create v1.2.0                  # Create lightweight
gz-git tag create v1.2.0 -m "Release"     # Create annotated
gz-git tag create v1.2.0 --sign           # Create signed
gz-git tag delete v1.0.0                  # Delete local
gz-git tag delete v1.0.0 --remote         # Delete remote too
gz-git tag push                           # Push all tags
gz-git tag push v1.2.0                    # Push specific tag
```

### Merge Operations

| Command | Description |
|---------|-------------|
| `gz-git merge detect SRC DST` | Detect merge conflicts |
| `gz-git merge do SRC` | Merge branch into current |

```bash
gz-git merge detect feature/x main        # Check for conflicts
gz-git merge do feature/x                 # Merge into current
gz-git merge do feature/x --no-ff         # No fast-forward
gz-git merge do feature/x --squash        # Squash merge
```

### History & Info

| Command | Description |
|---------|-------------|
| `gz-git history stats` | Commit statistics |
| `gz-git history contributors` | Contributor list |
| `gz-git history file PATH` | File history |
| `gz-git info` | Repository info |

```bash
gz-git history stats                      # Overall stats
gz-git history stats --since "1 month"    # Recent stats
gz-git history contributors               # All contributors
gz-git history contributors --top 10      # Top 10
gz-git history file src/main.go           # File changes
gz-git info                               # Repo information
```

### Watch & Update

| Command | Description |
|---------|-------------|
| `gz-git watch` | Watch for changes |
| `gz-git update` | Update repository |

```bash
gz-git watch                              # Monitor changes
gz-git watch --interval 5s                # Check every 5s
gz-git update                             # Pull and update deps
```

## Global Flags

```
-v, --verbose     Verbose output
-q, --quiet       Quiet mode (errors only)
--version         Show version
--help            Show help
```

## Common Workflows

### Daily Development
```bash
gz-git status                 # Check state
gz-git fetch                  # Get updates
gz-git pull                   # Merge changes
# ... work ...
gz-git commit auto            # Commit with auto message
gz-git push                   # Push changes
```

### Feature Branch
```bash
gz-git branch create feature/x --switch
# ... work ...
gz-git commit auto --type feat --scope api
gz-git push
gz-git merge detect feature/x main
```

### Release
```bash
gz-git tag create v1.0.0 -m "Release 1.0.0"
gz-git tag push v1.0.0
```
