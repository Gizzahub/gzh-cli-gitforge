# Scope

## In Scope

What we **WILL** build in gzh-cli-gitforge:

### Core CLI Operations

| Category      | Operations                                                |
| ------------- | --------------------------------------------------------- |
| **Status**    | Repository status, branch info, remote info               |
| **Commit**    | Bulk commit with auto/generated or user-provided messages |
| **Branch**    | List/switch branches (create/delete remains native `git`) |
| **History**   | Stats, contributors, file history, blame                  |
| **Remote**    | Fetch, pull, push, update (with safety guards)            |
| **Sync**      | Sync repos from forge APIs or YAML config                 |
| **Tag/Stash** | Tag and stash workflows (single or bulk)                  |
| **Watch**     | Real-time monitoring of repositories                      |

### Multi-Repository Operations

| Category        | Operations                                       |
| --------------- | ------------------------------------------------ |
| **Bulk Status** | Status across multiple repos in one command      |
| **Bulk Fetch**  | Fetch all repos in parallel                      |
| **Bulk Switch** | Switch branches across repos (optionally create) |
| **Bulk Commit** | Commit across repos (preview by default)         |
| **Bulk Diff**   | Diff across repos with uncommitted changes       |
| **Bulk Update** | Update repos via `git pull --rebase`             |
| **Bulk Clone**  | Clone many repos from URL list/file              |

### Repository Sync

| Category              | Operations                                    |
| --------------------- | --------------------------------------------- |
| **Organization Sync** | Clone/update all repos from GitHub/GitLab org |
| **User Sync**         | Clone/update all repos from user account      |
| **Fork Sync**         | Keep forks in sync with upstream              |
| **Selective Sync**    | Filter by language, topic, or pattern         |

### Go Library

| Category                    | Packages                         |
| --------------------------- | -------------------------------- |
| **pkg/repository**          | Repository abstraction           |
| **pkg/history**             | History analysis                 |
| **pkg/merge**               | Conflict detection               |
| **pkg/branch**              | Cleanup services/utilities       |
| **pkg/stash**               | Stash management                 |
| **pkg/tag**                 | Tag management                   |
| **pkg/watch**               | Repository monitoring            |
| **pkg/reposync**            | Repository sync planner/executor |
| **pkg/provider**            | Forge provider abstraction       |
| **pkg/github/gitlab/gitea** | Provider implementations         |
| **pkg/sync**                | Sync config/types                |

## Out of Scope

What we will **NOT** build (this phase):

### Explicitly Excluded

| Exclusion                | Rationale                                     |
| ------------------------ | --------------------------------------------- |
| **Git server/hosting**   | Use GitHub, GitLab, Gitea                     |
| **GUI interface**        | CLI and library only                          |
| **IDE plugins**          | VS Code, JetBrains have their own             |
| **Git hooks manager**    | Project-specific concern                      |
| **CI/CD system**         | Use existing CI tools                         |
| **Advanced TUI**         | Simple progress only; full TUI needs approval |
| **Interactive rebase**   | Too complex for first phase                   |
| **Submodule management** | Low priority, complex edge cases              |
| **LFS support**          | Specialized use case                          |
| **GPG signing UI**       | Git handles this well                         |

### Deferred to Future Phases

| Feature              | Phase     | Rationale                      |
| -------------------- | --------- | ------------------------------ |
| Advanced TUI         | Phase 8+  | Needs design approval          |
| Submodule support    | Phase 9+  | Edge cases need careful design |
| LFS integration      | Phase 9+  | Specialized audience           |
| Custom merge drivers | Phase 10+ | Advanced use case              |

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
