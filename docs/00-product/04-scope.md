# Scope

## In Scope

What we **WILL** build in gzh-cli-gitforge:

### Core CLI Operations

| Category | Operations |
|----------|------------|
| **Status** | Repository status, branch info, remote info |
| **Commit** | Create commits, amend, message templates |
| **Branch** | Create, delete, switch, list, merge |
| **History** | Log viewing, commit search, blame |
| **Remote** | Fetch, pull, push (with safety guards) |

### Multi-Repository Operations

| Category | Operations |
|----------|------------|
| **Bulk Status** | Status across multiple repos in one command |
| **Bulk Fetch** | Fetch all repos in parallel |
| **Bulk Branch** | Create/switch branches across repos |
| **Bulk Commit** | Commit with same message across repos |

### Worktree Management

| Category | Operations |
|----------|------------|
| **Create** | Create worktrees with sensible defaults |
| **List** | View active worktrees |
| **Remove** | Clean removal with safety checks |
| **Switch** | Navigate between worktrees |

### Repository Sync

| Category | Operations |
|----------|------------|
| **Organization Sync** | Clone/update all repos from GitHub/GitLab org |
| **User Sync** | Clone/update all repos from user account |
| **Fork Sync** | Keep forks in sync with upstream |
| **Selective Sync** | Filter by language, topic, or pattern |

### Go Library

| Category | Packages |
|----------|----------|
| **pkg/repository** | Repository abstraction |
| **pkg/branch** | Branch operations |
| **pkg/commit** | Commit operations |
| **pkg/history** | History/log operations |
| **pkg/merge** | Merge operations |
| **pkg/reposync** | Repository sync |
| **pkg/operations** | High-level workflows |

## Out of Scope

What we will **NOT** build (this phase):

### Explicitly Excluded

| Exclusion | Rationale |
|-----------|-----------|
| **Git server/hosting** | Use GitHub, GitLab, Gitea |
| **GUI interface** | CLI and library only |
| **IDE plugins** | VS Code, JetBrains have their own |
| **Git hooks manager** | Project-specific concern |
| **CI/CD system** | Use existing CI tools |
| **Advanced TUI** | Simple progress only; full TUI needs approval |
| **Interactive rebase** | Too complex for first phase |
| **Submodule management** | Low priority, complex edge cases |
| **LFS support** | Specialized use case |
| **GPG signing UI** | Git handles this well |

### Deferred to Future Phases

| Feature | Phase | Rationale |
|---------|-------|-----------|
| Advanced TUI | Phase 8+ | Needs design approval |
| Submodule support | Phase 9+ | Edge cases need careful design |
| LFS integration | Phase 9+ | Specialized audience |
| Custom merge drivers | Phase 10+ | Advanced use case |

## Scope Boundaries

### We Do
- Wrap Git CLI with convenience and safety
- Provide Go library for programmatic access
- Support GitHub and GitLab APIs for sync
- Enable bulk operations across repositories

### We Don't
- Replace Git functionality
- Compete with Git forges
- Build visual interfaces
- Manage Git server infrastructure
