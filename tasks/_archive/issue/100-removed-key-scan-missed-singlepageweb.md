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

[ISSUE-89](089-release-0-12-blocked-by-removed-keys.md) was
closed in `86f02984` (2026-10-22 09:59 KST) after a re-scan reported that "the
50 primary checkouts and the global config declare none of the removed keys".
That re-scan, and the 2026-10-21 impact table before it, did not include
`~/mydevbox/singlepageweb-devbox` or its child repositories. At close time:

- `singlepageweb-devbox` still declared `branch.readiness` and
  `branch.resourceHighLoad`. Its owner found this only when `ce task run-finish`
  was refused, and removed them in `66caae45` (14:37 KST) and `9475f38c`
  (14:42 KST).
- Seven child repositories still declare `branch.prepareProfile: pnpm-frozen-lockfile-v1` on their primary checkout (`master`) and on
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

## Additional gap: mst global config (2026-10-22)

The same closing scan checked the global config on mbp only. On mst,
`~/Library/Application Support/gz-git/config.yaml` still holds only
`integration:` / `readinessApproval: automatic`. A gz-git built from master
after ISSUE-97 (`tmp/bin/gz-git` in the DevBox) refuses every command there:
`failed to load config layers: ... integration.readinessApproval was removed`.
The homebrew 0.12.0 binary predates ISSUE-97 and still loads it. The operator
removes the key on that device; gz-git does not edit user config.

## Additional gap: brikspec and gitrump-client on mst (2026-10-25)

After the Homebrew upgrade to 0.14.0 on mst,
`cd ~/mydevbox && gz-git push --refspec HEAD:master -d3 --dry-run --format json`
exited 2. Two child repositories reported `status: error` with the
`error` field
`read workspace access config <repo>/.gz-git.yaml: branch.readiness was removed: ...`
(verified):

- `~/mydevbox/brikspec-devbox/brikspec`: `branch.readiness` at
  `.gz-git.yaml:10-12`, landed in `147e7d7` (2026-09-13). It is still on
  `origin/master` (`eb3b33cc`).
- `~/mydevbox/scripton-gitrump-devbox/gitrump-client`: `branch.readiness` at
  `.gz-git.yaml:23-25`, landed in `83e9672` (2026-09-15). It is still on
  `origin/master` (`4b2e8bde`).

Neither path appears in ISSUE-89's impact table or in its 2026-10-22 re-scan.
This is the same gap as singlepageweb: the scan did not cover nested child
repositories of these DevBoxes. Both keys predate 0.12.0, so 0.12.0 through
0.13.x refuse the same files (verified from source: the `branch.readiness`
entry in `pkg/config/removed_keys.go` and the `ReadConfigFile` call in
`pkg/config/workspace_access.go` are present from v0.12.0, and their diff
between v0.13.1 and v0.14.0 is empty). It is not a 0.14.0 regression.

Each owner fixes its own repository. Both fixes are pushed on
`dev/claude/mst/fix/remove-readiness-key` (brikspec `bb84744`, gitrump-client
`b30e27c`) and are not yet integrated. This card does not change them.

Enumeration used on mst: every `~/mydevbox/*/.gz-git.yaml` and
`~/mydevbox/*/*/.gz-git.yaml`, 121 files. Each was grepped for every removed
key at the start of a line:
`^\s*(makeTimeout|makeOutcomeReport|prepareProfile|readiness|prepareHighLoad|resourceHighLoad|lockWait|readinessApproval):`.

- There were three hits: the two above, and
  `gizzahub-devbox/gizzahub-web-svelte/.gz-git.yaml:26`.
- The gizzahub hit is a top-level `readiness:`, not `branch.readiness`.
  - In v0.11.1, `Readiness` was a field of `BranchConfig` (`pkg/config/types.go:222,247`), so the top-level key was never a gz-git key.
  - 0.14.0 loads it without error: `gz-git config show` exits 0, and the dry-run reports `would-push`.
  - It is outside this card. Its owner should know that the runner it declares was never read.
- The scan did not cover depth 3 or more under `~/mydevbox`, `~/mywork`,
  the mbp checkouts, or the global config.

UX note (my own judgment, not a confirmed defect): the bulk push summary shows
only the generic `Failed to resolve repository push access`
(`pkg/repository/bulk.go:2237`), and the removed-key reason appears only in the
`error` field. In a long JSON report, the removed-key cause is easy to miss. A
summary that names the removed key would point owners at the fix directly.

## Resolution

Closed 2026-10-26 by user decision. Both devices have now been scanned with a
recorded method, and every gap found is recorded here as ISSUE-89's
missing evidence:

- mbp (2026-10-24, run by the operator): 120 `.gz-git.yaml` files and the
  global config, 0 removed-key matches. singlepageweb-devbox is not on mbp.
- mst (2026-10-25): 121 files at depth 1–2 under `~/mydevbox` (method in the
  section above). The two `branch.readiness` hits are recorded above. The
  gizzahub top-level `readiness:` is not a gz-git key.
- Owners fix their own repositories: singlepageweb in its ISSUE-096 and
  TASK-283, brikspec and gitrump-client on
  `dev/claude/mst/fix/remove-readiness-key`, and the mst global config by its
  operator. gz-git changes none of them.

Not covered, and left as accepted: depth 3 or more, and `~/mywork`. A miss
there shows up as a load error that names the file and key, as it did for
singlepageweb, brikspec and gitrump-client. The bulk push summary wording is
an unconfirmed UX note, not a defect, and was not pursued.

## Notes

- Removed keys come from `pkg/config/removed_keys.go` (`prepareProfile` at
  lines 37 and 44).
- Fixing this card means re-running the scan with a recorded enumeration
  method and amending ISSUE-89's evidence, or recording why these roots were
  out of scope. Other owners' repositories are not edited from here.
