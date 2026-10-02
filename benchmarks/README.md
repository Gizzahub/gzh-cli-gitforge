# Performance Benchmarks

This directory contains performance benchmarks for the gz-git CLI tool.

## Benchmark Results

> **Historical snapshot (2025-11-29).** The numbers below were recorded in a
> single run on that date on the hardware and Go version noted. They describe
> what was observed then, not a current performance guarantee. Re-measure with
> `make bench` / `make bench-compare` before relying on any figure here.

**Platform**: Apple M1 Ultra (ARM64), macOS Darwin
**Go Version**: go1.21+
**Date**: 2025-11-29

### Command Performance

| Command                         | Avg Time (ms) | Memory (KB) | Allocs | Status     |
| ------------------------------- | ------------- | ----------- | ------ | ---------- |
| **history file**                | 24.2          | 20          | 46     | ✅ < 50ms  |
| **history blame**               | 25.2          | 20          | 46     | ✅ < 50ms  |
| **info**                        | 38.6          | 20          | 46     | ✅ < 50ms  |
| **history stats**               | 55.9          | 20          | 46     | ✅ < 100ms |
| **status**                      | 62.1          | 20          | 46     | ✅ < 100ms |
| **status (100 commits)**        | 61.5          | 20          | 46     | ✅ < 100ms |
| **history contributors**        | 68.3          | 20          | 46     | ✅ < 100ms |
| **history stats (200 commits)** | 83.6          | 20          | 46     | ✅ < 100ms |
| **branch list**                 | 107.4         | 20          | 46     | ⚠️ > 100ms |

### Observed vs Targets (2025-11-29 run)

Observations from that single run, kept as a record — not counted as met
targets today:

- 8/9 operations measured under 100ms (89%); the p95 target was not met in
  this run and is not claimed as achieved
- 9/9 operations measured under 500ms
- No operation exceeded 2s
- Every command used under 1MB of memory

### Benchmark Categories

#### Quick Operations (10-50ms)

- `history file`: 24.2ms - File history lookup
- `history blame`: 25.2ms - File blame attribution
- `info`: 38.6ms - Repository information

#### Standard Operations (50-100ms)

- `history stats`: 55.9ms - Repository statistics
- `status`: 62.1ms - Working tree status
- `history contributors`: 68.3ms - Contributor analysis

#### Complex Operations (> 100ms)

- `branch list`: 107.4ms - Branch listing (includes remote refs)
- `history stats (200 commits)`: 83.6ms - Stats on large repository

### Scalability

Performance scales well with repository size:

- Status with 100 commits: 61.5ms (vs 62.1ms baseline)
- Stats with 200 commits: 83.6ms (vs 55.9ms for 50 commits)

**Scaling factor**: ~0.14ms per commit for stats operation

### Memory Efficiency

All operations use minimal memory:

- Average allocation: ~20KB per operation
- Allocation count: 41-46 allocs per operation
- No memory leaks observed
- Efficient for long-running processes

## Running Benchmarks

### Run All Benchmarks

```bash
cd benchmarks
go test -bench=. -benchmem -count=1
```

### Run Specific Benchmark

```bash
go test -bench=BenchmarkCLIStatus -benchmem -count=1
```

### Run with Custom Parameters

```bash
# Run for longer time
go test -bench=. -benchtime=10s -benchmem -count=1

# Run multiple iterations
go test -bench=. -benchmem -count=5

# Save results
go test -bench=. -benchmem -count=1 | tee results.txt
```

### CPU Profiling

```bash
go test -bench=. -cpuprofile=cpu.prof
go tool pprof cpu.prof
```

### Memory Profiling

```bash
go test -bench=. -memprofile=mem.prof
go tool pprof mem.prof
```

## Benchmark Implementation

Each benchmark:

1. Sets up a temporary Git repository
1. Executes the CLI command
1. Measures execution time and memory usage
1. Cleans up temporary resources

The benchmarks test realistic scenarios:

- Small repositories (single commit)
- Medium repositories (50 commits)
- Large repositories (100-200 commits)
- Various file counts and complexities

## Performance Optimization Notes

Observations below refer to the same 2025-11-29 historical snapshot.

### What Measured Well (2025-11-29)

- ✅ Efficient memory usage (< 1MB)
- ✅ Good scalability with repository size
- ✅ Minimal allocations per operation

### Areas for Future Optimization

- ⚠️ Branch list could be faster (currently 107ms)
- Consider caching for repeated operations
- Potential for parallel processing in stats

### Comparison with Git

Our commands are designed to complement git, not replace it:

- Similar performance characteristics
- Minimal overhead above native git operations
- Additional features (commit message generation, validation) add value

## Notes

- Benchmarks include process startup time
- Results may vary based on:
  - Repository size and complexity
  - Disk I/O speed
  - CPU architecture
  - Go version
- Benchmarks use fresh repositories to avoid cache effects
