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
// (docs/design/RUN_LIFECYCLE_PARITY_CONTRACT.md). A ce that will not
// probe, or goldens that carry no build, are left to the scenarios.
func guardGoldenToolchain() {
	out, err := exec.Command(ceBin, "version").Output() //nolint:noctx // version probe
	if err != nil {
		return
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
