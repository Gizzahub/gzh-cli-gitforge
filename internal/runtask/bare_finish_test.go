package runtask

import (
	"strings"
	"testing"
)

func TestBareFinishOwnership(t *testing.T) {
	owner := TaskOwner{Actor: "codex", Host: "mst", Kind: "agent"}
	foreign := TaskOwner{Actor: "river", Host: "mst", Kind: "agent"}
	experiment := DerivedTaskState{Execution: TaskExecution{Task: "probe", Owner: owner, Purpose: "experiment"}, Status: TaskExecutionStatusActive}
	state := func(task string, who TaskOwner, status string) DerivedTaskState {
		return DerivedTaskState{Execution: TaskExecution{Task: task, Owner: who}, Status: status}
	}
	tests := []struct {
		name   string
		states []DerivedTaskState
		task   string
		reason string
	}{
		{"owned with foreign", []DerivedTaskState{state("foreign", foreign, TaskExecutionStatusActive), state("mine", owner, TaskExecutionStatusActive)}, "mine", ""},
		{"foreign only", []DerivedTaskState{state("foreign", foreign, TaskExecutionStatusActive)}, "", "no active execution owned by codex/mst"},
		{"same owner ambiguous", []DerivedTaskState{state("z", owner, TaskExecutionStatusActive), state("a", owner, TaskExecutionStatusActive)}, "", "ambiguous for codex/mst: a, z"},
		{"terminal only", []DerivedTaskState{state("finished", owner, TaskExecutionStatusDone)}, "", "no active execution owned by codex/mst"},
		{"experiment only", []DerivedTaskState{experiment}, "", "no active execution owned by codex/mst"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task, refusal := selectOwnedActiveTask(tc.states, owner)
			if task != tc.task {
				t.Fatalf("selected %q, want %q", task, tc.task)
			}
			if tc.reason == "" {
				if refusal != nil {
					t.Fatalf("unexpected refusal: %+v", refusal)
				}
				return
			}
			if refusal == nil || refusal.Status != TaskExecutionStatusBlocked || !strings.Contains(refusal.Reason, tc.reason) {
				t.Fatalf("refusal %+v, want blocked reason %q", refusal, tc.reason)
			}
		})
	}
}
