# Vision

## Vision

**A Git CLI and Go library that speeds up common workflows with safety and consistency.**

gzh-cli-gitforge exists to eliminate repetitive Git operations, reduce human error, and enable parallel development workflows. We believe Git should be fast, reliable, and consistent—not a source of friction in daily development.

## Why Now

Git workflows remain largely manual and error-prone despite decades of tool evolution:

- **Speed**: Developers waste minutes daily on repetitive Git commands across multiple repositories
- **Consistency**: Commit messages and branch naming vary wildly, even within teams
- **Safety**: Force pushes, accidental deletions, and merge conflicts still cause lost work
- **Scale**: Managing 5+ repositories simultaneously is tedious with standard Git CLI
- **Integration**: Go projects lack a native, library-first Git integration approach

Modern development demands:

- Faster iteration cycles (CI/CD, trunk-based development)
- Multi-repository workflows (microservices, monorepo alternatives)
- Stronger safety guarantees (protected branches, signed commits)
- Better developer experience (CLI tools with sane defaults)

The time is right because:

1. Go 1.25+ provides improved tooling and performance
1. Git 2.30+ has mature worktree and sparse-checkout features
1. GitHub/GitLab APIs enable powerful automation
1. Developer expectations for CLI tools have risen (see: gh, ripgrep, fd)

## Anti-Goals

What we will **NOT** build:

- **No GUI or web interface** - CLI and library only; GUIs belong in IDEs
- **No self-hosted Git server** - We integrate with Git forges, not replace them
- **No IDE plugins** - VS Code, JetBrains, etc. have their own Git integrations
- **No Git hooks manager** - Hooks are project-specific; we provide output for automation
- **No built-in CI/CD system** - We expose CLI commands for pipeline use, not orchestration
- **No advanced TUI** - Simple progress/status only; full TUI requires explicit approval

We stay focused on what we do best: making Git operations faster and safer for CLI-first workflows.
