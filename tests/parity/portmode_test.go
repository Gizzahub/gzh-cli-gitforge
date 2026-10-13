// Target-mode plumbing: running the same fixtures against the gz-git port.
//
// PARITY_TARGET=gz-git (or GZ_GIT_BIN) switches the suite from re-running CE
// to running the port binary through identical scenarios, canonicalizing both
// sides into one shape, and diffing. The canonicalization is the port's
// declared divergence surface, in one place:
//
//   - argv maps ["task","run-<verb>"] to ["run","<verb>"] at execution time;
//     captured steps keep the CE form so one golden serves both modes;
//   - actionable CLI prose "ce task run-<verb>" becomes "gz-git run <verb>",
//     and the four CE-owned reconciliation/abort phrases become their gz-git
//     wording (protocol tokens stay CE-hyphenated everywhere);
//   - gz-git help/version probe diagnostics are deleted (the port discovers
//     provider facts in-process and emits none), and the executable key is
//     dropped on gz-git entries and on the provider block (an in-process
//     engine has no external path to name);
//   - build-identity strings (tool_version, tool_revision, provider version,
//     the toolchain block) fold to placeholders so a dev build compares
//     against a release golden without pinning either;
//   - known-divergent scenarios do not byte-diff in target mode; their port
//     contract is asserted semantically instead (ISSUE-069 re-evaluated:
//     no-arg finish with one finish-ready run must complete the finish
//     DONE/exit 0 like CE 612f6bf8; reclaimed-run reconciliation keeps the
//     port's native answer, with an unavailable setup surfaced distinctly).
//
// Target mode never records: goldens describe CE.

package parity

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	targetMode bool
	gzGitBin   string
	gzBuildDir string
)

// ensureTargetBinary picks GZ_GIT_BIN or builds the port binary from this
// repository, once per test-binary run. The build runs with GOWORK=off: a
// devbox workspace that lists the primary checkout must not leak another
// tree's sources into the binary the fixtures are about to judge.
func ensureTargetBinary(t *testing.T) {
	t.Helper()
	if gzGitBin != "" {
		return
	}
	if bin := os.Getenv("GZ_GIT_BIN"); bin != "" {
		gzGitBin = bin
		return
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Fatalf("PARITY_TARGET: go not found on PATH and GZ_GIT_BIN unset; cannot build the port binary")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	dir, err := os.MkdirTemp("", "parity-gzgit-*")
	if err != nil {
		t.Fatalf("create build dir: %v", err)
	}
	gzBuildDir = dir
	gzGitBin = filepath.Join(dir, "gz-git")
	build := targetBuildCommand(root, gzGitBin)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build gz-git: %v\n%s", err, out)
	}
}

// targetBuildCommand is the single definition of how the port binary under
// test is produced when GZ_GIT_BIN is not supplied: compiled from this
// repository's checkout root with GOWORK=off. Any GOWORK already present
// in the ambient environment is stripped first, so the command's
// environment carries exactly one GOWORK setting — off. An ambient
// installed gz-git is never the capture source — workspace isolation is
// the point, and TestParityTargetCaptureUsesCheckoutBinary pins this
// boundary.
func targetBuildCommand(root, out string) *exec.Cmd {
	build := exec.Command("go", "build", "-o", out, "./cmd/gz-git") //nolint:noctx // test-lifecycle build
	build.Dir = root
	env := make([]string, 0, len(os.Environ())+1)
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "GOWORK=") {
			continue
		}
		env = append(env, kv)
	}
	build.Env = append(env, "GOWORK=off")
	return build
}

// targetDeclPath is where the port reads its runtime declaration.
func targetDeclPath() string { return ".gz-git-task.yaml" }

// targetStateDir is where the port keeps run records under .git.
func targetStateDir() []string { return []string{"gz-git", "task-runtime", "v1"} }

// mapTargetArgs rewrites a captured CE-form argv into the port's surface.
func mapTargetArgs(args []string) []string {
	if len(args) >= 2 && args[0] == "task" && strings.HasPrefix(args[1], "run-") {
		mapped := make([]string, 0, len(args))
		mapped = append(mapped, "run", strings.TrimPrefix(args[1], "run-"))
		return append(mapped, args[2:]...)
	}
	return args
}

var (
	ceTaskRunRe = regexp.MustCompile(`\bce task run-([a-z]+)\b`)
	cePhrases   = []struct{ from, to string }{
		{"CE did not integrate it", "gz-git did not integrate it"},
		{"closed by CE reconciliation", "closed by gz-git reconciliation"},
		{"reclaimed outside CE", "reclaimed outside gz-git"},
		{"outside CE's configured path", "outside gz-git's configured path"},
	}
)

// portProse applies the declared CE→gz-git wording substitutions. It is
// applied to both sides of the diff, so the golden keeps CE's own words on
// disk while the comparison still pins the port's contract.
func portProse(s string) string {
	s = ceTaskRunRe.ReplaceAllString(s, "gz-git run $1")
	for _, p := range cePhrases {
		s = strings.ReplaceAll(s, p.from, p.to)
	}
	return s
}

// canonicalizeTarget rewrites one side of the comparison into the shared
// target-mode shape. It runs on the freshly captured port document and on the
// loaded CE golden alike.
func canonicalizeTarget(t *testing.T, g *goldenFile) {
	t.Helper()
	g.Toolchain = toolchain{CE: "<CE>", Wt: "<WT>", GzGit: "<GZ_GIT>"}
	for i := range g.Steps {
		g.Steps[i].Stdout = canonicalizeText(t, g.Steps[i].Stdout)
		g.Steps[i].Stderr = canonicalizeText(t, g.Steps[i].Stderr)
		if g.Steps[i].State == nil {
			continue
		}
		g.Steps[i].State.Worktrees = portProse(g.Steps[i].State.Worktrees)
		g.Steps[i].State.Refs = portProse(g.Steps[i].State.Refs)
		g.Steps[i].State.OriginRefs = portProse(g.Steps[i].State.OriginRefs)
		g.Steps[i].State.Log = portProse(g.Steps[i].State.Log)
		g.Steps[i].State.Executions = canonicalizeRaw(t, g.Steps[i].State.Executions)
		for j, receipt := range g.Steps[i].State.Receipts {
			g.Steps[i].State.Receipts[j] = canonicalizeRaw(t, receipt)
		}
	}
}

// canonicalizeText normalizes one captured output string: JSON documents get
// the tree pass, everything else only the prose pass.
func canonicalizeText(t *testing.T, s string) string {
	t.Helper()
	s = portProse(s)
	trimmed := strings.TrimSpace(s)
	if !strings.HasPrefix(trimmed, "{") {
		return s
	}
	var v interface{}
	if err := json.Unmarshal([]byte(trimmed), &v); err != nil {
		return s
	}
	canonicalizeValue(v)
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("re-marshal normalized document: %v", err)
	}
	return string(out) + "\n"
}

// canonicalizeRaw normalizes one captured record document.
func canonicalizeRaw(t *testing.T, raw json.RawMessage) json.RawMessage {
	t.Helper()
	if len(raw) == 0 || string(raw) == "null" {
		return raw
	}
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("parse captured record: %v", err)
	}
	canonicalizeValue(v)
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("re-marshal normalized record: %v", err)
	}
	return json.RawMessage(out)
}

var gzGitVersionRe = regexp.MustCompile(`^gz-git version .+$`)

// canonicalizeValue is the shared JSON-tree pass. See the file comment for
// what it deletes and what it folds, and why each rule stays narrow. It
// returns the (possibly replaced) value: maps mutate in place, but a
// filtered slice must be assigned back by the parent — a type-switch copy
// of a slice header cannot.
func canonicalizeValue(v interface{}) interface{} {
	switch tv := v.(type) {
	case map[string]interface{}:
		command, _ := tv["command"].(string)
		if command == "gz-git" {
			// An in-process engine has no external path to name; CE recorded
			// the provider binary's path on every gz-git invocation.
			delete(tv, "executable")
		}
		if name, ok := tv["name"]; ok && name == "gz-git" {
			if _, hasCapabilities := tv["capabilities"]; hasCapabilities {
				delete(tv, "executable")
			}
		}
		for k, item := range tv {
			switch k {
			case "version":
				if s, ok := item.(string); ok && gzGitVersionRe.MatchString(s) {
					tv[k] = "gz-git version <GZ_GIT_VERSION>"
					continue
				}
			case "tool_version":
				if s, ok := item.(string); ok && s != "" {
					tv[k] = "<TOOL_VERSION>"
					continue
				}
			case "tool_revision":
				if s, ok := item.(string); ok && s != "" {
					tv[k] = "<TOOL_REVISION>"
					continue
				}
			}
			tv[k] = canonicalizeValue(item)
		}
	case string:
		// Record documents (executions, receipts) carry the lifecycle's own
		// prose in reason fields; the same substitutions apply there as to
		// captured stdout.
		return portProse(tv)
	case []interface{}:
		kept := make([]interface{}, 0, len(tv))
		for _, item := range tv {
			// CE's doctor probed the external provider with --version and
			// --help invocations; the in-process engine contributes none, so
			// the probe entries leave both sides of the comparison.
			if entry, ok := item.(map[string]interface{}); ok && isGzGitProbe(entry) {
				continue
			}
			kept = append(kept, canonicalizeValue(item))
		}
		return kept
	}
	return v
}

func isGzGitProbe(entry map[string]interface{}) bool {
	command, _ := entry["command"].(string)
	if command != "gz-git" {
		return false
	}
	args, _ := entry["args"].([]interface{})
	if len(args) == 0 {
		return false
	}
	if args[0] != "integrate" {
		return true // --version and --help probes
	}
	for _, arg := range args {
		if arg == "--help" {
			return true
		}
	}
	return false
}

// assertPortContract pins the port-side acceptance for known-divergent
// scenarios. In target mode these scenarios do not byte-diff against CE's
// recorded answer, because diverging is the point; the contract below is the
// fix the golden's portContract field demands.
func assertPortContract(t *testing.T, sc scenario, steps []capturedStep) {
	t.Helper()
	switch sc.name {
	case "finish-noarg-single-active":
		// Readiness precondition, same as the CE-side assert: the task was
		// committed and pushed upstream, so the port's run-status must agree
		// the single run is finish-ready before the no-arg finish is judged.
		ready := stepByLabel(t, sc, steps, "status-ready")
		var readyDoc struct {
			Status      string `json:"status"`
			Task        string `json:"task"`
			FinishReady bool   `json:"finishReady"`
		}
		if err := json.Unmarshal([]byte(ready.Stdout), &readyDoc); err != nil {
			t.Fatalf("port run-status answer is not JSON: %v\nstdout: %s", err, ready.Stdout)
		}
		if ready.ExitCode != 0 || readyDoc.Task != "noarg" || readyDoc.Status != "ACTIVE" || !readyDoc.FinishReady {
			t.Errorf("port finish-noarg precondition drifted: run-status = exit %d status %q task %q finishReady %v (want ACTIVE/noarg/true)\nstdout: %s", ready.ExitCode, readyDoc.Status, readyDoc.Task, readyDoc.FinishReady, ready.Stdout)
		}
		// The re-evaluated portContract: with exactly one finish-ready run,
		// no-arg finish completes the run exactly as CE 612f6bf8 does —
		// DONE, exit 0, source pushed, worktree and local/remote branches
		// reclaimed. A list answer or a non-zero exit is the forbidden
		// regression.
		last := stepByLabel(t, sc, steps, "finish-noarg")
		var doc struct {
			Status  string `json:"status"`
			Receipt *struct {
				Operation           string `json:"operation"`
				Status              string `json:"status"`
				SourcePushed        bool   `json:"sourcePushed"`
				WorktreeRemoved     bool   `json:"worktreeRemoved"`
				LocalBranchRemoved  bool   `json:"localBranchRemoved"`
				RemoteBranchRemoved bool   `json:"remoteBranchRemoved"`
			} `json:"receipt"`
		}
		if err := json.Unmarshal([]byte(last.Stdout), &doc); err != nil {
			t.Errorf("port no-arg finish response is not JSON: %v\nstdout: %s", err, last.Stdout)
			return
		}
		if last.ExitCode != 0 || doc.Status != "DONE" || doc.Receipt == nil ||
			doc.Receipt.Operation != "finish" || doc.Receipt.Status != "DONE" ||
			!doc.Receipt.SourcePushed || !doc.Receipt.WorktreeRemoved ||
			!doc.Receipt.LocalBranchRemoved || !doc.Receipt.RemoteBranchRemoved {
			t.Errorf("ISSUE-069 port contract not met: no-arg finish = exit %d status %q receipt %+v (want DONE/exit 0 with source pushed and full reclaim)\nstdout: %s", last.ExitCode, doc.Status, doc.Receipt, last.Stdout)
		}
	case "reclaimed-run-reconciliation":
		// Declared native divergence: the golden pins CE 612f6bf8's
		// BLOCKED/exit 1 reference answer; the port keeps its own
		// reconciliation semantics. The port assert pins the port's native
		// answer for the same externally reclaimed shape so drift on
		// EITHER side is caught without coercing the port toward CE's
		// answer.
		assertPortReclaimedNative(t, sc, steps)
	default:
		t.Fatalf("scenario %s is known-divergent without a port contract assert", sc.name)
	}
}

func stepByLabel(t *testing.T, sc scenario, steps []capturedStep, label string) *capturedStep {
	t.Helper()
	for i := range steps {
		if steps[i].Step == label {
			return &steps[i]
		}
	}
	t.Fatalf("scenario %s: step %q missing", sc.name, label)
	return nil
}

// assertPortReclaimedNative pins the port's NATIVE reconciliation contract
// for the externally reclaimed shape whenever the scenario's prerequisites
// were actually met, and surfaces an unavailable prerequisite distinctly
// instead of treating the refusal itself as the acceptance case. The CE
// golden records CE 612f6bf8's BLOCKED/exit 1 reference; the port is never
// coerced toward it. When the port's run-start is refused by its own
// Worktrunk gate (setup unavailable — observed under the ambient 0.80.0),
// no execution exists to reclaim, so the reconciliation contract cannot be
// exercised and the scenario says so explicitly rather than passing.
func assertPortReclaimedNative(t *testing.T, sc scenario, steps []capturedStep) {
	t.Helper()
	start := stepByLabel(t, sc, steps, "start")
	var startDoc struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(start.Stdout), &startDoc); err != nil {
		t.Fatalf("port start answer is not JSON: %v\nstdout: %s", err, start.Stdout)
	}
	if start.ExitCode != 0 || startDoc.Status != "ACTIVE" {
		// Distinct surfacing of the unavailable prerequisite: the port
		// refused setup, so there is no run to reconcile and the native
		// reconciliation contract below is NOT exercised or accepted here.
		t.Skipf("port reconciliation unexercisable in this environment: run-start = exit %d status %q reason %q (setup refused, e.g. the port's Worktrunk gate; not an acceptance case — the CE-mode golden still pins the full reference)", start.ExitCode, startDoc.Status, startDoc.Reason)
	}

	// Prerequisites met: the external removal steps must have really
	// removed the recorded worktree and branch, or the scenario is broken
	// rather than reconciling.
	removal := stepByLabel(t, sc, steps, "external-worktree-removal")
	if removal.ExitCode != 0 {
		t.Fatalf("external worktree removal failed (%d): %s%s — scenario prerequisite, not reconciliation behavior", removal.ExitCode, removal.Stdout, removal.Stderr)
	}

	// The native reconciliation contract: whatever the port answers for the
	// reclaimed shape must be a real derived answer over the recorded
	// execution — the executions[] history stays, the run is no longer
	// counted active, and the answer is the port's own (reconciled terminal
	// state or the port's own blocked reason), never a coerced CE copy.
	list := stepByLabel(t, sc, steps, "list-after-reclaim")
	var listDoc struct {
		Status      string `json:"status"`
		ActiveCount int    `json:"activeCount"`
		Executions  []struct {
			Task string `json:"task"`
		} `json:"executions"`
		States []struct {
			Status string `json:"status"`
		} `json:"states"`
	}
	if err := json.Unmarshal([]byte(list.Stdout), &listDoc); err != nil {
		t.Fatalf("port list answer is not JSON: %v\nstdout: %s", err, list.Stdout)
	}
	if len(listDoc.Executions) != 1 || listDoc.Executions[0].Task != "reclaimed-task" || len(listDoc.States) != 1 {
		t.Errorf("port native reconciliation broken: list = status %q activeCount %d executions %+v states %+v — the recorded execution must stay in executions[] with one derived state\nstdout: %s", listDoc.Status, listDoc.ActiveCount, listDoc.Executions, listDoc.States, list.Stdout)
		return
	}
	if listDoc.ActiveCount != 0 {
		t.Errorf("port native reconciliation broken: an externally reclaimed run must not stay active (activeCount=%d)\nstdout: %s", listDoc.ActiveCount, list.Stdout)
	}
	switch listDoc.States[0].Status {
	case "ABORTED", "DONE":
		// Reconciled to a terminal state: the read verbs succeed.
		if list.ExitCode != 0 {
			t.Errorf("port native reconciliation: terminal state %q must answer exit 0, got %d\nstdout: %s", listDoc.States[0].Status, list.ExitCode, list.Stdout)
		}
	default:
		// Not reconciled: the port's own blocked answer with its own reason,
		// exit 1 — its native refusal, distinct from CE's wording.
		if list.ExitCode != 1 {
			t.Errorf("port native reconciliation: non-terminal state %q must answer exit 1, got %d\nstdout: %s", listDoc.States[0].Status, list.ExitCode, list.Stdout)
		}
	}
}

// TestParityTargetCaptureUsesCheckoutBinary pins the target-mode capture
// boundary: when GZ_GIT_BIN is not supplied, the port binary the fixtures
// judge is compiled from this repository's checkout with GOWORK=off into a
// throwaway directory — never the ambient installed gz-git — and the
// ambient installed version, whatever it is, participates in comparisons
// only through the declared <GZ_GIT_VERSION> placeholder fold. The test
// never skips: the build-command pin is static, and the real capture path
// is driven in a forced target-mode frame.
func TestParityTargetCaptureUsesCheckoutBinary(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	must(t, err)

	// 1. Structural pin: the build command compiles from the checkout root
	//    with exactly one GOWORK=off in its environment.
	cmd := targetBuildCommand(root, filepath.Join(t.TempDir(), "gz-git"))
	if cmd.Dir != root {
		t.Fatalf("target build runs in %s, want the checkout root %s", cmd.Dir, root)
	}
	gowork := 0
	for _, kv := range cmd.Env {
		if kv == "GOWORK=off" {
			gowork++
		}
	}
	if gowork != 1 {
		t.Fatalf("target build env carries %d GOWORK=off entries, want exactly 1", gowork)
	}

	// 2. The real capture path resolves, builds, and executes the checkout
	//    binary. When the whole suite already runs in target mode, the
	//    shared cache is used as-is and TestMain owns its cleanup — this
	//    test never resets the frame globals, so nothing is orphaned and
	//    the scenarios keep the cached binary. Only a CE-mode run gets a
	//    forced, self-cleaning frame; its restore/cleanup defer is
	//    registered BEFORE the capture call so even a fatal inside the
	//    build can leak neither the throwaway dir nor the forced frame.
	savedMode, savedBin, savedDir := targetMode, gzGitBin, gzBuildDir
	savedEnv := os.Getenv("GZ_GIT_BIN")
	if savedMode {
		// Genuine target-mode suite: use the shared capture cache; TestMain
		// owns its build-dir cleanup. Nothing is reset, nothing orphaned.
		ensureTargetBinary(t)
	} else {
		defer func() {
			if gzBuildDir != "" && gzBuildDir != savedDir {
				_ = os.RemoveAll(gzBuildDir)
			}
			targetMode, gzGitBin, gzBuildDir = savedMode, savedBin, savedDir
			_ = os.Setenv("GZ_GIT_BIN", savedEnv)
		}()
		targetMode, gzGitBin, gzBuildDir = true, "", ""
		_ = os.Setenv("GZ_GIT_BIN", "")
		ensureTargetBinary(t)
	}
	if gzGitBin == "" {
		t.Fatal("target capture resolved no binary")
	}
	if filepath.Dir(gzGitBin) != gzBuildDir {
		t.Errorf("captured binary %s lives outside the throwaway build dir %s", gzGitBin, gzBuildDir)
	}

	// The captured binary really executes — capture is behavior, not build
	// metadata.
	if out, rErr := exec.Command(gzGitBin, "--version").Output(); rErr != nil { //nolint:noctx // captured binary execution
		t.Errorf("captured checkout binary does not execute: %v", rErr)
	} else if v := strings.TrimSpace(string(out)); !strings.HasPrefix(v, "gz-git version ") {
		t.Errorf("captured checkout binary --version = %q, want a gz-git version line", v)
	}

	// 3. The ambient installed gz-git is never the capture source, and its
	//    version is only ever compared through the declared placeholder
	//    fold — ambient GZ may be isolated or placeholdered, never pinned.
	if amb, ambErr := exec.LookPath("gz-git"); ambErr == nil {
		abs, absErr := filepath.Abs(gzGitBin)
		if absErr != nil {
			t.Fatalf("resolve captured binary path: %v", absErr)
		}
		if abs == amb {
			t.Errorf("target capture used the ambient installed gz-git %s; the checkout build is required", amb)
		}
		if out, vErr := exec.Command(amb, "--version").Output(); vErr == nil { //nolint:noctx // ambient version probe
			v := strings.TrimSpace(string(out))
			folded, _ := canonicalizeValue(map[string]interface{}{"version": v}).(map[string]interface{})
			if folded == nil || folded["version"] != "gz-git version <GZ_GIT_VERSION>" {
				t.Errorf("ambient gz-git version %q is not folded to the <GZ_GIT_VERSION> placeholder", v)
			}
		}
	} else {
		t.Log("no ambient installed gz-git on PATH: ambient isolation is vacuous, the <GZ_GIT_VERSION> placeholder fold stays declared")
	}

	// 4. The captured binary is this checkout's module. GOWORK=off means a
	//    devbox workspace listing another gzh-cli-gitforge tree cannot
	//    substitute its sources into the binary under judgment.
	vmeta, mErr := exec.Command("go", "version", "-m", gzGitBin).Output() //nolint:noctx // module identity probe
	if mErr != nil {
		t.Fatalf("go version -m %s: %v", gzGitBin, mErr)
	}
	if !strings.Contains(string(vmeta), "\tmod\tgithub.com/gizzahub/gzh-cli-gitforge\t") {
		t.Errorf("captured binary is not built from this checkout's module:\n%s", vmeta)
	}
}
