# Merge & Conflict Detection Example

Demonstrates **pre-merge conflict detection** with `merge.ConflictDetector`.

Merge/rebase *execution* is intentionally left to plain git (bulk-first identity:
gz-git diagnoses; interactive merge/rebase stays in git).

## Features Demonstrated

1. **Pre-Merge Conflict Detection**: Identify conflicts before attempting merge
2. **Fast-forward check**: Whether merge can fast-forward
3. **CLI pointer**: `gz-git conflict detect <source> <target>`

## Usage

### Detect Conflicts Before Merging

```bash
# Check for conflicts between branches
gz-git conflict detect feature/mybranch main

# Detailed conflict analysis
gz-git conflict detect feature/mybranch main --detailed
```

### Execute Merge (Git)

```bash
# Basic merge
git merge feature/mybranch

# Merge with specific strategy
git merge feature/mybranch --strategy recursive

# Merge without creating commit (for review)
git merge feature/mybranch --no-commit
```

### Abort Merge (Git)

```bash
# If merge has conflicts, abort and return to pre-merge state
git merge --abort
```

### Rebase Operations (Git)

```bash
# Rebase current branch onto main
git rebase main

# Interactive rebase
git rebase -i main
```

## Merge Strategies

gz-git library supports multiple merge strategies:

- **fast-forward**: Fast-forward only (no merge commit)
- **recursive**: Default 3-way merge (Git's default)
- **ours**: Prefer current branch on conflicts
- **theirs**: Prefer incoming branch on conflicts
- **octopus**: Merge multiple branches

## Library Usage

For library integration, see [Library Guide](../../docs/user/getting-started/library-usage.md).

See [pkg/merge](../../pkg/merge) for complete API documentation.
