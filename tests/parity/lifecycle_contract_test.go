package parity

import (
	"encoding/json"
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
			var doc struct{ Status, Task, Reason string }
			must(t, json.Unmarshal([]byte(st.Stdout), &doc))
			if st.ExitCode != 1 || doc.Status != "ACTIVE" || doc.Task != "noarg" || doc.Reason != "task branch has no upstream" {
				t.Fatalf("bare finish must select the owned task and refuse missing upstream: %+v (exit %d)", doc, st.ExitCode)
			}
		case sc.name == "finish-noarg-ambiguous-active" && st.Step == "finish-noarg":
			seen = true
			var doc struct{ Status, Task, Reason string }
			must(t, json.Unmarshal([]byte(st.Stdout), &doc))
			if st.ExitCode != 1 || doc.Status != "BLOCKED" || doc.Task != "" || !strings.Contains(doc.Reason, "ambiguous") {
				t.Fatalf("bare finish must refuse multiple owned tasks: %+v (exit %d)", doc, st.ExitCode)
			}
		case sc.name == "reclaimed-run-reconciliation" && st.Step == "list-after-reclaim":
			seen = true
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
