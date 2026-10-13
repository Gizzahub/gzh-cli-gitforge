// Worktrunk 0.80 contract tests. The runtime contracts with the installed,
// mise-authoritative Worktrunk 0.80.0: doctor gates on its version report,
// and every inventory read decodes its schema 2 `wt list --format=json`
// envelope. The focused tests here use the command-runner seam to prove which
// wt answers the runtime refuses (wrong version, nonzero command, malformed
// envelope); the lifecycle fixture runs the real installed binary end to end.

package runtask

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Fixture identity shared by the contract tests. The host names the
// worktree-roots key, the actor the branch owner segment; both come from the
// environment the way production resolves them, never from Git config, so the
// tests never depend on this machine's global identity.
const (
	worktrunkContractRepoID = "fixture-repo"
	worktrunkContractHost   = "wt080box"
	worktrunkContractActor  = "glm5p3"
	worktrunkContractTask   = "task-080-fixture"
)

// worktrunk080ListEnvelope is a schema 2 `wt list --format=json` envelope as
// the installed Worktrunk 0.80.0 emits it (machine-captured 2026-10-13 and
// trimmed only of display strings). Keys the decoder ignores are kept so the
// shape stays honest: one main checkout, one task worktree the runtime just
// created, one detached item with no branch, and one item whose branch names
// no worktree.path.
const worktrunk080ListEnvelope = `{
  "schema": 2,
  "repo": {"default_branch": "main"},
  "collected": {"ci": false, "summary": false},
  "items": [
    {
      "branch": "main",
      "head": {"sha": "e5c1ac40035515a3063271df8e17a2fd675078d2", "short_sha": "e5c1ac4", "subject": "fixture base", "committed_at": "2026-10-13T10:31:34Z"},
      "worktree": {"path": "/private/tmp/gz-wt080-fixture/src/fixture-repo", "main": true, "current": true, "previous": true, "detached": false, "branch_mismatch": false, "duplicate_branch": false, "changes": {"staged": false, "modified": false, "untracked": false, "renamed": false, "deleted": false, "conflicted": false, "diff": {"added": 0, "deleted": 0}}},
      "display": {"state": "is_main", "symbols": "^"}
    },
    {
      "branch": "dev/glm5p3/wt080box/chore/task-080-fixture",
      "head": {"sha": "e5c1ac40035515a3063271df8e17a2fd675078d2", "short_sha": "e5c1ac4", "subject": "fixture base", "committed_at": "2026-10-13T10:31:34Z"},
      "worktree": {"path": "/private/tmp/gz-wt080-fixture/wt/fixture-repo/glm5p3__wt080box__chore__task-080-fixture", "main": false, "current": false, "previous": false, "detached": false, "branch_mismatch": true, "duplicate_branch": false, "changes": {"staged": false, "modified": false, "untracked": false, "renamed": false, "deleted": false, "conflicted": false, "diff": {"added": 0, "deleted": 0}}},
      "default_branch": {"ahead": 0, "behind": 0, "diff": {"added": 0, "deleted": 0}, "orphan": false, "integration": {"reason": "same_commit"}, "merge_conflicts": false},
      "display": {"state": "empty", "symbols": "_"}
    },
    {
      "head": {"sha": "0000000000000000000000000000000000000000", "short_sha": "0000000", "subject": "detached probe", "committed_at": "2026-10-13T10:31:34Z"},
      "worktree": {"path": "/private/tmp/gz-wt080-fixture/src/detached", "main": false, "current": false, "previous": false, "detached": true, "branch_mismatch": false, "duplicate_branch": false, "changes": {"staged": false, "modified": false, "untracked": false, "renamed": false, "deleted": false, "conflicted": false, "diff": {"added": 0, "deleted": 0}}},
      "display": {"state": "detached"}
    },
    {
      "branch": "dev/glm5p3/wt080box/chore/orphaned-pathless",
      "head": {"sha": "0000000000000000000000000000000000000000", "short_sha": "0000000", "subject": "pathless probe", "committed_at": "2026-10-13T10:31:34Z"},
      "worktree": {"main": false, "current": false, "previous": false, "detached": false, "branch_mismatch": false, "duplicate_branch": false, "changes": {"staged": false, "modified": false, "untracked": false, "renamed": false, "deleted": false, "conflicted": false, "diff": {"added": 0, "deleted": 0}}},
      "display": {"state": "empty"}
    }
  ]
}`

// worktrunkStubAnswer is one recorded wt reply: separated output streams, the
// exit code the runtime classifies, and an optional start failure for the
// command-never-ran path (executor exit 127).
type worktrunkStubAnswer struct {
	stdout   string
	stderr   string
	exitCode int
	err      error
}

// worktrunkStubRunner is the command-runner seam for the focused Worktrunk
// contract tests. Everything that is not `wt` -- git above all -- executes for
// real through the production executor, so the store, lock, and receipt paths
// under test are the real ones; every `wt` invocation is answered from
// recorded stub output, so a test can prove exactly which wt answers the
// runtime refuses without faking any Git state.
type worktrunkStubRunner struct {
	executor *Executor
	// version answers `wt --version`.
	version worktrunkStubAnswer
	// list answers `wt --config-set list.json-schema = 2 list --format=json`.
	list worktrunkStubAnswer
}

// newWorktrunkStubRunner answers as the contracted install does: a 0.80.0
// version report and a healthy schema 2 inventory. A test overrides the half
// it is proving rejections for.
func newWorktrunkStubRunner() *worktrunkStubRunner {
	return &worktrunkStubRunner{
		executor: NewExecutor(),
		version:  worktrunkStubAnswer{stdout: "wt " + worktrunkVersion + "\n"},
		list:     worktrunkStubAnswer{stdout: worktrunk080ListEnvelope},
	}
}

func (r *worktrunkStubRunner) Run(ctx context.Context, req CommandRequest) (CommandResult, error) {
	if req.Command != "wt" {
		return r.executor.Run(ctx, req)
	}
	answer := r.list
	if len(req.Args) > 0 && req.Args[0] == "--version" {
		answer = r.version
	}
	result := CommandResult{
		Command: req.Command, Executable: "/worktrunk-stub/wt",
		Args: append([]string(nil), req.Args...), WorkDir: req.WorkDir,
		Stdout: answer.stdout, Stderr: answer.stderr, ExitCode: answer.exitCode,
	}
	if answer.err != nil {
		result.ExitCode, result.Error, result.NotStarted = 127, answer.err.Error(), true
		return result, answer.err
	}
	return result, nil
}

// wtContractRepo is a real git repository laid out the way the task runtime
// requires: a control checkout beside a declared worktree root, a local bare
// origin so remote probes have a real answer, and the declaration files the
// runtime reads. HOME is deliberately left alone so a mise-shimmed wt keeps
// resolving; only git's global config is pinned to a private file.
type wtContractRepo struct {
	root         string
	origin       string
	worktreeRoot string // canonicalized, the form expectedWorktreePath expects
	branch       string
}

func newWorktrunkContractRepo(t *testing.T, runner CommandRunner) (*Service, wtContractRepo) {
	t.Helper()
	base := t.TempDir()
	global := filepath.Join(base, "gitconfig")
	if err := os.WriteFile(global, []byte("[init]\n\tdefaultBranch = main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GZ_GIT_TASK_ACTOR", worktrunkContractActor)
	t.Setenv("GIT_WORK_HOST", worktrunkContractHost)
	git := func(args ...string) {
		t.Helper()
		out, err := exec.CommandContext(t.Context(), "git", args...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	fx := wtContractRepo{
		root:   filepath.Join(base, "src", worktrunkContractRepoID),
		origin: filepath.Join(base, "origin.git"),
	}
	git("init", "-q", "-b", "main", "--bare", fx.origin)
	git("init", "-q", "-b", "main", fx.root)
	git("-C", fx.root, "config", "user.email", worktrunkContractActor+"@example.invalid")
	git("-C", fx.root, "config", "user.name", "Worktrunk contract fixture")
	git("-C", fx.root, "remote", "add", "origin", fx.origin)
	if err := os.WriteFile(filepath.Join(fx.root, "README.md"), []byte("worktrunk 0.80 contract fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("-C", fx.root, "add", "README.md")
	git("-C", fx.root, "commit", "-qm", "fixture base")
	git("-C", fx.root, "push", "-q", "origin", "main")
	declaration := "branch:\n  integrationBranch:\n    - main\n"
	if err := os.WriteFile(filepath.Join(fx.root, ".gz-git.yaml"), []byte(declaration), 0o644); err != nil {
		t.Fatal(err)
	}
	worktreeRoot := filepath.Join(base, "wt", worktrunkContractRepoID)
	runtime := fmt.Sprintf("schema-version: 2\nrepository-id: %s\nworktree-roots:\n  %s: %s\nintegration-provider: gz-git\nintegration-network-policy: allow\n",
		worktrunkContractRepoID, worktrunkContractHost, worktreeRoot)
	if err := os.WriteFile(filepath.Join(fx.root, TaskRuntimeConfigFile), []byte(runtime), 0o644); err != nil {
		t.Fatal(err)
	}
	canonicalRoot, err := canonicalizePotentialPath(worktreeRoot)
	if err != nil {
		t.Fatalf("canonicalize worktree root: %v", err)
	}
	fx.worktreeRoot = canonicalRoot
	fx.branch = fmt.Sprintf("dev/%s/%s/chore/%s", worktrunkContractActor, worktrunkContractHost, worktrunkContractTask)
	return NewService(fx.root, runner, NewEngine()), fx
}

// fixtureGit runs one git command in dir and fails the test on any error, so
// a fixture step that quietly degrades cannot be read as a passing contract.
func fixtureGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.CommandContext(t.Context(), "git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			t.Fatalf("git -C %s %s: %v\n%s", dir, strings.Join(args, " "), err, exitErr.Stderr)
		}
		t.Fatalf("git -C %s %s: %v", dir, strings.Join(args, " "), err)
	}
	return string(out)
}

// classifyBranchAbsence turns the error from `git show-ref --verify --quiet`
// into the verdict the absence assertion fails on: nil only for the exact
// *exec.ExitError with exit code 1 that is git's documented "no such ref"
// answer. A nil error (the ref verified), any other exit code, a start failure
// that never ran git, or a canceled/expired context is returned as a non-nil
// verdict: the probe then proved nothing and must not be read as absence.
func classifyBranchAbsence(err error) error {
	switch {
	case err == nil:
		return errors.New("show-ref verified the ref; the branch still exists")
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("branch-absence check lost its context: %w", err)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return fmt.Errorf("branch-absence check never ran git: %w", err)
	}
	if code := exitErr.ExitCode(); code != 1 {
		return fmt.Errorf("branch-absence check exited %d, want 1: %w", code, err)
	}
	return nil
}

// expectBranchAbsent asserts git refuses refs/heads/branch in dir with its
// documented not-found exit status 1 -- and nothing else, per
// classifyBranchAbsence.
func expectBranchAbsent(t *testing.T, dir, branch string) {
	t.Helper()
	err := exec.CommandContext(t.Context(), "git", "-C", dir, "show-ref", "--verify", "--quiet", "refs/heads/"+branch).Run()
	if verdict := classifyBranchAbsence(err); verdict != nil {
		t.Fatalf("local branch %s: %v", branch, verdict)
	}
}

// TestWorktrunk080VersionContract pins the version half of the Worktrunk
// contract explicitly: the literal the runtime gates on, the doctor gate
// accepting the installed 0.80.0 report (and refusing the retired 0.74.0),
// and the schema 2 decoder resolving the 0.80.0 envelope into the branch-path
// lookup and skip accounting every consumer depends on.
func TestWorktrunk080VersionContract(t *testing.T) {
	t.Run("doctor gate accepts the installed 0.80.0 report", func(t *testing.T) {
		service, _ := newWorktrunkContractRepo(t, newWorktrunkStubRunner())
		res := service.Doctor(t.Context())
		if res.Status == TaskExecutionStatusBlocked {
			t.Fatalf("doctor blocked under the Worktrunk %s contract: %s (%s)", worktrunkVersion, res.Reason, res.NextAction)
		}
		if res.Provider == nil || !res.Provider.Present {
			t.Fatalf("doctor answered without the integration provider: %+v", res.Provider)
		}
	})
	t.Run("doctor gate refuses the retired 0.74.0 report", func(t *testing.T) {
		runner := newWorktrunkStubRunner()
		runner.version.stdout = "wt 0.74.0\n"
		service, _ := newWorktrunkContractRepo(t, runner)
		res := service.Doctor(t.Context())
		if res.Status != TaskExecutionStatusBlocked {
			t.Fatalf("doctor status = %s, want BLOCKED for the retired 0.74.0 report", res.Status)
		}
		if want := "Worktrunk " + worktrunkVersion + " is required"; res.Reason != want {
			t.Fatalf("doctor reason = %q, want %q", res.Reason, want)
		}
	})
	t.Run("schema 2 decoder preserves branch-path lookup and skip accounting", func(t *testing.T) {
		service, _ := newWorktrunkContractRepo(t, newWorktrunkStubRunner())
		inventory, err := service.worktrunkList(t.Context())
		if err != nil {
			t.Fatalf("worktrunkList on the 0.80.0 envelope: %v", err)
		}
		if inventory.SchemaVersion != 2 {
			t.Fatalf("decoded schema version = %d, want 2", inventory.SchemaVersion)
		}
		if len(inventory.Paths) != 2 {
			t.Fatalf("decoded %d branch paths, want 2 (main and the task worktree): %+v", len(inventory.Paths), inventory.Paths)
		}
		if got, ok := inventory.Paths["main"]; !ok || got != "/private/tmp/gz-wt080-fixture/src/fixture-repo" {
			t.Fatalf("main path = %q (found %t), want the listed checkout path", got, ok)
		}
		want := expectedWorktreePath("/private/tmp/gz-wt080-fixture/wt/fixture-repo", "dev/glm5p3/wt080box/chore/task-080-fixture")
		if got, ok := inventory.Paths["dev/glm5p3/wt080box/chore/task-080-fixture"]; !ok || got != want {
			t.Fatalf("task branch path = %q (found %t), want %q", got, ok, want)
		}
		if inventory.SkippedNoBranch != 1 || inventory.SkippedNoPath != 1 {
			t.Fatalf("skip accounting = no-branch %d, no-path %d, want 1 and 1", inventory.SkippedNoBranch, inventory.SkippedNoPath)
		}
		if inventory.Diagnostic.Command != "wt" || inventory.Diagnostic.ExitCode != 0 {
			t.Fatalf("inventory evidence = %s exit %d, want the recorded wt list command exiting 0", inventory.Diagnostic.Command, inventory.Diagnostic.ExitCode)
		}
	})
}

// TestWorktrunkListRejectsMalformedOutput proves the inventory read stays
// fail-closed: output that is not the schema 2 items envelope is an error and
// never decodes into a partial branch-path map a lookup could trust.
func TestWorktrunkListRejectsMalformedOutput(t *testing.T) {
	cases := []struct {
		name   string
		stdout string
		want   string
	}{
		{"human prose on stdout", "wt: this repository has no worktrees\n", "decode Worktrunk schema 2"},
		{"schema 1 envelope", `{"schema": 1, "items": [{"branch": "main", "worktree": {"path": "/tmp/fixture"}}]}`, "worktrunk list must return a schema 2 items envelope"},
		{"no items array", `{"schema": 2, "repo": {"default_branch": "main"}}`, "worktrunk list must return a schema 2 items envelope"},
		{"items null", `{"schema": 2, "items": null}`, "worktrunk list must return a schema 2 items envelope"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner := newWorktrunkStubRunner()
			runner.list.stdout = tc.stdout
			service, _ := newWorktrunkContractRepo(t, runner)
			inventory, err := service.worktrunkList(t.Context())
			if err == nil {
				t.Fatalf("malformed envelope accepted: %d paths decoded", len(inventory.Paths))
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not name the refusal %q", err, tc.want)
			}
			if len(inventory.Paths) != 0 {
				t.Fatalf("fail-closed decoder returned %d branch paths with the error", len(inventory.Paths))
			}
		})
	}
}

// TestWorktrunkListRejectsCommandFailure proves a wt list that fails is never
// decoded: a nonzero exit is refused with its stderr named, and a command that
// never started is refused outright.
func TestWorktrunkListRejectsCommandFailure(t *testing.T) {
	t.Run("nonzero wt exit", func(t *testing.T) {
		runner := newWorktrunkStubRunner()
		runner.list = worktrunkStubAnswer{exitCode: 1, stderr: "wt: repository metadata is corrupt\n"}
		service, _ := newWorktrunkContractRepo(t, runner)
		inventory, err := service.worktrunkList(t.Context())
		if err == nil {
			t.Fatalf("failed wt list accepted: %d paths decoded", len(inventory.Paths))
		}
		for _, want := range []string{"worktrunk list failed", "repository metadata is corrupt"} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("error %q does not name %q", err, want)
			}
		}
		if inventory.Diagnostic.ExitCode != 1 {
			t.Fatalf("diagnostic exit code = %d, want the recorded 1", inventory.Diagnostic.ExitCode)
		}
	})
	t.Run("wt command never started", func(t *testing.T) {
		runner := newWorktrunkStubRunner()
		runner.list.err = errors.New("resolve command wt: exec: wt: not found")
		service, _ := newWorktrunkContractRepo(t, runner)
		inventory, err := service.worktrunkList(t.Context())
		if err == nil {
			t.Fatalf("unstartable wt list accepted: %+v", inventory)
		}
		if !strings.Contains(err.Error(), "wt") {
			t.Fatalf("error %q does not name the wt command", err)
		}
	})
}

// TestWorktrunk080LifecycleFixture runs the run lifecycle against the real
// installed Worktrunk 0.80.0 -- doctor's version gate and inventory read, wt
// switch --create under run-start, list and status deriving live state, and
// discard reclaiming the worktree through wt remove -- on a temporary real git
// repository. Under normal execution (-short=false) it fails, never skips,
// when wt is unavailable or reports any version other than the contracted one:
// a synthetic runner cannot answer whether the installed binary still honors
// this contract. Only canonical `go test -short` skips it, because short mode
// cannot assume the wt install; it must not silently weaken into a pass there.
// Everything the fixture creates lives under one t.TempDir, and wt keeps its
// own state under the repository's .git, so cleanup removes only what this
// test built.
func TestWorktrunk080LifecycleFixture(t *testing.T) {
	if testing.Short() {
		t.Skip("real Worktrunk 0.80 lifecycle fixture is disabled in short tests")
	}
	wtPath, err := exec.LookPath("wt")
	if err != nil {
		t.Fatalf("this fixture requires the installed Worktrunk executable on PATH: %v", err)
	}
	versionOut, versionErr := exec.CommandContext(t.Context(), wtPath, "--version").CombinedOutput()
	if versionErr != nil {
		t.Fatalf("wt --version at %s failed: %v\n%s", wtPath, versionErr, versionOut)
	}
	if reported := strings.TrimSpace(string(versionOut)); reported != "wt "+worktrunkVersion {
		t.Fatalf("installed wt at %s reports %q; this fixture exercises Worktrunk %s and refuses any other version", wtPath, reported, worktrunkVersion)
	}
	service, fx := newWorktrunkContractRepo(t, NewExecutor())
	ctx := t.Context()
	task := worktrunkContractTask
	worktree := expectedWorktreePath(fx.worktreeRoot, fx.branch)

	doctor := service.Doctor(ctx)
	if doctor.Status == TaskExecutionStatusBlocked {
		t.Fatalf("doctor blocked against installed Worktrunk %s: %s (%s)", worktrunkVersion, doctor.Reason, doctor.NextAction)
	}

	start := service.Start(ctx, task, TaskExecutionTypeChore)
	if start.Status != TaskExecutionStatusActive || start.Execution == nil {
		t.Fatalf("run-start status = %s, reason = %s; want ACTIVE against installed Worktrunk %s", start.Status, start.Reason, worktrunkVersion)
	}
	if start.Execution.Branch != fx.branch {
		t.Fatalf("started branch = %q, want %q", start.Execution.Branch, fx.branch)
	}
	if start.Execution.Worktree != worktree {
		t.Fatalf("started worktree = %q, want the configured %q", start.Execution.Worktree, worktree)
	}
	if _, statErr := os.Stat(worktree); statErr != nil {
		t.Fatalf("wt did not create the worktree it reported: %v", statErr)
	}
	if got := strings.TrimSpace(fixtureGit(t, worktree, "branch", "--show-current")); got != fx.branch {
		t.Fatalf("worktree branch = %q, want %q", got, fx.branch)
	}
	if dirty := fixtureGit(t, worktree, "status", "--porcelain", "--untracked-files=all"); strings.TrimSpace(dirty) != "" {
		t.Fatalf("wt left the new worktree dirty:\n%s", dirty)
	}
	inventory, err := service.worktrunkList(ctx)
	if err != nil {
		t.Fatalf("worktrunkList after start: %v", err)
	}
	if listed, ok := inventory.Paths[fx.branch]; !ok || listed != worktree {
		t.Fatalf("wt 0.80.0 inventory lists %q for the branch (found %t), want the created worktree %q", listed, ok, worktree)
	}

	status := service.Status(ctx, task)
	if status.Status != TaskExecutionStatusActive || status.Execution == nil {
		t.Fatalf("run-status before upstream: status = %s, reason = %s; want ACTIVE", status.Status, status.Reason)
	}
	if status.FinishReady {
		t.Fatalf("run-status reports finish-ready before the task branch has an upstream")
	}
	fixtureGit(t, worktree, "push", "-q", "-u", "origin", fx.branch)
	status = service.Status(ctx, task)
	if status.Status != TaskExecutionStatusActive || !status.FinishReady {
		t.Fatalf("run-status after push: status = %s, finish-ready = %t, reason = %s; want ACTIVE and finish-ready", status.Status, status.FinishReady, status.Reason)
	}

	list := service.List(ctx)
	if list.Status != TaskExecutionStatusReady || list.ActiveCount != 1 || len(list.Executions) != 1 {
		t.Fatalf("run-list = %s with %d active of %d execution(s); want READY with 1 active of 1", list.Status, list.ActiveCount, len(list.Executions))
	}
	if len(list.States) != 1 || list.States[0].Status != TaskExecutionStatusActive {
		t.Fatalf("run-list states = %+v; want one ACTIVE state", list.States)
	}

	// Reclaim through the guarded verb: drop the remote branch first, the way
	// an integrated task would have none, then discard must remove the real
	// worktree and branch through wt remove and prove the removal from the
	// schema 2 inventory.
	fixtureGit(t, fx.root, "push", "-q", "origin", "--delete", fx.branch)
	discard, code := service.Discard(ctx, task, "worktrunk 0.80 lifecycle fixture reclaim", "")
	if code != 0 || discard.Status != TaskExecutionStatusAborted {
		t.Fatalf("run-discard = %s exit %d, reason = %s; want ABORTED exit 0", discard.Status, code, discard.Reason)
	}
	if discard.Receipt == nil || !discard.Receipt.WorktreeRemoved || !discard.Receipt.LocalBranchRemoved || !discard.Receipt.RemoteBranchRemoved {
		t.Fatalf("discard receipt does not prove the full reclaim: %+v", discard.Receipt)
	}
	if _, statErr := os.Stat(worktree); !os.IsNotExist(statErr) {
		t.Fatalf("worktree still exists after wt remove: %v", statErr)
	}
	expectBranchAbsent(t, fx.root, fx.branch)
	inventory, err = service.worktrunkList(ctx)
	if err != nil {
		t.Fatalf("worktrunkList after reclaim: %v", err)
	}
	if _, stillListed := inventory.Paths[fx.branch]; stillListed {
		t.Fatalf("wt 0.80.0 inventory still lists the reclaimed branch %s", fx.branch)
	}
	finalStatus := service.Status(ctx, task)
	if finalStatus.Status != TaskExecutionStatusAborted {
		t.Fatalf("run-status after reclaim = %s, reason = %s; want ABORTED", finalStatus.Status, finalStatus.Reason)
	}
	finalList := service.List(ctx)
	if finalList.Status != TaskExecutionStatusReady || finalList.ActiveCount != 0 {
		t.Fatalf("run-list after reclaim = %s with %d active; want READY with 0 active (terminal records do not block)", finalList.Status, finalList.ActiveCount)
	}
}

// TestBranchAbsenceClassification pins the classifier behind
// expectBranchAbsent so the lifecycle fixture's local-branch-absence assertion
// can never weaken: exit 1 from a real git show-ref is the one accepted
// "absent" answer, while a verified ref, a different exit code, a git that
// never started, and a canceled context are each rejected. These controls
// exercise the classifier directly; the fixture itself still runs the real
// command through expectBranchAbsent.
func TestBranchAbsenceClassification(t *testing.T) {
	base := t.TempDir()
	global := filepath.Join(base, "gitconfig")
	if err := os.WriteFile(global, []byte("[init]\n\tdefaultBranch = main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	repo := filepath.Join(base, "repo")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, repo, "init", "-q", "-b", "main")
	fixtureGit(t, repo, "-c", "user.email=fixture@example.invalid", "-c", "user.name=Worktrunk contract fixture", "commit", "--allow-empty", "-qm", "base")
	absent := func(t *testing.T) error {
		t.Helper()
		return exec.CommandContext(t.Context(), "git", "-C", repo, "show-ref", "--verify", "--quiet", "refs/heads/dev/gone").Run()
	}

	t.Run("exit 1 means absent", func(t *testing.T) {
		err := absent(t)
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			t.Fatalf("missing ref produced %v; want a real *exec.ExitError with exit code 1, so the control pins the accepted class", err)
		}
		if verdict := classifyBranchAbsence(err); verdict != nil {
			t.Fatalf("exit 1 must classify as absence, got rejection: %v", verdict)
		}
	})
	t.Run("verified ref is rejected", func(t *testing.T) {
		err := exec.CommandContext(t.Context(), "git", "-C", repo, "show-ref", "--verify", "--quiet", "refs/heads/main").Run()
		if err != nil {
			t.Fatalf("control ref main did not verify: %v", err)
		}
		if verdict := classifyBranchAbsence(err); verdict == nil {
			t.Fatal("nil success was classified as absence; the branch exists")
		}
	})
	t.Run("a different exit code is rejected", func(t *testing.T) {
		// Outside any repository show-ref dies with git's generic fatal exit
		// 128 -- a failure mode the assertion must not read as absence.
		err := exec.CommandContext(t.Context(), "git", "-C", filepath.Join(base, "not-a-repo"), "show-ref", "--verify", "--quiet", "refs/heads/main").Run()
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() == 1 {
			t.Fatalf("control produced %v; want an *exec.ExitError whose exit code is not 1", err)
		}
		verdict := classifyBranchAbsence(err)
		if verdict == nil {
			t.Fatalf("exit %d was classified as absence", exitErr.ExitCode())
		}
		if !strings.Contains(verdict.Error(), "want 1") {
			t.Fatalf("rejection %q does not name the demanded exit status", verdict)
		}
	})
	t.Run("a start failure is rejected", func(t *testing.T) {
		err := errors.New("exec: git: executable file not found in $PATH")
		verdict := classifyBranchAbsence(err)
		if verdict == nil {
			t.Fatal("a git that never started was classified as absence")
		}
		if !strings.Contains(verdict.Error(), "never ran git") {
			t.Fatalf("rejection %q does not say the check never ran git", verdict)
		}
	})
	t.Run("a canceled context is rejected", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := exec.CommandContext(ctx, "git", "-C", repo, "show-ref", "--verify", "--quiet", "refs/heads/dev/gone").Run()
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("control produced %v; want the context error, since Start refuses an already-canceled context", err)
		}
		if verdict := classifyBranchAbsence(err); verdict == nil {
			t.Fatal("context cancellation was classified as absence")
		}
	})
}
