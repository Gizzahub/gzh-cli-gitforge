# Roadmap

## Phases

### Phase 1-6: Foundation (COMPLETE) ✅

**Status**: Complete (v0.1.0 - v0.2.0)

| Phase | Focus          | Deliverables                          | Status |
| ----- | -------------- | ------------------------------------- | ------ |
| 1     | Core Setup     | Project structure, CI/CD, basic CLI   | ✅     |
| 2     | Git Operations | Status, branch, commit, history       | ✅     |
| 3     | Library Design | pkg/\* architecture, interfaces       | ✅     |
| 4     | Multi-Repo     | Bulk operations, parallel execution   | ✅     |
| 5     | Repo Sync      | GitHub/GitLab sync, fork management   | ✅     |
| 6     | Integration    | Testing, documentation, stabilization | ✅     |

### Phase 7: Release & Adoption (COMPLETE) ✅

**Goal**: Public release and gzh-cli integration

| Milestone | Deliverables                   | Target  | Status |
| --------- | ------------------------------ | ------- | ------ |
| M7.1      | v0.3.0 release candidate       | Q4 2025 | ✅     |
| M7.2      | gzh-cli full integration       | Q4 2025 | ✅     |
| M7.3      | Documentation completion       | Q4 2025 | ✅     |
| M7.4      | Community feedback integration | Q4 2025 | ✅     |
| M7.5      | v0.4.0 stable release          | Q1 2026 | ✅     |

**Exit criteria**: All met ✅

- All quality gates passing
- gzh-cli using 100% library operations
- Documentation complete and reviewed
- No critical bugs open

### Phase 8: Advanced Features (PARTIAL) 🔄

**Goal**: Enhanced user experience

| Feature           | Description                             | Status   |
| ----------------- | --------------------------------------- | -------- |
| Config profiles   | Per-project and global settings         | ✅ Done  |
| Workspace config  | Recursive hierarchical configuration    | ✅ Done  |
| Advanced TUI      | Rich terminal UI for complex operations | 📋 Plan  |
| Interactive mode  | Guided workflows for common tasks       | 📋 Plan  |

**Entry criteria**: Phase 7 complete ✅

### Phase 9: Performance & Scale (PLANNED)

**Goal**: Enterprise-grade reliability

| Feature                  | Description                    | Priority |
| ------------------------ | ------------------------------ | -------- |
| Performance optimization | Sub-50ms p95 for common ops    | P1       |
| Large repo support       | 10k+ files, 100k+ commits      | P1       |
| Caching layer            | Intelligent result caching     | P2       |
| Concurrent safety        | Thread-safe library operations | P2       |

### Phase 10: Ecosystem Growth (PLANNED)

**Goal**: Broader adoption and integration

| Feature                  | Description                   | Priority |
| ------------------------ | ----------------------------- | -------- |
| Additional forge support | Bitbucket, Azure DevOps       | P2       |
| GitHub Enterprise        | Custom base URL for GitHub provider (GitLab/Gitea already support self-hosted) | P2       |
| Submodule support        | Full submodule workflow       | P3       |
| LFS integration          | Large file storage operations | P3       |

## Milestones

### Completed (Phase 7)

| Milestone           | Description          | Target  | Status |
| ------------------- | -------------------- | ------- | ------ |
| v0.3.0              | Release candidate    | Q4 2025 | ✅     |
| gzh-cli integration | Full library usage   | Q4 2025 | ✅     |
| v0.4.0 stable       | Production ready     | Q1 2026 | ✅     |

### Current (Phase 8)

| Milestone        | Description           | Target  | Status |
| ---------------- | --------------------- | ------- | ------ |
| Config profiles  | Profile management    | Q1 2026 | ✅     |
| Workspace config | Hierarchical config   | Q1 2026 | ✅     |
| v0.5.0           | TUI improvements      | Q2 2026 | 📋     |

### Long-term (Phase 9-10)

| Milestone           | Description             | Target  | Status |
| ------------------- | ----------------------- | ------- | ------ |
| v0.6.0              | Performance release     | Q3 2026 | 📋     |
| v1.0.0              | Stable API guarantee    | Q4 2026 | 📋     |
| Enterprise adoption | Production use at scale | 2027+   | 📋     |

## Decision Points

| Decision          | When           | Options                        |
| ----------------- | -------------- | ------------------------------ |
| TUI framework     | Phase 8 start  | Bubble Tea, tview, custom      |
| Additional forges | Phase 10 start | Bitbucket, Azure, Gitea        |
| API stability     | Pre-v1.0.0     | Semantic versioning commitment |

## Legend

| Symbol | Meaning     |
| ------ | ----------- |
| ✅     | Complete    |
| 🔄     | In progress |
| 📋     | Planned     |
| ⏸️     | On hold     |
| ❌     | Cancelled   |
