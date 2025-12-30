# History Analysis Example

This example demonstrates gz-git history analysis features using the CLI.

## Features Demonstrated

1. **Commit Statistics**: Analyze commit counts, authors, and changes over time
1. **Contributor Analysis**: Rank contributors by commits and code changes
1. **File History**: Track changes to specific files

## Usage

### Get Commit Statistics

```bash
# Stats for last 30 days
gz-git history stats --since "30 days ago"

# Stats for specific date range
gz-git history stats --since "2025-01-01" --until "2025-01-31"
```

### Analyze Contributors

```bash
# Top 10 contributors
gz-git history contributors --top 10

# Contributors in last month
gz-git history contributors --since "1 month ago"
```

### View File History

```bash
# History of specific file
gz-git history file README.md

# Last 5 commits affecting file
gz-git history file src/main.go --limit 5
```

## Output Formats

All history commands support multiple output formats:

```bash
# Table format (default)
gz-git history stats

# JSON format
gz-git history stats --format json

# CSV format
gz-git history contributors --format csv
```

## Library Usage

For library integration, see [Library Guide](../../docs/LIBRARY.md).

See [pkg/history](../../pkg/history) for complete API documentation.
