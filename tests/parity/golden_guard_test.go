// Guard for the CE parity fixtures. A plain verify run executes the
// installed ce against goldens captured from a specific ce build; when
// the host toolchain has moved on, every scenario fails with an opaque
// output diff and the re-record path is only documented in prose. This
// file checks the build tokens up front and fails once, naming both
// builds and the re-record commands, so drift is diagnosed in one
// message instead of eleven subtest failures.

package parity

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// pinnedCEBuild is the `ce version` Build token of the approved CE
// reference identity. Together with harness pinnedCECommit (the full
// source commit) it forms the one CE pin every golden, the harness, and
// docs/design/RUN_LIFECYCLE_PARITY_CONTRACT.md must name coherently;
// TestParityReferencePinsAreCoherent enforces exactly that. A placeholder
// is never acceptable here — a drifted pin means re-record, not re-pin.
const pinnedCEBuild = "142-g612f6bf8"

// ceBuildToken extracts the "Build:" token from `ce version` output, e.g.
// "475-g950650ef". It returns "" when the output carries no build line.
func ceBuildToken(versionOutput string) string {
	for _, line := range strings.Split(versionOutput, "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "Build:"); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

// ceCommitToken extracts the "Commit:" line from `ce version` output — the
// full 40-hex source commit. It returns "" when the output carries no
// commit line.
func ceCommitToken(versionOutput string) string {
	for _, line := range strings.Split(versionOutput, "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "Commit:"); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

// goldenCEBuilds maps each committed golden scenario to the ce build
// token its toolchain block already embeds (every golden writes
// toolchain.ce at record time). Unreadable or build-less files are
// skipped: the scenarios that read them report their own parse failures
// with full context.
func goldenCEBuilds(dir string) map[string]string {
	matches, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil || len(matches) == 0 {
		return nil
	}
	builds := make(map[string]string, len(matches))
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var golden goldenFile
		if err := json.Unmarshal(data, &golden); err != nil {
			continue
		}
		if token := ceBuildToken(golden.Toolchain.CE); token != "" {
			builds[filepath.Base(path)] = token
		}
	}
	return builds
}

// guardGoldenToolchain refuses the run before any scenario executes when
// the installed ce is not the build the committed goldens were recorded
// with. The suite would fail anyway, but as one opaque subtest diff per
// scenario; one early exit naming both builds and the re-record command
// is the declared response to toolchain drift
// (docs/design/RUN_LIFECYCLE_PARITY_CONTRACT.md). Fail closed: a ce that
// will not probe is a failed precondition, never a pass — the goldens
// claim to describe that exact binary. Goldens that carry no build token
// are left to the scenarios.
func guardGoldenToolchain() {
	out, err := exec.Command(ceBin, "version").Output() //nolint:noctx // version probe
	if err != nil {
		fmt.Printf("parity: ce binary %s failed to report a version (%v); failing closed — goldens pin a specific CE\n", ceBin, err)
		os.Exit(1)
	}
	installed := ceBuildToken(string(out))
	if installed == "" {
		return
	}
	recorded := goldenCEBuilds(filepath.Join("testdata", "golden"))
	if len(recorded) == 0 {
		return
	}
	var stale []string
	for name, build := range recorded {
		if build != installed {
			stale = append(stale, fmt.Sprintf("  %s: recorded against %s", name, build))
		}
	}
	if len(stale) == 0 {
		return
	}
	sort.Strings(stale)
	fmt.Printf(`parity: ce toolchain drift — installed build %s does not match the committed goldens:
%s
re-record with: PARITY_RECORD=1 go test ./tests/parity/...
(or: go test ./tests/parity/ -record) and commit the refreshed fixtures with the re-pinned reference.
`,
		installed, strings.Join(stale, "\n"))
	os.Exit(1)
}

func TestCEBuildToken(t *testing.T) {
	out := "ce-agent-kit version 0.8.4\nRelease:  v0.8.4\nBuild:    475-g950650ef\nCommit:   950650ef3741cf662f8df32987746a717166a2e0\nDirty:    false\n"
	if got := ceBuildToken(out); got != "475-g950650ef" {
		t.Errorf("ceBuildToken = %q, want %q", got, "475-g950650ef")
	}
	for _, empty := range []string{"", "no version data", "Build:", "Builder: x", "Build-thing: 404-g771c54cf"} {
		if got := ceBuildToken(empty); got != "" {
			t.Errorf("ceBuildToken(%q) = %q, want empty", empty, got)
		}
	}
}

func TestGoldenCEBuilds(t *testing.T) {
	dir := t.TempDir()
	write := func(name, ce string) {
		t.Helper()
		body := fmt.Sprintf(`{"scenario": %q, "toolchain": {"ce": %q}}`, name, ce)
		if err := os.WriteFile(filepath.Join(dir, name+".json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a", "Build:    475-g950650ef\n")
	write("b", "no build line")
	write("c", "broken json")

	if got := goldenCEBuilds(t.TempDir()); got != nil {
		t.Errorf("goldenCEBuilds(empty dir) = %v, want nil", got)
	}

	builds := goldenCEBuilds(dir)
	if len(builds) != 1 || builds["a.json"] != "475-g950650ef" {
		t.Errorf("goldenCEBuilds = %v, want only a.json with 475-g950650ef", builds)
	}
}

// TestParityReferencePinsAreCoherent holds the whole CE reference pin set
// together: the harness constants, all 11 committed goldens, the scenario
// catalog, the installed ce binary, and the parity contract doc must name
// one CE identity — build 142-g612f6bf8, commit
// 612f6bf80d76563c60464e33802ef7c299940299. A blind re-pin (one site
// changed without re-recording) or a stale golden fails here with the site
// named, instead of surfacing as eleven opaque scenario diffs.
func TestParityReferencePinsAreCoherent(t *testing.T) {
	// 1. The harness pin is self-consistent: a full 40-hex commit whose
	//    short form is embedded in the build token.
	if len(pinnedCECommit) != 40 || strings.TrimLeft(pinnedCECommit, "0123456789abcdef") != "" {
		t.Fatalf("pinnedCECommit %q is not a full 40-hex commit", pinnedCECommit)
	}
	if want := "g" + pinnedCECommit[:8]; !strings.HasSuffix(pinnedCEBuild, want) {
		t.Errorf("pinnedCEBuild %q does not embed the short form of pinnedCECommit (want suffix %q)", pinnedCEBuild, want)
	}

	// In record mode the on-disk goldens are stale by design — this run is
	// rewriting them. The golden-set checks (2-4) re-assert on the next
	// verify run; the harness/doc/installed-identity checks stay active.
	if *recordMode {
		t.Log("record mode: golden pin checks deferred to the verify run that follows the re-record")
		assertPinSurroundings(t)
		return
	}

	// 2. The committed golden set and the scenario catalog are the same 11
	//    scenarios — no golden without a live scenario, no scenario without
	//    a golden.
	catalog := map[string]scenario{}
	for _, sc := range parityScenarios() {
		catalog[sc.name] = sc
	}
	if len(catalog) != 11 {
		t.Errorf("scenario catalog holds %d scenarios, want 11", len(catalog))
	}
	matches, err := filepath.Glob(filepath.Join("testdata", "golden", "*.json"))
	if err != nil {
		t.Fatalf("glob goldens: %v", err)
	}
	if len(matches) != 11 {
		t.Errorf("%d golden files committed, want 11", len(matches))
	}
	goldens := map[string]*goldenFile{}
	for _, path := range matches {
		name := strings.TrimSuffix(filepath.Base(path), ".json")
		data, err := os.ReadFile(path) //nolint:gosec // committed fixture path
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		var g goldenFile
		if err := json.Unmarshal(data, &g); err != nil {
			t.Errorf("%s: parse golden: %v", path, err)
			continue
		}
		goldens[name] = &g
		if _, ok := catalog[name]; !ok {
			t.Errorf("golden %s has no scenario in the catalog", name)
		}
	}
	for name := range catalog {
		if _, ok := goldens[name]; !ok {
			t.Errorf("scenario %s has no committed golden", name)
		}
	}

	// 3. Every golden names the one pinned CE identity: the reference line
	//    plus the toolchain block's Build and Commit tokens.
	for name, g := range goldens {
		if want := "ce-agent-kit master " + pinnedCECommit; g.Reference != want {
			t.Errorf("golden %s reference = %q, want %q", name, g.Reference, want)
		}
		if build := ceBuildToken(g.Toolchain.CE); build != pinnedCEBuild {
			t.Errorf("golden %s toolchain.ce build = %q, want %q", name, build, pinnedCEBuild)
		}
		if commit := ceCommitToken(g.Toolchain.CE); commit != pinnedCECommit {
			t.Errorf("golden %s toolchain.ce commit = %q, want %q", name, commit, pinnedCECommit)
		}
		if len(g.Steps) == 0 {
			t.Errorf("golden %s carries no steps", name)
		}
	}

	// 4. Known-divergent scenarios stay declared on both sides: catalog
	//    contract and golden portContract block must agree.
	for name, sc := range catalog {
		g, ok := goldens[name]
		if !ok {
			continue
		}
		if sc.knownDivergent != g.KnownDivergent {
			t.Errorf("scenario %s: catalog knownDivergent=%v but golden says %v", name, sc.knownDivergent, g.KnownDivergent)
		}
		if sc.knownDivergent {
			if sc.contract == nil || g.PortContract == nil {
				t.Errorf("scenario %s: known-divergent requires a contract on both catalog and golden", name)
				continue
			}
			if sc.contract.Issue != g.PortContract.Issue || sc.contract.ExitCode != g.PortContract.ExitCode || sc.contract.Status != g.PortContract.Status {
				t.Errorf("scenario %s: catalog contract %+v disagrees with golden portContract %+v", name, sc.contract, g.PortContract)
			}
		}
	}

	// 5+6. Installed CE identity and contract-doc pin.
	assertPinSurroundings(t)
}

// assertPinSurroundings checks the CE pin against everything outside the
// golden files: the installed ce binary (fail closed on a failed identity
// probe) and the parity contract doc. It runs in both record and verify
// modes.
func assertPinSurroundings(t *testing.T) {
	t.Helper()
	// The installed ce binary, when one resolves, is exactly the pinned
	// build and commit — the goldens claim to describe it. Fail closed: a
	// failed version probe is an error, never a silent pass. Without any
	// resolvable ce the suite has already exited loudly in TestMain; here
	// the check degrades to a note, never a skip.
	bin := ceBin
	if bin == "" {
		if found, err := exec.LookPath(envOr("CE_BIN", "ce")); err == nil {
			bin = found
		}
	}
	if bin != "" {
		out, err := exec.Command(bin, "version").Output() //nolint:noctx // version probe
		if err != nil {
			t.Errorf("installed ce %s failed to report a version: %v — a failed identity probe must fail closed", bin, err)
		} else {
			version := string(out)
			if build := ceBuildToken(version); build != pinnedCEBuild {
				t.Errorf("installed ce build = %q, want the pinned %q", build, pinnedCEBuild)
			}
			if commit := ceCommitToken(version); commit != pinnedCECommit {
				t.Errorf("installed ce commit = %q, want the pinned %q", commit, pinnedCECommit)
			}
		}
	} else {
		t.Log("no ce binary resolvable; installed-identity cross-check not applicable (see TestMain)")
	}

	// The parity contract doc pins the same identity in prose.
	doc, err := os.ReadFile(filepath.Join("..", "..", "docs", "design", "RUN_LIFECYCLE_PARITY_CONTRACT.md")) //nolint:gosec // committed doc path
	if err != nil {
		t.Fatalf("read contract doc: %v", err)
	}
	text := string(doc)
	if !strings.Contains(text, pinnedCECommit) || !strings.Contains(text, pinnedCEBuild) {
		t.Errorf("contract doc does not pin the CE identity commit %s / build %s", pinnedCECommit, pinnedCEBuild)
	}
}
