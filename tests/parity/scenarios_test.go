package parity

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The scenario catalog mirrors the contract doc's verb semantics section.
// Every step is executed for real inside a throwaway sandbox; the goldens
// under testdata/golden capture CE's exact answers and the resulting git
// + record state. Steps marked snapshot also pin the worktree, ref, log,
// execution, and receipt surface at that point.
//
// parityScenarios (not a package var) is the one catalog: TestCEParity
// executes it for the golden diff, TestParityReferencePinsAreCoherent
// checks it against the committed golden set 1:1, and
// TestParityFinishNoArgHasPushedUpstream re-executes one scenario's
// lifecycle with semantic assertions.
func TestCEParity(t *testing.T) {
	scenarios := parityScenarios()
	for _, sc := range scenarios {
		sc := sc
		t.Run(sc.name, func(t *testing.T) {
			runScenario(t, sc)
		})
	}
}

// parityScenarios is the authoritative 11-scenario CE reference catalog.
func parityScenarios() []scenario {
	return []scenario{
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
			// ISSUE-069, re-evaluated against CE 612f6bf8: the scenario
			// first drives the real lifecycle — start, edit the task
			// worktree, commit, push the task branch upstream, and assert
			// via run-status that the single active run is finish-ready.
			// No-arg run-finish then performs the finish itself in CE:
			// DONE, exit 0, source pushed, worktree and local/remote
			// branches reclaimed. (Pre-612f6bf8 CE answered the READY list
			// document with exit 1 — the original ISSUE-069 bug.) The
			// golden records CE's actual behavior; the native target port
			// instead answers its own READY list document and keeps the
			// run active with its refs — an intentional declared
			// divergence, never coerced toward CE's completion.
			name:           "finish-noarg-single-active",
			knownDivergent: true,
			contract: &portContract{
				Issue: "ISSUE-069",
				Note: "The single active run is made genuinely finishable first (edit + commit + upstream push + run-status finishReady), " +
					"then no-arg run-finish runs. CE 612f6bf8 completes the finish: DONE, exit 0, sourcePushed/worktreeRemoved/" +
					"localBranchRemoved/remoteBranchRemoved all true, refs reclaimed. Intentional divergence: the native target " +
					"port answers the READY list document instead — exit 0, task empty, no receipt — and keeps the run active " +
					"with refs/heads/dev/claude/mbp/chore/noarg retained in both the repo and origin; the port is never coerced " +
					"toward CE's completion.",
				Status:   "DONE",
				ExitCode: 0,
			},
			steps: []step{
				{label: "start", args: []string{"task", "run-start", "noarg", "--type", "chore", "--json"}, dir: "repo"},
				{label: "add-wip", cmd: "touch", args: []string{"noarg.txt"}, dir: wtDir("chore", "noarg")},
				{label: "stage-wip", cmd: "git", args: []string{"add", "-A"}, dir: wtDir("chore", "noarg")},
				{label: "commit-wip", cmd: "git", args: []string{"commit", "-qm", "wip"}, dir: wtDir("chore", "noarg")},
				{label: "push-branch", cmd: "git", args: []string{"push", "-qu", "origin", "dev/claude/mbp/chore/noarg"}, dir: "repo"},
				{label: "status-ready", args: []string{"task", "run-status", "noarg", "--json"}, dir: "repo"},
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
			// CE 612f6bf8 reference for an externally reclaimed run (worktree
			// and branch removed behind the lifecycle's back): both read
			// verbs answer BLOCKED with exit 1 ("Worktrunk inventory does
			// not match execution metadata") and the executions[] history
			// entry is retained — CE no longer auto-appends a
			// reconciliation abort receipt for this shape. The golden pins
			// that CE answer as reference only; the gz-git port's native
			// reconciliation semantics stay authoritative and are asserted
			// semantically in target mode (assertPortReclaimedNative),
			// never coerced toward CE's BLOCKED/exit 1. No port-source
			// change is implied by these fixtures.
			name:           "reclaimed-run-reconciliation",
			knownDivergent: true,
			contract: &portContract{
				Issue: "reclaimed-run-reconciliation",
				Note: "CE 612f6bf8 answers BLOCKED/exit 1 for an externally reclaimed run and retains the executions[] entry. " +
					"The port's own native reconciliation answer stays authoritative and is asserted semantically in target mode " +
					"whenever the scenario prerequisites hold (history entry retained, reclaimed run no longer active, terminal or " +
					"port-own blocked state with matching exit code). If the port refuses setup outright (e.g. its Worktrunk gate " +
					"under an ambient line it does not accept), that unavailability is surfaced distinctly and is never accepted " +
					"as the reconciliation answer. The golden is CE reference only — neither side is coerced.",
				Status:   "BLOCKED",
				ExitCode: 1,
			},
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
}

// wtDir names the CE-created task worktree path for branch
// dev/claude/mbp/<type>/<task> under the sandbox worktree root.
func wtDir(taskType, task string) string {
	return "wt/" + sandboxRepoID + "/" + actorDefault + "__" + host + "__" + taskType + "__" + task
}

// scenarioByName fetches one catalog entry by name.
func scenarioByName(t *testing.T, name string) scenario {
	t.Helper()
	for _, sc := range parityScenarios() {
		if sc.name == name {
			return sc
		}
	}
	t.Fatalf("scenario %q missing from the parity catalog", name)
	return scenario{}
}

// TestParityFinishNoArgHasPushedUpstream exercises the redesigned
// finish-noarg-single-active lifecycle end to end and asserts the
// semantics the golden can only show as bytes: the edit really lands in
// the task worktree, the task branch really reaches origin (upstream
// push, not a local-only ref), run-status proves the single active run is
// finish-ready, and the no-arg finish answer is then judged per mode — CE
// 612f6bf8 performs the finish itself (DONE, exit 0, source pushed,
// worktree and local/remote branches reclaimed; ISSUE-069 re-evaluated),
// while the native target port answers READY, keeps the run active, and
// retains the task refs (intentional divergence). Never skips: in CE
// mode TestMain has already required ce; in target mode the port binary
// is built from this checkout.
func TestParityFinishNoArgHasPushedUpstream(t *testing.T) {
	sc := scenarioByName(t, "finish-noarg-single-active")
	sb := newSandbox(t, sc)

	var statusStdout string
	taskRef := "refs/heads/dev/claude/mbp/chore/noarg"
	for _, st := range sc.steps {
		stdout, stderr, code := sb.run(st)
		switch st.label {
		case "push-branch":
			if code != 0 {
				t.Fatalf("upstream push step failed (%d): %s%s", code, stdout, stderr)
			}
		case "status-ready":
			if code != 0 {
				t.Fatalf("run-status after upstream push exited %d: %s%s", code, stdout, stderr)
			}
			statusStdout = stdout

			// Fixture assertion 1 (before the finish reclaims anything):
			// the edit step landed inside the task worktree the lifecycle
			// itself created, not somewhere else.
			edited := filepath.Join(sb.root, wtDir("chore", "noarg"), "noarg.txt")
			if _, err := os.Stat(edited); err != nil {
				t.Errorf("task worktree edit is missing at %s: %v", edited, err)
			}
			// Fixture assertion 2: the task branch really exists on origin
			// — the upstream push is real, not a local ref the tool chose
			// to believe.
			originRefs, _, _ := sb.run(step{cmd: "git", args: []string{"for-each-ref", "--format=%(refname)", "refs/heads/"}, dir: "origin.git"})
			if !strings.Contains(originRefs, taskRef) {
				t.Errorf("origin does not carry the pushed task branch %s; origin refs:\n%s", taskRef, originRefs)
			}
		case "finish-noarg":
			assertFinishNoArgAnswer(t, code, stdout, stderr)
		}
	}

	// Fixture assertion 3: readiness was proven before the no-arg finish —
	// the run is ACTIVE and finish-ready for this task. (activeCount is a
	// list-level field; CE's single-task answer legitimately carries 0.)
	var ready struct {
		Status      string `json:"status"`
		Task        string `json:"task"`
		FinishReady bool   `json:"finishReady"`
	}
	if err := json.Unmarshal([]byte(statusStdout), &ready); err != nil {
		t.Fatalf("run-status answer is not JSON: %v\n%s", err, statusStdout)
	}
	if ready.Task != "noarg" || ready.Status != "ACTIVE" || !ready.FinishReady {
		t.Errorf("run-status after upstream push = task %q status %q finishReady %v (want noarg/ACTIVE/true)\n%s", ready.Task, ready.Status, ready.FinishReady, statusStdout)
	}

	// Fixture assertion 4, mode-split. CE: completion reclaimed everything —
	// after the performed finish neither the local repo nor the bare origin
	// still carries the task branch. Target: the native READY answer keeps
	// the run active, so the exact task ref must still be present in both.
	// Each for-each-ref probe must first prove itself healthy (exit 0,
	// silent stderr) before its output counts as evidence: a failed probe's
	// empty stdout is probe failure, never branch evidence.
	for _, dir := range []string{"repo", "origin.git"} {
		stdout, stderr, code := sb.run(step{cmd: "git", args: []string{"for-each-ref", "--format=%(refname)", "refs/heads/"}, dir: dir})
		pr := probeResult{dir: dir, exitCode: code, stdout: stdout, stderr: stderr}
		if targetMode {
			if err := checkBranchPresence(pr, taskRef); err != nil {
				t.Error(err)
			}
			continue
		}
		if err := checkBranchAbsence(pr, taskRef); err != nil {
			t.Error(err)
		}
	}
}

// probeResult captures one `git for-each-ref` branch probe's full health:
// which repo was probed, its exit code, stdout, and stderr. It exists so
// reclaim assertions treat a probe as evidence only when the probe itself
// succeeded.
type probeResult struct {
	dir      string
	exitCode int
	stdout   string
	stderr   string
}

// checkBranchAbsence decides whether a for-each-ref probe may count as
// evidence that the task branch is gone. Evidence rules:
//
//   - a non-zero exit proves nothing; empty stdout then means the probe
//     failed, not that the branch is absent, so the probe is rejected;
//   - stderr emitted despite exit 0 likewise disqualifies the output;
//   - only a clean probe's stdout is scanned, for the exact task ref.
//
// It returns nil only when the probe is healthy and the exact ref is absent.
func checkBranchAbsence(pr probeResult, taskRef string) error {
	if pr.exitCode != 0 {
		return fmt.Errorf("%s for-each-ref probe exited %d — empty stdout is probe failure, not branch absence (stdout:\n%s\nstderr:\n%s)", pr.dir, pr.exitCode, pr.stdout, pr.stderr)
	}
	if pr.stderr != "" {
		return fmt.Errorf("%s for-each-ref probe emitted stderr despite exit 0: %s", pr.dir, pr.stderr)
	}
	for _, line := range strings.Split(pr.stdout, "\n") {
		if strings.TrimSpace(line) == taskRef {
			return fmt.Errorf("%s still carries the task branch %s after the performed finish:\n%s", pr.dir, taskRef, pr.stdout)
		}
	}
	return nil
}

// checkBranchPresence is checkBranchAbsence's target-mode counterpart: it
// accepts a probe as evidence only under the same health rules (exit 0,
// silent stderr) and then requires the exact task ref to still be listed —
// the native READY answer keeps the run active, so absence is the error.
func checkBranchPresence(pr probeResult, taskRef string) error {
	if pr.exitCode != 0 {
		return fmt.Errorf("%s for-each-ref probe exited %d — output is probe failure, not ref evidence (stdout:\n%s\nstderr:\n%s)", pr.dir, pr.exitCode, pr.stdout, pr.stderr)
	}
	if pr.stderr != "" {
		return fmt.Errorf("%s for-each-ref probe emitted stderr despite exit 0: %s", pr.dir, pr.stderr)
	}
	for _, line := range strings.Split(pr.stdout, "\n") {
		if strings.TrimSpace(line) == taskRef {
			return nil
		}
	}
	return fmt.Errorf("%s no longer carries the active task branch %s after the native READY answer:\n%s", pr.dir, taskRef, pr.stdout)
}

// assertFinishNoArgAnswer pins the ISSUE-069 (re-evaluated) answer, split
// by mode. CE: the no-arg finish with exactly one finish-ready active run
// completes the run — DONE, exit 0, receipt proving source push and full
// reclaim. Target: the native port answers its READY list document —
// exit 0, task "", no receipt — and keeps the run active; that divergence
// is declared intentional by the scenario contract.
func assertFinishNoArgAnswer(t *testing.T, code int, stdout, stderr string) {
	t.Helper()
	var doc struct {
		Status  string `json:"status"`
		Task    string `json:"task"`
		Receipt *struct {
			Operation           string `json:"operation"`
			Status              string `json:"status"`
			SourcePushed        bool   `json:"sourcePushed"`
			WorktreeRemoved     bool   `json:"worktreeRemoved"`
			LocalBranchRemoved  bool   `json:"localBranchRemoved"`
			RemoteBranchRemoved bool   `json:"remoteBranchRemoved"`
		} `json:"receipt"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("no-arg finish answer is not JSON: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	if targetMode {
		if doc.Status != "READY" || doc.Task != "" || doc.Receipt != nil {
			t.Errorf("target no-arg finish answer = status %q task %q receipt %+v (want the native READY list answer: task empty, no receipt)\nstdout: %s", doc.Status, doc.Task, doc.Receipt, stdout)
			return
		}
		if code != 0 {
			t.Errorf("target no-arg finish exited %d (want 0 — the native READY list answer is a healthy answer)\nstdout: %s", code, stdout)
		}
		return
	}
	if doc.Status != "DONE" || doc.Task != "noarg" || doc.Receipt == nil ||
		doc.Receipt.Operation != "finish" || doc.Receipt.Status != "DONE" ||
		!doc.Receipt.SourcePushed || !doc.Receipt.WorktreeRemoved ||
		!doc.Receipt.LocalBranchRemoved || !doc.Receipt.RemoteBranchRemoved {
		t.Errorf("no-arg finish answer = status %q task %q receipt %+v (want DONE with the performed finish: source pushed, worktree/local/remote reclaimed)\nstdout: %s", doc.Status, doc.Task, doc.Receipt, stdout)
		return
	}
	if code != 0 {
		t.Errorf("no-arg finish exited %d (want 0 — the performed finish succeeds); re-evaluate the known-divergent contract\nstdout: %s", code, stdout)
	}
}

// TestParityFinishNoArgRejectsFailedBranchProbe pins the evidence rule the
// no-arg finish reclaim assertions rest on: a failed for-each-ref probe
// with empty stdout must never be accepted as branch absence, while a
// healthy probe with genuinely empty stdout is accepted. Pure in-memory
// captured probe results — no engine, no git, no worktrees.
func TestParityFinishNoArgRejectsFailedBranchProbe(t *testing.T) {
	const taskRef = "refs/heads/dev/claude/mbp/chore/noarg"

	cases := []struct {
		name    string
		probe   probeResult
		wantErr bool
	}{
		{
			name:    "failed probe with empty stdout is not branch absence",
			probe:   probeResult{dir: "repo", exitCode: 128, stdout: "", stderr: "fatal: not a git repository"},
			wantErr: true,
		},
		{
			name:    "failed probe with empty stdout and empty stderr is still not branch absence",
			probe:   probeResult{dir: "origin.git", exitCode: 1, stdout: "", stderr: ""},
			wantErr: true,
		},
		{
			name:    "exit-0 probe that emitted stderr is disqualified",
			probe:   probeResult{dir: "origin.git", exitCode: 0, stdout: "", stderr: "warning: unable to read ref"},
			wantErr: true,
		},
		{
			name:    "successful empty probe is accepted as branch absence",
			probe:   probeResult{dir: "repo", exitCode: 0, stdout: "", stderr: ""},
			wantErr: false,
		},
		{
			name:    "successful probe still detects the exact task ref",
			probe:   probeResult{dir: "repo", exitCode: 0, stdout: "refs/heads/dev/main\n" + taskRef + "\n", stderr: ""},
			wantErr: true,
		},
		{
			name:    "successful probe listing only a longer similarly-named ref is accepted",
			probe:   probeResult{dir: "origin.git", exitCode: 0, stdout: taskRef + "-followup\n", stderr: ""},
			wantErr: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := checkBranchAbsence(tc.probe, taskRef)
			if tc.wantErr && err == nil {
				t.Fatalf("probe (dir %s, exit %d, stdout %q, stderr %q) was wrongly accepted as branch absence", tc.probe.dir, tc.probe.exitCode, tc.probe.stdout, tc.probe.stderr)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("healthy probe wrongly rejected: %v", err)
			}
		})
	}
}
