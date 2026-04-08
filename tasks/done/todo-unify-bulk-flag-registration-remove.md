# todo-unify-bulk-flag-registration-remove

## Metadata
- **id**: todo-unify-bulk-flag-registration-remove
- **title**: Replace manual flag registrations with unified helper
- **type**: cleanup
- **priority**: P2
- **effort**: S
- **parent**: tasks/plans/P2-refactor-bulk-flag-registration-helper.md
- **created-at**: 2026-04-07T11:00:00+09:00
- **depends-on**: todo-unify-bulk-flag-registration-create

## Objective
Remove manual bulk flag registrations in `clean.go`, `cleanup_branch.go`, `switch.go`, `diff.go`, and `branch_list.go` and replace them with calls to the newly created `addBulkFlagsWithOpts` helper. This ensures the default values (like 5m for `--interval`) are applied consistently.

## Verification
- Code no longer manually initializes bulk flags in those 5 commands.
- `make build && make test` pass.
- `--watch --interval` works with default `5m` without panics in commands like `clean`.

## Linkage
- **parent**: tasks/plans/P2-refactor-bulk-flag-registration-helper.md
- **depends-on**: [todo-unify-bulk-flag-registration-create]
