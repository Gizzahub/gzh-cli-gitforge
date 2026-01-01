# gzh-cli-gitforge

> Advanced Git automation CLI and Go library for developers

[![Go Version](https://img.shields.io/badge/go-1.24.0%2B-blue)](https://go.dev)
[![Version](https://img.shields.io/badge/version-v0.4.0-blue)](https://github.com/gizzahub/gzh-cli-gitforge/releases/tag/v0.4.0)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![Test Coverage](https://img.shields.io/badge/coverage-69.1%25-yellow)](docs/_deprecated/2025-12/COVERAGE.md)
[![Tests](https://img.shields.io/badge/tests-51%20integration%2F90%20e2e-brightgreen)](#testing)
[![GoDoc](https://pkg.go.dev/badge/github.com/gizzahub/gzh-cli-gitforge.svg)](https://pkg.go.dev/github.com/gizzahub/gzh-cli-gitforge)

**gzh-cli-gitforge** is a Git-specialized CLI tool and Go library that provides advanced Git automation capabilities. It serves dual purposes: a powerful standalone CLI for developers and a reusable library for embedding in other Go projects.

______________________________________________________________________

## Features

### ✅ Fully Implemented & Available (v0.4.0)

📦 **Repository Operations**

- Clone repositories with advanced options (branch, depth, single-branch, recursive)
- Check repository status (clean/dirty, modified/staged/untracked files)
- Get repository information (branch, remote, upstream, ahead/behind counts)
- Bulk operations (clone-or-update, fetch multiple repos in parallel)
- **Bulk fetch** from multiple repositories by depth (1-depth, 2-depth scanning)
- Flexible clone strategies (always-clone, update-if-exists, skip-if-exists)
- **Real-time monitoring** (watch repositories for changes)
- **Smart state detection** (conflicts, rebase/merge in progress)
  - Auto-detect repository problems before operations
  - Auto-abort on conflicts to prevent incomplete states
  - Clear error messages with actionable guidance

🚀 **Commit Automation**

- **Bulk commit** across multiple repositories (new in v0.4.0!)
- **Per-repository custom messages** via `--messages` flag
- Template-based commit messages (Conventional Commits support)
- Auto-generate commit messages from code changes
- Interactive message editing with `$EDITOR`
- Validate commit messages against templates
- Built-in template management (list, show, validate)
- JSON file support for batch message customization

🌿 **Branch Management**

- Create, list, and delete branches
- Worktree-based parallel development
- Branch creation with linked worktrees
- Local and remote branch operations

📊 **History Analysis**

- Commit statistics and trends
- Contributor analysis with metrics
- File change tracking and history
- Multiple output formats (Table, JSON, CSV)

🔀 **Advanced Merge/Rebase**

- Pre-merge conflict detection
- Execute merge with multiple strategies
- Abort and rebase operations
- Interactive conflict assistance

📚 **Go Library API**

- Clean, stable public APIs (all `pkg/*` packages)
- Zero CLI dependencies in library code
- Context-aware operations (cancellation, timeouts)
- Easy integration into other Go projects
- Full implementations: `repository`, `operations`, `commit`, `branch`, `history`, `merge`

🔧 **Quality & Testing**

- Integration tests: 51 passing
- E2E tests: 90 runs passing
- 69.1% code coverage
- Comprehensive integration tests
- Well-documented codebase

> **Note**: Version v0.4.0 reflects the actual feature completeness of this project. All major planned features are implemented and tested. See [IMPLEMENTATION_STATUS.md](docs/_deprecated/2025-12/IMPLEMENTATION_STATUS.md) for details.

______________________________________________________________________

## Quick Start

### Installation

**Via Go Install:**

```bash
go install github.com/gizzahub/gzh-cli-gitforge/cmd/gz-git@latest
```

**Via Homebrew (macOS/Linux):**

```bash
brew install gz-git  # Coming soon
```

**From Source:**

```bash
git clone https://github.com/gizzahub/gzh-cli-gitforge.git
cd gzh-cli-gitforge
make build    # Builds as 'gz-git'
make install  # Installs to $GOPATH/bin
```

### Requirements

- Git 2.30+
- Go 1.24+ (for building from source)

______________________________________________________________________

## Usage

### As CLI Tool

**Check Repository Status:**

```bash
# Show working tree status with smart state detection
gz-git status

# Displays:
# - ↻ Rebase in progress (with recovery commands)
# - ⇄ Merge in progress (with resolution guidance)
# - ⚡ Unresolved conflicts (with file list)
# - Modified, staged, untracked files

# Show status for specific repository
gz-git status /path/to/repo

# Quiet mode (exit code 1 if dirty)
gz-git status -q
```

**Monitor Repositories for Changes:**

```bash
# Watch current directory for changes
gz-git watch

# Watch multiple repositories
gz-git watch /path/to/repo1 /path/to/repo2

# Custom polling interval
gz-git watch --interval 5s

# Compact output format
gz-git watch --format compact

# JSON output for automation
gz-git watch --format json

# LLM-friendly output format
gz-git watch --format llm
```

**View Repository Information:**

```bash
# Show detailed repository information
gz-git info

# Displays: branch, remote URL, upstream, ahead/behind counts, dirty/clean status
gz-git info /path/to/repo
```

**Clone Repositories:**

```bash
# Basic clone
gz-git clone https://github.com/user/repo.git

# Clone specific branch
gz-git clone -b develop https://github.com/user/repo.git

# Shallow clone (faster)
gz-git clone --depth 1 https://github.com/user/repo.git

# Clone with submodules
gz-git clone --recursive https://github.com/user/repo.git

# Clone to specific directory
gz-git clone https://github.com/user/repo.git my-project
```

**Bulk Fetch Multiple Repositories:**

```bash
# Fetch all repositories in current directory (1-depth)
gz-git fetch -d 1

# Fetch repositories up to 2 levels deep
gz-git fetch -d 2 ~/projects

# Fetch with custom parallelism (short: -j)
gz-git fetch -j 10 ~/workspace

# Fetch from all remotes (not just origin)
gz-git fetch --all ~/projects

# Fetch and prune deleted remote branches
gz-git fetch --prune ~/repos

# Fetch all tags (short: -t)
gz-git fetch -t ~/repos

# Dry run to see what would be fetched (short: -n)
gz-git fetch -n ~/projects

# Filter by pattern
gz-git fetch --include "myproject.*" ~/workspace
gz-git fetch --exclude "test.*" ~/projects

# Recursively include nested repositories and submodules (short: -r)
gz-git fetch -r ~/projects

# Watch mode: continuously fetch at intervals
gz-git fetch -d 2 --watch --interval 5m ~/projects
gz-git fetch --watch --interval 1m ~/work
```

**Bulk Pull Multiple Repositories:**

```bash
# Pull all repositories with smart state detection
# - Skips repos with conflicts, rebase/merge in progress
# - Auto-aborts conflicted rebases to restore clean state
# - Shows clear status: ⚡ conflict, ↻ rebase, ⇄ merge
gz-git pull -d 1

# Pull repositories up to 2 levels deep
gz-git pull -d 2 ~/projects

# Pull with rebase strategy (short: -s)
gz-git pull -s rebase -d 2 ~/projects

# Pull with fast-forward only (fail if can't fast-forward)
gz-git pull -s ff-only ~/projects

# Pull with custom parallelism (short: -j)
gz-git pull -j 10 ~/workspace

# Pull and automatically stash local changes
gz-git pull --stash -d 2 ~/projects

# Pull and prune deleted remote branches (short: -p)
gz-git pull -p ~/repos

# Fetch all tags (short: -t)
gz-git pull -t ~/repos

# Dry run to see what would be pulled (short: -n)
gz-git pull -n ~/projects

# Recursively include nested repositories and submodules (short: -r)
gz-git pull -r ~/projects

# Filter by pattern
gz-git pull --include "myproject.*" ~/workspace
gz-git pull --exclude "test.*" ~/projects

# Compact output format
gz-git pull --format compact ~/projects

# Watch mode: continuously pull at intervals (default: 1m)
gz-git pull -d 2 --watch ~/projects
gz-git pull --watch --interval 5m ~/work

# Combined example with multiple shorthand flags
gz-git pull -s rebase -j 10 -n -t -p -r -d 2 ~/projects
```

**Bulk Push Multiple Repositories:**

```bash
# Push all repositories with smart state detection
# - Skips repos with conflicts, rebase/merge in progress, or uncommitted changes
# - Shows clear status: ⚡ conflict, ↻ rebase, ⇄ merge, ⚠ dirty
gz-git push -d 1

# Push repositories up to 2 levels deep
gz-git push -d 2 ~/projects

# Push with custom parallelism (short: -j)
gz-git push -j 10 ~/workspace

# Force push (use with caution!)
gz-git push --force ~/projects

# Push with force-with-lease (safer than --force)
gz-git push --force-with-lease ~/projects

# Push and set upstream branch automatically
gz-git push --set-upstream -d 2 ~/projects

# Push all tags
gz-git push --tags ~/repos

# Dry run to see what would be pushed (short: -n)
gz-git push -n ~/projects

# Filter by pattern
gz-git push --include "myproject.*" ~/workspace
gz-git push --exclude "test.*" ~/projects

# Recursively include nested repositories and submodules (short: -r)
gz-git push -r ~/projects

# Compact output format
gz-git push --format compact ~/projects

# Combined example with multiple flags
gz-git push -j 10 -n --set-upstream -r -d 2 ~/projects
```

**Bulk Switch Branches:**

```bash
# Switch all repositories to a branch
# - Skips repos without the target branch
# - Shows clear status for each repo
gz-git switch develop -d 1

# Switch repositories up to 2 levels deep
gz-git switch main -d 2 ~/projects

# Preview switch without executing (short: -n)
gz-git switch feature/new -n ~/projects

# Force switch even with uncommitted changes (DANGEROUS!)
gz-git switch main --force ~/projects

# Filter by pattern
gz-git switch develop --include "myproject.*" ~/workspace

# Compact output format (short: -f)
gz-git switch main -f compact ~/projects

# JSON output for automation
gz-git switch main --format json ~/projects

# LLM-friendly output format
gz-git switch main --format llm ~/projects
```

**Shorthand Flags Reference:**

| Flag           | Short | Description                                      | Commands                         |
| -------------- | ----- | ------------------------------------------------ | -------------------------------- |
| `--scan-depth` | `-d`  | Directory depth to scan (bulk commands)          | fetch, pull, push, switch        |
| `--parallel`   | `-j`  | Parallel operations (make -j convention)         | fetch, pull, push, switch        |
| `--dry-run`    | `-n`  | Preview without executing (GNU convention)       | fetch, pull, push, switch        |
| `--format`     | `-f`  | Output format (default, compact, json, llm)      | fetch, pull, push, switch, watch |
| `--strategy`   | `-s`  | Pull strategy (merge/rebase/ff-only)             | pull                             |
| `--tags`       | `-t`  | Fetch/push all tags (git convention)             | fetch, pull, push                |
| `--prune`      | `-p`  | Prune deleted remote branches (git convention)   | pull                             |
| `--recursive`  | `-r`  | Include nested repos/submodules (GNU convention) | fetch, pull, push                |

**Global Options:**

```bash
# Verbose output
gz-git -v status

# Quiet mode (errors only)
gz-git -q clone https://github.com/user/repo.git

# Show version
gz-git --version

# Show help
gz-git --help
```

______________________________________________________________________

### Advanced Features Usage

**Commit Automation:**

```bash
# Bulk commit across multiple repositories (v0.4.0+)
gz-git commit -d 1                    # Preview commits
gz-git commit -d 2 --yes              # Auto-approve and commit

# Common message for all repositories
gz-git commit -m "chore: update dependencies" --yes

# Per-repository custom messages (NEW!)
gz-git commit \
  --messages "frontend:feat(ui): add login button" \
  --messages "backend:fix(api): handle null values" \
  --messages "docs:docs: update API guide" \
  --yes

# Interactive message editing
gz-git commit -e --yes                # Edit in $EDITOR

# Load messages from JSON file
gz-git commit --messages-file /tmp/messages.json --yes

# Dry run (preview without committing)
gz-git commit --dry-run

# JSON output for automation
gz-git commit --format json --yes

# Filter repositories
gz-git commit --include "^frontend" --yes
gz-git commit --exclude "test-" --yes

# Single repository commands
gz-git commit auto                    # Auto-generate and commit
gz-git commit validate "feat(cli): add new command"
gz-git commit template list           # List templates
gz-git commit template show conventional
```

**Branch & Worktree Management:**

```bash
# List all branches
gz-git branch list --all

# Create new branch
gz-git branch create feature/new-feature

# Create branch with worktree
gz-git branch create feature/auth --worktree ~/work/auth

# Delete branch
gz-git branch delete old-feature
```

**History Analysis:**

```bash
# Show commit statistics
gz-git history stats --since "1 month ago"

# Analyze contributors
gz-git history contributors --top 10

# View file history
gz-git history file src/main.go
```

**Advanced Merge/Rebase:**

```bash
# Detect conflicts before merging
gz-git merge detect feature/new-feature main

# Execute merge
gz-git merge do feature/new-feature

# Abort merge if needed
gz-git merge abort

# Rebase current branch
gz-git merge rebase main
```

______________________________________________________________________

## Documentation

- 📖 [Quick Start](QUICK_START.md)
- 📚 [API Reference](https://pkg.go.dev/github.com/gizzahub/gzh-cli-gitforge)
- 🧪 [Examples](examples/)

______________________________________________________________________

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
