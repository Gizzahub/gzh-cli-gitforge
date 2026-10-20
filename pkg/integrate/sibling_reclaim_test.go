// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

func TestRun_ReclaimRetainsTaskUntilTamperedSiblingLeaseIsRepaired(t *testing.T) {
	t.Parallel()
	fx := runFixture(t, "dev/*")
	dependency, dep := siblingFixture(t)
	lease, err := ProvisionSibling(context.Background(), fx.Worktree, dependency.Clone, dep)
	if err != nil {
		t.Fatal(err)
	}
	lease, err = PersistSiblingLease(context.Background(), fx.Worktree, lease)
	if err != nil {
		t.Fatal(err)
	}

	payload := filepath.Join(lease.Destination, "payload.txt")
	writeFile(t, lease.Destination, "payload.txt", "tampered\n")

	report, err := Run(context.Background(), gitcmd.NewExecutor(), RunOptions{
		CheckOptions: CheckOptions{RepoPath: fx.Worktree},
	})
	if err != nil {
		t.Fatalf("Run: %v\n%s", err, FormatRun(report))
	}
	if !report.Integrated {
		t.Fatalf("integration did not complete:\n%s", FormatRun(report))
	}
	if !report.Reclaim.Incomplete() {
		t.Fatalf("tampered sibling must retain reclaim lease: %+v", report.Reclaim)
	}
	failed := strings.Join(report.Reclaim.Failed, "; ")
	if !strings.Contains(failed, "release provisioned siblings") || !strings.Contains(failed, "changed") {
		t.Fatalf("reclaim failure = %q, want release refusal", failed)
	}
	if _, err := os.Stat(fx.Worktree); err != nil {
		t.Fatalf("task worktree must remain after failed release: %v", err)
	}
	assertRef(t, fx.Clone, "refs/heads/dev/actor/feat/task")
	assertRef(t, fx.Origin, "refs/heads/dev/actor/feat/task")

	writeFile(t, lease.Destination, "payload.txt", "source\n")
	retry, err := Run(context.Background(), gitcmd.NewExecutor(), RunOptions{
		CheckOptions: CheckOptions{RepoPath: fx.Worktree},
	})
	if err != nil {
		t.Fatalf("retry Run: %v\n%s", err, FormatRun(retry))
	}
	if !retry.Integrated || retry.Reclaim.Incomplete() {
		t.Fatalf("retry must finish integration and reclaim:\n%s", FormatRun(retry))
	}
	if _, err := os.Stat(fx.Worktree); !os.IsNotExist(err) {
		t.Fatalf("task worktree remains after repaired release: %v", err)
	}
	if refExists(t, fx.Clone, "refs/heads/dev/actor/feat/task") {
		t.Fatal("local task branch remains after repaired release")
	}
	if refExists(t, fx.Origin, "refs/heads/dev/actor/feat/task") {
		t.Fatal("remote task branch remains after repaired release")
	}
	if _, err := os.Stat(payload); !os.IsNotExist(err) {
		t.Fatalf("released sibling snapshot remains: %v", err)
	}
}
