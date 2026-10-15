# ND checked-binding preparation

Readiness V1 supports an optional closed preparation profile:

```yaml
branch:
  prepareHighLoad: true
  readiness:
    version: 1
    runner: .gz-git/readiness/check
    prepareProfile: nd-stack-checked-tests-v1
```

The target and source contracts must agree, including the profile. Unknown,
empty, null, duplicate or arbitrary command declarations fail closed. Omitting
the field preserves the existing V1 contract digest and eight runner arguments.
This opt-in is a new contract and must be adopted through the normal readiness
update or first-bootstrap procedure; editing this example does not adopt it.

The profile requires an existing declared `ndea-rs` sibling on both sides.
Gitforge verifies and materializes each exact revision through the existing
sibling protocol. It then invokes that snapshot's regular executable
`scripts/project-cargo` with these fixed Cargo argument lists:

```text
test -p nd-common -p safend -p flownd --lib --no-run
test -p nd-common -p safend -p flownd --features nd-common/events --lib --no-run
test -p nd-events --no-run
```

These are arguments, not shell commands. The engine-owned script validates its
own exact installed toolchain; Gitforge does not install Cargo or override the
engine pin. Repository scripts and Cargo build scripts are trusted executable
code, with process isolation rather than a security sandbox.

Target preparation precedes source preparation. Each side has a new build target
outside its immutable sibling snapshot. Ambient `CARGO_TARGET_DIR` is replaced.
The target-owned V1 runner still grades `--source-dir`, so its
`CARGO_TARGET_DIR` selects the prepared **source** cache. The target cache is a
separate preparation check; it is not used as a baseline-board verdict. A custom
runner that independently grades target-side Cargo tests is outside this
profile's supported runner shape. No arbitrary per-side runner environment or
arguments are accepted. Shared `CARGO_HOME` is preserved so dependency paths
remain consistent, while compilation outputs stay separate.

Each side has a thirty-minute total preparation budget. With root
`branch.prepareHighLoad: true` on either endpoint, each executor uses the existing
FIFO host slot, records wait separately, and releases capacity before cleanup.
The runner's independent heavy classification is `branch.resourceHighLoad`.
Commands use the existing process-tree cancellation path. Failure, cancellation,
timeout, changed bound refs, detached HEAD changes or sibling manifest changes
refuse runner execution. Failed sibling cleanup retains its workspace path as
evidence rather than recursively deleting the refused lease.

The checked units are a bounded ND-specific contract, not all Rust workspace
tests. Changed packages, features or binding targets require a reviewed profile
update. Synthetic native regression tests prove orchestration and refusal;
actual ND cold source/target verification, installation and first human bootstrap
remain separate consumer adoption evidence. Warm-cache success is insufficient.
