# Roadmap

## How to Read This Document

이 문서는 과거 구현 기록과 현재 검증 상태를 구분한다.

- 과거 구현 완료는 현재 stable 게시·설치 검증을 의미하지 않는다.
- 현재 stable 게시·설치 증거는 issue28에서 확인한다. ([28-goreleaser-brews-deprecated-and-ci-pins-latest.md](../../tasks/issue/28-goreleaser-brews-deprecated-and-ci-pins-latest.md))
- 평가 기준: master 81d3543 (2026-10-02)
- adoption: 미측정
- 추가 TUI·notify backend 구현은 미승인 계획이다.

## Phases

### Phase 1-6: Foundation (구현 완료 — 과거 기록)

**Status**: v0.1.0 - v0.2.0 시점에 구현됨. 과거 구현 완료는 현재 stable 게시·설치 검증을 의미하지 않는다.

| Phase | Focus          | Deliverables                          | Status |
| ----- | -------------- | ------------------------------------- | ------ |
| 1     | Core Setup     | Project structure, CI/CD, basic CLI   | 구현   |
| 2     | Git Operations | Status, branch, commit, history       | 구현   |
| 3     | Library Design | pkg/\* architecture, interfaces       | 구현   |
| 4     | Multi-Repo     | Bulk operations, parallel execution   | 구현   |
| 5     | Repo Sync      | GitHub/GitLab sync, fork management   | 구현   |
| 6     | Integration    | Testing, documentation, stabilization | 구현   |

### Phase 7: Release & Adoption (구현 완료, stable 검증은 미완)

**Goal**: Public release and gzh-cli integration

| Milestone | Deliverables                   | Target  | Status |
| --------- | ------------------------------ | ------- | ------ |
| M7.1      | v0.3.0 release candidate       | Q4 2025 | 구현   |
| M7.2      | gzh-cli full integration       | Q4 2025 | 구현   |
| M7.3      | Documentation completion       | Q4 2025 | 구현   |
| M7.4      | Community feedback integration | Q4 2025 | 구현   |
| M7.5      | v0.4.0 stable release          | Q1 2026 | 구현   |

이 표는 과거 구현 완료의 기록이다. 현재 stable 게시·설치 증거는 issue28에서
확인한다. 근거 문서:
[28-goreleaser-brews-deprecated-and-ci-pins-latest.md](../../tasks/issue/28-goreleaser-brews-deprecated-and-ci-pins-latest.md).
게시·설치 검증이 확인될 때까지 이 단계를 stable 달성으로 표시하지 않는다.

### Phase 8: Advanced Features (진행 중) 🔄

**Goal**: Enhanced user experience

| Feature          | Description                             | Status          |
| ---------------- | --------------------------------------- | --------------- |
| Config profiles  | Per-project and global settings         | ✅ Done         |
| Workspace config | Recursive hierarchical configuration    | ✅ Done         |
| Advanced TUI     | Rich terminal UI for complex operations | 📋 Plan(미승인) |
| Interactive mode | Guided workflows for common tasks       | 📋 Plan(미승인) |

추가 TUI·notify backend 구현은 미승인 계획이다. 승인 없이 구현을 추가하지 않는다.

**Entry criteria**: Phase 7은 과거 구현 기록으로만 참조한다.

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

| Feature                  | Description                                                                    | Priority |
| ------------------------ | ------------------------------------------------------------------------------ | -------- |
| Additional forge support | Bitbucket, Azure DevOps                                                        | P2       |
| GitHub Enterprise        | Custom base URL for GitHub provider (GitLab/Gitea already support self-hosted) | P2       |
| Submodule support        | Full submodule workflow                                                        | P3       |
| LFS integration          | Large file storage operations                                                  | P3       |

## Milestones

### Historical (Phase 7 기록)

| Milestone           | Description        | Target  | Status                                   |
| ------------------- | ------------------ | ------- | ---------------------------------------- |
| v0.3.0              | Release candidate  | Q4 2025 | 구현 기록                                |
| gzh-cli integration | Full library usage | Q4 2025 | 구현 기록                                |
| v0.4.0 stable       | Production ready   | Q1 2026 | 구현 기록 (stable 게시·설치 검증은 미완) |

### Current (Phase 8)

| Milestone        | Description         | Target  | Status          |
| ---------------- | ------------------- | ------- | --------------- |
| Config profiles  | Profile management  | Q1 2026 | ✅              |
| Workspace config | Hierarchical config | Q1 2026 | ✅              |
| v0.5.0           | TUI improvements    | Q2 2026 | 📋 Plan(미승인) |

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

| Symbol     | Meaning                                                      |
| ---------- | ------------------------------------------------------------ |
| ✅         | Complete                                                     |
| 🔄         | In progress                                                  |
| 📋         | Planned                                                      |
| 📋(미승인) | Planned but not approved — 승인 전까지 구현하지 않는다       |
| 구현       | Implemented at the recorded time (과거 기록, 현재 검증 아님) |
| ⏸️         | On hold                                                      |
| ❌         | Cancelled                                                    |
