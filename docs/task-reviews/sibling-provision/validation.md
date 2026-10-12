# Sibling provisioning validation

Candidate source is based on `929d0d92ab8669d4d3f0fb8ac8c0493bca37a043`.

The optional contract provisions only validated local primary checkouts, keeps opaque receipts in Git common-dir state, validates ownership and revisions on resume, and prepares isolated target/source readiness snapshots. Undeclared repositories retain their existing workflow.

## Verification

- Strict config, archive aliases, provision/reclaim, and actual CLI acquire/inspect/release regression tests pass, including marker tampering, incomplete declarations, changed destinations, dirty sources, stale source tips, partial acquisition rollback, and tampered-reclaim recovery.
- Formatting, unlimited lint (zero findings), and direct gosec analysis pass.
- Private build and installation-path audit pass.
- Package integration, E2E, and all recorded critical coverage floors pass.
- Independent GPT-6 actual-diff reviews pass. The final command effect entries classify acquire/release as filesystem changes and inspect as read-only; effect/help regressions pass.

## Existing baseline failures

`make quality-check` is not reported as green. Two unchanged baseline failures are reproduced against the primary source checkout:

1. `govulncheck ./...` reports the same ten reachable Go 1.26.7 standard-library vulnerability IDs: GO-2026-6603, 6604, 6605, 6607, 6608, 6610, 6611, 6612, 6613, and 6617. The candidate adds no vulnerability ID. Repository toolchain declarations and workstation pins are unchanged. The reported fixed Go patch is 1.26.9; its adoption is separate from this feature.
1. `go test ./tests/parity -count=1` refuses installed CE build `125-gcdc11eaa` versus committed goldens from `498-gf540f972`. Both baseline and candidate fail before parity execution. Existing TASK-012 review evidence already records this toolchain drift. No older executable or arbitrary fixture refresh is substituted.

The first broad unit run additionally exposed missing new-command effect declarations, which were corrected and independently reviewed. A benchmark fixture local-clone object-copy race passed its isolated retry. Neither is silently represented as a full gate pass.

Source delivery does not activate this capability in the installed tools or adopt a sibling declaration in GitRump. Installed producer/consumer capability verification and real delivery/readiness validation remain required before closing ISSUE-099 or TASK-268 in the GitRump devbox.
