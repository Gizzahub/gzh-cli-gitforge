# gzh-cli-gitforge

> Git platform management CLI for GitHub, GitLab, and Gitea

[![Go Version](https://img.shields.io/badge/go-1.24.0%2B-blue)](https://go.dev)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

**gzh-cli-gitforge** is a CLI tool and Go library for managing Git platforms (GitHub, GitLab, Gitea). It provides unified commands for syncing repositories from organizations/groups, managing webhooks, and cross-platform operations.

---

## Features

- **Multi-Platform Support**: GitHub, GitLab, Gitea (Gogs planned)
- **Organization Sync**: Clone/update all repositories from an organization or group
- **Parallel Operations**: Configurable parallel processing for bulk operations
- **Unified Interface**: Same commands work across all platforms
- **Library API**: Use as a Go library in your own projects

---

## Quick Start

### Installation

**Via Go Install:**
```bash
go install github.com/gizzahub/gzh-cli-gitforge/cmd/gitforge@latest
```

**From Source:**
```bash
git clone https://github.com/gizzahub/gzh-cli-gitforge.git
cd gzh-cli-gitforge
make build
make install
```

### Requirements

- Go 1.24+
- Git 2.30+

---

## Usage

### Sync GitHub Organization

```bash
# Set token (or use config file)
export GITHUB_TOKEN=your_token

# Sync all repos from an organization
gz-gitforge sync github myorg --target ~/repos

# Dry run to see what would be synced
gz-gitforge sync github myorg --target ~/repos --dry-run

# Parallel operations
gz-gitforge sync github myorg --target ~/repos --parallel 10
```

### Sync GitLab Group

```bash
export GITLAB_TOKEN=your_token

# Sync all projects from a group
gz-gitforge sync gitlab mygroup --target ~/repos
```

### Sync Gitea Organization

```bash
export GITEA_TOKEN=your_token

# Sync all repos from a Gitea org
gz-gitforge sync gitea myorg --target ~/repos
```

### Configuration File

Create `~/.config/gzh-gitforge/config.yaml`:

```yaml
github:
  token: ${GITHUB_TOKEN}
  # base_url: https://github.mycompany.com/api/v3  # For GitHub Enterprise

gitlab:
  token: ${GITLAB_TOKEN}
  base_url: https://gitlab.com

gitea:
  token: ${GITEA_TOKEN}
  base_url: https://gitea.mycompany.com

sync:
  target_path: ~/repos
  parallel: 4
  include_archived: false
  include_forks: false
  include_private: true
```

---

## Architecture

```
gzh-cli-gitforge/
├── cmd/gitforge/        # CLI entry point
│   └── cmd/             # Cobra commands
├── pkg/                 # Public library API
│   ├── provider/        # Provider interface
│   ├── github/          # GitHub implementation
│   ├── gitlab/          # GitLab implementation
│   ├── gitea/           # Gitea implementation
│   └── sync/            # Sync operations
└── internal/            # Private implementation
    └── config/          # Configuration handling
```

### Integration with gzh-cli-git

gzh-cli-gitforge uses [gzh-cli-git](https://github.com/gizzahub/gzh-cli-git) as the Git engine for clone/pull operations:

```
gz-gitforge (platform API)  →  gzh-cli-git (Git operations)
     │                              │
     ├── List repos from org        ├── Clone repository
     ├── Get repo metadata          ├── Pull/fetch updates
     └── Manage webhooks            └── Detect repo state
```

---

## Development

### Prerequisites

- Go 1.24+
- Make

### Build

```bash
# Build binary
make build

# Run tests
make test

# Run linters
make lint

# All quality checks
make quality
```

### Project Structure

```
gzh-cli-gitforge/
├── cmd/gitforge/     # CLI application
├── pkg/              # Public library API
├── internal/         # Internal implementation
├── docs/             # Documentation
├── examples/         # Usage examples
└── Makefile          # Build automation
```

---

## Related Projects

- [gzh-cli](https://github.com/gizzahub/gzh-cli) - Unified developer CLI
- [gzh-cli-git](https://github.com/gizzahub/gzh-cli-git) - Git automation library

---

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
