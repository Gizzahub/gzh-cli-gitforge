// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"fmt"
	"os"
	"path/filepath"
)

// evidenceExecutionRunLockPath is the outer lock path next to one execution's
// receipt: <receipt-path>.run.
func evidenceExecutionRunLockPath(common, executionID string) string {
	return evidenceReceiptPath(common, executionID) + ".run"
}

// acquireExecutionRunLock keeps two invocations of one execution id from
// running at once, from preflight through push/reclaim. Receipt writes retain
// their narrower CAS lock; this distinct outer lock serializes a run against
// other invocations of the same execution identity, so no second invocation
// can advance to intent or push while the first holds it. A second invocation
// never waits: it refuses at once with ErrResourceBusy, naming the holder.
func acquireExecutionRunLock(common, executionID string) (*resourceLock, error) {
	dir := filepath.Dir(evidenceReceiptPath(common, executionID))
	if err := os.MkdirAll(dir, 0o700); err != nil { // #nosec G301 -- Git common directory is repository-owned
		return nil, fmt.Errorf("create integration run lock directory: %w", err)
	}
	return tryAcquireResourceLock(evidenceExecutionRunLockPath(common, executionID), "integration run "+executionID)
}
