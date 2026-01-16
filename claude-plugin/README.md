# gz-git Plugin for Claude Code

gz-git CLI integration - Safe Git operations with bulk-first design.

## Installation

```bash
/plugin marketplace add gizzahub/gz-git
/plugin install gz-git@gz-git-marketplace
```

## Requirements

- gz-git binary: `go install github.com/gizzahub/gzh-cli-gitforge/cmd/gz-git@latest`
- Git 2.30+

## Core Concept: Bulk-First

**gz-git operates in BULK MODE by default.** All major commands scan directories and process multiple repositories in parallel.

```bash
gz-git status          # Scans ALL repos in current dir (depth=1)
gz-git fetch           # Fetches ALL repos
gz-git pull            # Pulls ALL repos
```

### Default Settings

| Setting | Value | Description |
|---------|-------|-------------|
| Scan Depth | `1` | Current dir + 1 level |
| Parallel | `10` | 5 concurrent repos |

### Single Repo

```bash
gz-git status /path/to/repo
```

## Skill

**gz-git**: Complete CLI reference covering bulk operations, clone, commit, branch, tag, stash, and more.

## Local Test

```bash
claude --plugin-dir ./claude-plugin
```
