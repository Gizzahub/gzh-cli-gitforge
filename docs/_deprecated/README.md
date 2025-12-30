# Deprecated Documentation

Files auto-migrated during documentation reorganization. These are superseded by new structure.

## Cleanup Policy

| Age | Action |
|-----|--------|
| 30 days | Review for deletion |
| 90 days | Safe to delete |

## Batches

| Batch | Date | Files | Status |
|-------|------|-------|--------|
| 2025-12 | 2025-12-31 | 12 | Can delete after 2026-03-31 |

## To Delete a Batch

```bash
rm -rf docs/_deprecated/2025-12/
```

## To Permanently Archive

Move to `docs/archive/` (without underscore):
```bash
mv docs/_deprecated/2025-12/IMPORTANT.md docs/archive/
```
