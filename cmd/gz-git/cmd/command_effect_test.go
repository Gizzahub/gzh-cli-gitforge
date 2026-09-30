// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

// fullCommandTree prepares rootCmd the way Execute does — groups, usage
// template, and the commands Cobra only adds inside Execute (help,
// completion) — so a walk sees what the binary exposes.
func fullCommandTree() *cobra.Command {
	if !rootCmd.ContainsGroup("core") {
		setCommandGroups(rootCmd)
	}
	applyUsageTemplateRecursive(rootCmd, buildUsageTemplate())
	rootCmd.InitDefaultHelpCmd()
	rootCmd.InitDefaultCompletionCmd()
	return rootCmd
}

// walkAll visits every command, hidden ones included, except the help
// children Cobra adds below the root (they are not reachable as commands).
func walkAll(c *cobra.Command, visit func(*cobra.Command)) {
	visit(c)
	for _, child := range c.Commands() {
		if child.Name() == "help" && c.HasParent() {
			continue
		}
		walkAll(child, visit)
	}
}

func TestEveryRunnableCommandDeclaresEffect(t *testing.T) {
	seen := map[string]bool{}
	walkAll(fullCommandTree(), func(c *cobra.Command) {
		if !c.Runnable() {
			return
		}
		key := commandKey(c)
		seen[key] = true
		if _, _, ok := declaredEffect(c); !ok {
			t.Errorf("runnable command %q has no effect declaration in commandEffects", c.CommandPath())
		}
	})
	for key := range commandEffects {
		if !seen[key] {
			t.Errorf("commandEffects entry %q names no runnable command", key)
		}
	}
}

func TestEffectDeclarationsUseKnownTargets(t *testing.T) {
	for key, targets := range commandEffects {
		dup := map[string]bool{}
		for _, target := range targets {
			if !knownMutationTargets[target] {
				t.Errorf("%q declares unknown mutation target %q", key, target)
			}
			if dup[target] {
				t.Errorf("%q declares %q twice", key, target)
			}
			dup[target] = true
		}
	}
}

// The readiness contract is what a policy consumer keys on to keep contract
// changes with a person; a wrong declaration here silently widens that policy.
func TestReadinessContractDeclarations(t *testing.T) {
	want := map[string]bool{
		"integrate bootstrap apply":        true,
		"integrate readiness update apply": true,
	}
	for key, targets := range commandEffects {
		has := false
		for _, target := range targets {
			if target == mutatesReadinessContract {
				has = true
			}
		}
		if has != want[key] {
			t.Errorf("%q readiness-contract declared = %v, want %v", key, has, want[key])
		}
	}
	for _, key := range []string{"integrate bootstrap plan", "integrate readiness update plan"} {
		if targets, ok := commandEffects[key]; !ok || len(targets) != 1 || targets[0] != mutatesTrackingRefs {
			t.Errorf("%q must declare only tracking-refs, got %v (declared=%v)", key, targets, ok)
		}
	}
}
