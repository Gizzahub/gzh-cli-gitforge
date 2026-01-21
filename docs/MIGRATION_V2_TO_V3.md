# Migration Guide: v2.x → v3.0

**Date**: 2026-01-21
**Status**: Breaking Changes
**Impact**: Command-line interface, config file naming

---

## Executive Summary

gz-git v3.0 removes legacy workspace management commands and simplifies the configuration system. The underlying data structures (Workspaces map) remain unchanged, but the command-line interface has been cleaned up.

**Key Changes**:
- ❌ Removed 3 workspace management commands
- ❌ Removed 1 deprecated flag from `config init`
- ✅ Simplified command naming (`sync forge` → `sync from-forge`)
- ✅ All existing config files continue to work

---

## Breaking Changes

### 1. Workspace Management Commands (REMOVED)

The following commands have been removed entirely:

| Old Command | Status | Alternative |
|-------------|--------|-------------|
| `gz-git config add-workspace` | ❌ Removed | Edit `.gz-git.yaml` directly |
| `gz-git config list-workspaces` | ❌ Removed | Use `config hierarchy` to view structure |
| `gz-git config remove-workspace` | ❌ Removed | Edit `.gz-git.yaml` directly |

**Migration**: Instead of using workspace commands, manually edit `.gz-git.yaml` files to define your project structure using the `workspaces:` map.

#### Example: Before (v2.x)

```bash
# Old: Use commands to manage workspaces
gz-git config add-workspace devbox ~/mydevbox \
  --provider gitlab --org devbox

gz-git config list-workspaces

gz-git config remove-workspace old-workspace
```

#### Example: After (v3.0)

```bash
# New: Edit config files directly
cat > ~/.gz-git.yaml <<EOF
profile: default
parallel: 10

workspaces:
  devbox:
    path: ~/mydevbox
    type: config
    profile: opensource

  myproject:
    path: ~/myproject
    type: git
EOF

# View hierarchy
gz-git config hierarchy
```

### 2. Sync Command Naming (RENAMED)

| Old Command | New Command | Notes |
|-------------|-------------|-------|
| `gz-git sync forge` | `gz-git sync from-forge` | More explicit naming |
| `gz-git sync run` | `gz-git sync from-config` | Clearer intent |

**Migration**: Update all scripts and aliases to use the new command names.

#### Example: Before (v2.x)

```bash
gz-git sync forge --provider gitlab --org mygroup --target ~/repos
gz-git sync run -c sync.yaml
```

#### Example: After (v3.0)

```bash
gz-git sync from-forge --provider gitlab --org mygroup --target ~/repos
gz-git sync from-config -c sync.yaml
```

### 3. Config Init Flag (REMOVED)

The `--workstation` flag has been removed from `gz-git config init`.

| Old Flag | Status | Alternative |
|----------|--------|-------------|
| `--workstation` | ❌ Removed | Use `--local` or default behavior |

**Migration**:
- For global config: `gz-git config init` (no flag)
- For project config: `gz-git config init --local`

#### Example: Before (v2.x)

```bash
# Initialize workstation config
gz-git config init --workstation
```

#### Example: After (v3.0)

```bash
# Initialize global config directory (new behavior)
gz-git config init

# Or initialize project config
gz-git config init --local
```

---

## Non-Breaking Changes

### Config Files Still Work

**All existing config files continue to work without modification**:

- ✅ `.gz-git.yaml` files (unchanged)
- ✅ `~/.config/gz-git/config.yaml` (unchanged)
- ✅ `~/.config/gz-git/profiles/*.yaml` (unchanged)
- ✅ `workspaces:` map syntax (unchanged)

### Features That Remain

- ✅ Profile management (`config profile create/use/list/delete`)
- ✅ Hierarchical configuration
- ✅ Config hierarchy visualization (`config hierarchy`)
- ✅ Precedence system (flags → project → profile → global → defaults)
- ✅ All sync functionality (`from-forge`, `from-config`, `status`)
- ✅ Discovery modes (explicit, auto, hybrid)

---

## Quick Migration Checklist

### Step 1: Update Scripts and Aliases

```bash
# Find and replace in your scripts
sed -i 's/sync forge/sync from-forge/g' ~/bin/*.sh
sed -i 's/sync run/sync from-config/g' ~/bin/*.sh
sed -i 's/config init --workstation/config init/g' ~/bin/*.sh
```

### Step 2: Remove Workspace Command Usage

If you were using workspace management commands, convert to manual config editing:

1. Run `gz-git config hierarchy` to see current structure
2. Edit `.gz-git.yaml` files directly instead of using commands
3. Use `gz-git config hierarchy --validate` to check for errors

### Step 3: Test Your Workflow

```bash
# Verify config loads correctly
gz-git config show

# Test hierarchy display
gz-git config hierarchy

# Test sync operations
gz-git sync from-config -c sync.yaml --dry-run
```

---

## API Changes (Go Library)

If you're using gz-git as a Go library:

### Removed Functions

The following manager methods have been removed from `pkg/config/Manager`:

```go
// ❌ REMOVED
func (m *Manager) LoadWorkstationConfig() (*Config, error)
func (m *Manager) SaveWorkstationConfig(config *Config) error
func (m *Manager) LoadWorkspaceConfig() (*Config, error)
func (m *Manager) SaveWorkspaceConfig(config *Config) error
```

### Migration

Use the generic methods instead:

```go
// ✅ USE THESE
func (m *Manager) LoadConfigRecursiveFromPath(path string, configFile string) (*Config, error)
func (m *Manager) SaveConfig(path string, configFile string, config *Config) error
```

#### Example: Before (v2.x)

```go
mgr, _ := config.NewManager()

// Load workstation config
cfg, err := mgr.LoadWorkstationConfig()

// Save workstation config
err = mgr.SaveWorkstationConfig(cfg)
```

#### Example: After (v3.0)

```go
mgr, _ := config.NewManager()

// Load config from home directory
home, _ := os.UserHomeDir()
cfg, err := mgr.LoadConfigRecursiveFromPath(home, ".gz-git.yaml")

// Save config
err = mgr.SaveConfig(home, ".gz-git.yaml", cfg)
```

### Removed Files

- `pkg/config/workspace.go` - All types and functions removed

---

## Troubleshooting

### Q: Command not found: add-workspace

**A**: This command was removed in v3.0. Edit `.gz-git.yaml` files directly instead.

### Q: sync forge doesn't work

**A**: The command was renamed. Use `sync from-forge` instead.

### Q: How do I manage workspaces now?

**A**: Edit `.gz-git.yaml` files manually. Use `config hierarchy` to view the structure.

### Q: My old config files stopped working

**A**: They shouldn't! v3.0 is fully backward compatible with existing config files. If you're having issues, please file a bug report.

### Q: What happened to the --workstation flag?

**A**: It was removed. Use `gz-git config init` for global config or `gz-git config init --local` for project config.

---

## Support

If you encounter issues during migration:

1. Check this guide first
2. Run `gz-git config hierarchy --validate` to check for config errors
3. File an issue at https://github.com/gizzahub/gzh-cli-gitforge/issues

---

## Version History

- **v3.0.0** (2026-01-21): Initial v3.0 release with breaking changes
- **v2.x**: Last version with workspace management commands

---

**Last Updated**: 2026-01-21
