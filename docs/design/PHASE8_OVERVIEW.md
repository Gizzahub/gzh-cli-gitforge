# Phase 8: Advanced Features - Overview

## Status

**Phase**: Phase 8 (PLANNED)
**Priority**: Post-v0.4.0, requires Phase 7 completion
**Goal**: Enhanced user experience through interactive and extensible features

## Background

Phase 8 focuses on improving developer experience beyond core Git operations. While Phase 1-7 established solid foundations for multi-repo management, Phase 8 adds convenience features that reduce cognitive load and enable customization.

**Entry Criteria**:
- ✅ Phase 7 complete (v0.3.0+ stable)
- ✅ User feedback collected
- ✅ Core features battle-tested
- ❓ Community adoption metrics met

**Current Status** (v0.4.0):
- Phase 7 substantially complete
- Basic multi-repo operations working well
- Ready for UX enhancements

## Three Feature Pillars

### 1. Config Profiles (P2)
**Goal**: Simplify multi-context workflows (work/personal, different forges)

**Problem**:
```bash
# Today: Repeat flags for every command
gz-git sync from-forge --provider gitlab --base-url https://work.gitlab.com \
  --org backend --token $WORK_TOKEN --clone-proto ssh --ssh-port 2224

gz-git sync from-forge --provider github --org personal-projects \
  --token $PERSONAL_TOKEN --clone-proto https
```

**Solution**: Named profiles + auto-detection
```bash
# One-time setup
gz-git config profile create work --provider gitlab --base-url ...
gz-git config profile create personal --provider github ...

# Usage
gz-git sync from-forge --profile work --org backend
```

**Design**: [CONFIG_PROFILES.md](CONFIG_PROFILES.md)

---

### 2. Advanced TUI (P1)
**Goal**: Rich terminal UI for complex operations

**Problem**: Text-only output for bulk operations lacks interactivity
```bash
$ gz-git status
Repository: repo1 (clean, ahead 2)
Repository: repo2 (dirty, 3 files modified)
Repository: repo3 (clean, behind 5)
...
# No way to: select repos, batch operations, real-time updates
```

**Solution**: Interactive TUI with selection, filtering, live updates
```
┌─ gz-git status ──────────────────────────────────┐
│ [x] repo1    main     ↑2 ↓1   Clean            │
│ [ ] repo2    develop  ↑0 ↓5   Dirty (3 files)  │
│ [x] repo3    main     ↑1 ↓0   Clean            │
│                                                  │
│ Space: Toggle  Enter: Details  s: Sync selected │
└──────────────────────────────────────────────────┘
```

**Design**: [ADVANCED_TUI.md](ADVANCED_TUI.md)

---

### 3. Interactive Mode (P2)
**Goal**: Guided workflows for common tasks

**Problem**: Users need to know all flags upfront
```bash
# Complex command - easy to make mistakes
gz-git sync from-forge --provider gitlab --org ... --base-url ... --token ...
```

**Solution**: Wizard-style prompts
```bash
$ gz-git sync setup

? Select forge provider: GitLab
? Organization name: devbox
? API token: ********************
? Target directory: ~/repos
? Include subgroups? Yes (flat mode)

✓ Configuration saved! Run: gz-git sync from-config
```

**Design**: [INTERACTIVE_MODE.md](INTERACTIVE_MODE.md)

---

## Feature Priorities

| Feature | Priority | Complexity | User Impact | Dependencies |
|---------|----------|------------|-------------|--------------|
| **Advanced TUI** | P1 | High | High - visual clarity | Bubble Tea library |
| **Config Profiles** | P2 | Medium | High - DX improvement | Config library (viper) |
| **Interactive Mode** | P2 | Medium | Medium - onboarding | Prompt library (survey) |

**Rationale**:
- **TUI first** (P1): Biggest UX improvement for existing users
- **Profiles + Interactive** (P2): Reduce friction for new users

## Implementation Strategy

### Phase 8.1: Advanced TUI (P1)
**Target**: 2-3 weeks
1. Evaluate TUI frameworks (Bubble Tea vs tview)
2. Implement interactive status view
3. Add batch operations (select → sync)
4. Real-time progress updates

**Deliverable**: `gz-git status --interactive`

### Phase 8.2: Config Profiles (P2)
**Target**: 1-2 weeks
1. Design config file format
2. Implement profile CRUD operations
3. Add auto-detection (.gz-git.yaml in project)
4. Integrate with existing commands

**Deliverable**: `gz-git config profile` commands

### Phase 8.3: Interactive Mode (P2)
**Target**: 1-2 weeks
1. Integrate prompt library
2. Implement sync setup wizard
3. Add cleanup wizard (branch selection)
4. Expand to other commands

**Deliverable**: `gz-git sync setup` wizard

**Total Estimate**: 4-7 weeks (with testing/docs)

## Feature Relationships

```
Config Profiles
    ↓ (provides defaults)
Interactive Mode
    ↓ (configures)
Advanced TUI
    ↓ (uses for display/selection)
```

**Synergies**:
- Interactive wizards can create profiles
- TUI can show profile-based configs
- Profiles provide defaults for TUI operations

## Success Metrics

### Quantitative
- TUI adoption: 40%+ of users use `--interactive` flag
- Profile usage: 60%+ of users create at least one profile
- Wizard completion: 80%+ complete wizard without errors

### Qualitative
- User feedback: "Much easier to use"
- Reduced support questions about complex flags
- Positive sentiment on GitHub/Reddit

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| TUI framework choice wrong | High | Prototype both, user testing |
| Config format conflicts | Medium | Versioning, migration tools |
| Feature bloat | Medium | Keep features optional (flags) |

## Decision Points

### TUI Framework (Phase 8.1 start)
**Options**:
1. **Bubble Tea** (Charm.sh) - Modern, popular, rich ecosystem
2. **tview** - Mature, stable, widget-based
3. **Custom** - Full control, high maintenance

**Recommendation**: Bubble Tea (modern, active development)

## Dependencies

### External Libraries
- **TUI**: `github.com/charmbracelet/bubbletea`
- **Config**: `github.com/spf13/viper`
- **Prompts**: `github.com/AlecAivazis/survey/v2`

### Internal
- All features depend on stable core (Phase 7 complete)

## Exit Criteria (Phase 8 Complete)

- ✅ All 3 features implemented and documented
- ✅ User acceptance testing passed
- ✅ Performance regression tests passed
- ✅ Migration guide for existing users
- ✅ Ready for v1.0.0 API stability commitment

## References

- [Roadmap](../00-product/06-roadmap.md) - High-level phases
- [Vision](../00-product/01-vision.md) - Product direction
- [Principles](../00-product/02-principles.md) - Design principles

**Detailed Designs**:
- [CONFIG_PROFILES.md](CONFIG_PROFILES.md)
- [ADVANCED_TUI.md](ADVANCED_TUI.md)
- [INTERACTIVE_MODE.md](INTERACTIVE_MODE.md)

---

**Version**: 1.0
**Last Updated**: 2026-01-16
**Status**: Design in progress
