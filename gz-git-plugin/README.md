# gz-git Plugin for Claude Code

gz-git CLI integration - Safe Git operations with bulk repository support.

## Installation

```bash
/plugin marketplace add gizzahub/gz-git
/plugin install gz-git@gz-git-marketplace
```

## Requirements

- gz-git binary: `go install github.com/gizzahub/gzh-cli-gitforge/cmd/gz-git@latest`
- Git 2.20+

## Skills

| Skill | Description |
|-------|-------------|
| **gz-git** | Single repository operations - commit, branch, tag, stash |
| **gz-git-bulk** | Multi-repository operations - bulk clone, fetch, pull, push |

## Local Test

```bash
claude --plugin-dir ./gz-git-plugin
```

## Quick Reference

### Single Repo
```bash
gz-git status
gz-git commit auto
gz-git branch cleanup
```

### Multi-Repo
```bash
gz-git clone --org mycompany --provider github
gz-git fetch .
gz-git status . --dirty-only
```
