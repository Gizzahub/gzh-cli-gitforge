# Integration preparation profiles

A repository may select a fixed preparation profile in its root `.gz-git.yaml`:

```yaml
branch:
  integrationBranch: develop
  prepareProfile: flow-taskchain-local-subprojects-v1
```

`branch.prepareProfile` accepts only built-in profile IDs. It does not accept
commands, paths, environment variables, or network endpoints. A profile from
the source commit is compared with the target commit before either side runs.
When a profile is declared, its root config file must be at most 64 KiB;
configs without this declaration keep their existing size behavior.

| Target declaration | Source declaration | Result                                                       |
| ------------------ | ------------------ | ------------------------------------------------------------ |
| absent             | absent             | Existing legacy Make gate                                    |
| absent             | supported profile  | Apply that built-in profile to both commits (first adoption) |
| profile A          | absent             | Fail: declaration removed                                    |
| profile A          | profile A          | Apply A to both commits                                      |
| profile A          | profile B          | Fail: profile changed                                        |

An explicit controller's `integration.prepareProfile` remains supported for
existing callers. If both controller and repository declare a profile, their
IDs must agree; a conflicting controller fails the check. A controller-only
profile keeps the existing behavior.

`flow-taskchain-local-subprojects-v1` prepares the two local child repositories
needed by the flow-taskchain devbox Make gate: engine and mcp. It locates the
root repository's registered primary checkout, then checks each child's
primary checkout, canonical remote, clean tracked and untracked
state, and symlink-free location. It captures each full HEAD object ID once,
then reads that object's raw archive through a temporary bare repository whose
object alternates point only to the child checkout. The temporary repository
uses an isolated Git environment: no inherited Git configuration or attribute
settings, filter drivers, replacement refs, lazy fetches, system or global
configuration. It then copies the resulting archive into both detached root
worktrees before measuring target and source sequentially. It performs no fetch
or clone. The result reports the profile ID and object IDs used for that
comparison.

This is a comparison against the **local child HEAD snapshot at check time**.
Those object IDs can differ from versioned CI pins and from a later check.
The profile makes one target/source comparison symmetric; it does not claim
that local child HEADs are pinned by the root repository. When a required
child is missing, dirty, or has an unexpected remote, the check fails with a
preparation error instead of treating the baseline as measurable.

`cargo-workspace-v1` runs its two steps with the same environment recipe the
make probe uses — the inherited environment plus LC_ALL=C, CARGO_TERM_COLOR=never,
and GIT_TERMINAL_PROMPT=0 — because a prepared tree only helps if the probe
later resolves the same registry, toolchain, and target directory the
preparation built against. Isolating CARGO_HOME would relocate every
dependency source path and force the probe to recompile the graph the
preparation was supposed to hand it. The 30-minute ceiling bounds fetch plus
a cold full-workspace check combined; a timeout fails the preparation instead
of producing a silent baseline.

`familybook-ent-v1` remains the fixed Ent generation profile. Details and its
controller example are in [controller-config integration](integrate-controller-config.md).
