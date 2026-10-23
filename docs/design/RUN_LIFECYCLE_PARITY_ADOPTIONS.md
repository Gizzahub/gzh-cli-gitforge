# CE Parity Reference Adoptions

Earlier CE references of the
[run lifecycle parity contract](RUN_LIFECYCLE_PARITY_CONTRACT.md) §7, newest
first. Each record is kept as written when it was adopted; commands, gates and
`tmp/` paths it names describe that time, not the current suite. Section
references (`§6 above`, `the commands below`) point into the contract.

The prior clean `c6d5bf84` reference (`412-gc6d5bf84`) replaces `f88f4331`
together with the readiness contract removal. CE's non-test changes in that
range (`access*.go`, `task_cleanup.go`, `task_lint.go`) are off the captured
paths. Normalized for CE identity, ten fixtures are byte-identical and two
(discard-no-fetch, doctor-active) differ only in captured `integrate --help`
text. Exits, status, ownership, refs, push and cleanup are unchanged.
Evidence is in `reports/2026-10-21-ce-parity-reference-c6d5bf84.md`.

The prior `f88f4331` (legacy make gate removal) and `f66d06c1` (identity
only) references are recorded in
`reports/2026-10-21-ce-parity-reference-f88f4331.md` and
`reports/2026-10-20-ce-parity-reference-f66d06c1.md`.

The prior clean `6baa7f9e` reference resolves ISSUE-41. The installed CE
moved from `42363c6e` to `6baa7f9e` (`345-g6baa7f9e`), and CE's integrate run
now prints `integrate: run device=<host> gz-git=<version>` on stderr. The
sandbox links the host's gz-git, so the harness masks that release as
`gz-git=<VER>`; a gz-git release alone no longer invalidates the fixtures. The
twelve re-recorded fixtures differ from the `42363c6e` capture in CE identity
leaves plus that masked stderr line in the two finish scenarios. Status, exits,
ownership, refs, push and cleanup remain unchanged. The prior captures below
retain their original reference and scope. Evidence is in
`reports/2026-10-20-ce-parity-reference-6baa7f9e.md`.

The prior clean `42363c6e` reference follows a separate user-approved adoption
of the two subsequent owner fixes. Unsupported issue-resolution frontmatter now
fails before writes or success output. The readiness evidence consumer emits a
verdict only after expectation-directory cleanup succeeds. Neither operation is
called by the twelve lifecycle scenarios; owning regression tests verify them
separately.
Its adoption evidence is in `reports/2026-10-18-ce-parity-reference-42363c6e.md`.

The prior clean `8f7c2963` reference was adopted after a separate user decision
when the installed CE and source moved beyond `ef33833b` during integration.
The additional source-branch resolver extraction preserves declaration-first,
origin/HEAD fallback and fail-closed parse/read errors. Decisions validation now
has its own command; the lifecycle fixtures do not exercise that command.
Owning CE source/recovery and decisions regression checks passed separately.
The twelve newly captured fixtures differ from the prior `ef33833b` capture in
37 CE identity leaves only. Status, exits, ownership, refs, push, cleanup and
`config[0]` diagnostics remain unchanged. Historical captures below retain their
original reference and scope. Evidence is in the primary checkout's
`tmp/parity-8f7c2963-*` and the adoption report under `reports/`.

After TASK-289 landed at `abee75f2`, the same clean `ef33833b` candidate was
freshly machine-captured against that updated checkout provider. Its predecessor
reference `7140701a` is an ancestor of `ef33833b`, not a replacement candidate.
This capture preserves TASK-289's required `config[0]` provider diagnostics.
Relative to the new baseline's twelve goldens, 39 decoded leaves changed:
37 CE identity values and two literal physical `processHost` values from
`CE-Mac-Studio.local` to `CEui-MacBookPro.local`. Status, exit, ownership, refs,
push and cleanup outcomes are unchanged; no normalizer or scenario changed.
The separate record and non-record logs are retained under
`tmp/parity-ef33833b-rebased-{record,verify}.log`, with decoded evidence in
`tmp/parity-ef33833b-rebased-decoded-diff.json` in the primary checkout.

The initial pre-TASK-289 capture on 2026-10-18 used clean CE 0.10.13
`ef33833b542cbe84be842f8a77293e61bce449ea`. The installed executable and clean
source checkout agreed before capture; the installed CE executable SHA-256
was unchanged across capture. The provider was built from this checkout by the
fixture harness, not taken from the installed ambient gz-git. All twelve scenarios were machine-captured in
18.222s. Decoding embedded JSON showed 39 changed leaves: 37 CE identity values
and two refusal reason/stderr values in `recover-requires-cleanup-failure`.
The new refusal names bound native completion, matching CE `e5fc498c` and its
`3b7df148` dangling-worktree hardening. This is a real recovery contract extension,
not just a stamp update. Existing scenario statuses, exits, refs and cleanup
outcomes are unchanged. No scenario or normalizer was changed; the new success
path is outside these twelve captures, as specified in §6 above.
Raw capture, non-record verification and decoded difference evidence are retained
in the primary checkout under `tmp/parity-ef33833b-{record,verify,decoded-diff}`
(with `.log` for runs and `.json` for the decoded diff). These are host-local
observations; full delivery acceptance belongs to the native integration gate.

The earlier TASK-289 follow-up used clean CE 0.10.13
`7140701a8f024cd8112d03f25e8595398e3557ea`, matching its owning source and
installed executable. The checkout-built provider now reports the integration
resolution source for every target, including `integration=master (config[0])`.
That provider prose is required by TASK-289, rather than a lifecycle outcome
change. Reference adoption requires a machine capture of all twelve scenarios,
fresh non-record verification, and independent comparison of the complete diff.

Physical `processHost` remains literal capture provenance. That follow-up captured
on `CE-Mac-Studio.local`; it does not normalize host identities or accept a foreign
host as local. A different physical host may still differ from the reference:
only the native gate's measured baseline comparison can accept a proven
non-worsening difference. Scenario statuses, ownership, refs, push and cleanup
results must remain unchanged. Generation alone is not native acceptance.

Note (2026-10-21): the native make gate and its measured baseline comparison
were removed, and so was the readiness contract (`contract-v1`). This adoption record is kept as written; it no longer describes
current `gz-git integrate` behavior.

CE's `recover-requires-cleanup-failure` refusal now also names a bound native
completion as an eligible recovery input. The fixture still has no such input:
its refusal status and exit code remain unchanged. The new native-completion
success path is not exercised or accepted by these twelve scenarios.

On 2026-10-18, the readiness-approval follow-up adopted clean CE 0.10.13
`ad92901e322e3c64f350363feecc9c8b6b3807c3`. The source checkout and executable
Go metadata agree, including `vcs.modified=false`. Fresh machine capture passed
all twelve scenarios in 22.873s; separate full non-record verification passed
in 26.745s. All twelve provenance stamps changed. Two finish scenarios also
retain this capture host's physical `processHost` and successful
`git merge-base --is-ancestor <taskHead> <sourceHead>` diagnostics. The latter
reflect CE's fail-closed task-containment proof, introduced by `e8307aa4`, rather
than a cosmetic stamp change. Status, schema, allowed actions, exit results,
refs, worktree and cleanup outcomes did not change. These scenarios prove only
the direct-ancestor success case, not rebase/provider-binding behavior. No
normalizer changed; this reference adoption alone is not a full integration
gate verdict. Reproduction uses the commands below; raw logs are retained in
the source primary checkout's `tmp/readiness-parity-{record,verify}-ad92901e.log`.

On 2026-10-16, `verification-evidence-followup` freshly machine-captured all
twelve scenarios with the same clean CE `9fe1ce45` pin and the checkout-built
provider. Four goldens changed: two embedded provider help strings describe
legacy JSON outcomes, and two retained physical `processHost` fields reflect
the capture host. No normalization, lifecycle schema, or CE pin changed.
Capture provenance and fresh verification results are recorded in
`reports/2026-10-16-verification-evidence-followup.md`; generation alone is not
full integration acceptance.

Historical fixtures were first authored against `bb970b24` /
`400-gbb970b24`, re-recorded across `f927ae5d` / `401-gf927ae5d`, re-recorded
again across the move to `771c54cf` / `404-g771c54cf` (stamp-only), again
across the move to `950650ef` / `475-g950650ef` (stamp plus captured wording:
the `nextAction` guidance grew a run-finish clause and the captured
`gz-git integrate` `--help` output gained `Effect: mutating (...)` lines), and
again across the move to `f540f972` / `498-gf540f972`. That last move changed
captured behaviour, not only stamps: CE `b09a605e` accepts the Worktrunk 0.80
line, and CE `f540f972` reads the run source from the remote-tracking ref — it
verifies `refs/remotes/origin/<integration>` (and `refs/heads/<integration>`)
with `git show-ref --verify`, creates the task worktree from that ref's SHA
instead of the local branch name, passes an explicit
`--target origin/<integration>` to `gz-git integrate check/run` (previously
no `--target`), and its pushed-source probe counts
`refs/remotes/origin/<integration>..refs/heads/<integration>` instead of
`origin/<integration>..<integration>`. The sandbox therefore fetches its
origin once, as a real clone would. Schema, statuses and allowed actions were
unchanged, verified by golden diff. The gz-git port does not follow either
change yet; that is a port decision tracked in
`tasks/issue/42-runtask-port-follow-ce-f540f972.md`, not a known-divergent
scenario.
