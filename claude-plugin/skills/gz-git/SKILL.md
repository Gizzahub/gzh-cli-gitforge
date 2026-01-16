---
name: gz-git
description: |
  gz-git CLI for safe Git operations. Use when:
  - Managing Git repositories (single or multiple)
  - Bulk status/fetch/pull/push/update/diff/commit/switch across repos
  - Syncing repos from GitHub/GitLab/Gitea (sync from-forge/from-config)
  - Generating config from local scan or forge API (sync config scan/generate)
  - Watching repos for changes (watch)
  gz-git operates in BULK MODE by default (scans directories for repos).
allowed-tools: Bash, Read, Grep, Glob
---

# gz-git CLI Reference

This is a lightweight, stable reference for the `gz-git` CLI.
For the full and authoritative flag list, prefer:

```bash
gz-git --help
gz-git <command> --help
```

Curated docs in this repo:

- `docs/commands/README.md`: `../../../docs/commands/README.md`
- `docs/commands/watch.md`: `../../../docs/commands/watch.md`

## Core Concept: Bulk-First Design

Most `gz-git` commands scan a directory for repositories and process them in parallel.

### Defaults

| Setting | Default | Description |
|---------|---------|-------------|
| `--scan-depth` | `1` | Current directory + 1 level deep |
| `--parallel`   | `5` | Process 5 repos concurrently |

### Common Bulk Flags

```bash
-d, --scan-depth INT     # Directory depth (default: 1)
-j, --parallel INT       # Parallel workers (default: 5)
-n, --dry-run            # Preview without executing
-r, --recursive          # Include nested repos/submodules
    --include REGEX      # Include repos matching pattern
    --exclude REGEX      # Exclude repos matching pattern
-f, --format FORMAT      # default, compact, json, llm
    --watch              # Run continuously at intervals
    --interval DURATION  # Interval when watching
```

## Common Commands

### Status / Fetch / Pull / Push / Update

```bash
gz-git status -d 2 ~/projects
gz-git fetch  -d 2 ~/projects --all-remotes --prune --tags
gz-git pull   -d 2 ~/projects --strategy rebase --stash
gz-git push   -d 2 ~/projects --set-upstream
gz-git update -d 2 ~/projects --watch --interval 5m
```

### Diff

```bash
gz-git diff -d 2 ~/projects
gz-git diff --staged --format compact ~/projects
gz-git diff --include-untracked --context 5 ~/projects
```

### Commit (Bulk)

Default is preview; use `--yes` to commit.

```bash
gz-git commit --dry-run -d 2 ~/projects
gz-git commit --yes -d 2 ~/projects
gz-git commit --all "chore: sync all repos" --yes -d 2 ~/projects
gz-git commit -m "frontend:feat: add login" -m "backend:fix: null check" --yes -d 2 ~/projects
gz-git commit --file /tmp/messages.json --yes -d 2 ~/projects
```

### Branch Helpers

Basic branch creation/deletion remains native `git`. `gz-git` provides:

```bash
gz-git branch list -a -d 2 ~/projects
gz-git switch feature/new --create -d 2 ~/projects
gz-git cleanup branch --merged --force -d 2 ~/projects
gz-git merge detect feature/new main
```

### Clone (Bulk)

```bash
gz-git clone --url https://github.com/user/repo1.git --url https://github.com/user/repo2.git
gz-git clone ~/projects --file repos.txt
gz-git clone --update --file repos.txt
```

### Sync (Multi-Repo Management)

```bash
# From Git Forge (GitHub/GitLab/Gitea)
gz-git sync from-forge --provider github --org myorg --target ./repos --token $GITHUB_TOKEN
gz-git sync from-forge --provider gitlab --org devbox --include-subgroups --subgroup-mode flat

# From YAML Config
gz-git sync from-config -c sync.yaml --dry-run

# Config Management
gz-git sync config scan ~/mydevbox --strategy unified -o sync.yaml
gz-git sync config scan ~/mydevbox --strategy per-directory --no-gitignore
gz-git sync config generate --provider gitlab --org devbox --token $TOKEN -o sync.yaml
gz-git sync config merge --provider gitlab --org another-group --into sync.yaml --mode append
gz-git sync config init -o sample.yaml
gz-git sync config validate -c sync.yaml
```

### Watch

```bash
gz-git watch
gz-git watch /path/to/repo1 /path/to/repo2 --interval 5s --format llm
```

### Stash / Tag

```bash
gz-git stash save . -m "WIP: before refactor"
gz-git stash list .
gz-git stash pop .

gz-git tag list .
gz-git tag create v1.0.0 . -m "Release 1.0.0"
gz-git tag auto . --bump=patch
gz-git tag push .
```

