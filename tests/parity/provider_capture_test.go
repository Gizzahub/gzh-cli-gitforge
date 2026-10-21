//go:build parity

package parity

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestParityReferencePinsMatchCapture(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "golden", "*.json"))
	must(t, err)
	if len(paths) != 12 {
		t.Fatalf("reference captures = %d, want 12", len(paths))
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		must(t, err)
		var capture goldenFile
		must(t, json.Unmarshal(data, &capture))
		if capture.Reference != "ce-agent-kit master "+pinnedCECommit || ceBuildToken(capture.Toolchain.CE) != pinnedCECommit {
			t.Errorf("%s: reference/build revision does not match %s", path, pinnedCECommit)
		}
		if capture.Toolchain.GzGit != gzGitVersionPlaceholder {
			t.Errorf("%s: checkout build identity leaked into the golden: %q", path, capture.Toolchain.GzGit)
		}
		if capture.Scenario == "doctor-active" {
			if len(capture.Steps) != 1 {
				t.Fatalf("%s: expected one doctor observation", path)
			}
			var report struct {
				Provider struct {
					Executable string `json:"executable"`
				} `json:"provider"`
			}
			must(t, json.Unmarshal([]byte(capture.Steps[0].Stdout), &report))
			if report.Provider.Executable != "<SANDBOX>/bin/gz-git" {
				t.Errorf("%s: missing fixture-local provider evidence", path)
			}
		}
	}
	contract, err := os.ReadFile(filepath.Join("..", "..", "docs", "design", "RUN_LIFECYCLE_PARITY_CONTRACT.md"))
	must(t, err)
	// Historical references may contain the new pin too. Require the active
	// declaration and toolchain row, rather than any occurrence in the file.
	reference := regexp.MustCompile("(?s)The current behavioral reference is \\*\\*CE source `ce-agent-kit`\\s*`([0-9a-f]{40})`\\*\\*").FindSubmatch(contract)
	if len(reference) != 2 || string(reference[1]) != pinnedCECommit {
		t.Fatal("parity contract current reference does not match the captured CE revision")
	}
	toolchainRow := regexp.MustCompile("(?m)^\\| `ce`\\s*\\|[^\\n]+").Find(contract)
	if !bytes.Contains(toolchainRow, []byte("`"+pinnedCECommit[:8]+"`")) {
		t.Fatal("parity contract CE toolchain row does not match the captured CE revision")
	}
}

func TestCheckoutProviderVersionIdentity(t *testing.T) {
	for _, version := range []string{
		"gz-git version 0.10.1",
		"gz-git version 0.10.1+dirty",
		"gz-git version 0.9.5-0.20261015160834-92afce325e53+dirty",
		"gz-git version 0.9.5-0.20261015160834-92afce325e53",
	} {
		if got := canonicalGZGitVersion(version); got != gzGitVersionPlaceholder {
			t.Errorf("build identity was not folded: %q -> %q", version, got)
		}
	}
	for _, text := range []string{"gz-git version missing", "gz-git version 0.10.1+dirty+extra", "error: gz-git version 0.10.1"} {
		if got := canonicalGZGitVersion(text); got != text {
			t.Errorf("non-version protocol text changed: %q -> %q", text, got)
		}
	}
}

// Exercise CE's real provider discovery with a hostile ambient executable.
// Matching a normalized version alone cannot establish which binary CE used.
func TestParityReferenceUsesCheckoutProvider(t *testing.T) {
	if targetMode {
		t.Skip("CE reference-provider contract; run without PARITY_TARGET/GZ_GIT_BIN")
	}
	ambient := t.TempDir()
	marker := filepath.Join(ambient, "gz-git.invoked")
	trap := "#!/bin/sh\nprintf invoked > \"$0.invoked\"\nexit 91\n"
	must(t, os.WriteFile(filepath.Join(ambient, "gz-git"), []byte(trap), 0o700))
	t.Setenv("PATH", ambient+string(os.PathListSeparator)+os.Getenv("PATH"))
	sb := newSandbox(t, scenario{name: "provider-capture"})
	out, errOut, code := sb.run(step{args: []string{"task", "run-doctor", "--json"}, dir: "repo"})
	if code != 0 {
		t.Fatalf("CE provider discovery exit %d: %s\n%s", code, out, errOut)
	}
	var result struct {
		Status   string `json:"status"`
		Provider struct {
			Executable   string          `json:"executable"`
			Present      bool            `json:"present"`
			Capabilities map[string]bool `json:"capabilities"`
		} `json:"provider"`
	}
	must(t, json.Unmarshal([]byte(out), &result))
	want := filepath.Join(sb.root, "bin", "gz-git")
	if result.Status != "ACTIVE" || !result.Provider.Present || result.Provider.Executable != want ||
		!result.Provider.Capabilities["integrate-check"] || !result.Provider.Capabilities["integrate-run"] {
		t.Fatalf("CE did not discover the checkout provider %q: %s", want, out)
	}
	resolved, err := filepath.EvalSymlinks(result.Provider.Executable)
	must(t, err)
	built, err := filepath.EvalSymlinks(gzGitBin)
	must(t, err)
	if resolved != built {
		t.Fatalf("CE provider resolves to %q, want checkout build %q", resolved, built)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("ambient provider was invoked or marker inspection failed: %v", err)
	}
	// The version probe must also bypass the ambient trap.
	_ = probeToolchain(t)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("toolchain probe used ambient provider: %v", err)
	}
}
