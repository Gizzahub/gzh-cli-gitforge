// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestWorkspaceSyncJSONContract pins the machine-output contract of
// `workspace sync --format json`: whatever the planning outcome (empty or
// non-empty planning, explicit config or auto-discovery, dry-run or normal
// run), machine stdout must carry exactly one SyncResultJSON document and
// nothing else — no auto-discovery notice, no trailing sentence. Fixtures are
// local-only (path URLs to a bare origin); no network access happens.
func TestWorkspaceSyncJSONContract(t *testing.T) {
	for _, tc := range []struct {
		name          string
		empty         bool
		autoDiscovery bool
		dryRun        bool
	}{
		{name: "empty/explicit-config/dry-run", empty: true, dryRun: true},
		{name: "empty/explicit-config/normal", empty: true},
		{name: "empty/auto-discovery/dry-run", empty: true, autoDiscovery: true, dryRun: true},
		{name: "empty/auto-discovery/normal", empty: true, autoDiscovery: true},
		{name: "non-empty/explicit-config/dry-run", dryRun: true},
		{name: "non-empty/explicit-config/normal"},
		{name: "non-empty/auto-discovery/dry-run", autoDiscovery: true, dryRun: true},
		{name: "non-empty/auto-discovery/normal", autoDiscovery: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			workspace, configPath := syncJSONContractFixture(t, !tc.empty)
			confineSyncContractEnv(t)

			// Always run from the workspace directory: relative `path:` entries
			// in the config resolve against the process cwd, matching how the
			// command is used for real. Auto-discovery additionally omits -c.
			t.Chdir(workspace)
			args := []string{"--format", "json"}
			if !tc.autoDiscovery {
				args = append(args, "-c", configPath)
			}
			if tc.dryRun {
				args = append(args, "--dry-run")
			}

			var cobraOut, cobraErr bytes.Buffer
			cmd := CommandFactory{}.newSyncCmd()
			cmd.SetOut(&cobraOut)
			cmd.SetErr(&cobraErr)
			cmd.SetArgs(args)

			stray := captureProcessStdout(t, func() {
				if err := cmd.Execute(); err != nil {
					t.Errorf("workspace sync failed: %v\nstdout=%s stderr=%s", err, cobraOut.String(), cobraErr.String())
				}
			})

			assertSingleJSONObject(t, cobraOut.String(), func(parsed SyncResultJSON) {
				assertSyncJSONSemantics(t, tc.empty, parsed)
			})

			if stray != "" {
				t.Errorf("direct stdout writes leaked past the cobra writer in machine mode: %q", stray)
			}
			if tc.autoDiscovery && strings.Contains(cobraOut.String(), "Using config:") {
				t.Errorf("auto-discovery notice leaked to machine stdout: %q", cobraOut.String())
			}
		})
	}

	// Human output must stay as before: the empty sentence survives on stdout
	// for the default format.
	t.Run("human-default-format-keeps-empty-sentence", func(t *testing.T) {
		_, configPath := syncJSONContractFixture(t, false)
		confineSyncContractEnv(t)

		var cobraOut, cobraErr bytes.Buffer
		cmd := CommandFactory{}.newSyncCmd()
		cmd.SetOut(&cobraOut)
		cmd.SetErr(&cobraErr)
		cmd.SetArgs([]string{"-c", configPath, "--dry-run"})

		stray := captureProcessStdout(t, func() {
			if err := cmd.Execute(); err != nil {
				t.Errorf("workspace sync failed: %v\nstdout=%s stderr=%s", err, cobraOut.String(), cobraErr.String())
			}
		})

		combined := cobraOut.String() + stray
		if !strings.Contains(combined, "No repositories found to sync.") {
			t.Errorf("human empty sentence missing, stdout=%q stray=%q", cobraOut.String(), stray)
		}
	})
}

// assertSyncJSONSemantics asserts the existing totals/results contract for the
// empty and non-empty planning outcomes against the SyncResultJSON schema.
func assertSyncJSONSemantics(t *testing.T, empty bool, parsed SyncResultJSON) {
	t.Helper()
	if empty {
		if parsed.Total != 0 || parsed.Succeeded != 0 || parsed.Failed != 0 {
			t.Errorf("empty run totals = %d/%d/%d, want 0/0/0", parsed.Total, parsed.Succeeded, parsed.Failed)
		}
		// Empty-collection semantics: "repositories" must be an empty JSON
		// array, never null or a missing field.
		if parsed.Repos == nil {
			t.Error(`"repositories" must decode as [], not null`)
		}
		if len(parsed.Repos) != 0 {
			t.Errorf("empty run repositories = %+v, want none", parsed.Repos)
		}
		return
	}

	if parsed.Total != 1 || parsed.Succeeded != 1 || parsed.Failed != 0 {
		t.Errorf("non-empty run totals = %d/%d/%d, want 1/1/0", parsed.Total, parsed.Succeeded, parsed.Failed)
	}
	if len(parsed.Repos) != 1 {
		t.Fatalf("non-empty run repositories = %+v, want exactly one entry", parsed.Repos)
	}
	repo := parsed.Repos[0]
	if repo.Name != "fresh" {
		t.Errorf("repository name = %q, want %q", repo.Name, "fresh")
	}
	if repo.Action != "update" {
		t.Errorf("repository action = %q, want %q", repo.Action, "update")
	}
	if repo.Status != "success" {
		t.Errorf("repository status = %q, want %q", repo.Status, "success")
	}
}

// assertSingleJSONObject decodes exactly one JSON object from stdout: the
// first Decode must succeed and expose the existing SyncResultJSON field
// names, the second Decode must return io.EOF (no trailing notices, no second
// object). Typed semantics run through the supplied check.
func assertSingleJSONObject(t *testing.T, stdout string, check func(SyncResultJSON)) {
	t.Helper()

	dec := json.NewDecoder(strings.NewReader(stdout))
	var fields map[string]any
	if err := dec.Decode(&fields); err != nil {
		t.Fatalf("machine stdout is not one JSON object: %v\nstdout=%q", err, stdout)
	}
	for _, key := range []string{"total", "succeeded", "failed", "duration_ms", "repositories"} {
		if _, ok := fields[key]; !ok {
			t.Errorf("JSON result missing existing field %q: %q", key, stdout)
		}
	}

	// A second Decode must hit clean EOF: no trailing notice, no second object.
	var extra map[string]any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		t.Errorf("second Decode after the JSON object = %v, want io.EOF (stdout=%q)", err, stdout)
	}

	typed := json.NewDecoder(strings.NewReader(stdout))
	var parsed SyncResultJSON
	if err := typed.Decode(&parsed); err != nil {
		t.Fatalf("decoding SyncResultJSON failed: %v\nstdout=%q", err, stdout)
	}
	check(parsed)
}

// captureProcessStdout runs fn while capturing writes to the process stdout,
// which is where fmt.Print* output lands when it bypasses the cobra writer.
func captureProcessStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = w
	fn()
	os.Stdout = original
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// confineSyncContractEnv pins HOME, XDG state, and git global config to a
// throwaway directory so auto-discovery and config-layer resolution cannot
// read the developer's machine state.
func confineSyncContractEnv(t *testing.T) {
	t.Helper()
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, "gitconfig"))
}

// syncJSONContractGit runs git with explicit argv for the local fixtures used
// by TestWorkspaceSyncJSONContract. No network: every remote is a local path.
func syncJSONContractGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

// syncJSONContractFixture builds a local-only workspace: a seed repository,
// a bare origin cloned from it, and a workspace directory holding a
// .gz-git.yaml that references the origin. With withRepo=false the config's
// repository list is empty, yielding the empty-planning case.
func syncJSONContractFixture(t *testing.T, withRepo bool) (workspace, configPath string) {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join(root, "source")
	origin := filepath.Join(root, "origin.git")
	workspace = filepath.Join(root, "workspace")

	reposEntries := "repositories: []\n"
	if withRepo {
		syncJSONContractGit(t, "", "init", "--initial-branch=master", source)
		syncJSONContractGit(t, source, "config", "user.email", "test@example.com")
		syncJSONContractGit(t, source, "config", "user.name", "Test")
		syncJSONContractGit(t, source, "config", "commit.gpgsign", "false")
		if err := os.WriteFile(filepath.Join(source, "README"), []byte("fixture\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		// Declaring the integration branch in the seeded repository keeps the
		// normal (non-dry-run) sync's participation reconcile side-effect-free.
		if err := os.WriteFile(filepath.Join(source, DefaultConfigFile),
			[]byte("branch:\n  integrationBranch: [master]\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		syncJSONContractGit(t, source, "add", "README", DefaultConfigFile)
		syncJSONContractGit(t, source, "commit", "-m", "seed")
		syncJSONContractGit(t, "", "clone", "--bare", source, origin)
		reposEntries = fmt.Sprintf("repositories:\n  - name: fresh\n    url: %q\n    path: fresh\n", origin)
	}

	if err := os.MkdirAll(workspace, 0o750); err != nil {
		t.Fatal(err)
	}
	configPath = filepath.Join(workspace, DefaultConfigFile)
	cfg := "version: 1\nkind: repositories\nstrategy: reset\n" + reposEntries
	if err := os.WriteFile(configPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	return workspace, configPath
}
