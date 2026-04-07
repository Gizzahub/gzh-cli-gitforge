# todo-chore-disable-revive-stutter-check

## Metadata
- **id**: todo-chore-disable-revive-stutter-check
- **title**: Disable revive stutter check in .golangci.yml
- **type**: chore
- **priority**: P3
- **effort**: XS
- **parent**: tasks/plans/P3-chore-add-nolint-revive-repository-result-types.md
- **created-at**: 2026-04-07T11:00:00+09:00

## Objective
Add revive linter configuration to `.golangci.yml` to set `disableStutteringCheck: true` for the `exported` rule, preventing stuttering warnings for `repository.RepositoryXxxResult` naming conventions.

## Verification
- `golangci-lint run --new-from-rev=HEAD ./...` yields no revive stutter warnings for `Repository*Result`.
- `make lint` passes.

## Linkage
- **parent**: tasks/plans/P3-chore-add-nolint-revive-repository-result-types.md
