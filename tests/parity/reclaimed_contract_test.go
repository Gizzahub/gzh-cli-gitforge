package parity

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func reclaimedContractError(steps []capturedStep) error {
	byLabel := make(map[string]capturedStep, len(steps))
	for _, step := range steps {
		if _, duplicate := byLabel[step.Step]; duplicate {
			return fmt.Errorf("duplicate reclaim step %s", step.Step)
		}
		byLabel[step.Step] = step
	}
	for _, label := range []string{"start", "external-worktree-removal", "external-branch-removal", "list-after-reclaim", "status-after-reclaim"} {
		if _, ok := byLabel[label]; !ok {
			return fmt.Errorf("missing reclaim step %s", label)
		}
	}
	for _, label := range []string{"start", "external-worktree-removal", "external-branch-removal"} {
		if step := byLabel[label]; step.ExitCode != 0 || step.Stderr != "" {
			return fmt.Errorf("reclaim setup %s failed: exit %d, stderr %s", label, step.ExitCode, step.Stderr)
		}
	}
	var start struct{ Status, Task string }
	if err := json.Unmarshal([]byte(byLabel["start"].Stdout), &start); err != nil {
		return err
	}
	if start.Status != "ACTIVE" || start.Task != "reclaimed-task" {
		return fmt.Errorf("reclaim setup did not create reclaimed-task: %+v", start)
	}
	list := byLabel["list-after-reclaim"]
	var doc struct {
		Status      string
		ActiveCount int
		Executions  []json.RawMessage
		States      []struct{ Status, Reason string }
	}
	if err := json.Unmarshal([]byte(list.Stdout), &doc); err != nil {
		return err
	}
	const reason = "Worktrunk inventory does not match execution metadata"
	if list.ExitCode != 1 || list.Stderr != "Error: one or more task executions are blocked\n" || doc.Status != "BLOCKED" || doc.ActiveCount != 0 || len(doc.Executions) != 1 || len(doc.States) != 1 || doc.States[0].Status != "BLOCKED" || doc.States[0].Reason != reason {
		return fmt.Errorf("reclaimed observation must be exactly BLOCKED/inactive with retained history: %+v", doc)
	}
	var executions []json.RawMessage
	if list.State == nil {
		return fmt.Errorf("missing reclaimed resource snapshot")
	}
	if err := json.Unmarshal(list.State.Executions, &executions); err != nil {
		return err
	}
	if len(executions) != 1 {
		return fmt.Errorf("reclaim changed execution history")
	}
	type retainedExecution struct {
		Task, Branch, Worktree, Source string
		Owner                          struct{ Actor, Host, Kind string }
	}
	var execution retainedExecution
	if err := json.Unmarshal(executions[0], &execution); err != nil {
		return err
	}
	const branch = "dev/claude/mbp/chore/reclaimed-task"
	const worktree = "<SANDBOX>/wt/" + sandboxRepoID + "/claude__mbp__chore__reclaimed-task"
	if execution.Task != "reclaimed-task" || execution.Branch != branch || execution.Worktree != worktree || execution.Source != "master" || execution.Owner.Actor != "claude" || execution.Owner.Host != host || execution.Owner.Kind != "agent" {
		return fmt.Errorf("wrong retained execution identity: %+v", execution)
	}
	var listed retainedExecution
	if err := json.Unmarshal(doc.Executions[0], &listed); err != nil {
		return err
	}
	if listed != execution {
		return fmt.Errorf("list execution disagrees with stored identity")
	}
	if strings.Contains(list.State.Worktrees, worktree) || strings.Contains(list.State.Refs, "refs/heads/"+branch) || strings.Contains(list.State.OriginRefs, "refs/heads/"+branch) {
		return fmt.Errorf("external reclaim retained task worktree or local ref")
	}
	if len(list.State.Receipts) != 1 {
		return fmt.Errorf("observation changed receipt history")
	}
	var receipt struct{ Operation, Status, Task string }
	if err := json.Unmarshal(list.State.Receipts[0], &receipt); err != nil {
		return err
	}
	if receipt.Operation != "start" || receipt.Status != "ACTIVE" || receipt.Task != "reclaimed-task" {
		return fmt.Errorf("observation replaced original start receipt: %+v", receipt)
	}
	status := byLabel["status-after-reclaim"]
	var after struct{ Status, Task, Reason string }
	if err := json.Unmarshal([]byte(status.Stdout), &after); err != nil {
		return err
	}
	if status.ExitCode != 1 || status.Stderr != "Error: "+reason+"\n" || after.Status != "BLOCKED" || after.Task != "reclaimed-task" || after.Reason != reason {
		return fmt.Errorf("status disagrees with reclaimed observation: %+v", after)
	}
	return nil
}

func TestReclaimedContractRejectsFailedCleanup(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "golden", "reclaimed-run-reconciliation.json")) //nolint:gosec // committed fixture
	if err != nil {
		t.Fatal(err)
	}
	var baseline goldenFile
	if err := json.Unmarshal(data, &baseline); err != nil {
		t.Fatal(err)
	}
	if err := reclaimedContractError(baseline.Steps); err != nil {
		t.Fatalf("valid capture rejected: %v", err)
	}
	for _, label := range []string{"start", "external-worktree-removal", "external-branch-removal"} {
		t.Run(label, func(t *testing.T) {
			steps := append([]capturedStep(nil), baseline.Steps...)
			for i := range steps {
				if steps[i].Step == label {
					steps[i].ExitCode = 1
				}
			}
			if err := reclaimedContractError(steps); err == nil {
				t.Fatal("failed cleanup/setup was accepted")
			}
		})
	}
}
