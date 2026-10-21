# Controller-config integration

`integrate check`, `integrate run`, and `integrate queue` normally use only
the repository-root declaration. A devbox/controller config is opt-in:

```sh
gz-git integrate check --controller-config /path/to/devbox/.gz-git.yaml
gz-git integrate run --controller-config /path/to/devbox/.gz-git.yaml
gz-git integrate queue --controller-config /path/to/devbox/.gz-git.yaml
```

The file is never searched for in parent directories. It must contain exactly
one `workspaces` entry whose `url` canonically equals the selected repository
remote. That entry must declare exactly one `branch.integrationBranch`; its
`branch.taskPattern` is the only reclaim policy used by this mode. The file's
resolved path and content digest are rechecked before push and reclaim.

`integrate queue` uses the same explicit binding without searching for a
controller. Its comparison base is always the selected controller remote plus
the declared integration branch. `--base` is accepted only when it normalizes
to that same branch (for example `origin/develop` or `develop`). When
`branch.taskPattern` is non-empty, queue lists only matching task refs; an
empty pattern deliberately leaves the queue unfiltered. Controller resolution
errors remain exit 2 even with `--quiet`.

The selected workspace entry must carry an `integration` block; an empty one
marks it as the integration workspace:

```yaml
workspaces:
  engine:
    url: https://example.invalid/engine.git
    branch:
      integrationBranch: develop
      taskPattern: dev/*
    integration: {}
```

The controller binding supplies branch and reclaim policy only. gz-git does
not build, execute, or lint repository code itself, so the controller cannot
select a preparation step or a gate. The removed `integration.prepareProfile`
(with its `familybook-ent-v1` profile) and `branch.makeTimeout` /
`branch.prepareProfile`, as well as the readiness contract keys, fail to load,
naming the key and the controller file to delete it from; see
[removed gate declarations](integrate-prepare-profile.md). Code-level
verification belongs to the task runtime, before gz-git is called.
