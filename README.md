# gzh-cli-gitforge

> Bulk-first Git operations CLI (`gz-git`) + Go library

[![Go Version](https://img.shields.io/badge/go-1.25.1%2B-blue)](https://go.dev)
[![Version](https://img.shields.io/badge/version-v3.0.0-blue)](https://github.com/gizzahub/gzh-cli-gitforge/releases/tag/v3.0.0)
[![CI](https://github.com/gizzahub/gzh-cli-gitforge/actions/workflows/ci.yml/badge.svg)](https://github.com/gizzahub/gzh-cli-gitforge/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![GoDoc](https://pkg.go.dev/badge/github.com/gizzahub/gzh-cli-gitforge.svg)](https://pkg.go.dev/github.com/gizzahub/gzh-cli-gitforge)

`gzh-cli-gitforge` provides:

- **`gz-git` (CLI)**: scan directories and run safe Git operations across many repositories in parallel.
- **Go library**: reusable packages under `pkg/` for repository operations, bulk workflows, and forge sync.

______________________________________________________________________

## CLI Highlights (`gz-git`)

- Bulk operations with `--scan-depth` and `--parallel`: `status`, `fetch`, `pull`, `push`, `update`, `diff`, `commit`, `switch`
- Bulk clone: `clone --url ...` / `clone --file ...` (+ `--update`)
- Git forge operations:
  - `forge from-forge` (GitHub/GitLab/Gitea org/group/user)
  - `forge config generate` → then `workspace sync` (YAML config workflow)
- Maintenance: `cleanup branch` (dry-run by default)
- Monitoring: `watch` (default/compact/json/llm)
- Insights: `history` (stats/contributors/file/blame), `info`, `conflict detect`
- Tag/stash helpers: `tag`, `stash`

______________________________________________________________________

## Quick Start

### Install

```bash
go install github.com/gizzahub/gzh-cli-gitforge/cmd/gz-git@latest
gz-git --version
```

### Common Workflows

```bash
# Bulk status (current directory + 1 level)
gz-git status

# Fetch everything under ~/projects (2 levels deep)
gz-git fetch -d 2 ~/projects

# Update repos (pull --rebase), continuously
gz-git update --watch --interval 5m -d 2 ~/projects

# Bulk commit (preview → apply)
gz-git commit -d 2 ~/projects
gz-git commit --yes -d 2 ~/projects

# Switch branch across repos (create if missing)
gz-git switch feature/foo --create -d 2 ~/projects

# Bulk clone into ~/projects
gz-git clone ~/projects --url https://github.com/user/repo1.git --url https://github.com/user/repo2.git

# Sync all repos from a GitHub org
gz-git forge from-forge --provider github --org myorg --path ./repos --token $GITHUB_TOKEN
```

______________________________________________________________________

## Requirements

- Git 2.30+
- Go 1.25.1+ (building from source / using as a library)

______________________________________________________________________

## Documentation

- [Docs index](docs/README.md)
- [5-minute quick start (Korean)](QUICK_START.md)
- [Command reference (curated)](docs/commands/README.md)
- [Watch command guide](docs/commands/watch.md)
- [Go library usage](docs/user/getting-started/library-usage.md)
- API reference: https://pkg.go.dev/github.com/gizzahub/gzh-cli-gitforge

______________________________________________________________________

## License

MIT. See `LICENSE`.
