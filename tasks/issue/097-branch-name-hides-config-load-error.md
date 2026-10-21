---
id: ISSUE-97
title: "`gz-git branch name` hides a config load error and its test reads the real global config"
type: bug
priority: P3
severity: low
discovered-in: "ISSUE-86 gate re-run"
discovered-at: 2026-10-21
created: 2026-10-21
---

## Summary

`LoadEffectiveConfig` swallows a `loader.Load()` failure
(`cmd/gz-git/cmd/config_helper.go:36-40`) and prints a warning only when
`verbose` is set; the error it returns comes only from `ResolveConfig`. So
a config that exists but fails to load (for example one declaring the
removed key `integration.readinessApproval`, see ISSUE-89) drops every
layer: the global config fails first, and the project `branch.naming` is
never read. `runBranchName` also discards the returned error
(`cmd/gz-git/cmd/branch_name.go:86`), but the load failure never reaches
it. The name falls back to the default template and nothing reports why.
The comment above that call covers only a *missing* config, which is
correctly not an error.

Its test reads the developer's real global config instead of an isolated
one. `TestBranchNameReadsTheProjectTemplate`
(`cmd/gz-git/cmd/branch_name_test.go:47`) sets no `HOME` or
`XDG_CONFIG_HOME`; `config_test.go:16` and
`command_effect_readonly_test.go:134` point both at a temp directory.

## Reproduction

1. On a machine whose global config
   `~/Library/Application Support/gz-git/config.yaml` declares
   `integration.readinessApproval: automatic` (a removed key,
   `pkg/config/removed_keys.go:45`), run
   `go test ./cmd/gz-git/cmd/ -run TestBranchNameReadsTheProjectTemplate -count=1`.
1. The test fails with
   `got "feat/task-002/dave-office", want "wip/dave-office/task-002"`;
   observed on master `fb5062fa` and on `4214a587`.
1. With `HOME` pointed at an empty directory, the same test passes.

## Fix note

- Isolate `HOME` and `XDG_CONFIG_HOME` in the branch-name tests, the way
  `config_test.go` and `command_effect_readonly_test.go` do.
- Make the load failure visible to callers: return it from
  `LoadEffectiveConfig`, or warn on stderr without `verbose`. Then let
  `runBranchName` tell "no config" from "config failed to load" and report
  the latter. Decide the exit code or stderr warning in the fix; stdout
  must stay only the name, because callers run `$(gz-git branch name ...)`.
- Because `LoadEffectiveConfig` swallows the load failure, the commands
  below, which also write `, _ := LoadEffectiveConfig` (all under
  `cmd/gz-git/cmd/`), degrade to defaults the same way. Changing their
  call sites alone would not help: `clean.go:80`, `clone.go:88`, `exec.go:85`,
  `fetch.go:62`, `handoff_end.go:137`, `handoff_start.go:112`,
  `info.go:81`, `pr_create.go:82`, `pull.go:75`, `push.go:77`,
  `status.go:55`, `update.go:75`.

## Expected vs Actual

- Expected: a config that exists but fails to load is reported, and the
  branch-name tests pass whatever the developer's global config contains.
- Actual: the load failure is swallowed in `LoadEffectiveConfig` and only
  warned about under `--verbose`, the name
  falls back to the default template, and the test fails only on machines
  whose real global config declares a removed key.

## Resolution

Resolved on 2026-10-22 by `fix(config): fail on a config load error instead of ignoring it` and `fix(config): fail on a bad --profile override instead of ignoring it`.

- `LoadEffectiveConfig` returns the `NewLoader` and `loader.Load()` errors, wrapped, keeping the file and the key. The old
  `NewLoader` fallback used a zero-value loader whose nil manager panicked in `Load`, so it never produced defaults.
- A missing global, profile or project file is still not an error.
- The `--profile` override fails when the manager cannot be created, when the profile does not exist, or when its file
  fails to load; before, each case ran on the active profile.
- `branch name` exits 2 on a load error, and its help says so. The twelve commands listed in the Fix note return the
  error and exit 1, the repository's configuration-error code (`pkg/cliutil/exit.go`); exit 2 there means a partial run.
- The branch-name tests, `TestRunStatus_NoRepos`, `TestRunExec_DryRunAndSuccess`, `TestPullAllReposFailExitsTwo`,
  `TestPullConflictExitsTwo` and `TestPullBadFormatExitsOne` isolate the config home. New tests cover a removed key in
  the global config (exit 2, key named), no global config (defaults), and a missing, broken and valid `--profile`.
- Impact: once released, a device whose global config still declares a removed key fails every one of these commands
  until the key is deleted (ISSUE-89).
