package runtask

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

type rejectedObservationCommand struct{ calls []CommandRequest }

func (r *rejectedObservationCommand) Run(_ context.Context, req CommandRequest) (CommandResult, error) {
	r.calls = append(r.calls, req)
	return CommandResult{}, fmt.Errorf("observation must not probe landing evidence")
}

func TestMissingInventoryObservationDoesNotCloseExecution(t *testing.T) {
	root, service := newImportRepo(t)
	worktrees := filepath.Join(t.TempDir(), "worktrees", "fixture")
	worktrees, err := canonicalizePotentialPath(worktrees)
	if err != nil {
		t.Fatal(err)
	}
	config := fmt.Sprintf("schema-version: 2\nrepository-id: fixture\nworktree-roots:\n  mst: %q\nintegration-provider: gz-git\nintegration-network-policy: allow\n", worktrees)
	if err := os.WriteFile(filepath.Join(root, TaskRuntimeConfigFile), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &rejectedObservationCommand{}
	service.runner = runner
	execution := TaskExecution{
		Task: "lost", Owner: TaskOwner{Actor: "codex", Host: "mst", Kind: "agent"},
		Branch: "dev/codex/mst/chore/lost", Source: "master",
	}
	execution.Worktree = expectedWorktreePath(worktrees, execution.Branch)
	for range 2 {
		state := service.deriveState(t.Context(), execution, TaskReceipt{}, worktrunkInventory{}, false)
		if state.Status != TaskExecutionStatusBlocked || state.Reason != "Worktrunk inventory does not match execution metadata" {
			t.Fatalf("missing execution must remain blocked: %+v", state)
		}
	}
	if len(runner.calls) != 0 {
		t.Fatalf("read derivation probed landing evidence: %+v", runner.calls)
	}
	if _, err := os.Stat(filepath.Join(root, ".git", "gz-git", "task-runtime")); !os.IsNotExist(err) {
		t.Fatalf("read derivation created runtime state or a mutation lock: %v", err)
	}
}
