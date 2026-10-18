# ISSUE: Cross-filesystem install does not publish the executable atomically

- status: resolved (installer behavior; delivery governed by the native integration gate)
- priority: P2
- category: installation/publication
- owner: gzh-cli-gitforge — `.make/build.mk` (`build` and `install`)

## Evidence

Source `37e32cf09ef1a71176da391ae8d9bb77cef19e2d` builds `$(BINARY)` in the
current checkout, then `install` moves that file directly over
`$(INSTALL_BINDIR)/$(BINARY)`. The install directory is selected from `BINDIR`,
`GOBIN`, or the first `GOPATH` entry. The final installed path is verified only
after this move. There is no destination-directory staging step.

Read-only filesystem observations on mst, 2026-10-17:

| Path                                                         | `st_dev`   |
| ------------------------------------------------------------ | ---------- |
| `/Users/archmagece/mydevbox/gzh-cli-devbox/gzh-cli-gitforge` | `16777245` |
| `/Users/archmagece/go/bin`                                   | `16777232` |
| `/Users/archmagece/go/bin/gz-git`                            | `16777232` |

The source observation is the checkout directory where `build` writes its
output, not a newly built executable. The official documentation task worktree
also resides on a different device (`16777251`) from the install destination.
Different device IDs establish that a direct rename between these locations
cannot be the atomic publication operation.

Apple's published [`mv` implementation](https://github.com/apple-oss-distributions/file_cmds/blob/main/mv/mv.c)
first attempts `rename`. Its cross-device (`EXDEV`) path for a regular file
uses `fastcopy`, which removes an existing destination and opens the final path
for copying before the copy is complete. Consequently, this install recipe
cannot guarantee that a concurrent consumer always opens a complete old or new
executable. A failed or interrupted copy can also leave the installed path
unavailable. This is a source-based consequence of the observed filesystem
layout; no concurrent-execution failure or interrupted installation was injected
for this registration.

## Scope and follow-up

The confirmed condition concerns the macOS cross-filesystem layout above.
Same-filesystem installation, other platforms, and all supported `BINDIR`/`GOBIN`/
`GOPATH` combinations still need explicit regression coverage. The existing
install-path audit and successful post-install `--version` do not demonstrate
atomic publication during installation.

A minimal candidate is to prepare a uniquely named staging executable inside
the resolved destination directory, finish copying and validating it there,
then publish it with a same-filesystem rename. On preparation failure, preserve
the previous installed executable and remove only the attempt's own staging
file. Preserve the current install-path selection and directory-target refusal.
This is a proposed direction, not an adopted design or implemented fix; atomic
visibility and crash durability must not be conflated.

The current operational mitigation is an observed quiescent installation
window: avoid installation while relevant consumers are observed running.
This is not an exclusion lock, cannot prevent a new concurrent invocation, and
does not repair the publication defect. It must not be reported as regression
coverage or a permanent solution.

Follow-up validation should cover cross-filesystem source-to-destination
staging, concurrent consumer opens, failed/interrupted preparation with the old
executable retained, and current install destination-selection behavior. This
registration changes no installer code, platform contract, or installation.

## 2026-10-18 implementation and focused verification

The delivery candidate stages the executable inside the resolved destination
directory, sets mode 0755, runs the staged executable's `--version` as validation,
and publishes by same-filesystem
rename. Build completes before publication, including parallel make invocation.
Preparation failure and terminal signal handling preserve the previous target.
Final path verification, installation audit, destination selection, and plugin
installation dependencies remain in place. This establishes atomic visibility;
it makes no crash-durability claim.

`test-install-atomic` is a prerequisite of the existing `test-install-audit`
quality-check surface. It checks concurrent complete old/new observations,
failed validation, TERM interruption, directory-target refusal, permissions,
and attempt-owned staging cleanup. Independent review passed, including a
7.98-second run of
`make --no-print-directory test-install-atomic ATOMIC_INSTALL_REQUIRE_CROSSFS=1`.
The source fixture device was 16777251 and destination device 16777232 on mst.
Default runs explicitly report structural-only coverage when both devices match;
the opt-in requirement rejects that case. Windows behavior remains unverified.

The device-ID helper accepts only successful numeric output from GNU/BSD stat.
Failed GNU `stat -f` can emit partial filesystem information, so failed output
must never participate in device comparisons. BSD stat and GNU gstat IDs were
compared with `os.stat` on two same-device paths. The final independent
cross-device regression passed in 9.12 seconds after this portability fix.

The delivery must pass the required `verify-commit` profile and native full gate.
This record alone does not claim integration or installation of the candidate executable.

The original delivery was interrupted while waiting for the host measurement slot
and emitted no native pre-mutation proof. Its execution was aborted with its
`be521940` worktree and refs explicitly retained as evidence. A fresh delivery,
`53-atomic-install-publication-retry`, starts from source `1db93c1b` and reauthors
the same installer change. Its focused atomic-install regression passed in
9.185 seconds with source device 16777251 and destination device 16777232.
The fresh delivery must produce its own full-gate and integration receipts.
