# Merge & Conflict Detection Example

This example demonstrates gz-git merge and conflict detection features using the CLI.

## Features Demonstrated

1. **Pre-Merge Conflict Detection**: Identify conflicts before attempting merge
1. **Merge Execution**: Execute merges with various strategies
1. **Merge Abort**: Safely abort in-progress merges
1. **Rebase Operations**: Rebase branches interactively or non-interactively

## Usage

### Detect Conflicts Before Merging

```bash
# Check for conflicts between branches
gz-git merge detect feature/mybranch main

# Detailed conflict analysis
gz-git merge detect feature/mybranch main --detailed
```

### Execute Merge

```bash
# Basic merge
gz-git merge do feature/mybranch

# Merge with specific strategy
gz-git merge do feature/mybranch --strategy recursive

# Merge without creating commit (for review)
gz-git merge do feature/mybranch --no-commit
```

### Abort Merge

```bash
# If merge has conflicts, abort and return to pre-merge state
gz-git merge abort
```

### Rebase Operations

```bash
# Rebase current branch onto main
gz-git merge rebase main

# Interactive rebase
gz-git merge rebase main --interactive
```

## Merge Strategies

gz-git supports multiple merge strategies:

- **fast-forward**: Fast-forward only (no merge commit)
- **recursive**: Default 3-way merge (Git's default)
- **ours**: Prefer current branch on conflicts
- **theirs**: Prefer incoming branch on conflicts
- **octopus**: Merge multiple branches

## Library Usage

For library integration, see [Library Guide](../../docs/LIBRARY.md).

See [pkg/merge](../../pkg/merge) for complete API documentation.
