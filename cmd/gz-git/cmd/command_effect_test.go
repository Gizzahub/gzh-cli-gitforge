// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"slices"
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

// These commands write .git/config on some path (a found done-review gap):
// handoff end pushes with --set-upstream, switch and worktree add check out a
// remote-only branch as a tracking branch, workspace sync adds remotes and
// records access and integration-branch markers, and cleanup and integrate
// run delete local branches together with their branch.<name>.* section.
func TestGitConfigWritersDeclareConfig(t *testing.T) {
	for _, key := range []string{
		"push", "switch", "handoff end", "sync", "workspace sync",
		"worktree add", "cleanup branch", "cleanup wizard", "integrate run",
	} {
		if !slices.Contains(commandEffects[key], mutatesConfig) {
			t.Errorf("%q writes git config but does not declare %q: %v", key, mutatesConfig, commandEffects[key])
		}
	}
}

// These commands write a gz-git config file (workspace, profile or global
// config) on some path. forge setup does so only when the wizard is asked to
// save its answers, which is still a path the command can take.
func TestGzGitConfigFileWritersDeclareConfig(t *testing.T) {
	for _, key := range []string{
		"config init", "config recommended",
		"config profile create", "config profile delete", "config profile use",
		"forge config generate", "forge setup",
		"workspace add", "workspace init", "workspace generate-config", "workspace sync", "sync",
	} {
		if !slices.Contains(commandEffects[key], mutatesConfig) {
			t.Errorf("%q writes a gz-git config file but does not declare %q: %v", key, mutatesConfig, commandEffects[key])
		}
	}
}

// run finish integrates through the same engine as integrate run, in-process,
// so whatever integrate run can change, run finish can change too.
func TestRunFinishCoversIntegrateRun(t *testing.T) {
	for _, target := range commandEffects["integrate run"] {
		if !slices.Contains(commandEffects["run finish"], target) {
			t.Errorf("run finish calls the integrate run engine but does not declare %q", target)
		}
	}
	if !slices.Contains(commandEffects["run finish"], mutatesRunState) {
		t.Errorf("run finish records the run outcome but does not declare %q", mutatesRunState)
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
