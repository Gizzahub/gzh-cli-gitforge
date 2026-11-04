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

// TestCommandEffectGroupParentContract pins the leaf/has_subcommands side of
// the effect table: every group parent is declared (it is a runnable
// help-only command, so it reads read-only), while an unknown subcommand
// under a parent matches no key at all — lookup is exact-key only, never a
// prefix or nearest-ancestor match — so it reads undeclared and a consumer
// fails closed instead of inheriting the parent's or a sibling's effect.
func TestCommandEffectGroupParentContract(t *testing.T) {
	found := map[string]bool{}
	walkAll(fullCommandTree(), func(c *cobra.Command) {
		if c.HasParent() && c.HasSubCommands() && c.IsAvailableCommand() && c.Runnable() && c.Name() != "completion" {
			key := commandKey(c)
			found[key] = true
			if _, _, ok := declaredEffect(c); !ok {
				t.Errorf("group parent %q is runnable but has no effect declaration", c.CommandPath())
			}
			if effect, _, _ := declaredEffect(c); effect != effectReadOnly {
				t.Errorf("group parent %q declares %q; printing help changes nothing, so it must be read-only", c.CommandPath(), effect)
			}
			probe := key + " " + unknownSubcommandProbe
			if _, ok := commandEffects[probe]; ok {
				t.Errorf("effect table contains probe key %q; lookup must be exact-key only", probe)
			}
		}
	})
	if len(found) < len(expectedGroupParents) {
		t.Fatalf("walk found %d runnable group parents (%v); expected at least the %d pinned parents", len(found), found, len(expectedGroupParents))
	}
}

// These commands write git config on some path (a found done-review gap):
// push and handoff end push with --set-upstream, switch and worktree add
// check out a remote-only branch as a tracking branch, sync and workspace
// sync add remotes and record access and integration-branch markers, cleanup
// and integrate run delete local branches together with their
// branch.<name>.* section, and config recommended --apply writes git
// settings (global by default).
func TestGitConfigWritersDeclareConfig(t *testing.T) {
	for _, key := range []string{
		"push", "switch", "handoff end", "sync", "workspace sync",
		"worktree add", "cleanup branch", "cleanup wizard", "integrate run",
		"config recommended",
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
		"config init",
		"config profile create", "config profile delete", "config profile use",
		"forge config generate", "forge setup",
		"workspace add", "workspace init", "workspace generate-config", "workspace sync", "sync",
	} {
		if !slices.Contains(commandEffects[key], mutatesConfig) {
			t.Errorf("%q writes a gz-git config file but does not declare %q: %v", key, mutatesConfig, commandEffects[key])
		}
	}
}

// These commands launch a program that gz-git does not fix: exec runs the
// caller's argv, observe runs the described ce, commit --edit runs $EDITOR,
// and clone and the sync commands run configured hooks.
func TestProgramLaunchersDeclareArbitrary(t *testing.T) {
	for _, key := range []string{
		"exec", "observe", "commit", "clone", "sync", "workspace sync",
	} {
		if !slices.Contains(commandEffects[key], mutatesArbitrary) {
			t.Errorf("%q launches a caller- or repository-chosen program but does not declare %q: %v", key, mutatesArbitrary, commandEffects[key])
		}
	}
}

// The integrate engine runs no repository code: no make target, gate
// runner, or preparation step. Declaring arbitrary here would tell a
// policy consumer that integrating executes the repository.
func TestIntegrateEngineLaunchesNoRepositoryProgram(t *testing.T) {
	for _, key := range []string{"integrate run", "integrate cleanup", "run finish"} {
		if slices.Contains(commandEffects[key], mutatesArbitrary) {
			t.Errorf("%q declares %q, but the integrate engine launches no repository program: %v", key, mutatesArbitrary, commandEffects[key])
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
