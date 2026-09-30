// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

// runRootHelp executes the shared root tree and resets the help command's
// flags afterwards; Cobra keeps parsed values between Execute calls.
func runRootHelp(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := fullCommandTree()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(append([]string{"help"}, args...))
	t.Cleanup(func() {
		root.SetOut(nil)
		root.SetErr(nil)
		root.SetArgs(nil)
		for _, c := range root.Commands() {
			if c.Name() != "help" {
				continue
			}
			c.Flags().VisitAll(func(f *pflag.Flag) {
				_ = f.Value.Set(f.DefValue)
				f.Changed = false
			})
		}
	})
	err := executeCommand(root)
	return out.String(), err
}

func TestHelpAllTextListsWholeTreeWithEffects(t *testing.T) {
	out, err := runRootHelp(t, "--all")
	if err != nil {
		t.Fatalf("help --all: %v", err)
	}
	for _, want := range []string{
		"Global Flags:",
		"gz-git integrate bootstrap plan\n",
		"--expires-in duration",
		"gz-git integrate bootstrap apply\n",
		"Effect: mutating (refs, tracking-refs, remote, readiness-contract)",
		"gz-git integrate readiness update plan\n",
		"gz-git status\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("help --all output missing %q", want)
		}
	}
	if strings.Contains(out, effectUndeclared) {
		t.Errorf("help --all shows an undeclared command:\n%s", out)
	}
}

func TestHelpAllScopesToSubtree(t *testing.T) {
	out, err := runRootHelp(t, "--all", "integrate", "bootstrap")
	if err != nil {
		t.Fatalf("help --all integrate bootstrap: %v", err)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "gz-git ") && !strings.HasPrefix(line, "gz-git integrate bootstrap") {
			t.Errorf("subtree output leaked %q", line)
		}
	}
	if !strings.Contains(out, "gz-git integrate bootstrap plan\n  Create a read-only, expiring bootstrap plan\n  Usage:  gz-git integrate bootstrap plan [flags]\n  Effect: mutating (tracking-refs)\n") {
		t.Errorf("bootstrap plan block not rendered as expected:\n%s", out)
	}
}

func TestHelpAllJSONDeclaresEveryRunnableCommand(t *testing.T) {
	out, err := runRootHelp(t, "--all", "--format", "json")
	if err != nil {
		t.Fatalf("help --all --format json: %v", err)
	}
	var ref commandReference
	if err := json.Unmarshal([]byte(out), &ref); err != nil {
		t.Fatalf("decode reference: %v\n%s", err, out)
	}
	if ref.Schema != commandReferenceSchema {
		t.Fatalf("schema = %q, want %q", ref.Schema, commandReferenceSchema)
	}
	byPath := map[string]referenceCommand{}
	for _, c := range ref.Commands {
		byPath[c.Path] = c
		if c.Runnable && c.Effect != effectReadOnly && c.Effect != effectMutating {
			t.Errorf("%s effect = %q", c.Path, c.Effect)
		}
		if !c.Runnable && c.Effect != "" {
			t.Errorf("group %s must not carry an effect, got %q", c.Path, c.Effect)
		}
	}
	for path, want := range map[string]string{
		"gz-git help":                      effectReadOnly,
		"gz-git stash list":                effectReadOnly,
		"gz-git integrate bootstrap plan":  effectMutating,
		"gz-git integrate bootstrap apply": effectMutating,
	} {
		if got := byPath[path].Effect; got != want {
			t.Errorf("%s effect = %q, want %q", path, got, want)
		}
	}
	apply := byPath["gz-git integrate bootstrap apply"]
	if !slices.Contains(apply.Mutates, mutatesReadinessContract) {
		t.Errorf("bootstrap apply mutates = %v, want readiness-contract", apply.Mutates)
	}
	if !hasFlag(byPath["gz-git integrate bootstrap plan"].Flags, "output") {
		t.Errorf("bootstrap plan flags missing --output")
	}
	if len(ref.GlobalFlags) == 0 {
		t.Errorf("global flags missing")
	}
}

func hasFlag(flags []referenceFlag, name string) bool {
	for _, f := range flags {
		if f.Name == name {
			return true
		}
	}
	return false
}

func TestHelpRejectsAmbiguousFlagUse(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{name: "format without all", args: []string{"--format", "json"}, want: "--format requires --all"},
		{name: "unknown format", args: []string{"--all", "--format", "yaml"}, want: "unsupported --format"},
		{name: "unknown topic", args: []string{"--all", "no-such-command"}, want: "unknown help topic"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := runRootHelp(t, tc.args...)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestPlainHelpShowsEffect(t *testing.T) {
	out, err := runRootHelp(t, "integrate", "readiness", "update", "apply")
	if err != nil {
		t.Fatalf("help integrate readiness update apply: %v", err)
	}
	if !strings.Contains(out, "Effect: mutating (refs, tracking-refs, remote, readiness-contract)") {
		t.Errorf("plain help missing effect line:\n%s", out)
	}
}
