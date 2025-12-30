# Roadmap

## Phases

### Phase 1-6: Foundation (COMPLETE) ✅

**Status**: Complete (v0.1.0 - v0.2.0)

| Phase | Focus | Deliverables | Status |
|-------|-------|--------------|--------|
| 1 | Core Setup | Project structure, CI/CD, basic CLI | ✅ |
| 2 | Git Operations | Status, branch, commit, history | ✅ |
| 3 | Library Design | pkg/* architecture, interfaces | ✅ |
| 4 | Multi-Repo | Bulk operations, parallel execution | ✅ |
| 5 | Repo Sync | GitHub/GitLab sync, fork management | ✅ |
| 6 | Integration | Testing, documentation, stabilization | ✅ |

### Phase 7: Release & Adoption (CURRENT) 🔄

**Goal**: Public release and gzh-cli integration

| Milestone | Deliverables | Target |
|-----------|--------------|--------|
| M7.1 | v0.3.0 release candidate | Q1 |
| M7.2 | gzh-cli full integration | Q1 |
| M7.3 | Documentation completion | Q1 |
| M7.4 | Community feedback integration | Q2 |
| M7.5 | v0.3.0 stable release | Q2 |

**Exit criteria**:
- All quality gates passing
- gzh-cli using 100% library operations
- Documentation complete and reviewed
- No critical bugs open

### Phase 8: Advanced Features (PLANNED)

**Goal**: Enhanced user experience

| Feature | Description | Priority |
|---------|-------------|----------|
| Advanced TUI | Rich terminal UI for complex operations | P1 |
| Interactive mode | Guided workflows for common tasks | P2 |
| Config profiles | Per-project and global settings | P2 |
| Plugin system | Extensible command architecture | P3 |

**Entry criteria**: Phase 7 complete, user feedback collected

### Phase 9: Performance & Scale (PLANNED)

**Goal**: Enterprise-grade reliability

| Feature | Description | Priority |
|---------|-------------|----------|
| Performance optimization | Sub-50ms p95 for common ops | P1 |
| Large repo support | 10k+ files, 100k+ commits | P1 |
| Caching layer | Intelligent result caching | P2 |
| Concurrent safety | Thread-safe library operations | P2 |

### Phase 10: Ecosystem Growth (PLANNED)

**Goal**: Broader adoption and integration

| Feature | Description | Priority |
|---------|-------------|----------|
| Additional forge support | Bitbucket, Azure DevOps | P2 |
| Submodule support | Full submodule workflow | P3 |
| LFS integration | Large file storage operations | P3 |
| Community plugins | Plugin marketplace | P3 |

## Milestones

### Near-term (Phase 7)

| Milestone | Description | Target | Status |
|-----------|-------------|--------|--------|
| v0.3.0-rc1 | Release candidate | Q1 2025 | 🔄 |
| gzh-cli integration | Full library usage | Q1 2025 | 🔄 |
| 100 GitHub stars | Community validation | Q2 2025 | 📋 |
| v0.3.0 stable | Production ready | Q2 2025 | 📋 |

### Medium-term (Phase 8-9)

| Milestone | Description | Target | Status |
|-----------|-------------|--------|--------|
| v0.4.0 | Advanced features | Q3 2025 | 📋 |
| v0.5.0 | Performance release | Q4 2025 | 📋 |
| 500 GitHub stars | Growing adoption | Q4 2025 | 📋 |

### Long-term (Phase 10+)

| Milestone | Description | Target | Status |
|-----------|-------------|--------|--------|
| v1.0.0 | Stable API guarantee | 2026 | 📋 |
| 1000 GitHub stars | Established project | 2026 | 📋 |
| Enterprise adoption | Production use at scale | 2026+ | 📋 |

## Decision Points

| Decision | When | Options |
|----------|------|---------|
| TUI framework | Phase 8 start | Bubble Tea, tview, custom |
| Plugin architecture | Phase 8 mid | Go plugins, subprocess, RPC |
| Additional forges | Phase 10 start | Bitbucket, Azure, Gitea |
| API stability | Pre-v1.0.0 | Semantic versioning commitment |

## Legend

| Symbol | Meaning |
|--------|---------|
| ✅ | Complete |
| 🔄 | In progress |
| 📋 | Planned |
| ⏸️ | On hold |
| ❌ | Cancelled |
