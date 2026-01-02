# gz-git Command Reference

Complete reference for all `gz-git` commands.

## Quick Navigation

- [Repository Commands](#repository-commands) - Initialize and manage repositories
- [Bulk Operations](#bulk-operations) - Multi-repository operations
- [Branch Commands](#branch-commands) - Branch and worktree management
- [History Commands](#history-commands) - Analyze repository history
- [Merge Commands](#merge-commands) - Merge and rebase operations

## Global Flags

All commands support these global flags:

| Flag        | Short | Description               |
| ----------- | ----- | ------------------------- |
| `--quiet`   | `-q`  | Suppress non-error output |
| `--verbose` | `-v`  | Show detailed output      |
| `--help`    | `-h`  | Show command help         |

## Repository Commands

### status

Show repository status with detailed change information.

```bash
gz-git status [flags]
```

**Flags:**

- `--porcelain`: Machine-readable output

**Examples:**

```bash
# Show status
gz-git status

# Machine-readable format
gz-git status --porcelain
```

### clone

Clone a repository with advanced options.

```bash
gz-git clone <url> [directory] [flags]
```

**Flags:**

- `--branch <name>`: Clone specific branch
- `--depth <n>`: Create shallow clone
- `--single-branch`: Clone only one branch

**Examples:**

```bash
# Clone repository
gz-git clone https://github.com/user/repo.git

# Clone specific branch
gz-git clone --branch develop https://github.com/user/repo.git

# Shallow clone
gz-git clone --depth 1 https://github.com/user/repo.git
```

## Bulk Operations

Commands for operating on multiple repositories at once.

### fetch

Fetch from multiple repositories.

```bash
gz-git fetch [directory] [flags]
```

**Flags:**

| Flag           | Short | Description                                 | Default |
| -------------- | ----- | ------------------------------------------- | ------- |
| `--scan-depth` | `-d`  | Directory depth to scan                     | 1       |
| `--parallel`   | `-j`  | Number of parallel operations               | 5       |
| `--dry-run`    | `-n`  | Preview without executing                   | false   |
| `--recursive`  | `-r`  | Include nested repos/submodules             | false   |
| `--all`        |       | Fetch from all remotes                      | false   |
| `--prune`      |       | Prune deleted remote branches               | false   |
| `--tags`       | `-t`  | Fetch all tags                              | false   |
| `--format`     | `-f`  | Output format (default, compact, json, llm) | default |

**Examples:**

```bash
# Fetch all repositories
gz-git fetch -d 2 ~/projects

# Fetch with parallelism
gz-git fetch -j 10 ~/workspace

# Dry run
gz-git fetch -n ~/projects
```

### pull

Pull from multiple repositories with smart state detection.

```bash
gz-git pull [directory] [flags]
```

**Flags:**

| Flag           | Short | Description                                 | Default |
| -------------- | ----- | ------------------------------------------- | ------- |
| `--scan-depth` | `-d`  | Directory depth to scan                     | 1       |
| `--parallel`   | `-j`  | Number of parallel operations               | 5       |
| `--dry-run`    | `-n`  | Preview without executing                   | false   |
| `--strategy`   | `-s`  | Pull strategy (merge, rebase, ff-only)      | merge   |
| `--stash`      |       | Auto-stash local changes                    | false   |
| `--prune`      | `-p`  | Prune deleted remote branches               | false   |
| `--tags`       | `-t`  | Fetch all tags                              | false   |
| `--format`     | `-f`  | Output format (default, compact, json, llm) | default |

**Examples:**

```bash
# Pull all repositories
gz-git pull -d 2 ~/projects

# Pull with rebase
gz-git pull -s rebase ~/projects
```

### push

Push to multiple repositories.

```bash
gz-git push [directory] [flags]
```

**Flags:**

| Flag                 | Short | Description                                 | Default |
| -------------------- | ----- | ------------------------------------------- | ------- |
| `--scan-depth`       | `-d`  | Directory depth to scan                     | 1       |
| `--parallel`         | `-j`  | Number of parallel operations               | 5       |
| `--dry-run`          | `-n`  | Preview without executing                   | false   |
| `--force`            |       | Force push (dangerous!)                     | false   |
| `--force-with-lease` |       | Safer force push                            | false   |
| `--set-upstream`     |       | Set upstream branch                         | false   |
| `--tags`             |       | Push all tags                               | false   |
| `--format`           | `-f`  | Output format (default, compact, json, llm) | default |

**Examples:**

```bash
# Push all repositories
gz-git push -d 2 ~/projects

# Dry run
gz-git push -n ~/projects
```

### switch

Switch branches across multiple repositories.

```bash
gz-git switch <branch> [directory] [flags]
```

**Flags:**

| Flag           | Short | Description                                 | Default |
| -------------- | ----- | ------------------------------------------- | ------- |
| `--scan-depth` | `-d`  | Directory depth to scan                     | 1       |
| `--parallel`   | `-j`  | Number of parallel operations               | 5       |
| `--dry-run`    | `-n`  | Preview without executing                   | false   |
| `--force`      |       | Force switch (dangerous!)                   | false   |
| `--include`    |       | Include repos matching pattern              |         |
| `--exclude`    |       | Exclude repos matching pattern              |         |
| `--format`     | `-f`  | Output format (default, compact, json, llm) | default |

**Examples:**

```bash
# Switch all repos to develop
gz-git switch develop -d 2 ~/projects

# Preview switch
gz-git switch main -n ~/projects

# JSON output
gz-git switch main --format json ~/projects

# LLM-friendly output
gz-git switch main --format llm ~/projects
```

### commit

Commit changes across multiple repositories with auto-generated messages.

```bash
gz-git commit [directory] [flags]
```

**Flags:**

| Flag              | Short | Description                                 | Default |
| ----------------- | ----- | ------------------------------------------- | ------- |
| `--dry-run`       | `-n`  | Preview only, don't commit                  | false   |
| `--message`       | `-m`  | Common message for all repos                | (auto)  |
| `--messages`      |       | Per-repo messages (repo:message format)     |         |
| `--edit`          | `-e`  | Edit messages in editor                     | false   |
| `--yes`           | `-y`  | Execute commits without preview             | false   |
| `--scan-depth`    | `-d`  | Scan depth for repositories                 | 1       |
| `--include`       |       | Include repos matching pattern              | *       |
| `--exclude`       |       | Exclude repos matching pattern              |         |
| `--parallel`      | `-j`  | Parallel execution count                    | 5       |
| `--format`        | `-f`  | Output format (default, compact, json, llm) | default |
| `--messages-file` |       | Load messages from JSON file                |         |

**Examples:**

```bash
# Preview dirty repositories
gz-git commit

# Commit all with auto-generated messages
gz-git commit -y

# Use common message for all
gz-git commit -y -m "chore: sync all repos"

# Per-repository messages
gz-git commit --messages "repo1:feat: add feature" --messages "repo2:fix: bug fix" -y

# Edit messages in editor before committing
gz-git commit -e

# Filter repositories
gz-git commit --include "myproject-*" --exclude "*-test*"

# Load messages from JSON file
gz-git commit --messages-file /tmp/messages.json -y

# JSON output for CI/automation
gz-git commit --dry-run --format json
```

**Workflow:**

1. **Preview mode** (default, no `-y`): Scans and shows dirty repos
2. **Execute mode** (`-y`): Commits with auto-generated messages
3. **Editor mode** (`-e`): Opens editor to customize messages

**Messages File JSON Schema:**

```json
{
  "repo-name": "commit message for repo-name",
  "another-repo": "feat(scope): description"
}
```

### watch

Monitor repositories for changes in real-time.

```bash
gz-git watch [paths...] [flags]
```

**Flags:**

| Flag              | Short | Description                                 | Default |
| ----------------- | ----- | ------------------------------------------- | ------- |
| `--interval`      |       | Polling interval                            | 2s      |
| `--include-clean` |       | Notify when repo becomes clean              | false   |
| `--format`        | `-f`  | Output format (default, compact, json, llm) | default |
| `--notify`        |       | Play sound on changes                       | false   |

**Examples:**

```bash
# Watch current directory
gz-git watch

# Watch multiple repos
gz-git watch /path/to/repo1 /path/to/repo2

# Custom interval
gz-git watch --interval 5s

# LLM-friendly output
gz-git watch --format llm
```

### diff

Show diffs for multiple repositories at once.

```bash
gz-git diff [flags]
```

**Flags:**

| Flag           | Short | Description                                 | Default |
| -------------- | ----- | ------------------------------------------- | ------- |
| `--staged`     |       | Show only staged changes                    | false   |
| `--scan-depth` | `-d`  | Scan depth for repositories                 | 1       |
| `--include`    |       | Include repos matching pattern              | *       |
| `--exclude`    |       | Exclude repos matching pattern              |         |
| `--context`    | `-c`  | Context lines around changes                | 3       |
| `--max-size`   |       | Max diff size per repo (bytes)              | 102400  |
| `--format`     | `-f`  | Output format (default, compact, json, llm) | default |

**Examples:**

```bash
# Show all diffs
gz-git diff

# Show only staged changes
gz-git diff --staged

# Filter repositories
gz-git diff --include "myproject-*"

# JSON output
gz-git diff --format json
```

## Branch Commands

### branch list

List local and remote branches.

```bash
gz-git branch list [flags]
```

**Flags:**

- `--all`, `-a`: Show both local and remote branches
- `--remote`, `-r`: Show only remote branches
- `--merged`: Show only merged branches
- `--no-merged`: Show only unmerged branches

**Examples:**

```bash
# List local branches
gz-git branch list

# List all branches
gz-git branch list --all

# Show merged branches
gz-git branch list --merged
```

### branch create

Create a new branch with optional worktree.

```bash
gz-git branch create <name> [flags]
```

**Flags:**

- `--base <ref>`: Create from specific ref (default: HEAD)
- `--track`: Set up tracking
- `--worktree <path>`: Create linked worktree

**Examples:**

```bash
# Create branch
gz-git branch create feature/new-feature

# Create from specific commit
gz-git branch create hotfix/bug --base abc123

# Create with worktree
gz-git branch create feature/parallel --worktree ../parallel-work
```

### branch delete

Delete local or remote branches.

```bash
gz-git branch delete <name> [flags]
```

**Flags:**

- `--force`, `-f`: Force delete even if not merged
- `--remote`, `-r`: Delete remote branch

**Examples:**

```bash
# Delete branch
gz-git branch delete feature/old

# Force delete
gz-git branch delete experimental --force

# Delete remote branch
gz-git branch delete feature/done --remote
```

### branch cleanup

Clean up merged branches across multiple repositories.

```bash
gz-git branch cleanup [directory] [flags]
```

**Flags:**

| Flag           | Short | Description                     | Default |
| -------------- | ----- | ------------------------------- | ------- |
| `--scan-depth` | `-d`  | Directory depth to scan         | 1       |
| `--parallel`   | `-j`  | Number of parallel operations   | 5       |
| `--dry-run`    | `-n`  | Preview without executing       | false   |
| `--remote`     | `-r`  | Also delete remote branches     | false   |
| `--force`      | `-f`  | Force delete unmerged branches  | false   |

**Examples:**

```bash
# Preview cleanup
gz-git branch cleanup -n

# Execute cleanup
gz-git branch cleanup -y

# Include remote branches
gz-git branch cleanup -r
```

## History Commands

### history stats

Show commit statistics and trends.

```bash
gz-git history stats [flags]
```

**Flags:**

- `--since <date>`: Start date (e.g., '2024-01-01', '1 month ago')
- `--until <date>`: End date
- `--branch <name>`: Specific branch
- `--author <name>`: Filter by author
- `--format <type>`: Output format (table, json, csv, markdown, llm)

**Examples:**

```bash
# Overall statistics
gz-git history stats

# Last month
gz-git history stats --since "1 month ago"

# Export as JSON
gz-git history stats --format json > stats.json
```

### history contributors

Analyze repository contributors.

```bash
gz-git history contributors [flags]
```

**Flags:**

- `--top <n>`: Show only top N contributors
- `--since <date>`: Start date
- `--until <date>`: End date
- `--min-commits <n>`: Minimum commits threshold
- `--sort <field>`: Sort by (commits, additions, deletions, recent)
- `--format <type>`: Output format (table, json, csv, markdown, llm)

**Examples:**

```bash
# List all contributors
gz-git history contributors

# Top 10 contributors
gz-git history contributors --top 10

# Contributors with at least 5 commits
gz-git history contributors --min-commits 5
```

### history file

Show file change history.

```bash
gz-git history file <path> [flags]
```

**Flags:**

- `--since <date>`: Start date
- `--until <date>`: End date
- `--max <n>`: Maximum number of commits
- `--follow`: Follow file renames
- `--author <name>`: Filter by author
- `--format <type>`: Output format (table, json, csv, markdown, llm)

**Examples:**

```bash
# Show file history
gz-git history file src/main.go

# Follow renames
gz-git history file --follow src/main.go

# Limit to 10 commits
gz-git history file --max 10 README.md
```

## Merge Commands

### merge detect

Detect potential merge conflicts before merging.

```bash
gz-git merge detect <source> <target> [flags]
```

**Flags:**

- `--include-binary`: Include binary file conflicts
- `--base <commit>`: Specify base commit

**Examples:**

```bash
# Detect conflicts
gz-git merge detect feature/new-feature main

# Include binary files
gz-git merge detect feature/new-feature main --include-binary
```

## Exit Codes

| Code | Meaning                            |
| ---- | ---------------------------------- |
| 0    | Success                            |
| 1    | General error                      |
| 2    | Invalid arguments                  |
| 3    | Not a git repository               |
| 4    | Operation failed (e.g., conflicts) |

## Environment Variables

| Variable         | Description                       | Default            |
| ---------------- | --------------------------------- | ------------------ |
| `GZH_GIT_EDITOR` | Editor for interactive operations | `$EDITOR` or `vim` |

## See Also

- [Quick Start](../../QUICK_START.md)
