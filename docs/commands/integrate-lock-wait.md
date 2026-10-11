# Host integration-lock waiting

`gz-git integrate check` and `gz-git integrate run` use the host measurement
lock before a gate runs. Each acquisition waits for at most 60 minutes by
default. The limit applies separately to each acquisition; it does not include
gate measurement time.

Set a repository-specific limit only in its root `.gz-git.yaml`:

```yaml
branch:
  lockWait: 90m
```

The value must be a non-empty YAML string: use `lockWait: "0"` or
`lockWait: unbounded` for an unbounded repository setting. The precedence is
`--lock-wait`, then repo-root `branch.lockWait`, then the 60-minute default.
`--lock-wait 0` explicitly waits without a limit. Negative values are rejected. When a finite wait expires,
the error names the current holder, the effective limit, and its source; no gate
is measured.
