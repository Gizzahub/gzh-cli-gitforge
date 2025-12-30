# Principles

## Core Principles

These principles guide all decisions in gzh-cli-gitforge development:

### 1. Library-First Architecture

Core logic lives in `pkg/*` and is fully reusable. The CLI is a thin wrapper that delegates to library packages. Any operation possible via CLI must also be possible via library import.

**Implication**: New features start as library code, CLI integration follows.

### 2. Git CLI as Source of Truth

We wrap Git CLI (`git` command), not replace it. Git's behavior is the standard—we add convenience, not divergence. Users can always fall back to raw Git commands.

**Implication**: No custom Git implementations; always shell out to `git`.

### 3. Safety Over Speed

Destructive operations require explicit confirmation or flags. Force push is blocked on protected branches by default. Inputs are sanitized before execution. Recovery paths exist for common mistakes.

**Implication**: Default to safe; opt-in to dangerous.

### 4. Context-Aware Operations

All operations accept `context.Context` for cancellation and timeouts. Long-running operations are interruptible. Progress feedback is provided for bulk operations.

**Implication**: Operations can be cancelled, timed out, and monitored.

### 5. Clean Dependency Boundaries

`pkg/` packages have zero dependency on CLI frameworks. `internal/` contains implementation details. Only `cmd/` and designated adapter packages may import CLI dependencies.

**Implication**: Library users don't inherit CLI framework dependencies.

## Trade-offs

When principles conflict, this is the priority order:

| Conflict | Winner | Rationale |
|----------|--------|-----------|
| Safety vs Speed | Safety | Lost work is worse than slow operations |
| Simplicity vs Features | Simplicity | Fewer features done well beats many done poorly |
| Library vs CLI | Library | CLI is a consumer; library serves multiple consumers |
| Consistency vs Flexibility | Consistency | Predictable behavior reduces user errors |
| Compatibility vs Innovation | Compatibility | Breaking changes hurt adoption |

### Decision Framework

When in doubt:
1. Does it prevent data loss? → Do it
2. Does it simplify the mental model? → Prefer it
3. Does it work as a library function? → Required
4. Does it maintain Git CLI compatibility? → Required
5. Is it the simplest solution? → Prefer it
