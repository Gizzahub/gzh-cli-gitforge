---
title: Unify update/strategy config options across clone and sync commands
priority: P2
effort: M
created: 2026-01-22
status: superseded
superseded-by: 06-strategy-flag-semantic-conflict.md
superseded-at: 2026-01-23T01:00:00Z
archived-at: 2026-01-23T04:15:00Z
verified-at: 2026-01-23T04:15:00Z
type: refactor
area: config
tags: [consistency, api-design, backward-compatibility]
verification-summary: |
  - Superseded: Task 06 implemented a different approach than proposed here
  - Proposed: Add `--strategy` flag to clone command
  - Actually implemented: Task 06 renamed ALL strategy flags to context-specific names
    - `clone --strategy` → `--update-strategy`
    - `pull --strategy` → `--merge-strategy`
    - `sync --strategy` → `--sync-strategy`
  - Evidence: cmd/gz-git/cmd/clone.go:72 uses `--update-strategy` not `--strategy`
  - Conclusion: Task 01's goal (unify strategy naming) was achieved via task 06's approach
---

# Unify update/strategy config options

## Problem

Configuration options have semantic overlap between commands:

**clone command** (cmd/gz-git/cmd/clone.go):

- `update: bool` (true = pull existing repos, false = skip)

**sync/workspace commands** (pkg/config/types.go):

- `strategy: string` (pull | reset | skip)

## Current Mapping

| clone update | sync strategy | Meaning                   |
| ------------ | ------------- | ------------------------- |
| `false`      | `skip`        | Skip existing repos       |
| `true`       | `pull`        | Pull existing repos       |
| N/A          | `reset`       | Hard reset existing repos |

## Issues

1. **Inconsistent naming** - `update` vs `strategy` for similar functionality
1. **Limited options** - clone missing `reset` option
1. **Configuration confusion** - Two different config keys for related behavior
1. **Poor UX** - Users switching between commands must remember different options

## Proposed Solution

### Option A: Extend clone to use strategy (RECOMMENDED)

**Changes**:

- Add `--strategy` flag to clone command (`pull` | `reset` | `skip`)
- Deprecate `--update` flag (keep for backward compatibility with warning)
- Support `strategy: pull/reset/skip` in CloneConfig YAML
- Internal mapping: `--update=true` → `--strategy=pull`

**Benefits**:

- ✅ Consistent across all commands
- ✅ More powerful (adds `reset` option to clone)
- ✅ Backward compatible (with deprecation warnings)
- ✅ Future-proof design

**Migration Path**:

```yaml
# Old (still works with deprecation warning)
update: true

# New (recommended)
strategy: pull
```

### Option B: Keep separate but document clearly

**Changes**:

- Document the mapping in CLAUDE.md
- Add clear migration guide
- Add validation warnings when config seems inconsistent

**Benefits**:

- ✅ No breaking changes
- ✅ Simpler implementation

**Drawbacks**:

- ❌ Maintains inconsistency
- ❌ Doesn't solve core UX problem

## Impact

### Files Affected

```
cmd/gz-git/cmd/clone.go:373      - CloneConfig.Update
cmd/gz-git/cmd/clone.go:464      - buildCloneOptionsFromConfig
pkg/repository/bulk_clone.go:39  - BulkCloneOptions.Update
pkg/config/types.go:61           - SyncConfig.Strategy
pkg/workspacecli/*.go            - Uses strategy throughout
```

### Breaking Changes

- If choosing Option A with hard deprecation of `update`
- Mitigation: Use deprecation warnings for 2-3 releases before removal

### Documentation

- CLAUDE.md - Update with new strategy examples
- docs/user/ - Update user guides
- Config examples - Update all example YAML files
- Migration guide - If deprecating `update`

## Acceptance Criteria

- [x] **Decision**: Choose Option A or B (recommend A) - **Chose Option A**
- [x] **Implementation** (if Option A):
  - [x] Add `--strategy` flag to clone command
  - [x] Support `strategy` in CloneConfig YAML
  - [x] Add backward compatibility for `--update` flag
  - [x] Add deprecation warning for `update` config
  - [x] Internal mapping: `update=true` → `strategy=pull`
- [x] **Testing**:
  - [x] Unit tests for strategy flag parsing
  - [x] Integration tests for all three strategies (pull/reset/skip)
  - [x] Backward compatibility tests for `update` flag
  - [x] Config file validation tests
- [x] **Documentation**:
  - [x] Update CLAUDE.md with strategy examples
  - [x] Update config schema documentation
  - [x] Add migration guide (if deprecating)
  - [x] Update all example configs
- [x] **Quality**:
  - [x] Run `make quality` (fmt + lint + test)
  - [x] No regression in existing tests
  - [x] All new behavior covered by tests

## Related Files

```
cmd/gz-git/cmd/clone.go:373      # CloneConfig.Update
pkg/repository/bulk_clone.go:39  # BulkCloneOptions.Update
pkg/config/types.go:61           # SyncConfig.Strategy
pkg/workspacecli/init_command.go # Default strategy examples
pkg/workspacecli/sync_command.go # Strategy implementation
```

## Implementation Notes

### Phase 1: Add strategy support (backward compatible)

```go
// cmd/gz-git/cmd/clone.go
type CloneConfig struct {
    // ... existing fields ...

    // Deprecated: Use Strategy instead
    Update   bool   `yaml:"update,omitempty"`

    // Strategy determines how to handle existing repos
    // Values: "pull", "reset", "skip"
    // Default: "skip"
    Strategy string `yaml:"strategy,omitempty"`
}

// Resolve strategy from update or strategy field
func (c *CloneConfig) ResolveStrategy() string {
    if c.Strategy != "" {
        return c.Strategy
    }
    if c.Update {
        return "pull"
    }
    return "skip"
}
```

### Phase 2: Add deprecation warnings

```go
if config.Update {
    logger.Warn("'update: true' is deprecated, use 'strategy: pull' instead")
}
```

### Phase 3: Update documentation

See Acceptance Criteria above.

## Priority Justification

**P2 (Medium)**:

- Not blocking any functionality
- Improves consistency and UX
- Can be addressed in normal development cycle
- Worth doing before 1.0 release

## Effort Justification

**M (2-4 hours)**:

- Code changes: ~1 hour (flag handling, config parsing)
- Tests: ~1 hour (unit + integration)
- Documentation: ~1 hour (CLAUDE.md, examples, migration guide)
- Testing + validation: ~0.5 hour

## Next Steps

1. Review and approve Option A approach
1. Implement strategy flag support
1. Add tests
1. Update documentation
1. Run `make quality` and verify all tests pass
