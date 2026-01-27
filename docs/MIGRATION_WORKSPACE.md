# Migration Guide: sync → workspace (and forge)

This document describes the migration from `gz-git sync from-config` to `gz-git workspace`.

## Overview

In the latest release, config-based repository management has been moved from the historical `sync` command to a new `workspace` command. Forge API operations now live under `forge`.

## Command Mapping

| Old Command                       | New Command                     | Description              |
| --------------------------------- | ------------------------------- | ------------------------ |
| `gz-git sync from-config -c FILE` | `gz-git workspace sync -c FILE` | Sync repos from config   |
| `gz-git sync config init`         | `gz-git workspace init`         | Create sample config     |
| `gz-git sync config scan DIR`     | `gz-git workspace scan DIR`     | Scan and generate config |
| `gz-git sync config validate`     | `gz-git workspace validate`     | Validate config file     |
| N/A                               | `gz-git workspace add URL`      | Add repo to config       |
| N/A                               | `gz-git workspace status`       | Check workspace health   |

## What Stays in `forge`

The following commands are available for Git Forge API operations:

```bash
# Sync directly from GitLab/GitHub/Gitea organization
gz-git forge from-forge --provider gitlab --org devbox --path ./repos

# Generate config from forge (for later use with workspace)
gz-git forge config generate --provider gitlab --org devbox -o .gz-git.yaml

# Check repository health (remote-focused)
gz-git forge status --path ~/repos
```

## Migration Examples

### Before (Old Commands)

```bash
# Create config
gz-git sync config init

# Scan directory
gz-git sync config scan ~/mydevbox -o .gz-git.yaml

# Validate config
gz-git sync config validate -c .gz-git.yaml

# Sync repositories
gz-git sync from-config -c .gz-git.yaml
```

### After (New Commands)

```bash
# Create config
gz-git workspace init

# Scan directory
gz-git workspace scan ~/mydevbox -c .gz-git.yaml

# Validate config
gz-git workspace validate -c .gz-git.yaml

# Sync repositories
gz-git workspace sync -c .gz-git.yaml

# New: Add a repo to config
gz-git workspace add https://github.com/user/repo.git

# New: Check workspace status
gz-git workspace status
```

## Config File Compatibility

**Config files are fully compatible.** Use `.gz-git.yaml` with `workspace` commands; for forge operations, use `forge status -c .gz-git.yaml` for health checks.

Example config:

```yaml
strategy: reset
parallel: 4
maxRetries: 3
cloneProto: ssh
repositories:
  - name: my-repo
    url: https://github.com/user/repo.git
    targetPath: ./repos/my-repo
```

## Typical Workflows

### Workflow 1: Config-Based Workspace Management

```bash
# Initialize workspace
gz-git workspace init

# Add repositories
gz-git workspace add https://github.com/user/repo1.git
gz-git workspace add https://github.com/user/repo2.git

# Sync workspace
gz-git workspace sync

# Check status
gz-git workspace status
```

### Workflow 2: Generate from Forge, Manage Locally

```bash
# Generate config from GitLab organization
gz-git forge config generate --provider gitlab --org myorg -o .gz-git.yaml

# Edit config as needed, then sync
gz-git workspace sync

# Add additional repos manually
gz-git workspace add https://github.com/other/repo.git
```

### Workflow 3: Scan Existing Directory

```bash
# Scan existing git repos
gz-git workspace scan ~/mydevbox

# Review and edit generated config
vim .gz-git.yaml

# Sync based on config
gz-git workspace sync
```

## Breaking Changes

1. **`sync from-config` removed** - Use `workspace sync` instead
1. **`sync config init` removed** - Use `workspace init` instead
1. **`sync config scan` removed** - Use `workspace scan` instead
1. **`sync config validate` removed** - Use `workspace validate` instead
1. **`sync config merge` removed** - Planned for future `workspace merge`

## FAQ

**Q: Will my existing scripts break?**
A: Yes, you need to update scripts that use `sync from-config` or `sync config` subcommands.

**Q: Do I need to recreate my config files?**
A: No, config file format is unchanged.

**Q: What's the benefit of this change?**
A: Clear separation of concerns:

- `sync` = Git Forge API operations (remote-first)
- `workspace` = Local config-based management (local-first)

**Q: How do I add a repo that's not in a Forge?**
A: Use `gz-git workspace add <URL>` to add any Git repository to your config.

## Support

For issues or questions, please file an issue at:
https://github.com/gizzahub/gzh-cli-gitforge/issues
