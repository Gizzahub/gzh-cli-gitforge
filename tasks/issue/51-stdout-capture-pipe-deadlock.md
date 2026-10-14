# ISSUE: stdout capture blocks before draining its pipe

- status: resolved
- priority: P2
- category: testing
- owner: gzh-cli-gitforge — `cmd/gz-git/cmd/switch_test.go`, `cleanup_branch_test.go`

## Evidence

`captureStdout` calls the producer synchronously and only reads its pipe after
the producer returns. Output exceeding the pipe capacity blocks the producer.
The full CLI suite timed out after 600 seconds in `TestRunInfoTagStashDiff`,
writing from `printBulkTagResult` to this undrained pipe.

The same isolated test times out after 60 seconds both on unchanged source
`0ff0b2213153d2a8a7448ee2a1e3305d7c4a962c` and on the JSON stderr diagnostic
task. That test runs independently of the new integrate diagnostic test;
the failure is a confirmed baseline defect, not a successful full-suite result.

Reproduce from the repository root:

```bash
go test ./cmd/gz-git/cmd -run '^TestRunInfoTagStashDiff$' -count=1 -timeout=60s
```

Machine-local evidence is retained under
`/Users/archmagece/mycache/tmp/gzh-cli-quality.5wDebO/`:
`native-json-diagnostics-full-cmd.log`,
`native-json-diagnostics-bulk-baseline.log`, and
`native-json-diagnostics-bulk-isolated.log`.

## Resolution

Both `captureStdout` and `captureOutErr` now drain their pipes concurrently
before calling the producer and restore streams and close descriptors in
deferred cleanup. A second captured stack showed the same defect in
`TestPrintBulkCleanup_CandidateCarriesSameBasis`; its two-stream helper also
needed repair. Regression tests preserve 256 KiB on each stream independently.

The complete, unskipped CLI suite passed in 75.183 seconds after both fixes;
format and lint passed with zero issues. Independent strong reviews passed.
Evidence: `native-json-diagnostics-full-cmd-both-pipes-fixed.log` and
`native-json-diagnostics-both-pipes-fixed-style.log` in the same evidence
directory. Earlier timed-out, interrupted, and partial-skip runs are not
acceptance evidence. No production gate or assertion was weakened.
