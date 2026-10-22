---
id: ISSUE-100
title: "ISSUE-89 removed-key scans missed singlepageweb-devbox and seven of its child repositories"
type: bug
priority: P2
effort: S
exec-tier: standard
severity: medium
discovered-in: "singlepageweb-devbox gz-git 0.12.0 cutover and child-repository re-check (singlepageweb-devbox ISSUE-096), 2026-10-22"
discovered-at: 2026-10-22
created: 2026-10-22
---

## Summary

[ISSUE-89](../_archive/issue/089-release-0-12-blocked-by-removed-keys.md) was
closed in `86f02984` (2026-10-22 09:59 KST) after a re-scan reported that "the
50 primary checkouts and the global config declare none of the removed keys".
That re-scan, and the 2026-10-21 impact table before it, did not include
`~/mydevbox/singlepageweb-devbox` or its child repositories. At close time:

- `singlepageweb-devbox` still declared `branch.readiness` and
  `branch.resourceHighLoad`. Its owner found this only when `ce task run-finish`
  was refused, and removed them in `66caae45` (14:37 KST) and `9475f38c`
  (14:42 KST).
- Seven child repositories still declare `branch.prepareProfile:
  pnpm-frozen-lockfile-v1` on their primary checkout (`master`) and on
  `develop`: singlepageweb-calculator, -date, -image, -network, -pdf,
  -template, -timer. Their next integration will be refused the same way.

The release itself is not wrong: ISSUE-89 says each owner fixes its own
repository. The defect is that the impact list and the closing scan were
reported as complete when they were not, so owners not listed had no warning.
The singlepageweb owner tracks its own fix in singlepageweb-devbox ISSUE-096
and TASK-283; this card does not ask gz-git to change those repositories.

## Reproduction

1. Count removed keys in the singlepageweb child primaries. Seven lines print a
   `prepareProfile` hit:

   ```sh
   cd /Users/archmagece/mydevbox/singlepageweb-devbox && for d in singlepageweb-*/; do grep -HnE '^\s*(makeTimeout|makeOutcomeReport|prepareProfile|readiness|prepareHighLoad|resourceHighLoad|lockWait|readinessApproval):' "$d.gz-git.yaml" 2>/dev/null; done
   ```

1. Confirm the DevBox root still had the keys after ISSUE-89 closed:

   ```sh
   git -C /Users/archmagece/mydevbox/singlepageweb-devbox show 66caae45^:.gz-git.yaml | grep -nE '^\s*(readiness|resourceHighLoad):'
   ```

1. Compare with the "Re-scan on mbp, 2026-10-22" section of ISSUE-89, which
   lists no singlepageweb path.

## Expected vs Actual

- Expected: the ISSUE-89 re-scan covers every primary checkout on the device
  that carries a `.gz-git.yaml`, including nested child repositories of a
  DevBox, or states which roots it covered so a gap is visible.
- Actual: singlepageweb-devbox and its children were outside the scan, and
  the card closed as an empty scan. The cause (how the 50 checkouts were
  enumerated) is not recorded in ISSUE-89 and was not investigated here.

## Notes

- Removed keys come from `pkg/config/removed_keys.go` (`prepareProfile` at
  lines 37 and 44).
- Fixing this card means re-running the scan with a recorded enumeration
  method and amending ISSUE-89's evidence, or recording why these roots were
  out of scope. Other owners' repositories are not edited from here.
