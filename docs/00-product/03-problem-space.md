# Problem Space

## Core Problem

**Git CLI operations are slow, manual, and error-prone for developers working with multiple repositories.**

Developers spend significant time on repetitive Git commands, inconsistent workflows, and recovering from avoidable mistakes—time that should go to building software.

## Target Users

### Primary Users

| User Type                  | Description                                | Key Pain                        |
| -------------------------- | ------------------------------------------ | ------------------------------- |
| **Go Developers**          | Daily Git users building Go applications   | Need native library integration |
| **Multi-Repo Maintainers** | Manage 5+ repositories regularly           | Bulk operations are tedious     |
| **Worktree Enthusiasts**   | Use parallel branches for features/reviews | Manual worktree management      |
| **CLI Tool Authors**       | Build tools that need Git integration      | Lack reusable Go library        |

### Secondary Users

| User Type                   | Description                     | Key Pain                     |
| --------------------------- | ------------------------------- | ---------------------------- |
| **Team Leads**              | Enforce commit conventions      | Inconsistent message formats |
| **DevOps Engineers**        | Automate Git workflows in CI/CD | Need scriptable CLI output   |
| **Open Source Maintainers** | Sync forks across organizations | Manual fork management       |

## Current Pain Points

### Speed

- Running `git status` across 10 repos takes 30+ seconds manually
- Checking out branches requires navigating to each repo
- Fetching updates is repetitive and slow

### Consistency

- Commit message formats vary per developer
- Branch naming conventions are unenforced
- No standard way to apply the same operation across repos

### Safety

- Force push mistakes cause lost work
- Accidental branch deletion happens
- Merge conflicts from outdated branches
- Credential exposure in logs

### Scale

- No native bulk operations for multi-repo workflows
- Worktree setup is verbose and error-prone
- Organization-wide repository sync is manual

### Integration

- No Go library for common Git operations
- Existing wrappers are CLI-first, library-second
- Context cancellation and timeouts not supported

## Non-Problems

What we are **NOT** trying to solve:

| Non-Problem          | Why Not                                      |
| -------------------- | -------------------------------------------- |
| Git GUI needs        | IDEs and dedicated GUI tools serve this well |
| IDE Git integration  | VS Code, JetBrains have mature solutions     |
| Git hosting/servers  | GitHub, GitLab, Gitea exist and excel        |
| Git education        | Documentation and tutorials exist            |
| Code review workflow | Forge-specific UIs are purpose-built         |
| CI/CD orchestration  | Jenkins, GitHub Actions, etc. handle this    |
| Git hooks management | Project-specific; many tools exist           |

We focus on **CLI and library operations**, not replacing existing specialized tools.
