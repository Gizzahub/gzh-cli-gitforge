// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// unknownSubcommandProbe is a name no command, alias, or flag uses, so the
// only way to "handle" it is the parent's refusal.
const unknownSubcommandProbe = "__no_such_subcommand__"

// expectedGroupParents pins the tree re-enumeration: every group parent the
// binary registers today. A new parent fails here until it is added, and the
// sweep below then holds it to the same contract.
var expectedGroupParents = map[string]bool{
	"branch":                       true,
	"cleanup":                      true,
	"config":                       true,
	"config profile":               true,
	"config token":                 true,
	"conflict":                     true,
	"forge":                        true,
	"forge bootstrap":              true,
	"forge config":                 true,
	"handoff":                      true,
	"history":                      true,
	"integrate":                    true,
	"integrate provision":          true,
	"integrate provision siblings": true,
	"pr":                           true,
	"run":                          true,
	"stash":                        true,
	"tag":                          true,
	"worktree":                     true,
	"workspace":                    true,
}

// groupParentArgs is the invocation path of cmd below the root.
func groupParentArgs(cmd *cobra.Command) []string {
	path := cmd.CommandPath()
	if i := strings.IndexByte(path, ' '); i >= 0 {
		return strings.Fields(path[i+1:])
	}
	return nil
}

// TestGroupParentsRefuseUnknownSubcommands walks the real command tree and
// holds every group parent to the refusal contract: an unknown or retired
// subcommand argument exits 1 with an explicit unknown-command error, while
// a bare parent invocation still prints help and exits 0. Cobra only checks
// Args after its runnable gate, so a non-runnable parent would swallow any
// argument as help; a parent passes here only when its RunE makes NoArgs
// effective. Cobra's built-in completion command is not a gz-git
// registration and is out of scope.
func TestGroupParentsRefuseUnknownSubcommands(t *testing.T) {
	root := fullCommandTree()

	seen := map[string]bool{}
	walkAll(root, func(c *cobra.Command) {
		if c == root || !c.HasSubCommands() || !c.IsAvailableCommand() {
			return
		}
		if c.Name() == "completion" {
			return
		}
		seen[commandKey(c)] = true
	})
	for key := range expectedGroupParents {
		if !seen[key] {
			t.Errorf("expected group parent %q was not found in the command tree", key)
		}
	}

	var out, errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	defer func() {
		root.SetArgs(nil)
		root.SetOut(nil)
		root.SetErr(nil)
	}()

	// The sweep runs over the walked set, not the pinned list, so a parent
	// registered later is held to the same contract without a test change;
	// the pinned list above only guards the enumeration itself.
	for key := range seen {
		t.Run(key, func(t *testing.T) {
			args := strings.Fields(key)

			root.SetArgs(append(append([]string{}, args...), unknownSubcommandProbe))
			out.Reset()
			errOut.Reset()
			err := root.Execute()
			if err == nil {
				t.Fatalf("%s %s exited 0; an unknown subcommand must be refused", key, unknownSubcommandProbe)
			}
			if !strings.Contains(err.Error(), "unknown command") || !strings.Contains(err.Error(), unknownSubcommandProbe) {
				t.Fatalf("%s %s error = %v; want an explicit unknown-command error naming the probe", key, unknownSubcommandProbe, err)
			}

			root.SetArgs(args)
			out.Reset()
			errOut.Reset()
			if err := root.Execute(); err != nil {
				t.Fatalf("bare %s error = %v; a bare parent must keep printing help and exit 0", key, err)
			}
			if !strings.Contains(out.String(), "Usage:") {
				t.Fatalf("bare %s printed no usage help; output was %q", key, out.String())
			}
		})
	}
}
