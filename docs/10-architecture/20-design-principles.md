# 3. Design Principles

> gzh-cli-gitforge 아키텍처 문서 · [인덱스](README.md) · [ARCHITECTURE.md](../../ARCHITECTURE.md)

### 3.1 SOLID Principles

**Single Responsibility:**

- Each package has one clear purpose
- `pkg/branch/` only handles branch operations
- `internal/gitcmd/` only executes Git commands

**Open/Closed:**

- Interfaces open for extension
- Functional options allow new parameters without breaking API

**Liskov Substitution:**

- All interface implementations are substitutable
- Mocks can replace real implementations

**Interface Segregation:**

- Small, focused interfaces (not god interfaces)
- `CommitManager`, `BranchManager` separate, not combined

**Dependency Inversion:**

- Depend on interfaces, not concretions
- Accept `Logger` interface, not `*zap.Logger`

### 3.2 Library-First Principles

**P1: Zero CLI Dependencies in pkg/**

```go
// ❌ WRONG: pkg/ code importing CLI framework
import "github.com/spf13/cobra"

// ✅ CORRECT: pkg/ only uses stdlib and interfaces
import (
    "context"
    "io"
)
```

**P2: Dependency Injection via Interfaces**

```go
// ❌ WRONG: Hard-coded logger
func Process() {
    log.Println("processing...")
}

// ✅ CORRECT: Logger injected via interface
func Process(ctx context.Context, logger Logger) {
    logger.Info("processing...")
}
```

**P3: Context Propagation**

```go
// ❌ WRONG: No context
func Clone(url, path string) error

// ✅ CORRECT: Context as first parameter
func Clone(ctx context.Context, url, path string) error
```

### 3.3 Go Idioms

**Functional Options Pattern:**

```go
type CloneOption func(*CloneConfig)

func WithBranch(b string) CloneOption {
    return func(c *CloneConfig) { c.Branch = b }
}

// Usage allows evolution without breaking changes
Clone(ctx, url, path, WithBranch("main"), WithDepth(1))
```

**Error Wrapping:**

```go
if err != nil {
    return fmt.Errorf("failed to clone repository: %w", err)
}
```

**Interface Satisfaction:**

```go
var _ CommitManager = (*commitManager)(nil) // Compile-time check
```
