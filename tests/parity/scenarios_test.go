package parity

import "testing"

// The scenario catalog mirrors the contract doc's verb semantics section.
// Every step is executed for real inside a throwaway sandbox; the goldens
// under testdata/golden capture CE's exact answers and the resulting git
// + record state. Steps marked snapshot also pin the worktree, ref, log,
// execution, and receipt surface at that point.
func TestCEParity(t *testing.T) {
	scenarios := []scenario{
		{
			name: "doctor-active",
			steps: []step{
				{label: "doctor", args: []string{"task", "run-doctor", "--json"}, dir: "repo", snapshot: true},
			},
		},
		{
			name: "start-created-existing",
			steps: []step{
				{label: "start-created", args: []string{"task", "run-start", "smoke-task", "--type", "test", "--json"}, dir: "repo", snapshot: true},
				{label: "start-existing", args: []string{"task", "run-start", "smoke-task", "--type", "test", "--json"}, dir: "repo"},
			},
		},
		{
			name: "owner-mismatch-takeover",
			steps: []step{
				{label: "start-owner", args: []string{"task", "run-start", "takeover", "--type", "test", "--json"}, dir: "repo"},
				{label: "start-mismatch", args: []string{"task", "run-start", "takeover", "--type", "test", "--json"}, dir: "repo", actor: "river"},
				{label: "discard-mismatch", args: []string{"task", "run-discard", "takeover", "--reason", "not mine", "--json"}, dir: "repo", actor: "river"},
				{label: "discard-wrong-takeover", args: []string{"task", "run-discard", "takeover", "--reason", "not mine", "--take-over-from", "river/mbp", "--json"}, dir: "repo", actor: "river"},
				{label: "discard-takeover-ok", args: []string{"task", "run-discard", "takeover", "--reason", "handed over", "--take-over-from", "claude/mbp", "--json"}, dir: "repo", actor: "river", snapshot: true},
			},
		},
		{
			name: "discard-refusals",
			steps: []step{
				{label: "start", args: []string{"task", "run-start", "discard-guards", "--type", "test", "--json"}, dir: "repo"},
				{label: "dirty-worktree", cmd: "touch", args: []string{"notes.md"}, dir: wtDir("test", "discard-guards")},
				{label: "discard-dirty", args: []string{"task", "run-discard", "discard-guards", "--reason", "dirty", "--json"}, dir: "repo"},
				{label: "clean-worktree", cmd: "git", args: []string{"clean", "-fdq", "."}, dir: wtDir("test", "discard-guards")},
				{label: "push-branch", cmd: "git", args: []string{"push", "-q", "origin", "dev/claude/mbp/test/discard-guards"}, dir: "repo"},
				{label: "discard-remote-branch", args: []string{"task", "run-discard", "discard-guards", "--reason", "remote exists", "--json"}, dir: "repo"},
				{label: "delete-remote-branch", cmd: "git", args: []string{"push", "-q", "origin", "--delete", "dev/claude/mbp/test/discard-guards"}, dir: "repo"},
				{label: "discard-ok", args: []string{"task", "run-discard", "discard-guards", "--reason", "clean now", "--json"}, dir: "repo", snapshot: true},
			},
		},
		{
			name:            "discard-no-fetch",
			taskRuntimeYAML: "schema-version: 2\nrepository-id: " + sandboxRepoID + "\nworktree-roots:\n  " + host + ": WORKTREE_ROOT\nintegration-provider: gz-git\nintegration-network-policy: no-fetch\n",
			steps: []step{
				{label: "doctor", args: []string{"task", "run-doctor", "--json"}, dir: "repo"},
				{label: "start", args: []string{"task", "run-start", "nf-guard", "--type", "test", "--json"}, dir: "repo"},
				{label: "discard-refused", args: []string{"task", "run-discard", "nf-guard", "--reason", "no fetch", "--json"}, dir: "repo", snapshot: true},
			},
		},
		{
			name: "finish-explicit",
			steps: []step{
				{label: "start", args: []string{"task", "run-start", "finish-me", "--type", "feat", "--json"}, dir: "repo"},
				{label: "add-feature", cmd: "touch", args: []string{"feature.txt"}, dir: wtDir("feat", "finish-me")},
				{label: "stage-feature", cmd: "git", args: []string{"add", "-A"}, dir: wtDir("feat", "finish-me")},
				{label: "commit-feature", cmd: "git", args: []string{"commit", "-qm", "feature"}, dir: wtDir("feat", "finish-me")},
				{label: "push-branch", cmd: "git", args: []string{"push", "-qu", "origin", "dev/claude/mbp/feat/finish-me"}, dir: "repo"},
				{label: "status-ready", args: []string{"task", "run-status", "finish-me", "--json"}, dir: "repo"},
				{label: "finish", args: []string{"task", "run-finish", "finish-me", "--json"}, dir: "repo", snapshot: true},
			},
		},
		{
			name: "finish-refusal-unpushed",
			steps: []step{
				{label: "start", args: []string{"task", "run-start", "unpushed", "--type", "fix", "--json"}, dir: "repo"},
				{label: "add-wip", cmd: "touch", args: []string{"bug.txt"}, dir: wtDir("fix", "unpushed")},
				{label: "stage-wip", cmd: "git", args: []string{"add", "-A"}, dir: wtDir("fix", "unpushed")},
				{label: "commit-wip", cmd: "git", args: []string{"commit", "-qm", "wip"}, dir: wtDir("fix", "unpushed")},
				{label: "finish-refused", args: []string{"task", "run-finish", "unpushed", "--json"}, dir: "repo", snapshot: true},
			},
		},
		{
			// ISSUE-069: with exactly one active run, no-arg run-finish must
			// answer READY with exit 0. CE pins READY but exits 1. The golden
			// records CE's actual behavior; the port contract requires the fix.
			name:           "finish-noarg-single-active",
			knownDivergent: true,
			contract: &portContract{
				Issue:    "ISSUE-069",
				Note:     "CE reuses the list reader for no-arg finish and hits the nil-execution guard. The port must answer READY with exit 0 when exactly one run is active.",
				Status:   "READY",
				ExitCode: 1,
			},
			steps: []step{
				{label: "start", args: []string{"task", "run-start", "noarg", "--type", "chore", "--json"}, dir: "repo"},
				{label: "finish-noarg", args: []string{"task", "run-finish", "--json"}, dir: "repo", snapshot: true},
			},
		},
		{
			name: "abort-terminal-noop",
			steps: []step{
				{label: "start", args: []string{"task", "run-start", "aborted-run", "--type", "fix", "--json"}, dir: "repo"},
				{label: "status-active", args: []string{"task", "run-status", "aborted-run", "--json"}, dir: "repo"},
				{label: "abort", args: []string{"task", "run-abort", "aborted-run", "--reason", "changed mind", "--json"}, dir: "repo", snapshot: true},
				{label: "abort-noop", args: []string{"task", "run-abort", "aborted-run", "--reason", "again", "--json"}, dir: "repo"},
				{label: "status-aborted", args: []string{"task", "run-status", "aborted-run", "--json"}, dir: "repo"},
				{label: "list", args: []string{"task", "run-list", "--json"}, dir: "repo"},
			},
		},
		{
			// A run judged from states[].status, not from len(executions):
			// after an external reclaim CE reconciles the record to ABORTED
			// on the next read and activeCount drops to 0 while executions
			// still carries the history entry.
			name: "reclaimed-run-reconciliation",
			steps: []step{
				{label: "start", args: []string{"task", "run-start", "reclaimed-task", "--type", "chore", "--json"}, dir: "repo"},
				{label: "external-worktree-removal", cmd: "git", args: []string{"worktree", "remove", "--force", "../wt/" + sandboxRepoID + "/claude__mbp__chore__reclaimed-task"}, dir: "repo"},
				{label: "external-branch-removal", cmd: "git", args: []string{"branch", "-D", "dev/claude/mbp/chore/reclaimed-task"}, dir: "repo"},
				{label: "list-after-reclaim", args: []string{"task", "run-list", "--json"}, dir: "repo", snapshot: true},
				{label: "status-after-reclaim", args: []string{"task", "run-status", "reclaimed-task", "--json"}, dir: "repo"},
			},
		},
		{
			name: "recover-requires-cleanup-failure",
			steps: []step{
				{label: "start", args: []string{"task", "run-start", "cleanup-fail", "--type", "chore", "--json"}, dir: "repo"},
				{label: "abort", args: []string{"task", "run-abort", "cleanup-fail", "--reason", "simulate", "--json"}, dir: "repo"},
				{label: "recover-refused", args: []string{"task", "run-recover", "cleanup-fail", "--json"}, dir: "repo", snapshot: true},
			},
		},
	}
	for _, sc := range scenarios {
		sc := sc
		t.Run(sc.name, func(t *testing.T) {
			runScenario(t, sc)
		})
	}
}

// wtDir names the CE-created task worktree path for branch
// dev/claude/mbp/<type>/<task> under the sandbox worktree root.
func wtDir(taskType, task string) string {
	return "wt/" + sandboxRepoID + "/" + actorDefault + "__" + host + "__" + taskType + "__" + task
}
