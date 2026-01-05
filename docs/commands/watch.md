# `gz-git watch`

Monitor one or more Git repositories for changes in real-time.

For the full and most up-to-date flag list, run:

```bash
gz-git watch --help
```

## What It Detects

- Modified files (unstaged)
- Staged files
- Untracked files
- Deleted files
- Branch switches
- Repository becoming clean (`--include-clean`)

## CLI Usage

```bash
# Watch current directory
gz-git watch

# Watch specific repositories
gz-git watch /path/to/repo1 /path/to/repo2

# Custom polling interval (default: 2s)
gz-git watch --interval 5s

# Output formats
gz-git watch --format compact
gz-git watch --format json
gz-git watch --format llm

# Notify on changes (macOS/Linux)
gz-git watch --notify
```

## Output Formats

- `default`: detailed output (timestamp + repo + change + files)
- `compact`: one line per event
- `json`: newline-delimited JSON events
- `llm`: prompt-friendly, stable formatting

Design notes for watch output: [docs/design/WATCH_OUTPUT_FORMATS.md](../design/WATCH_OUTPUT_FORMATS.md).

## Troubleshooting

### High CPU usage

- Increase `--interval` (e.g., `5s` or `10s`)
- Reduce the number of repositories being watched
- Avoid network filesystems (NFS/SMB) if possible

### Missing or delayed events

- Decrease `--interval` (e.g., `1s`)
- Verify the repository path is correct and accessible

## Library Usage (Go)

`pkg/watch` can be used programmatically:

```go
watcher, err := watch.NewWatcher(repoClient, watch.WatchOptions{
	Interval:         2 * time.Second,
	IncludeClean:     true,
	DebounceDuration: 500 * time.Millisecond,
})
```

See [`pkg/watch/`](../../pkg/watch/) and [`docs/user/getting-started/library-usage.md`](../user/getting-started/library-usage.md).
