# Success Metrics

## Success Metrics

How we know gzh-cli-gitforge is successful:

### M1: Performance - 30% Faster Git Operations

| Metric | Target | Current | Status |
|--------|--------|---------|--------|
| Common ops p50 latency | < 50ms | TBD | 🔄 |
| Common ops p95 latency | < 100ms | TBD | 🔄 |
| Common ops p99 latency | < 500ms | TBD | 🔄 |
| Bulk status (10 repos) | < 2s | TBD | 🔄 |

**Common operations**: status, branch list, commit, fetch

### M2: Consistency - 90% Commit Message Compliance

| Metric | Target | Current | Status |
|--------|--------|---------|--------|
| Commits following template | ≥ 90% | TBD | 🔄 |
| Conventional commit format | ≥ 90% | TBD | 🔄 |
| Branch naming compliance | ≥ 85% | TBD | 🔄 |

### M3: Adoption - 50% Parallel Workflow Usage

| Metric | Target | Current | Status |
|--------|--------|---------|--------|
| Weekly worktree users | ≥ 50% of active | TBD | 🔄 |
| Weekly bulk command users | ≥ 50% of active | TBD | 🔄 |
| Multi-repo workflows | ≥ 30% of sessions | TBD | 🔄 |

### M4: Integration - 100% gzh-cli Coverage

| Metric | Target | Current | Status |
|--------|--------|---------|--------|
| gzh-cli Git operations via library | 100% | TBD | 🔄 |
| Library API coverage | 100% public APIs | TBD | 🔄 |
| Integration test coverage | ≥ 80% | TBD | 🔄 |

### M5: Reliability - 99% Sync Success Rate

| Metric | Target | Current | Status |
|--------|--------|---------|--------|
| Org sync success rate | ≥ 99% | TBD | 🔄 |
| User sync success rate | ≥ 99% | TBD | 🔄 |
| Fork sync success rate | ≥ 98% | TBD | 🔄 |
| Recovery from network errors | ≥ 95% | TBD | 🔄 |

### M6: Quality - Test Coverage Targets

| Package | Target | Current | Status |
|---------|--------|---------|--------|
| internal/* | ≥ 80% | 93.6% (gitcmd) | ✅ |
| pkg/* | ≥ 85% | Mixed | 🔄 |
| cmd/* | ≥ 70% | TBD | 🔄 |

## Measurement Plan

### Performance Measurement

| Method | Frequency | Tool |
|--------|-----------|------|
| Benchmark suite | Per commit | `go test -bench` |
| p50/p95/p99 tracking | Weekly | Custom benchmark runner |
| Regression detection | Per PR | CI benchmark comparison |

**Benchmark commands**:
```bash
make benchmark          # Run all benchmarks
make benchmark-report   # Generate report
```

### Consistency Measurement

| Method | Frequency | Tool |
|--------|-----------|------|
| Commit message validation | Per commit | Pre-commit hook |
| Template compliance | Weekly | Log analysis |
| Format report | Monthly | Custom script |

### Adoption Measurement

| Method | Frequency | Tool |
|--------|-----------|------|
| Command usage (opt-in) | Daily | Telemetry (opt-in) |
| Feature usage patterns | Weekly | Aggregated analytics |
| User surveys | Quarterly | Forms |

**Privacy**: All telemetry is opt-in with clear disclosure.

### Reliability Measurement

| Method | Frequency | Tool |
|--------|-----------|------|
| Sync success/failure logs | Per run | Structured logging |
| Error categorization | Daily | Log aggregation |
| Recovery rate tracking | Weekly | Retry success analysis |

### Quality Measurement

| Method | Frequency | Tool |
|--------|-----------|------|
| Unit test coverage | Per commit | `go test -cover` |
| Integration test runs | Per PR | CI pipeline |
| E2E test suite | Daily | Scheduled CI |

**Coverage commands**:
```bash
make test-coverage      # Generate coverage report
make coverage-report    # View detailed report
```
