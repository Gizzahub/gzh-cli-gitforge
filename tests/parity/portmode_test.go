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
//     contract is asserted semantically instead (ISSUE-069: no-arg finish
//     with one active run exits 0).
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
	build := exec.Command("go", "build", "-o", gzGitBin, "./cmd/gz-git") //nolint:noctx // test-lifecycle build
	build.Dir = root
	build.Env = append(os.Environ(), "GOWORK=off")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build gz-git: %v\n%s", err, out)
	}
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
		// The portContract on the scenario pins it: with exactly one ACTIVE
		// run, no-arg finish answers the READY aggregate document and exits
		// 0, where CE recorded the same document with exit 1.
		last := stepByLabel(t, sc, steps, "finish-noarg")
		if last.ExitCode != 0 {
			t.Errorf("ISSUE-069 not fixed: no-arg gz-git run finish exited %d (want 0)\nstdout: %s", last.ExitCode, last.Stdout)
			return
		}
		var doc struct {
			Status      string `json:"status"`
			ActiveCount int    `json:"activeCount"`
		}
		if err := json.Unmarshal([]byte(last.Stdout), &doc); err != nil {
			t.Errorf("ISSUE-069 finish response is not JSON: %v\nstdout: %s", err, last.Stdout)
			return
		}
		if doc.Status != "READY" || doc.ActiveCount != 1 {
			t.Errorf("ISSUE-069 finish response = status %q activeCount %d (want READY/1)\nstdout: %s", doc.Status, doc.ActiveCount, last.Stdout)
		}
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
