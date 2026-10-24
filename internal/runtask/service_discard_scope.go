// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

// Discard scope plumbing: what a recorded discard intent promises about the
// remote, and the command shaping that keeps a local-only discard from
// touching the network.

package runtask

import (
	"context"
)

// validDiscardIntentScope judges whether a recorded discard intent's scope
// and remote evidence are internally consistent, so a retry can trust the
// scope it is asked to match. An intent that claims a verified remote removal
// is never valid: proof belongs to the terminal receipt, not the intent
// recorded before removal. The empty scope is valid only with no remote
// answer at all; local-only is valid only with the explicit "unchecked"
// answer; anything else is a scope this runtime never wrote.
func validDiscardIntentScope(receipt TaskReceipt) bool {
	if receipt.RemoteBranchRemoved {
		return false
	}
	switch receipt.DiscardScope {
	case "":
		return receipt.RemoteBranchStatus == ""
	case "local-only":
		return receipt.RemoteBranchStatus == "unchecked"
	default:
		return false
	}
}

// discardGitCommand is the git probe command for discard evidence. A
// local-only discard shapes every git call with the offline-observation env,
// so no evidence step can turn a lazy fetch into a network read.
func (s *Service) discardGitCommand(ctx context.Context, dir string, localOnly bool, args ...string) (CommandResult, error) {
	probeCtx := ctx
	if localOnly {
		probeCtx = offlineObservationContext(ctx)
	}
	return s.command(probeCtx, dir, "git", args...)
}

// discardInventory reads the Worktrunk inventory a discard verifies against.
// A local-only discard reads it through the offline Worktrunk command, so the
// check runs without the operator's Worktrunk config or any network.
func (s *Service) discardInventory(ctx context.Context, execution TaskExecution, localOnly bool) (worktrunkInventory, error) {
	if !localOnly {
		return s.worktrunkList(ctx)
	}
	result, err := s.discardWorktrunkCommand(ctx, execution, true,
		"--config-set", "list.json-schema = 2", "--config-set", "list.full = false",
		"--config-set", "list.summary = false", "--config-set", "list.branches = false",
		"--config-set", "list.remotes = false", "list", "--format=json")
	return decodeWorktrunkInventory(result, err)
}

// discardWorktrunkCommand runs the Worktrunk mutation or list a discard
// needs. LocalOnly routes through the offline Worktrunk command against the
// recorded source branch, and inserts --delete-branch into a remove so the
// local branch is dropped together with its worktree -- which is also why
// the flag goes before the path argument, matching Worktrunk's argument
// order rather than being appended where it would be read as the path.
func (s *Service) discardWorktrunkCommand(ctx context.Context, execution TaskExecution, localOnly bool, args ...string) (CommandResult, error) {
	if localOnly {
		if len(args) > 0 && args[0] == "remove" {
			insertAt := len(args) - 1
			args = append(append([]string(nil), args[:insertAt]...), "--delete-branch", args[insertAt])
		}
		return s.offlineWorktrunkCommand(ctx, execution.Source, args...)
	}
	return s.runner.Run(ctx, CommandRequest{Command: "wt", Args: args, WorkDir: s.root})
}
