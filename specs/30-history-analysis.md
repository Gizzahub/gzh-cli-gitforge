# Phase 4: History Analysis Specification

**Phase**: 4
**Priority**: P0 (High)
**Status**: ✅ Implemented
**Created**: 2025-11-27
**Last Updated**: 2026-01-05
**Version**: 2.0
**Dependencies**: Phase 3 (Branch Management)

> **Implementation Note**: All history analysis features are implemented and available via CLI commands (`gz-git history stats/contributors/file/blame`).

______________________________________________________________________

## Overview

Phase 4 implements comprehensive Git history analysis capabilities, enabling developers to gain insights into repository evolution, contributor patterns, and file change history. This phase focuses on statistical analysis, contributor metrics, and file-level history tracking with multiple output formats.

### Goals

1. **Commit Statistics** - Analyze commit patterns, frequency, and trends
1. **Contributor Analysis** - Track developer contributions and collaboration patterns
1. **File History** - Trace file evolution, changes, and ownership
1. **Multiple Output Formats** - Support table, JSON, CSV, and markdown outputs
1. **Performance** - Efficient analysis of large repositories

### Non-Goals

- Real-time monitoring or alerting
- Code quality metrics (complexity, maintainability)
- CI/CD integration
- Web UI or dashboard

______________________________________________________________________

## Architecture

### Package Structure

```
pkg/history/
├── types.go           # Core types and interfaces
├── errors.go          # History-specific errors
├── analyzer.go        # Commit statistics analyzer
├── contributor.go     # Contributor analysis
├── file_history.go    # File history tracker
├── formatter.go       # Output formatters
├── analyzer_test.go
├── contributor_test.go
├── file_history_test.go
└── formatter_test.go
```

### Core Interfaces

```go
// HistoryAnalyzer analyzes commit history
type HistoryAnalyzer interface {
    Analyze(ctx context.Context, repo *repository.Repository, opts AnalyzeOptions) (*CommitStats, error)
    GetTrends(ctx context.Context, repo *repository.Repository, opts TrendOptions) (*CommitTrends, error)
}

// ContributorAnalyzer analyzes contributor activity
type ContributorAnalyzer interface {
    Analyze(ctx context.Context, repo *repository.Repository, opts ContributorOptions) ([]*Contributor, error)
    GetTopContributors(ctx context.Context, repo *repository.Repository, limit int) ([]*Contributor, error)
}

// FileHistoryTracker tracks file evolution
type FileHistoryTracker interface {
    GetHistory(ctx context.Context, repo *repository.Repository, path string, opts HistoryOptions) ([]*FileCommit, error)
    GetBlame(ctx context.Context, repo *repository.Repository, path string) (*BlameInfo, error)
}

// Formatter formats analysis results
type Formatter interface {
    FormatCommitStats(stats *CommitStats) ([]byte, error)
    FormatContributors(contributors []*Contributor) ([]byte, error)
    FormatFileHistory(history []*FileCommit) ([]byte, error)
}
```

______________________________________________________________________

## Component 1: History Analyzer

### Purpose

Analyze commit history to extract statistical insights about repository activity patterns.

### Features

1. **Commit Statistics**

   - Total commits count
   - Date range (first/last commit)
   - Average commits per day/week/month
   - Commit frequency distribution
   - Peak activity periods

1. **Trend Analysis**

   - Commit trends over time
   - Activity patterns (hourly, daily, weekly)
   - Growth rate analysis
   - Seasonal patterns

1. **Branch Analysis**

   - Commits per branch
   - Branch activity comparison
   - Merge frequency

### Data Types

```go
// CommitStats represents commit statistics
type CommitStats struct {
    TotalCommits    int
    FirstCommit     time.Time
    LastCommit      time.Time
    DateRange       time.Duration
    AvgPerDay       float64
    AvgPerWeek      float64
    AvgPerMonth     float64
    PeakDay         time.Time
    PeakCount       int
    UniqueAuthors   int
    TotalFiles      int
    TotalAdditions  int
    TotalDeletions  int
}

// CommitTrends represents trend data
type CommitTrends struct {
    Daily   map[string]int // date -> count
    Weekly  map[string]int // week -> count
    Monthly map[string]int // month -> count
    Hourly  map[int]int    // hour -> count
}

// AnalyzeOptions configures history analysis
type AnalyzeOptions struct {
    Since      time.Time
    Until      time.Time
    Branch     string
    Author     string
    MaxCommits int
}
```

### Git Commands

```bash
# Get commit count
git rev-list --count HEAD

# Get first and last commit
git log --reverse --format='%H %ct' | head -1
git log --format='%H %ct' | head -1

# Get commit statistics
git log --shortstat --format='%H|%an|%ae|%ct'

# Get commits by date range
git log --since="2025-01-01" --until="2025-12-31" --format='%H|%ct'

# Get commits per branch
git for-each-ref --format='%(refname:short)' refs/heads/ | \
  xargs -I {} git rev-list --count {}
```

### Implementation

```go
type historyAnalyzer struct {
    executor *gitcmd.Executor
}

func (h *historyAnalyzer) Analyze(ctx context.Context, repo *repository.Repository, opts AnalyzeOptions) (*CommitStats, error) {
    // Build git log command with options
    args := []string{"log", "--shortstat", "--format=%H|%an|%ae|%ct"}

    if !opts.Since.IsZero() {
        args = append(args, fmt.Sprintf("--since=%s", opts.Since.Format(time.RFC3339)))
    }

    if !opts.Until.IsZero() {
        args = append(args, fmt.Sprintf("--until=%s", opts.Until.Format(time.RFC3339)))
    }

    if opts.Branch != "" {
        args = append(args, opts.Branch)
    }

    // Execute git log
    result, err := h.executor.Run(ctx, repo.Path, args...)
    if err != nil {
        return nil, fmt.Errorf("failed to get commit log: %w", err)
    }

    // Parse output and calculate statistics
    stats := h.parseCommitStats(result.Stdout)

    return stats, nil
}
```

______________________________________________________________________

## Component 2: Contributor Analyzer

### Purpose

Analyze contributor activity to understand team dynamics, individual contributions, and collaboration patterns.

### Features

1. **Contributor Metrics**

   - Total commits per contributor
   - Lines added/deleted per contributor
   - Files touched per contributor
   - Contribution period (first/last commit)
   - Activity level (commits per week)

1. **Ranking**

   - Top contributors by commits
   - Top contributors by lines changed
   - Most active contributors (recent period)

1. **Collaboration**

   - Co-authorship patterns
   - File overlap between contributors
   - Pair programming detection

### Data Types

```go
// Contributor represents a repository contributor
type Contributor struct {
    Name            string
    Email           string
    TotalCommits    int
    FirstCommit     time.Time
    LastCommit      time.Time
    LinesAdded      int
    LinesDeleted    int
    FilesTouched    int
    ActiveDays      int
    CommitsPerWeek  float64
    Rank            int
}

// ContributorOptions configures contributor analysis
type ContributorOptions struct {
    Since      time.Time
    Until      time.Time
    MinCommits int
    SortBy     ContributorSortBy
}

type ContributorSortBy string

const (
    SortByCommits     ContributorSortBy = "commits"
    SortByLinesAdded  ContributorSortBy = "additions"
    SortByLinesDeleted ContributorSortBy = "deletions"
    SortByRecent      ContributorSortBy = "recent"
)
```

### Git Commands

```bash
# Get contributors with commit count
git shortlog -sne --all

# Get detailed contributor stats
git log --format='%an|%ae|%ct' --numstat

# Get contributor activity by date
git log --author="John Doe" --format='%ct' --numstat

# Get files touched by contributor
git log --author="John Doe" --name-only --format='' | sort -u
```

### Implementation

```go
type contributorAnalyzer struct {
    executor *gitcmd.Executor
}

func (c *contributorAnalyzer) Analyze(ctx context.Context, repo *repository.Repository, opts ContributorOptions) ([]*Contributor, error) {
    // Get shortlog for basic stats
    result, err := c.executor.Run(ctx, repo.Path, "shortlog", "-sne", "--all")
    if err != nil {
        return nil, fmt.Errorf("failed to get shortlog: %w", err)
    }

    contributors := c.parseShortlog(result.Stdout)

    // Enrich with detailed stats for each contributor
    for _, contributor := range contributors {
        if err := c.enrichContributor(ctx, repo, contributor, opts); err != nil {
            // Log error but continue with other contributors
            continue
        }
    }

    // Sort contributors
    c.sortContributors(contributors, opts.SortBy)

    return contributors, nil
}
```

______________________________________________________________________

## Component 3: File History Tracker

### Purpose

Track file evolution, changes, and ownership to understand file-level development patterns.

### Features

1. **File History**

   - Commit history for specific file
   - Change summary (additions/deletions per commit)
   - Authors who modified file
   - Rename detection

1. **File Blame**

   - Line-by-line authorship
   - Last modification date per line
   - Commit hash per line

1. **File Statistics**

   - Total modifications
   - Total authors
   - Average change size
   - Churn rate (changes / age)

### Data Types

```go
// FileCommit represents a commit affecting a file
type FileCommit struct {
    Hash        string
    Author      string
    AuthorEmail string
    Date        time.Time
    Message     string
    LinesAdded  int
    LinesDeleted int
    IsBinary    bool
    WasRenamed  bool
    OldPath     string
}

// BlameInfo represents file blame information
type BlameInfo struct {
    FilePath string
    Lines    []*BlameLine
}

// BlameLine represents blame info for a single line
type BlameLine struct {
    LineNumber  int
    Content     string
    Hash        string
    Author      string
    AuthorEmail string
    Date        time.Time
}

// HistoryOptions configures file history retrieval
type HistoryOptions struct {
    MaxCount   int
    Since      time.Time
    Until      time.Time
    Follow     bool // Follow renames
    Author     string
}
```

### Git Commands

```bash
# Get file history
git log --follow --format='%H|%an|%ae|%ct|%s' --numstat -- path/to/file

# Get file blame
git blame -e --date=iso path/to/file

# Get file with rename detection
git log --follow --format='%H|%an|%ae|%ct' --name-status -- path/to/file

# Get file statistics
git log --format='%H' --numstat -- path/to/file | \
  awk '/^[0-9]/ {add+=$1; del+=$2} END {print add, del}'
```

### Implementation

```go
type fileHistoryTracker struct {
    executor *gitcmd.Executor
}

func (f *fileHistoryTracker) GetHistory(ctx context.Context, repo *repository.Repository, path string, opts HistoryOptions) ([]*FileCommit, error) {
    // Build git log command
    args := []string{"log", "--format=%H|%an|%ae|%ct|%s", "--numstat"}

    if opts.Follow {
        args = append(args, "--follow")
    }

    if opts.MaxCount > 0 {
        args = append(args, fmt.Sprintf("--max-count=%d", opts.MaxCount))
    }

    if !opts.Since.IsZero() {
        args = append(args, fmt.Sprintf("--since=%s", opts.Since.Format(time.RFC3339)))
    }

    args = append(args, "--", path)

    // Execute git log
    result, err := f.executor.Run(ctx, repo.Path, args...)
    if err != nil {
        return nil, fmt.Errorf("failed to get file history: %w", err)
    }

    // Parse output
    commits := f.parseFileHistory(result.Stdout, path)

    return commits, nil
}
```

______________________________________________________________________

## Component 4: Output Formatters

### Purpose

Format analysis results in multiple output formats for different use cases.

### Supported Formats

1. **Table** - Human-readable ASCII table
1. **JSON** - Machine-readable structured data
1. **CSV** - Spreadsheet-compatible format
1. **Markdown** - Documentation-friendly format

### Implementation

```go
type OutputFormat string

const (
    FormatTable    OutputFormat = "table"
    FormatJSON     OutputFormat = "json"
    FormatCSV      OutputFormat = "csv"
    FormatMarkdown OutputFormat = "markdown"
)

type formatter struct {
    format OutputFormat
}

func (f *formatter) FormatCommitStats(stats *CommitStats) ([]byte, error) {
    switch f.format {
    case FormatTable:
        return f.formatStatsTable(stats)
    case FormatJSON:
        return json.MarshalIndent(stats, "", "  ")
    case FormatCSV:
        return f.formatStatsCSV(stats)
    case FormatMarkdown:
        return f.formatStatsMarkdown(stats)
    default:
        return nil, fmt.Errorf("unsupported format: %s", f.format)
    }
}
```

______________________________________________________________________

## Error Handling

### Custom Errors

```go
var (
    ErrEmptyHistory     = errors.New("repository has no commit history")
    ErrInvalidDateRange = errors.New("invalid date range (since > until)")
    ErrFileNotFound     = errors.New("file not found in repository history")
    ErrInvalidFormat    = errors.New("invalid output format")
    ErrNoContributors   = errors.New("no contributors found")
)
```

______________________________________________________________________

## Testing Strategy

### Unit Tests

1. **History Analyzer Tests**

   - Parse commit statistics
   - Calculate trends
   - Handle date ranges
   - Handle empty repositories

1. **Contributor Analyzer Tests**

   - Parse shortlog output
   - Sort contributors
   - Handle duplicate emails
   - Calculate metrics

1. **File History Tests**

   - Parse file commits
   - Handle renames
   - Handle binary files
   - Parse blame output

1. **Formatter Tests**

   - Format in all supported formats
   - Handle empty data
   - Handle special characters
   - Validate JSON structure

### Integration Tests

Deferred to Phase 6 (requires real Git repositories).

### Coverage Target

- Unit tests: ≥85%
- Integration tests: ≥80%
- Overall: ≥85%

______________________________________________________________________

## Performance Considerations

### Large Repositories

1. **Limit commit count** - Use `--max-count` for initial queries
1. **Incremental analysis** - Support date range filtering
1. **Caching** - Cache parsed results for repeated queries
1. **Streaming** - Process large outputs in chunks

### Memory Management

```go
// Process commits in batches
const batchSize = 1000

func (h *historyAnalyzer) analyzeLargeRepo(ctx context.Context, repo *repository.Repository) error {
    offset := 0
    for {
        commits, err := h.getCommitBatch(ctx, repo, offset, batchSize)
        if err != nil {
            return err
        }

        if len(commits) == 0 {
            break
        }

        // Process batch
        h.processCommitBatch(commits)

        offset += batchSize
    }

    return nil
}
```

______________________________________________________________________

## CLI Commands (Implemented)

### Command Structure

```bash
# Commit statistics
gz-git history stats                          # Show commit statistics
gz-git history stats --since "2025-01-01"     # Filter by date
gz-git history stats --format json            # JSON output

# Contributor analysis
gz-git history contributors                   # List all contributors
gz-git history contributors --top 10          # Top 10 contributors
gz-git history contributors --sort commits    # Sort by commits
gz-git history contributors --format table    # Table output

# File history
gz-git history file src/main.go               # File change history
gz-git history file src/main.go --follow      # Follow renames
gz-git history file src/main.go --max 20      # Limit results

# File blame
gz-git history blame src/main.go              # Line-by-line authorship
```

### Output Formats

- `default` - Human-readable colored output
- `table` - ASCII table format
- `json` - Machine-readable JSON
- `compact` - Condensed single-line format

______________________________________________________________________

## Dependencies

### Internal

- `pkg/repository` - Repository operations
- `internal/gitcmd` - Git command execution
- `internal/parser` - Output parsing utilities

### External

- Standard library only (no external dependencies)

______________________________________________________________________

## Success Criteria

1. ✅ All core interfaces implemented
1. ✅ Comprehensive unit tests (≥85% coverage)
1. ✅ Support for all output formats (table, JSON, CSV, markdown)
1. ✅ Handle large repositories efficiently (>10K commits)
1. ✅ Accurate statistics and metrics
1. ✅ Error handling for all edge cases
1. ✅ Documentation and examples

______________________________________________________________________

## Implementation Checklist (Completed)

### Phase 4.1: History Analyzer

- [x] Define types.go (CommitStats, CommitTrends, AnalyzeOptions)
- [x] Define errors.go (history-specific errors)
- [x] Implement analyzer.go (HistoryAnalyzer interface)
- [x] Write analyzer_test.go (unit tests)
- [x] Validation: All tests passing

### Phase 4.2: Contributor Analyzer

- [x] Add Contributor types to types.go
- [x] Implement contributor.go (ContributorAnalyzer interface)
- [x] Write contributor_test.go (unit tests)
- [x] Validation: All tests passing

### Phase 4.3: File History Tracker

- [x] Add FileCommit, BlameInfo types to types.go
- [x] Implement file_history.go (FileHistoryTracker interface)
- [x] Write file_history_test.go (unit tests)
- [x] Validation: All tests passing

### Phase 4.4: Output Formatters

- [x] Implement formatter.go (all format types)
- [x] Write formatter_test.go (unit tests)
- [x] Validation: All tests passing

### Phase 4.5: CLI Integration

- [x] Implement cmd/gz-git/cmd/history.go (parent command)
- [x] Implement cmd/gz-git/cmd/history_stats.go
- [x] Implement cmd/gz-git/cmd/history_contributors.go
- [x] Implement cmd/gz-git/cmd/history_file.go
- [x] Validation: All commands functional

______________________________________________________________________

## Timeline

- **Phase 4.1**: 1-2 days (History Analyzer)
- **Phase 4.2**: 1-2 days (Contributor Analyzer)
- **Phase 4.3**: 1-2 days (File History Tracker)
- **Phase 4.4**: 1 day (Output Formatters)
- **Phase 4.5**: 1 day (Integration & Documentation)

**Total Estimated**: 5-8 days

______________________________________________________________________

## References

- [Git Log Documentation](https://git-scm.com/docs/git-log)
- [Git Shortlog Documentation](https://git-scm.com/docs/git-shortlog)
- [Git Blame Documentation](https://git-scm.com/docs/git-blame)
- Phase 1: Foundation (Complete)
- Phase 2: Commit Automation (Complete)
- Phase 3: Branch Management (Complete)

______________________________________________________________________

## Revision History

| Version | Date       | Changes |
| ------- | ---------- | ------- |
| 1.0     | 2025-11-27 | Initial specification |
| 2.0     | 2026-01-05 | Updated for implementation: CLI commands added, all checklist items completed |

______________________________________________________________________

**Specification Status**: ✅ Implemented
**Implementation Version**: v0.3.0+
**Key Files**:
- `cmd/gz-git/cmd/history.go` - History command group
- `cmd/gz-git/cmd/history_stats.go` - Statistics subcommand
- `cmd/gz-git/cmd/history_contributors.go` - Contributors subcommand
- `cmd/gz-git/cmd/history_file.go` - File history subcommand
- `pkg/history/analyzer.go` - Commit statistics analyzer
- `pkg/history/contributor.go` - Contributor analysis
- `pkg/history/file_history.go` - File history tracker
- `pkg/history/formatter.go` - Output formatters
