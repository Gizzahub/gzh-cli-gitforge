# Branch Management Example

This example demonstrates gz-git branch management features using the CLI.

## Features Demonstrated

1. **List Branches**: Show all local and remote branches
1. **Create Branches**: Create new branches with various options
1. **Delete Branches**: Remove branches safely
1. **Worktree Management**: Create branches with linked worktrees

## Usage

### List All Branches

```bash
# List local branches
gz-git branch list

# List all branches (including remote)
gz-git branch list --all
```

### Create a New Branch

```bash
# Create from current HEAD
gz-git branch create feature/new-feature

# Create from specific commit/branch
gz-git branch create feature/new-feature --from main
```

### Create Branch with Worktree

```bash
# Create branch in separate working directory
gz-git branch create feature/parallel --worktree /tmp/parallel-work
```

### Delete a Branch

```bash
# Delete local branch
gz-git branch delete feature/old-feature

# Force delete (if not fully merged)
gz-git branch delete feature/experimental --force
```

## Library Usage

For library integration, see [Library Guide](../../docs/LIBRARY.md).

Basic library example:

```go
import "github.com/gizzahub/gzh-cli-gitforge/pkg/branch"

// Branch manager provides branch operations
// Worktree manager handles worktree operations
```

See [pkg/branch](../../pkg/branch) for complete API documentation.
