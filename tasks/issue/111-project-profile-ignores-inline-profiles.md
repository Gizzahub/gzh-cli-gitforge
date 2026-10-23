---
id: ISSUE-111
title: "The config loader ignores inline profiles, so a project naming one fails every command since ISSUE-97"
type: bug
priority: P2
severity: medium
discovered-in: "operator report: gz-git fetch in ~/mydevbox"
discovered-at: 2026-10-24
created: 2026-10-24
---

## Summary

`ConfigLoader.Load` (`pkg/config/loader.go`) resolves the profile a project
config names with `Manager.LoadProfile`, which reads only
`<config-dir>/profiles/<name>.yaml` (`pkg/config/manager.go`). Inline
profiles declared in the project config or along its `parent:` chain are
never consulted, although `GetProfileFromChain` (`pkg/config/recursive.go`)
implements that lookup and `workspace sync` and `forge from` use it.

Before ISSUE-97 the resulting `Load()` error was swallowed, so such a project
silently ran without its profile (token, base URL, SSH port, parallelism).
Since ISSUE-97 (v0.13.0) the error is returned, and every command that loads
config layers fails.

## Reproduction

`~/mydevbox/.gz-git.yaml` declares `parent: ~/devenv/workstation/.gz-git.yaml`
and `profile: polypia`; `polypia` exists only in the parent's `profiles:`
section, and the profiles directory holds only `default.yaml`.

```text
$ gz-git fetch
failed to load config layers: failed to load project profile 'polypia': profile 'polypia' not found
```

## Expected vs Actual

- Expected: inline profiles along the project config's parent chain, then
  the external profile file, the order the workstation config documents and
  `GetProfileByName` states.
- Actual: only the external profile file.

## Fix note

Resolve the project profile through the project config file's parent chain
(loaded without workspaces) with `GetProfileFromChain`, then fall back to the
external file; name both locations when neither has it.

## Resolution

Resolved on 2026-10-24 by `fix(config): resolve a project's profile from inline profiles first`.

- `Manager.LoadProjectProfile` loads the project config file's parent chain without workspaces and takes the profile
  from `GetProfileFromChain`, then falls back to `profiles/<name>.yaml`. The map key names an inline profile whose
  `name` field is empty; the profile is validated either way.
- A name found in neither place reports the project config path and the profiles directory. A `parent:` that fails to
  load is an error.
- The loader resolves the project's profile even when it matches the active profile's name, so an inline profile of
  that name wins.
- Tests: `pkg/config/project_profile_test.go` (six cases). Disabling the inline lookup fails the three inline tests.
- Verified against the operator's `~/mydevbox`: the built binary runs `fetch --dry-run` and `config show` reports
  `profile: polypia`.
- Docs: `docs/.claude-context/config-guide.md`, Project Config Example.
