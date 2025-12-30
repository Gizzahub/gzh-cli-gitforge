# gz-git Command Reference

Complete reference for all `gz-git` commands.

## Quick Navigation

- [Repository Commands](#repository-commands) - Initialize and manage repositories
- [Commit Commands](#commit-commands) - Automate commit operations
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

### info

Display repository information.

```bash
gz-git info [flags]
```

**Examples:**

```bash
# Show repository info
gz-git info
```

## Commit Commands

### commit auto

Automatically generate and create commits from staged changes.

```bash
gz-git commit auto [flags]
```

**Flags:**

- `--template <name>`: Use specific commit template
- `--dry-run`: Show generated message without committing

**Examples:**

```bash
# Auto-generate and commit
gz-git commit auto

# Use specific template
gz-git commit auto --template conventional

# Preview without committing
gz-git commit auto --dry-run
```

**How it works:**

1. Analyzes staged changes
1. Generates commit message using template
1. Validates message against rules
1. Creates commit if validation passes

### commit validate

Validate commit messages against templates.

```bash
gz-git commit validate <message> [flags]
```

**Flags:**

- `--template <name>`: Validate against specific template

**Examples:**

```bash
# Validate message
gz-git commit validate "feat: add new feature"

# Validate with template
gz-git commit validate "feat(api): add endpoint" --template conventional
```

### commit template

Manage commit message templates.

```bash
gz-git commit template <command> [args]
```

**Subcommands:**

- `list`: List available templates
- `show <name>`: Show template details
- `validate <file>`: Validate custom template file

**Examples:**

```bash
# List templates
gz-git commit template list

# Show template
gz-git commit template show conventional

# Validate custom template
gz-git commit template validate my-template.yaml
```

### commit bulk

Bulk commit across multiple repositories with auto-generated messages.

```bash
gz-git commit bulk [flags]
```

**Flags:**

| Flag              | Short | Description                       | Default    |
| ----------------- | ----- | --------------------------------- | ---------- |
| `--dry-run`       |       | Preview only, don't commit        | false      |
| `--message`       | `-m`  | Common message for all repos      | (auto)     |
| `--edit`          | `-e`  | Edit messages in editor           | false      |
| `--yes`           | `-y`  | Execute commits without preview   | false      |
| `--depth`         | `-d`  | Scan depth for repositories       | 1          |
| `--include`       |       | Include repos matching pattern    | *          |
| `--exclude`       |       | Exclude repos matching pattern    |            |
| `--parallel`      | `-p`  | Parallel execution count          | (CPU num)  |
| `--format`        | `-f`  | Output format (text/json)         | text       |
| `--messages-file` |       | Load messages from JSON file      |            |

**Examples:**

```bash
# Preview dirty repositories
gz-git commit bulk

# Commit all with auto-generated messages
gz-git commit bulk -y

# Use common message for all
gz-git commit bulk -y -m "chore: sync all repos"

# Edit messages in editor before committing
gz-git commit bulk -e

# Filter repositories
gz-git commit bulk --include "myproject-*" --exclude "*-test*"

# Load messages from JSON file
gz-git commit bulk --messages-file /tmp/messages.json -y

# JSON output for CI/automation
gz-git commit bulk --dry-run --format json
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

Keys can be:
- Relative path: `myproject-api`
- Base name: `api`
- Full path: `/path/to/myproject-api`

**JSON Output Schema (--format json):**

Preview output:
```json
{
  "scan_depth": 1,
  "total_repositories": 3,
  "total_files": 15,
  "total_additions": 100,
  "total_deletions": 50,
  "repositories": [
    {
      "path": "/full/path/repo1",
      "relative_path": "repo1",
      "branch": "main",
      "status": "dirty",
      "files_changed": 5,
      "additions": 30,
      "deletions": 10,
      "suggested_message": "fix(cmd): update handler"
    }
  ]
}
```

Result output:
```json
{
  "success": true,
  "total_scanned": 3,
  "total_dirty": 2,
  "total_committed": 2,
  "total_failed": 0,
  "duration_ms": 1250,
  "repositories": [
    {
      "path": "repo1",
      "status": "success",
      "commit_hash": "abc1234",
      "message": "fix(cmd): update handler"
    }
  ]
}
```

### diff

Show diffs for multiple repositories at once.

```bash
gz-git diff [flags]
```

**Flags:**

| Flag              | Short | Description                       | Default    |
| ----------------- | ----- | --------------------------------- | ---------- |
| `--staged`        |       | Show only staged changes          | false      |
| `--depth`         | `-d`  | Scan depth for repositories       | 1          |
| `--include`       |       | Include repos matching pattern    | *          |
| `--exclude`       |       | Exclude repos matching pattern    |            |
| `--context`       | `-c`  | Context lines around changes      | 3          |
| `--max-size`      |       | Max diff size per repo (bytes)    | 102400     |
| `--format`        | `-f`  | Output format (text/json)         | text       |

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
- `--format <type>`: Output format (table|json|csv|markdown)

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
- `--sort <field>`: Sort by (commits|additions|deletions|recent)
- `--format <type>`: Output format

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
- `--format <type>`: Output format

**Examples:**

```bash
# Show file history
gz-git history file src/main.go

# Follow renames
gz-git history file --follow src/main.go

# Limit to 10 commits
gz-git history file --max 10 README.md
```

### history blame

Show line-by-line authorship.

```bash
gz-git history blame <file> [flags]
```

**Examples:**

```bash
# Show blame
gz-git history blame src/main.go
```

## Merge Commands

### merge do

Execute a merge operation.

```bash
gz-git merge do <source-branch> [flags]
```

**Flags:**

- `--strategy <type>`: Merge strategy (auto|ours|theirs|recursive)
- `--ff-only`: Only allow fast-forward merge
- `--no-commit`: Merge but don't commit
- `--squash`: Squash all commits into one
- `--message <text>`: Custom merge commit message

**Examples:**

```bash
# Merge branch
gz-git merge do feature/new-feature

# Fast-forward only
gz-git merge do feature/new-feature --ff-only

# Squash merge
gz-git merge do feature/new-feature --squash

# Custom message
gz-git merge do feature/new-feature --message "Merge feature X"
```

**Merge Strategies:**

- `auto` (default): Let Git choose the best strategy
- `recursive`: Standard three-way merge
- `ours`: Keep our version in conflicts
- `theirs`: Keep their version in conflicts

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

### merge abort

Abort an in-progress merge.

```bash
gz-git merge abort
```

**Examples:**

```bash
# Abort merge
gz-git merge abort
```

### merge rebase

Rebase current branch onto another.

```bash
gz-git merge rebase [branch] [flags]
```

**Flags:**

- `--onto <commit>`: Rebase onto specific commit
- `--continue`: Continue after resolving conflicts
- `--skip`: Skip current commit
- `--abort`: Abort rebase

**Examples:**

```bash
# Rebase onto main
gz-git merge rebase main

# Continue after conflicts
gz-git merge rebase --continue

# Abort rebase
gz-git merge rebase --abort
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

| Variable           | Description                       | Default            |
| ------------------ | --------------------------------- | ------------------ |
| `GZH_GIT_TEMPLATE` | Default commit template           | `conventional`     |
| `GZH_GIT_EDITOR`   | Editor for interactive operations | `$EDITOR` or `vim` |

## See Also

- [Installation Guide](../INSTALL.md)
- [Quick Start](../QUICKSTART.md)
- [Troubleshooting](../TROUBLESHOOTING.md)
- [Library Integration](../LIBRARY.md)
