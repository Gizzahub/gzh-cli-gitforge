// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/integrate"
)

const integrateDeclareHint = "integrate: declare the device with GIT_WORK_HOST=<name> or git config --global ce.workHost <name> (lowercase letters, digits, single hyphens)\n"

const integrateInvalidHint = "integrate: the declared device name is invalid; fix GIT_WORK_HOST or git config --global ce.workHost to lowercase letters, digits, single hyphens\n"

// isolateIntegrateDevice keeps the developer's real ce.workHost out of the
// test: HOME, XDG_STATE_HOME and the global Git config all point at temp
// paths, and the global config declares globalHost when it is nonempty.
func isolateIntegrateDevice(t *testing.T, envHost, globalHost string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	global := filepath.Join(home, "gitconfig")
	content := ""
	if globalHost != "" {
		content = "[ce]\n\tworkHost = " + globalHost + "\n"
	}
	if err := os.WriteFile(global, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_WORK_HOST", envHost)
	orig := appVersion
	appVersion = "0.10.6"
	t.Cleanup(func() { appVersion = orig })
}

// TestIntegrateRunPrintsProvenanceFirst covers the declared, undeclared, and
// invalid device: the provenance line is the first stderr line, and only an
// unusable device adds the declaration hint.
func TestIntegrateRunPrintsProvenanceFirst(t *testing.T) {
	for _, tc := range []struct {
		name, envHost, globalHost, want string
	}{
		{"env declared", "mst", "", "integrate: run device=mst gz-git=0.10.6\n"},
		{"global declared", "", "mbp", "integrate: run device=mbp gz-git=0.10.6\n"},
		{"undeclared", "", "", "integrate: run device=undeclared gz-git=0.10.6\n" + integrateDeclareHint},
		{"invalid separator", "m;x", "", "integrate: run device=invalid gz-git=0.10.6\n" + integrateInvalidHint},
		{"invalid uppercase", "", "MBP", "integrate: run device=invalid gz-git=0.10.6\n" + integrateInvalidHint},
	} {
		t.Run(tc.name, func(t *testing.T) {
			restore := setIntegrateRunGlobals(t)
			defer restore()
			isolateIntegrateDevice(t, tc.envHost, tc.globalHost)
			t.Chdir(t.TempDir())
			var stdout, stderr bytes.Buffer
			integrateRunCmd.SetOut(&stdout)
			integrateRunCmd.SetErr(&stderr)
			t.Cleanup(func() { integrateRunCmd.SetOut(nil); integrateRunCmd.SetErr(nil) })

			err := runIntegrateRun(integrateRunCmd, nil)
			if got := cliutil.ExitCodeForError(err); got != 2 {
				t.Fatalf("not-a-repo exit = %d, want 2; err=%v", got, err)
			}
			if !strings.HasPrefix(stderr.String(), tc.want) {
				t.Fatalf("stderr = %q, want prefix %q", stderr.String(), tc.want)
			}
			if strings.Count(stderr.String(), "integrate: run device=") != 1 {
				t.Fatalf("provenance printed more than once: %q", stderr.String())
			}
			if strings.Contains(stdout.String(), "integrate: run device=") {
				t.Fatalf("provenance leaked to stdout: %q", stdout.String())
			}
		})
	}
}

// TestIntegrateRunJSONKeepsProvenanceOffStdout keeps the --json protocol one
// parseable document: provenance goes to the notice stream only.
func TestIntegrateRunJSONKeepsProvenanceOffStdout(t *testing.T) {
	restore := setIntegrateRunGlobals(t)
	defer restore()
	origJSON := integrateRunJSON
	integrateRunJSON = true
	t.Cleanup(func() { integrateRunJSON = origJSON })
	isolateIntegrateDevice(t, "mst", "")
	t.Chdir(t.TempDir())
	var stdout, stderr bytes.Buffer
	integrateRunCmd.SetOut(&stdout)
	integrateRunCmd.SetErr(&stderr)
	t.Cleanup(func() { integrateRunCmd.SetOut(nil); integrateRunCmd.SetErr(nil) })

	err := runIntegrateRun(integrateRunCmd, nil)
	if got := cliutil.ExitCodeForError(err); got != 2 {
		t.Fatalf("not-a-repo exit = %d, want 2; err=%v", got, err)
	}
	var doc map[string]any
	if decodeErr := json.Unmarshal(stdout.Bytes(), &doc); decodeErr != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%s", decodeErr, stdout.String())
	}
	if !strings.HasPrefix(stderr.String(), "integrate: run device=mst gz-git=0.10.6\n") {
		t.Fatalf("stderr = %q, want provenance first", stderr.String())
	}
}

// TestIntegrateCleanupPrintsProvenance covers the receipt cleanup, which
// mutates git and prints its refusals on the same stream.
func TestIntegrateCleanupPrintsProvenance(t *testing.T) {
	isolateIntegrateDevice(t, "", "")
	t.Chdir(t.TempDir())
	origRecord, origJSON := integrateCleanupRecord, integrateCleanupJSON
	integrateCleanupRecord, integrateCleanupJSON = "run-1@g1", true
	t.Cleanup(func() {
		integrateCleanupRecord, integrateCleanupJSON = origRecord, origJSON
	})
	var stdout, stderr bytes.Buffer
	integrateCleanupCmd.SetOut(&stdout)
	integrateCleanupCmd.SetErr(&stderr)
	t.Cleanup(func() { integrateCleanupCmd.SetOut(nil); integrateCleanupCmd.SetErr(nil) })

	_ = runIntegrateCleanup(integrateCleanupCmd, nil)
	if want := "integrate: run device=undeclared gz-git=0.10.6\n" + integrateDeclareHint; !strings.HasPrefix(stderr.String(), want) {
		t.Fatalf("stderr = %q, want prefix %q", stderr.String(), want)
	}
	if strings.Contains(stdout.String(), "integrate: ") {
		t.Fatalf("provenance leaked to stdout: %q", stdout.String())
	}
}

// TestIntegrateCleanupRefusesMalformedRecordWithoutProvenance keeps the
// argument refusal ahead of the provenance line (TASK-113): a malformed
// --record exits 2 and the refusal is the first output, before any receipt
// read or device notice.
func TestIntegrateCleanupRefusesMalformedRecordWithoutProvenance(t *testing.T) {
	isolateIntegrateDevice(t, "mst", "")
	t.Chdir(t.TempDir())
	origRecord, origJSON := integrateCleanupRecord, integrateCleanupJSON
	integrateCleanupRecord, integrateCleanupJSON = "run-1", true
	t.Cleanup(func() {
		integrateCleanupRecord, integrateCleanupJSON = origRecord, origJSON
	})
	var stdout, stderr bytes.Buffer
	integrateCleanupCmd.SetOut(&stdout)
	integrateCleanupCmd.SetErr(&stderr)
	t.Cleanup(func() { integrateCleanupCmd.SetOut(nil); integrateCleanupCmd.SetErr(nil) })

	err := runIntegrateCleanup(integrateCleanupCmd, nil)
	var exitErr *cliutil.ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 2 {
		t.Fatalf("malformed record: error = %v, want exit 2", err)
	}
	if !strings.Contains(err.Error(), "--record must be <id>@<generation>") {
		t.Fatalf("malformed record error = %v, want the record refusal", err)
	}
	if strings.Contains(stderr.String(), "integrate: run device=") {
		t.Fatalf("provenance printed before the argument refusal: %q", stderr.String())
	}
	if strings.Contains(stdout.String(), "integrate: ") {
		t.Fatalf("refusal leaked to stdout: %q", stdout.String())
	}
}

// TestIntegrateRunOwnerFlagUsesDeviceGrammar keeps the host part of --owner
// on the device catalog grammar: an uppercase or punctuated name is refused
// before any check, while actor and ids stay opaque.
func TestIntegrateRunOwnerFlagUsesDeviceGrammar(t *testing.T) {
	restore := setIntegrateRunGlobals(t)
	defer restore()
	isolateIntegrateDevice(t, "mbp", "")
	t.Chdir(t.TempDir())
	integrateRunCmd.SetOut(&bytes.Buffer{})
	integrateRunCmd.SetErr(&bytes.Buffer{})
	t.Cleanup(func() { integrateRunCmd.SetOut(nil); integrateRunCmd.SetErr(nil) })
	quiet = true

	for _, owner := range []string{"claude/MBP/agent", "claude/m;x/agent", "claude/a--b/agent"} {
		resetIntegrateRunFlags(t)
		if err := integrateRunCmd.ParseFlags([]string{
			"--record", "Run_42@G.7",
			"--owner", owner,
			"--source", strings.Repeat("a", 40),
		}); err != nil {
			t.Fatalf("ParseFlags(--owner %s) = %v", owner, err)
		}
		err := runIntegrateRun(integrateRunCmd, nil)
		// The host part must fail the device grammar: the refusal is exit 2,
		// names the execution-context invalidity, and names the refused host.
		host := strings.Split(owner, "/")[1]
		if got := cliutil.ExitCodeForError(err); got != 2 || err == nil ||
			!errors.Is(err, integrate.ErrInvalidExecutionContext) ||
			!strings.Contains(err.Error(), "is not a device name") ||
			!strings.Contains(err.Error(), host) {
			t.Fatalf("--owner %q: exit = %d err = %v, want exit 2 device-grammar refusal naming %s", owner, got, err, host)
		}
	}
}
