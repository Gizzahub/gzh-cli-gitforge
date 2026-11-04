// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
)

// runIntegrateParent executes the root tree with an argument list whose first
// word addresses the integrate parent, the way Execute does — groups, usage
// template, and the scan-depth reset applied — and returns what the tree
// printed.
func runIntegrateParent(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := fullCommandTree()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(append([]string{"integrate"}, args...))
	t.Cleanup(func() {
		root.SetOut(nil)
		root.SetErr(nil)
		root.SetArgs(nil)
	})
	err := executeCommand(root)
	return out.String(), err
}

// The parent must refuse an unknown or retired subcommand name with an
// explicit error and a non-zero exit, and keep answering a bare invocation
// with the Quick Start and exit 0. Cobra used to swallow any argument here:
// the parent was not runnable, so Execute returned flag.ErrHelp before the
// NoArgs check ran, and a stale caller read success where the surface was
// removed.
func TestIntegrateRefusesUnknownSubcommands(t *testing.T) {
	for name, args := range map[string][]string{
		"retired subcommand":  {"check"},
		"nonsense subcommand": {"does-not-exist"},
	} {
		t.Run(name, func(t *testing.T) {
			out, err := runIntegrateParent(t, args...)
			if err == nil {
				t.Fatalf("unknown subcommand %q: error = nil, want refusal", args[0])
			}
			if code := cliutil.ExitCodeForError(err); code == cliutil.ExitOK {
				t.Errorf("unknown subcommand %q: exit code %d, want non-zero", args[0], code)
			}
			if !strings.Contains(err.Error(), "unknown command") || !strings.Contains(err.Error(), args[0]) {
				t.Errorf("unknown subcommand %q: error = %v, want an unknown-command message naming it", args[0], err)
			}
			if strings.Contains(out, "Quick Start:") {
				t.Errorf("unknown subcommand %q printed the Quick Start:\n%s", args[0], out)
			}
		})
	}

	t.Run("bare invocation keeps the Quick Start", func(t *testing.T) {
		out, err := runIntegrateParent(t)
		if err != nil {
			t.Fatalf("bare integrate parent: %v", err)
		}
		if !strings.Contains(out, "Quick Start:") {
			t.Errorf("bare integrate parent output missing the Quick Start:\n%s", out)
		}
	})
}
