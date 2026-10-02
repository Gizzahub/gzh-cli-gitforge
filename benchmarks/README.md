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

## Converting Captured Output to a Report (offline)

Collecting and converting are separate steps:

- **Collection** runs the benchmarks and captures raw text:
  `make bench` (or `go test -bench=. -benchmem -count=5 | tee bench.txt`).
  Use a higher `-count` so each benchmark name has several samples to
  aggregate.

- **Conversion** turns one already-captured text file plus a metadata sidecar
  into a machine-readable JSON report — no benchmarks run, no network, no
  telemetry:

  ```bash
  make benchmark-report \
    INPUT=bench.txt \
    METADATA=run-metadata.json \
    OUTPUT=report.json
  ```

  `INPUT`, `METADATA` and `OUTPUT` are all required. The converter refuses to
  overwrite an existing `OUTPUT` file, so re-running with the same path fails
  fast instead of silently replacing earlier evidence.

The metadata sidecar is a small JSON object recording how the input was
produced — `sourceCommit`, `goVersion`, `gitVersion`, `os`, `arch`,
`workload`, `observedAt`, `measurementCommand` (all required, non-empty;
`note` optional). The converter embeds it verbatim, so a report always states
which commit, toolchain, and command produced its numbers.

The output is a `schemaVersion: 1` document with three parts:

- `metadata` — the sidecar, unchanged;
- `samples` — one entry per benchmark result line (`name`, `iterations`,
  `nsPerOp`), sorted by name; entries sharing a name keep input order;
- `summary` — per name: `sampleCount`, `minMeanNsPerOp`,
  `medianMeanNsPerOp`, `maxMeanNsPerOp`, sorted by name.

### What ns/op means — and what this report does not measure

`ns/op` is the **mean** nanoseconds per operation across that sample's `b.N`
iterations, as printed by `go test -bench`. Each sample line is one mean;
`summary` statistics describe the distribution of those sample means. They
are **not** per-operation latencies, so no p95 or any other per-operation
percentile is computed or emitted — a mean of means cannot recover the
operation-level distribution. Benchmark names keep the `-N` GOMAXPROCS suffix
Go prints (for example `BenchmarkCLIStatus-10`), because samples taken at
different `GOMAXPROCS` values are different configurations and must not be
merged. For an even number of samples the median is the arithmetic mean of
the two central values; for an odd count it is the middle value.

Adoption, reliability, and real-world workload coverage remain unmeasured by
this converter. There are no CI performance thresholds: a report is recorded
evidence, not a gate.

### Synthetic fixture

`testdata/report.input.txt`, `testdata/report.metadata.json`, and
`testdata/report.expected.json` are a **synthetic** example used by the
converter contract tests (`internal/benchmarkreport`). The metadata marks it
(`workload: "synthetic-example"` and a note), and its numbers are invented to
exercise parsing and aggregation — including repeated samples for one name
with both even and odd counts. It is not product performance evidence and
must not be quoted as current gz-git performance. Real measurement
collection is a separate task; until it runs, no non-synthetic report
exists.

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
