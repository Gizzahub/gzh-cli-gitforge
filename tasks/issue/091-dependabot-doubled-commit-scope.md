---
id: ISSUE-91
title: "Dependabot commit subjects get a doubled scope from prefix chore(deps) plus include scope"
type: bug
priority: P3
severity: low
discovered-in: "gzh-cli family dependabot.yml audit"
discovered-at: 2026-10-21
created: 2026-10-21
---

## Summary

`.github/dependabot.yml` sets `commit-message.prefix` to `chore(deps)` (gomod)
and `chore(ci)` (github-actions) together with `include: "scope"`. Dependabot
appends its own scope after the prefix, so the subjects come out as
`chore(deps)(deps): ...` and `chore(ci)(deps): ...`. Those subjects are not
valid Conventional Commits and the scope carries no information.

## Evidence

- `.github/dependabot.yml:12-13`: `prefix: "chore(deps)"`, `include: "scope"`
- `.github/dependabot.yml:32-33`: `prefix: "chore(ci)"`, `include: "scope"`
- `gzh-cli-os-env`, `gzh-cli-net-env` and `gzh-cli-mcp-plugin` already use
  `prefix: "chore"` with the same `include: "scope"` and are not affected.
- Observed on origin/master `0e883d00`.

## Reproduction

1. Read the title of any Dependabot PR or squash commit this repository
   received from the `gomod` or `github-actions` ecosystem.
1. The subject carries the doubled scope shown above.

## Expected vs Actual

- Expected: `chore(deps): bump ...`
- Actual: `chore(deps)(deps): bump ...` and `chore(ci)(deps): bump ...`

## Proposal

Set `prefix: "chore"` for both ecosystems and keep `include: "scope"`. Other
gzh-cli repositories with the same template are tracked in their own
repositories (gzh-cli, gzh-cli-core, gzh-cli-dev-env, gzh-cli-template,
gzh-cli-shellforge, gzh-cli-package-manager, gzh-cli-quality).
