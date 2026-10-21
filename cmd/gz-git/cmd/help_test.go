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
		"gz-git integrate legacy-cleanup plan\n",
		"--released-artifact string",
		"gz-git integrate legacy-cleanup apply\n",
		"Effect: mutating (filesystem, refs, tracking-refs, remote, config)",
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
	out, err := runRootHelp(t, "--all", "integrate", "legacy-cleanup")
	if err != nil {
		t.Fatalf("help --all integrate legacy-cleanup: %v", err)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "gz-git ") && !strings.HasPrefix(line, "gz-git integrate legacy-cleanup") {
			t.Errorf("subtree output leaked %q", line)
		}
	}
	if !strings.Contains(out, "gz-git integrate legacy-cleanup plan\n  Create a read-only prospective legacy cleanup plan\n  Usage:  gz-git integrate legacy-cleanup plan [flags]\n  Effect: mutating (filesystem)\n") {
		t.Errorf("legacy-cleanup plan block not rendered as expected:\n%s", out)
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
		"gz-git help":                           effectReadOnly,
		"gz-git stash list":                     effectReadOnly,
		"gz-git integrate legacy-cleanup plan":  effectMutating,
		"gz-git integrate legacy-cleanup apply": effectMutating,
	} {
		if got := byPath[path].Effect; got != want {
			t.Errorf("%s effect = %q, want %q", path, got, want)
		}
	}
	apply := byPath["gz-git integrate legacy-cleanup apply"]
	if !slices.Contains(apply.Mutates, mutatesRefs) {
		t.Errorf("legacy-cleanup apply mutates = %v, want refs", apply.Mutates)
	}
	if !hasFlag(byPath["gz-git integrate legacy-cleanup plan"].Flags, "output") {
		t.Errorf("legacy-cleanup plan flags missing --output")
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
	out, err := runRootHelp(t, "integrate", "legacy-cleanup", "apply")
	if err != nil {
		t.Fatalf("help integrate legacy-cleanup apply: %v", err)
	}
	if !strings.Contains(out, "Effect: mutating (filesystem, refs, tracking-refs, remote, config)") {
		t.Errorf("plain help missing effect line:\n%s", out)
	}
}
