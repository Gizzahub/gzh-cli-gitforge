package parity

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// assertLifecycleContract preserves safety checks independently of byte captures.
func assertLifecycleContract(t *testing.T, sc scenario, steps []capturedStep) {
	t.Helper()
	required := sc.name == "finish-noarg-single-active" || sc.name == "finish-noarg-ambiguous-active" || sc.name == "reclaimed-run-reconciliation"
	seen := false
	for _, st := range steps {
		switch {
		case sc.name == "finish-noarg-single-active" && st.Step == "finish-noarg":
			seen = true
			assertBareFinishContract(t, steps, st)
		case sc.name == "finish-noarg-ambiguous-active" && st.Step == "finish-noarg":
			seen = true
			var doc struct{ Status, Task, Reason string }
			must(t, json.Unmarshal([]byte(st.Stdout), &doc))
			if st.ExitCode != 1 || doc.Status != "BLOCKED" || doc.Task != "" || !strings.Contains(doc.Reason, "ambiguous") {
				t.Fatalf("bare finish must refuse multiple owned tasks: %+v (exit %d)", doc, st.ExitCode)
			}
		case sc.name == "reclaimed-run-reconciliation" && st.Step == "list-after-reclaim":
			seen = true
			if err := reclaimedContractError(steps); err != nil {
				t.Fatalf("reclaimed lifecycle contract: %v", err)
			}
			var doc struct {
				Status      string
				ActiveCount int
				Executions  []json.RawMessage
				States      []struct{ Status string }
			}
			must(t, json.Unmarshal([]byte(st.Stdout), &doc))
			if st.ExitCode != 1 || doc.Status != "BLOCKED" || doc.ActiveCount != 0 || len(doc.Executions) != 1 || len(doc.States) != 1 || doc.States[0].Status != "BLOCKED" {
				t.Fatalf("observation must retain the missing execution as blocked: %+v (exit %d)", doc, st.ExitCode)
			}
			if st.State == nil || len(st.State.Receipts) != 1 {
				t.Fatal("read-only observation must leave exactly the original start receipt")
			}
			var receipt struct{ Operation, Status string }
			must(t, json.Unmarshal(st.State.Receipts[0], &receipt))
			if receipt.Operation != "start" || receipt.Status != "ACTIVE" {
				t.Fatalf("observation replaced the original start receipt: %+v", receipt)
			}
		}
	}
	if required && !seen {
		t.Fatalf("scenario %s omitted its protected lifecycle step", sc.name)
	}
}

func assertBareFinishContract(t *testing.T, steps []capturedStep, finish capturedStep) {
	t.Helper()
	find := func(label string) capturedStep {
		for _, step := range steps {
			if step.Step == label {
				return step
			}
		}
		t.Fatalf("bare finish omitted %s", label)
		return capturedStep{}
	}
	ready := find("status-ready")
	var before struct {
		Status, Task string
		FinishReady  bool
	}
	must(t, json.Unmarshal([]byte(ready.Stdout), &before))
	if ready.ExitCode != 0 || before.Status != "ACTIVE" || before.Task != "noarg" || !before.FinishReady || ready.State == nil {
		t.Fatalf("bare finish precondition must be ready ACTIVE/noarg: %+v", before)
	}
	var doc struct {
		Status, Task string
		ActiveCount  int
		Receipt      json.RawMessage
	}
	must(t, json.Unmarshal([]byte(finish.Stdout), &doc))
	if finish.ExitCode != 0 || finish.Stderr != "" || finish.State == nil {
		t.Fatalf("bare finish failed or omitted its state: %+v", finish)
	}
	after := find("status-after-finish")
	var state struct {
		Status, Task string
		FinishReady  bool
	}
	must(t, json.Unmarshal([]byte(after.Stdout), &state))
	const taskRef = "refs/heads/dev/claude/mbp/chore/noarg"
	const taskPath = "<SANDBOX>/wt/" + sandboxRepoID + "/claude__mbp__chore__noarg"
	if targetMode {
		if doc.Status != "READY" || doc.Task != "" || doc.ActiveCount != 1 || len(doc.Receipt) != 0 {
			t.Fatalf("native bare finish must return READY/task empty/activeCount 1/no receipt: %+v", doc)
		}
		if !reflect.DeepEqual(ready.State, finish.State) {
			t.Fatal("native bare finish changed retained Git resources or execution/receipt records")
		}
		retained := false
		for _, block := range strings.Split(finish.State.Worktrees, "\n\n") {
			lines := "\n" + strings.TrimSpace(block) + "\n"
			if strings.Contains(lines, "\nworktree "+taskPath+"\n") && strings.Contains(lines, "\nbranch "+taskRef+"\n") {
				retained = true
			}
		}
		if !retained || !strings.Contains(finish.State.Refs, taskRef+" ") || !strings.Contains(finish.State.OriginRefs, taskRef+" ") {
			t.Fatal("native READY omitted exact task worktree and local/remote refs")
		}
		if after.ExitCode != 0 || state.Status != "ACTIVE" || state.Task != "noarg" || !state.FinishReady {
			t.Fatalf("native bare finish did not retain ready ACTIVE execution: %+v", state)
		}
		return
	}
	if doc.Status != "DONE" || doc.Task != "noarg" || after.ExitCode != 0 || state.Status != "DONE" {
		t.Fatalf("CE bare finish must complete noarg: %+v / %+v", doc, state)
	}
	if strings.Contains(finish.State.Worktrees, taskPath) || strings.Contains(finish.State.Refs, taskRef) || strings.Contains(finish.State.OriginRefs, taskRef) {
		t.Fatal("CE DONE retained task worktree or refs")
	}
}
