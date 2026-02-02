---
id: TASK-001
title: "Forge Repository Filtering (language, stars, activity)"
type: feature

priority: P2
effort: M

parent: null
depends-on: []
blocks: []

created-at: 2026-02-02T00:00:00Z
origin: gizzahub-devbox/TASK-004
phase-0-completed: 2026-02-02T00:00:00Z
---

## Purpose

Add repository filtering options to `forge` commands based on language, star count, and activity level. This enables users to selectively sync repositories from GitHub/GitLab/Gitea based on metadata criteria.

## Background

Migrated from gizzahub-devbox TASK-004 (FR-A02.3). The `forge from` and `forge config generate` commands already support org-level sync but lack metadata-based filtering.

---

## Phase 0: Fit Analysis Result (COMPLETED 2026-02-02)

### 1. Command Structure Analysis

**Current `forge from` flags** (from `pkg/reposynccli/from_forge_command.go`):
```
--provider           Git forge: github, gitlab, gitea [required]
--org                Organization/group name [required]
--path               Target directory [required]
--include-archived   Include archived repositories
--include-forks      Include forked repositories
--include-private    Include private repositories (default: true)
--include-subgroups  Include subgroups (GitLab only)
```

**Existing Pattern**: Boolean include/exclude flags, client-side filtering after API fetch.

### 2. Provider Data Availability

| Field | GitHub | GitLab | Gitea |
|-------|--------|--------|-------|
| **Language** | ✅ `repo.Language` | ❌ Empty string | ❌ Empty string |
| **Stars** | ✅ via `StargazersCount` (not in current types) | ✅ via `StarCount` (not in current types) | ✅ via `Stars` (not in current types) |
| **PushedAt** | ✅ `repo.PushedAt` | ⚠️ `LastActivityAt` (approx) | ⚠️ `Updated` (approx) |
| **Topics** | ✅ `repo.Topics` | ✅ `project.Topics` | ❌ Not exposed |

**Critical Finding**: `provider.Repository` struct lacks `Stars` field. Must add.

### 3. API Filtering Capabilities

| Filter | GitHub API | GitLab API | Gitea API |
|--------|------------|------------|-----------|
| **Language** | ✅ `/search/repositories?q=language:go` | ❌ No API filter (Issue #223065 open) | ❌ No API filter |
| **Stars** | ✅ `stars:>100` in search | ⚠️ Sort by `star_count` only | ⚠️ Limited |
| **Activity** | ✅ `pushed:>2025-01-01` | ⚠️ `last_activity_after` param | ❌ No API filter |

**Conclusion**: API-side filtering only reliable for GitHub. GitLab/Gitea require client-side filtering.

### 4. Recommended Approach

**Decision: Add metadata filter flags to existing commands + client-side filtering**

Reasons:
1. Consistent UX across all providers
2. Minimal API changes (avoid GitHub Search API rate limits: 30/min)
3. Fits existing pattern (`--include-archived`, `--include-forks`)
4. Works with current `ListOrganizationRepos` flow

### 5. Proposed Implementation

#### 5.1 Add Stars to Repository Struct

```go
// pkg/provider/types.go
type Repository struct {
    // ... existing fields ...
    Stars     int       // ← ADD
}
```

Update converters in:
- `pkg/github/provider.go`: `Stars: repo.GetStargazersCount()`
- `pkg/gitlab/provider.go`: `Stars: project.StarCount`
- `pkg/gitea/provider.go`: `Stars: repo.Stars`

#### 5.2 Add Filter Flags

```go
// pkg/reposynccli/from_forge_command.go - FromForgeOptions
type FromForgeOptions struct {
    // ... existing fields ...

    // Metadata filters (NEW)
    FilterLanguage    string   // --language go,rust,python
    FilterMinStars    int      // --min-stars 100
    FilterMaxStars    int      // --max-stars 10000 (0 = unlimited)
    FilterLastPush    string   // --last-push-within 30d, 1y
}
```

#### 5.3 Flags Naming Convention

Following existing patterns:
```
--language <lang>[,lang2]    Filter by primary language (case-insensitive)
--min-stars <n>              Minimum star count (default: 0)
--max-stars <n>              Maximum star count (default: unlimited)
--last-push-within <dur>     Pushed within duration (7d, 30d, 6m, 1y)
```

Examples:
```bash
# Go repos with 100+ stars, active in last 6 months
gz-git forge from --provider github --org kubernetes \
  --language go --min-stars 100 --last-push-within 6m \
  --path ./repos

# Multiple languages
gz-git forge from --provider github --org facebook \
  --language typescript,javascript --min-stars 50 \
  --path ./repos
```

#### 5.4 Client-Side Filter Logic

```go
// pkg/reposynccli/filter.go (NEW FILE)
type MetadataFilter struct {
    Languages     []string      // Lowercase
    MinStars      int
    MaxStars      int           // 0 = unlimited
    LastPushAfter time.Time
}

func (f MetadataFilter) Match(repo *provider.Repository) bool {
    // Language filter
    if len(f.Languages) > 0 {
        repoLang := strings.ToLower(repo.Language)
        if repoLang == "" || !contains(f.Languages, repoLang) {
            return false
        }
    }

    // Stars filter
    if repo.Stars < f.MinStars {
        return false
    }
    if f.MaxStars > 0 && repo.Stars > f.MaxStars {
        return false
    }

    // Activity filter
    if !f.LastPushAfter.IsZero() && repo.PushedAt.Before(f.LastPushAfter) {
        return false
    }

    return true
}
```

### 6. Provider Limitations (IMPORTANT)

| Provider | Language | Workaround |
|----------|----------|------------|
| **GitLab** | Empty string | Must call `/projects/:id/languages` per repo (expensive) OR skip language filter |
| **Gitea** | Empty string | Skip language filter, warn user |

**Recommendation**:
- For GitLab/Gitea: Warn user that `--language` filter may not work
- Future: Add optional `--fetch-languages` flag for GitLab (extra API calls)

### 7. Blockers or Concerns

1. **GitLab Language Detection**: Requires extra API call per repo
2. **Rate Limits**: GitHub Search API = 30/min vs REST = 5000/hr
3. **Large Orgs**: Filtering 1000+ repos client-side is acceptable

### 8. Output Interaction

- `--dry-run`: Shows matched repos with filter summary
- `--format json`: Includes filter metadata in output
- Summary: "Matched 45/120 repositories (filtered: language=go, min-stars=100)"

---

## Implementation Checklist (Updated)

### Phase 1: Data Layer (Day 1)
- [ ] Add `Stars int` to `pkg/provider/types.go`
- [ ] Update `pkg/github/provider.go` converter
- [ ] Update `pkg/gitlab/provider.go` converter
- [ ] Update `pkg/gitea/provider.go` converter
- [ ] Write unit tests for converters

### Phase 2: Filter Logic (Day 1-2)
- [ ] Create `pkg/reposynccli/filter.go`
- [ ] Implement `MetadataFilter` struct
- [ ] Implement `Match()` method
- [ ] Parse duration strings (`30d`, `6m`, `1y`)
- [ ] Write unit tests for filter logic

### Phase 3: CLI Integration (Day 2)
- [ ] Add flags to `forge from` command
- [ ] Add flags to `forge config generate` command
- [ ] Integrate filter into `fetchRepositoriesFromForge`
- [ ] Add filter summary to output
- [ ] Test with `--dry-run`

### Phase 4: Documentation & Polish (Day 3)
- [ ] Update `--help` with examples
- [ ] Add warning for GitLab/Gitea language filter
- [ ] Update `docs/usage/forge-command.md`
- [ ] Run `make quality`

---

## Verification

```bash
# Phase 1: Types
go build ./pkg/provider/...
go test ./pkg/github/... ./pkg/gitlab/... ./pkg/gitea/...

# Phase 2: Filter logic
go test ./pkg/reposynccli/... -run TestMetadataFilter

# Phase 3: Integration
gz-git forge from --provider github --org golang --language go --min-stars 500 --dry-run --path ./test
gz-git forge config generate --provider github --org kubernetes --language go --min-stars 1000 -o test.yaml --path ./test

# Phase 4: Quality
make quality
```

---

## Technical Notes

### Sources Referenced
- [GitHub Search API](https://docs.github.com/en/search-github/searching-on-github/searching-for-repositories)
- [GitLab Projects API](https://docs.gitlab.com/ee/api/projects.html)
- [GitLab Language Filter Issue #223065](https://gitlab.com/gitlab-org/gitlab/-/issues/223065)

### Estimated Effort
- Phase 1: 2 hours
- Phase 2: 3 hours
- Phase 3: 3 hours
- Phase 4: 2 hours
- **Total: ~10 hours (M effort)**
