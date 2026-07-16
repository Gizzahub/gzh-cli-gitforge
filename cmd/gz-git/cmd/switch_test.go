// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

// captureStdout redirects os.Stdout for the duration of fn and returns what was written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()

	_ = w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("copy stdout: %v", err)
	}
	return buf.String()
}

// TestDisplaySwitchResults_DefaultMode_ShowsRebaseInProgress verifies that a repo skipped
// because of an in-progress rebase remains visible in the default (non-verbose) switch
// output — previously the default-mode filter dropped it silently.
func TestDisplaySwitchResults_DefaultMode_ShowsRebaseInProgress(t *testing.T) {
	origVerbose := verbose
	verbose = false
	defer func() { verbose = origVerbose }()

	result := &repository.BulkSwitchResult{
		TotalScanned:   2,
		TotalProcessed: 2,
		TargetBranch:   "develop",
		Duration:       time.Millisecond,
		Summary: map[string]int{
			repository.StatusSwitched:         1,
			repository.StatusRebaseInProgress: 1,
		},
		Repositories: []repository.RepositorySwitchResult{
			{RelativePath: "clean-repo", Status: repository.StatusSwitched, Message: "switched to develop"},
			{RelativePath: "rebasing-repo", Status: repository.StatusRebaseInProgress, Message: "Repository has rebase in progress - skipping"},
		},
	}

	out := captureStdout(t, func() {
		displaySwitchResults(result, "default")
	})

	// The skipped repo must appear in the per-repo detail lines.
	if !strings.Contains(out, "rebasing-repo") {
		t.Errorf("default-mode output should list the rebase-in-progress repo; got:\n%s", out)
	}
	// The one-line summary must count it under the 'rebasing' bucket.
	if !strings.Contains(out, "rebasing") {
		t.Errorf("summary line should include the 'rebasing' bucket; got:\n%s", out)
	}
}
