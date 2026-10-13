// Package parity pins the CE `ce task run-*` lifecycle behavior as the
// reference for the gz-git port, per ce-agent-kit ADR-0055 and the
// run-lifecycle row of docs/00-product/11-task-migration-ledger.md.
//
// Golden files under testdata/golden are machine-captured from the CE
// binary (PARITY_RECORD=1), never hand-written. A plain `go test` run
// re-executes every scenario against the CE binary and diffs the
// normalized result against the committed goldens. See
// docs/design/RUN_LIFECYCLE_PARITY_CONTRACT.md for the contract itself.
package parity

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Reference toolchain pinned by the contract doc. The suite runs whatever
// `ce` resolves on PATH; the versions below are recorded into every golden
// so a toolchain drift shows up as a diff, and re-recording is the
// declared response. The CE pin is the full 40-hex source commit of the
// approved build (142-g612f6bf8); TestParityReferencePinsAreCoherent holds
// goldens, harness, and contract doc to this one identity.
const (
	pinnedCECommit = "612f6bf80d76563c60464e33802ef7c299940299"
	host           = "mbp"
	actorDefault   = "claude"
	pinnedGitDate  = "2026-01-01T00:00:00+0000"
)

var (
	recordMode = flag.Bool("record", false, "re-record golden files from the CE binary instead of verifying")
	ceBin      string
)

type toolchain struct {
	CE    string `json:"ce"`
	Wt    string `json:"wt"`
	GzGit string `json:"gzGit"`
}

// TestMain resolves the CE binary once for the package. Without it the
// suite cannot say anything about parity, so it skips loudly instead of
// failing on machines that do not carry the CE toolchain. Target mode
// (PARITY_TARGET=gz-git) runs the same fixtures against the port binary
// instead and needs no CE at all.
func TestMain(m *testing.M) {
	if os.Getenv("PARITY_RECORD") == "1" {
		*recordMode = true
	}
	targetMode = os.Getenv("PARITY_TARGET") == "gz-git" || os.Getenv("GZ_GIT_BIN") != ""
	if targetMode {
		if *recordMode {
			fmt.Println("parity: PARITY_RECORD is ignored in target mode; goldens describe CE and are never rewritten from the port")
			*recordMode = false
		}
		code := m.Run()
		if gzBuildDir != "" {
			_ = os.RemoveAll(gzBuildDir)
		}
		os.Exit(code)
	}
	found, err := exec.LookPath(envOr("CE_BIN", "ce"))
	if err != nil {
		if os.Getenv("CE_BIN") != "" {
			// Fail closed: an explicitly supplied CE_BIN that does not
			// resolve must never pass as a skipped run — the goldens claim
			// to describe that exact binary.
			fmt.Printf("parity: CE_BIN=%s does not resolve; failing closed (goldens pin a specific CE)\n", os.Getenv("CE_BIN"))
			os.Exit(1)
		}
		fmt.Println("parity: ce binary not found on PATH (or CE_BIN); skipping parity fixtures")
		os.Exit(0)
	}
	ceBin = found
	if !*recordMode {
		guardGoldenToolchain()
	}
	os.Exit(m.Run())
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// step is one captured command invocation inside a scenario sandbox.
type step struct {
	label    string   // unique within the scenario
	cmd      string   // "ce" (default), "git", or any other sandbox-local binary
	args     []string // argv after the command name
	actor    string   // CE_TASK_ACTOR for this step; "" leaves it unset
	human    bool     // GIT_WORK_ACTOR (kind human) instead of CE_TASK_ACTOR when actor is set via this field
	dir      string   // sandbox-relative cwd; "" means the sandbox root
	snapshot bool     // capture git + CE record state after the step
}

// portContract records the behavior the gz-git port must implement when it
// diverges from CE. knownDivergent scenarios pin CE's actual bytes as the
// reference and carry the fix requirement here, never by editing the
// captured output.
type portContract struct {
	Issue    string `json:"issue"`
	Note     string `json:"note"`
	Status   string `json:"status"`
	ExitCode int    `json:"exitCode"`
}

type scenario struct {
	name string
	// taskRuntimeYAML overrides the generated .ce/task-runtime.yaml; the
	// WORKTREE_ROOT placeholder is resolved to the sandbox worktree root.
	taskRuntimeYAML string
	knownDivergent  bool
	contract        *portContract
	steps           []step
}

type capturedStep struct {
	Step     string         `json:"step"`
	Cmd      string         `json:"cmd"`
	Argv     []string       `json:"argv"`
	ExitCode int            `json:"exitCode"`
	Stdout   string         `json:"stdout"`
	Stderr   string         `json:"stderr"`
	State    *stateSnapshot `json:"state,omitempty"`
}

type stateSnapshot struct {
	Worktrees  string            `json:"worktrees"`
	Refs       string            `json:"refs"`
	OriginRefs string            `json:"originRefs"`
	Log        string            `json:"log"`
	Executions json.RawMessage   `json:"executions"`
	Receipts   []json.RawMessage `json:"receipts"`
}

type goldenFile struct {
	Scenario       string         `json:"scenario"`
	Reference      string         `json:"reference"`
	Toolchain      toolchain      `json:"toolchain"`
	KnownDivergent bool           `json:"knownDivergent,omitempty"`
	PortContract   *portContract  `json:"portContract,omitempty"`
	Steps          []capturedStep `json:"steps"`
}

var (
	tsRe    = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?Z`)
	shaRe   = regexp.MustCompile(`\b[0-9a-f]{40}\b`)
	shortRe = regexp.MustCompile(`\b[0-9a-f]{7,12}\b`)
	pidRe   = regexp.MustCompile(`\bpid[=": ]+\d+`)
	// wt remove summaries report the reclaimed worktree's byte size, which
	// tracks the length of the sandbox paths inside fixture files.
	sizeRe = regexp.MustCompile(`(\d+ files? · )[0-9.]+ ?(B|kB|MB|GB)`)
	// Technical-only folds. The ambient gz-git provider version identity is
	// fixture context, never the pin — the CE identity above stays literal
	// everywhere. The fold replaces ONLY the version token, never trailing
	// words, so error and state wording after a version line survives
	// verbatim. The integrate validation wall-clock duration is
	// nondeterministic run to run. Neither fold carries state, error, or
	// exit meaning; every semantic literal in captured output stays
	// literal.
	gzVersionAnyRe = regexp.MustCompile(`gz-git version \d+\.\d+(?:\.\d+)?(?:[-+][0-9A-Za-z.-]+)?`)
	durationRe     = regexp.MustCompile(`(validation \x{2014} Duration=)[0-9.]+(?:ms|\x{b5}s|us|ns|s)\b`)
)

// normalizer rewrites volatile fields into stable placeholders. The full
// SHA map is per-scenario so every occurrence of the same commit maps to
// the same placeholder, in deterministic first-appearance order.
type normalizer struct {
	sandbox  string
	tildeBox string
	home     string
	shas     map[string]string
	shorts   map[string]string
	nextSHA  int
}

func newNormalizer(sandbox string) *normalizer {
	home, _ := os.UserHomeDir()
	tilde := sandbox
	if home != "" && strings.HasPrefix(sandbox, home) {
		tilde = "~" + strings.TrimPrefix(sandbox, home)
	}
	return &normalizer{sandbox: sandbox, tildeBox: tilde, home: home, shas: map[string]string{}, shorts: map[string]string{}}
}

func (n *normalizer) norm(s string) string {
	// macOS getcwd reports the resolved /private/var path while os.TempDir
	// hands out /var — fold the resolved prefix into the same placeholder.
	s = strings.ReplaceAll(s, "/private"+n.sandbox, "<SANDBOX>")
	s = strings.ReplaceAll(s, n.sandbox, "<SANDBOX>")
	if n.tildeBox != n.sandbox {
		s = strings.ReplaceAll(s, n.tildeBox, "<SANDBOX>")
	}
	// Tool paths reported by diagnostics (gz-git, wt, ce live under the
	// user home, not the sandbox) must not pin this machine's layout.
	if n.home != "" && n.home != "/" {
		s = strings.ReplaceAll(s, n.home, "<HOME>")
	}
	s = tsRe.ReplaceAllString(s, "<TS>")
	s = pidRe.ReplaceAllString(s, "pid=<PID>")
	s = sizeRe.ReplaceAllString(s, "${1}<SIZE> ${2}")
	s = gzVersionAnyRe.ReplaceAllString(s, "gz-git version <GZ_GIT_VERSION>")
	s = durationRe.ReplaceAllString(s, "${1}<DUR>")
	s = shaRe.ReplaceAllStringFunc(s, func(m string) string {
		if _, ok := n.shas[m]; !ok {
			n.nextSHA++
			n.shas[m] = "<SHA" + strconv.Itoa(n.nextSHA) + ">"
		}
		return n.shas[m]
	})
	// Short SHAs only map when they extend an already-seen full SHA, so
	// ordinary hex-ish words survive untouched.
	s = shortRe.ReplaceAllStringFunc(s, func(m string) string {
		if mapped, ok := n.shorts[m]; ok {
			return mapped
		}
		for full, ph := range n.shas {
			if strings.HasPrefix(full, m) {
				n.shorts[m] = ph
				return ph
			}
		}
		return m
	})
	return s
}

// sandbox is a throwaway fixture environment: a git repo on master with a
// trivial integration gate, a bare origin cloned from it, a CE task
// runtime declaration whose worktree root is jailed inside the sandbox,
// and isolated HOME/XDG dirs so no ambient identity leaks in.
type sandbox struct {
	root string
	t    *testing.T
	norm *normalizer
}

const sandboxRepoID = "parity-sandbox"

func newSandbox(t *testing.T, sc scenario) *sandbox {
	t.Helper()
	name := sc.name
	root := filepath.Join(t.TempDir(), name)
	wtRoot := filepath.Join(root, "wt", sandboxRepoID)
	must(t, os.MkdirAll(wtRoot, 0o755))
	must(t, os.MkdirAll(filepath.Join(root, "home"), 0o755))
	must(t, os.MkdirAll(filepath.Join(root, "xdg", "ce"), 0o755))
	sb := &sandbox{root: root, t: t, norm: newNormalizer(root)}

	decl := sc.taskRuntimeYAML
	if decl == "" {
		decl = "schema-version: 1\nrepository-id: " + sandboxRepoID + "\nworktree-roots:\n  " + host + ": WORKTREE_ROOT\nintegration-provider: gz-git\n"
	}
	decl = strings.ReplaceAll(decl, "WORKTREE_ROOT", wtRoot)

	repo := filepath.Join(root, "repo")
	sb.git("", "init", "-q", "-b", "master", "repo")
	write(t, filepath.Join(repo, "README.md"), "parity sandbox\n")
	// The integration gate: gz-git integrate check fails a gate-less repo,
	// and run-finish drives that check. A trivial target is the smallest
	// repo that can reach DONE.
	write(t, filepath.Join(repo, "Makefile"), "check:\n\t@echo sandbox gate ok\nlint:\n\t@echo sandbox lint ok\n")
	// The branch declaration is runtime-tool-agnostic: both CE and the port
	// read the integration branch and task pattern from it.
	write(t, filepath.Join(repo, ".gz-git.yaml"), "branch:\n  integrationBranch:\n    - master\n  taskPattern:\n    - dev/*/*/*\n")
	if targetMode {
		// The port's runtime declaration lives at .gz-git-task.yaml, and it
		// must stay git-ignored so finish's clean-tree precondition never
		// sees the fixture's own configuration as a dirty path.
		write(t, filepath.Join(repo, ".gitignore"), ".gz-git-task.yaml\n.ce/\ntmp/\n")
		write(t, filepath.Join(repo, targetDeclPath()), decl)
	} else {
		write(t, filepath.Join(repo, ".gitignore"), ".ce/\ntmp/\n")
		write(t, filepath.Join(repo, ".ce", "task-runtime.yaml"), decl)
	}
	sb.git("repo", "add", ".")
	sb.git("repo", "commit", "-q", "-m", "init")
	// clone --bare (not init --bare) so origin's HEAD/default branch match
	// the source repo and gz-git resolves the integration target the same
	// way it would against a real remote.
	sb.git("", "clone", "-q", "--bare", "repo", "origin.git")
	sb.git("repo", "remote", "add", "origin", filepath.Join(root, "origin.git"))
	// A real clone carries refs/remotes/origin/<integration>; CE reads the
	// run source from that remote-tracking ref (ce-agent-kit 612f6bf8), so
	// the sandbox fetches once to match a cloned primary checkout.
	sb.git("repo", "fetch", "-q", "origin")
	return sb
}

// baseEnv is rebuilt from scratch for every exec: nothing ambient
// (CE_TASK_ACTOR, GIT_WORK_ACTOR, identity files) may leak into a fixture.
func (sb *sandbox) baseEnv() []string {
	return append(
		os.Environ()[:0],
		"PATH="+os.Getenv("PATH"),
		"HOME="+filepath.Join(sb.root, "home"),
		"XDG_CONFIG_HOME="+filepath.Join(sb.root, "xdg"),
		"GIT_WORK_HOST="+host,
		"GIT_AUTHOR_DATE="+pinnedGitDate,
		"GIT_COMMITTER_DATE="+pinnedGitDate,
		"GIT_AUTHOR_NAME=parity-test",
		"GIT_AUTHOR_EMAIL=parity@example.invalid",
		"GIT_COMMITTER_NAME=parity-test",
		"GIT_COMMITTER_EMAIL=parity@example.invalid",
		"TZ=UTC",
	)
}

func (sb *sandbox) stepEnv(st step) []string {
	env := sb.baseEnv()
	// The agent-identity variable is the one runtime-owned knob: CE reads
	// CE_TASK_ACTOR, the port reads GZ_GIT_TASK_ACTOR; the human fallback
	// (GIT_WORK_ACTOR) is shared.
	agentActor := "CE_TASK_ACTOR"
	if targetMode {
		agentActor = "GZ_GIT_TASK_ACTOR"
	}
	switch {
	case st.actor != "" && st.human:
		env = append(env, "GIT_WORK_ACTOR="+st.actor)
	case st.actor != "":
		env = append(env, agentActor+"="+st.actor)
	default:
		// The isolated HOME/XDG leave the identity fallback chain empty,
		// so the default actor is always set explicitly.
		env = append(env, agentActor+"="+actorDefault)
	}
	return env
}

func (sb *sandbox) dir(st step) string {
	if st.dir == "" {
		return sb.root
	}
	return filepath.Join(sb.root, st.dir)
}

// run executes one command and returns its raw output; it never swallows a
// non-zero exit — the exit code is the fixture data.
func (sb *sandbox) run(st step) (stdout, stderr string, exitCode int) {
	sb.t.Helper()
	cmdName := st.cmd
	args := st.args
	if cmdName == "" {
		if targetMode {
			ensureTargetBinary(sb.t)
			cmdName = gzGitBin
			// Executed argv is mapped to the port's surface; the captured
			// step keeps the CE form so one golden serves both modes.
			args = mapTargetArgs(st.args)
		} else {
			cmdName = ceBin
		}
	}
	cmd := exec.Command(cmdName, args...) //nolint:noctx // fixture step under a pinned env
	cmd.Dir = sb.dir(st)
	cmd.Env = sb.stepEnv(st)
	var out, errBuf strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	runErr := cmd.Run()
	if runErr != nil {
		var ee *exec.ExitError
		if errors.As(runErr, &ee) {
			exitCode = ee.ExitCode()
		} else {
			sb.t.Fatalf("step %q: %v", st.label, runErr)
		}
	}
	return out.String(), errBuf.String(), exitCode
}

// git runs a plumbing helper step; failures here are setup bugs, so they
// abort the scenario rather than being captured as data.
func (sb *sandbox) git(dir string, args ...string) {
	sb.t.Helper()
	stdout, stderr, code := sb.run(step{cmd: "git", args: args, dir: dir})
	if code != 0 {
		sb.t.Fatalf("git %v in %s failed (%d): %s%s", args, dir, code, stdout, stderr)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	must(t, os.MkdirAll(filepath.Dir(path), 0o755))
	must(t, os.WriteFile(path, []byte(content), 0o644))
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// snapshotState captures the git surface and the CE record files. Order is
// fixed because the SHA placeholder numbering follows capture order.
func (sb *sandbox) snapshotState() *stateSnapshot {
	sb.t.Helper()
	snap := &stateSnapshot{}
	probe := func(dir string, args ...string) string {
		stdout, stderr, code := sb.run(step{cmd: "git", args: args, dir: dir})
		if code != 0 || stderr != "" {
			sb.t.Fatalf("snapshot git %v in %s is unhealthy: exit %d stderr %q stdout %q", args, dir, code, stderr, stdout)
		}
		return stdout
	}
	snap.Worktrees = probe("repo", "worktree", "list", "--porcelain")
	snap.Refs = probe("repo", "for-each-ref", "--format=%(refname) %(objectname)")
	snap.OriginRefs = probe("origin.git", "for-each-ref", "--format=%(refname) %(objectname)")
	snap.Log = probe("repo", "log", "--all", "--topo-order", "--format=%H %s")
	stateDir := filepath.Join(sb.root, "repo", ".git", "ce", "task-runtime", "v1")
	if targetMode {
		stateDir = filepath.Join(append([]string{sb.root, "repo", ".git"}, targetStateDir()...)...)
	}
	snap.Executions = sb.readJSON(filepath.Join(stateDir, "executions.json"))
	snap.Receipts = sb.readJSONL(filepath.Join(stateDir, "receipts.jsonl"))
	return snap
}

func (sb *sandbox) readJSON(path string) json.RawMessage {
	data, err := os.ReadFile(path) //nolint:gosec // fixture-controlled path
	if err != nil {
		return json.RawMessage("null")
	}
	return json.RawMessage(sb.normJSON(data))
}

func (sb *sandbox) readJSONL(path string) []json.RawMessage {
	data, err := os.ReadFile(path) //nolint:gosec // fixture-controlled path
	if err != nil {
		return nil
	}
	var lines []json.RawMessage
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line != "" {
			lines = append(lines, json.RawMessage(sb.normJSON([]byte(line))))
		}
	}
	return lines
}

// normJSON round-trips captured record JSON through normalize so volatile
// strings inside structured records get the same treatment as raw output.
func (sb *sandbox) normJSON(data []byte) []byte {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return json.RawMessage(strconv.Quote(sb.norm.norm(string(data))))
	}
	sb.walk(v)
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		sb.t.Fatalf("re-marshal captured json: %v", err)
	}
	return out
}

func (sb *sandbox) walk(v interface{}) {
	switch tv := v.(type) {
	case map[string]interface{}:
		for k, item := range tv {
			switch k {
			case "processId":
				// CE 0.10.1 records the provider process id in finish
				// diagnostics; it is nondeterministic technical data with
				// no semantic content. Only this key folds — stdout, stderr,
				// and error wording around it stay literal.
				tv[k] = "<PID>"
				continue
			}
			if s, ok := item.(string); ok {
				tv[k] = sb.norm.norm(s)
				continue
			}
			sb.walk(item)
		}
	case []interface{}:
		for i, item := range tv {
			if s, ok := item.(string); ok {
				tv[i] = sb.norm.norm(s)
				continue
			}
			sb.walk(item)
		}
	}
}

// normStdoutJSON applies the structured walk to a captured step's stdout
// whenever it is a JSON envelope — unconditionally, because ordinary
// finish answers embed receipts whose diagnostics carry the nondeterministic
// processId. Inside the walk only technical values fold (processId, the
// ambient GZ version token, shared normalizer folds); statuses, reasons,
// error wording, and exit-relevant text stay literal. Ambient gz-git probe
// stdout/stderr and their errors are captured as-is — only the version
// identity token is placeholder-folded, so distinct errors never collide.
func (sb *sandbox) normStdoutJSON(s string) string {
	trimmed := strings.TrimSpace(s)
	if !strings.HasPrefix(trimmed, "{") {
		return s
	}
	var v interface{}
	if err := json.Unmarshal([]byte(trimmed), &v); err != nil {
		return s
	}
	sb.walk(v)
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return s
	}
	return string(out) + "\n"
}

func runScenario(t *testing.T, sc scenario) {
	t.Helper()
	sb := newSandbox(t, sc)
	// Target mode folds the toolchain to placeholders: the comparison must
	// not pin the dev-built gz-git or the machine's wt any more than the
	// CE golden pins this machine's CE.
	toolchainBlock := toolchain{CE: "<CE>", Wt: "<WT>", GzGit: "<GZ_GIT>"}
	if !targetMode {
		toolchainBlock = probeToolchain(t)
	}
	golden := goldenFile{
		Scenario:       sc.name,
		Reference:      "ce-agent-kit master " + pinnedCECommit,
		KnownDivergent: sc.knownDivergent,
		PortContract:   sc.contract,
		Toolchain:      toolchainBlock,
	}
	for _, st := range sc.steps {
		stdout, stderr, code := sb.run(st)
		stdout = sb.norm.norm(stdout)
		if !targetMode {
			stdout = sb.normStdoutJSON(stdout)
		}
		cs := capturedStep{
			Step:     st.label,
			Cmd:      firstNonEmpty(st.cmd, "ce"),
			Argv:     st.args,
			ExitCode: code,
			Stdout:   stdout,
			Stderr:   sb.norm.norm(stderr),
		}
		if st.snapshot {
			snap := sb.snapshotState()
			snap.Worktrees = sb.norm.norm(snap.Worktrees)
			snap.Refs = sb.norm.norm(snap.Refs)
			snap.OriginRefs = sb.norm.norm(snap.OriginRefs)
			snap.Log = sb.norm.norm(snap.Log)
			cs.State = snap
		}
		golden.Steps = append(golden.Steps, cs)
	}
	if targetMode {
		// Both sides of the comparison are pushed through the same
		// declared-divergence canonicalization before diffing.
		canonicalizeTarget(t, &golden)
		if sc.knownDivergent && sc.contract != nil {
			// The golden pins CE's recorded answer, including the behavior
			// the port deliberately fixes, so byte-diffing is wrong in both
			// directions; the port contract is asserted semantically.
			assertPortContract(t, sc, golden.Steps)
			return
		}
	}
	if sc.knownDivergent && sc.contract != nil {
		assertContract(t, sc, golden.Steps)
	}

	goldenPath := filepath.Join("testdata", "golden", sc.name+".json")
	if *recordMode {
		data, err := json.MarshalIndent(golden, "", "  ")
		must(t, err)
		write(t, goldenPath, string(data)+"\n")
		t.Logf("recorded %s", goldenPath)
		return
	}
	want, err := os.ReadFile(goldenPath) //nolint:gosec // committed fixture path
	if err != nil {
		t.Fatalf("golden %s missing; re-record with PARITY_RECORD=1 (or -record) go test ./tests/parity/...: %v", goldenPath, err)
	}
	got, err := json.MarshalIndent(golden, "", "  ")
	must(t, err)
	wantText := strings.TrimRight(string(want), "\n")
	if targetMode {
		// The committed golden describes CE in CE's own words. It goes
		// through the same canonicalization as the captured port document,
		// so the diff names a divergence beyond the declared surface.
		var wantGolden goldenFile
		if err := json.Unmarshal([]byte(wantText), &wantGolden); err != nil {
			t.Fatalf("parse golden %s: %v", goldenPath, err)
		}
		canonicalizeTarget(t, &wantGolden)
		remarshaled, err := json.MarshalIndent(wantGolden, "", "  ")
		must(t, err)
		wantText = string(remarshaled)
	}
	if diff := diffLines(wantText, string(got)); diff != "" {
		if targetMode {
			t.Errorf("%s diverged from the CE golden after target-mode canonicalization:\n%s", sc.name, diff)
		} else {
			t.Errorf("%s diverged from the recorded CE reference:\n%s", sc.name, diff)
		}
	}
}

// assertContract pins the semantic acceptance criteria on top of the byte
// compare — the points the migration ledger calls out by name.
func assertContract(t *testing.T, sc scenario, steps []capturedStep) {
	t.Helper()
	stepByLabel := func(label string) *capturedStep {
		for i := range steps {
			if steps[i].Step == label {
				return &steps[i]
			}
		}
		t.Fatalf("scenario %s: contract step %q missing", sc.name, label)
		return nil
	}
	switch sc.name {
	case "finish-noarg-single-active":
		// Re-evaluated against CE 612f6bf8: readiness is proven first (the
		// task is committed and pushed upstream; run-status answers ACTIVE
		// with finishReady), and no-arg run-finish then performs the
		// finish: DONE, exit 0, source pushed, worktree and both branches
		// reclaimed.
		ready := stepByLabel("status-ready")
		var readyDoc struct {
			Status      string `json:"status"`
			Task        string `json:"task"`
			FinishReady bool   `json:"finishReady"`
		}
		must(t, json.Unmarshal([]byte(ready.Stdout), &readyDoc))
		if ready.ExitCode != 0 || readyDoc.Task != "noarg" || readyDoc.Status != "ACTIVE" || !readyDoc.FinishReady {
			t.Errorf("finish-noarg precondition drifted: run-status after upstream push = exit %d status %q task %q finishReady %v; the single active run must be finish-ready before the no-arg finish", ready.ExitCode, readyDoc.Status, readyDoc.Task, readyDoc.FinishReady)
		}
		last := stepByLabel("finish-noarg")
		var done struct {
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
		must(t, json.Unmarshal([]byte(last.Stdout), &done))
		if last.ExitCode != 0 || done.Status != "DONE" || done.Receipt == nil ||
			done.Receipt.Operation != "finish" || done.Receipt.Status != "DONE" ||
			!done.Receipt.SourcePushed || !done.Receipt.WorktreeRemoved ||
			!done.Receipt.LocalBranchRemoved || !done.Receipt.RemoteBranchRemoved {
			t.Errorf("known-divergent contract stale: CE no-arg finish = exit %d status %q receipt %+v; expected the performed finish (DONE/exit 0, source pushed, worktree/local/remote reclaimed) — re-evaluate ISSUE-069", last.ExitCode, done.Status, done.Receipt)
		}
	case "reclaimed-run-reconciliation":
		// CE 612f6bf8 reference for an externally reclaimed run: both read
		// verbs answer BLOCKED with exit 1 and the execution history entry
		// stays. The port keeps its native reconciliation semantics — this
		// assert pins the CE reference only and never coerces the port.
		listStep := stepByLabel("list-after-reclaim")
		var envelope struct {
			Status      string `json:"status"`
			ActiveCount int    `json:"activeCount"`
			Executions  []struct {
				Task string `json:"task"`
			} `json:"executions"`
			States []struct {
				Status string `json:"status"`
			} `json:"states"`
		}
		must(t, json.Unmarshal([]byte(listStep.Stdout), &envelope))
		if listStep.ExitCode != 1 || envelope.Status != "BLOCKED" || envelope.ActiveCount != 0 ||
			len(envelope.Executions) != 1 || envelope.Executions[0].Task != "reclaimed-task" ||
			len(envelope.States) != 1 || envelope.States[0].Status != "BLOCKED" {
			t.Errorf("reclaimed CE reference drifted: list = exit %d status %q activeCount %d executions %d states %+v — expected BLOCKED/exit 1 with the history entry retained; re-evaluate the declared divergence", listStep.ExitCode, envelope.Status, envelope.ActiveCount, len(envelope.Executions), envelope.States)
		}
		statusStep := stepByLabel("status-after-reclaim")
		if statusStep.ExitCode != 1 || !strings.Contains(statusStep.Stdout, "Worktrunk inventory does not match execution metadata") {
			t.Errorf("reclaimed CE reference drifted: status = exit %d stdout %q — expected BLOCKED/exit 1 with the inventory-mismatch reason; re-evaluate the declared divergence", statusStep.ExitCode, statusStep.Stdout)
		}
	}
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func probeToolchain(t *testing.T) toolchain {
	t.Helper()
	version := func(name string, args ...string) string {
		out, err := exec.Command(name, args...).Output() //nolint:noctx // version probe
		if err != nil {
			t.Fatalf("probe %s: %v", name, err)
		}
		return strings.TrimSpace(string(out))
	}
	gz := version("gz-git", "--version")
	// Ambient-GZ isolation at the toolchain block: the provider version is
	// fixture context and folds to the declared placeholder; the CE and wt
	// tokens stay literal — they are part of the pinned reference identity.
	gz = gzVersionAnyRe.ReplaceAllString(gz, "gz-git version <GZ_GIT_VERSION>")
	return toolchain{
		CE:    version(ceBin, "version"),
		Wt:    version("wt", "--version"),
		GzGit: gz,
	}
}

// diffLines renders the first differing regions of two normalized
// documents; it exists so a parity failure names the divergent bytes
// instead of failing with an opaque mismatch.
func diffLines(want, got string) string {
	w := strings.Split(want, "\n")
	g := strings.Split(got, "\n")
	var out []string
	shown := 0
	for i := 0; i < len(w) || i < len(g); i++ {
		var wl, gl string
		if i < len(w) {
			wl = w[i]
		}
		if i < len(g) {
			gl = g[i]
		}
		if wl == gl {
			continue
		}
		out = append(out, fmt.Sprintf("  line %d\n  - %s\n  + %s", i+1, wl, gl))
		shown++
		if shown >= 20 {
			out = append(out, "  ... (further differences elided)")
			break
		}
	}
	if len(out) == 0 {
		return ""
	}
	return strings.Join(out, "\n")
}
