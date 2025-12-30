# Quick Start

Get `gz-git` running in 5 minutes.

## Prerequisites

- Go 1.23+
- Git 2.30+
- Linux/macOS/Windows (amd64/arm64)

## Installation

### Option 1: From Source

```bash
# Clone and build
git clone https://github.com/gizzahub/gzh-cli-gitforge.git
cd gzh-cli-gitforge
make build
sudo make install

# Verify
gz-git --version
```

### Option 2: Using Go

```bash
go install github.com/gizzahub/gzh-cli-gitforge/cmd/gz-git@latest
```

## Basic Usage

### Check Repository Status

```bash
gz-git status
```

### Commit with Template

```bash
gz-git commit -m "feat(api): add user authentication"
```

### Sync Multiple Repositories

```bash
gz-git sync --org myorg --provider github
```

## Verify It's Working

```bash
# Should show available commands
gz-git --help

# Should show repository info
cd /path/to/your/repo
gz-git status
```

## Next Steps

- [Full Documentation](docs/)
- [Development Guide](CLAUDE.md)
- [Product Goals](PRODUCT.md)
- [Library Usage](docs/_deprecated/2025-12/LIBRARY.md) (to be migrated)
