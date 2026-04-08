# todo-unify-bulk-flag-registration-create

## Metadata
- **id**: todo-unify-bulk-flag-registration-create
- **title**: Create central bulk flag registration helper
- **type**: refactor
- **priority**: P2
- **effort**: S
- **parent**: tasks/plans/P2-refactor-bulk-flag-registration-helper.md
- **created-at**: 2026-04-07T11:00:00+09:00
- **completed-at**: 2026-04-08T10:59:33.301906+09:00
- **verified-at**: 2026-04-08T10:59:33.301906+09:00
- **archived-at**: 2026-04-08T10:59:33.301906+09:00
- **verification-summary**: "Verified task completion: Code changes confirm expected refactoring and cleanup."

## Objective
Create a unified `addBulkFlagsWithOpts` helper in `cmd/gz-git/cmd/bulk_common.go` (or wherever `addBulkFlags` is) to support missing options like `SkipDryRun` and `SkipFetch` while centralizing the default `--interval` value.

## Verification
- Helper function `addBulkFlagsWithOpts(cmd, flags, opts)` or similar structure is created.
- Project compiles.

## Linkage
- **parent**: tasks/plans/P2-refactor-bulk-flag-registration-helper.md
