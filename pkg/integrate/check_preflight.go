// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"fmt"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

// preflightTargetRef determines the destination using only local Git reads,
// before any fetch. Check binds the post-fetch plan to it, and Run's
// pre-mutation outcome names it. The returned identity is a full destination
// ref, independent of local remote-tracking spellings.
type targetPreflight struct {
	branch, remote, targetRef string
}

func preflightTargetRef(ctx context.Context, g gitRepo, exec *gitcmd.Executor, opts CheckOptions, controller *controllerBinding) (targetPreflight, error) {
	branch := strings.TrimSpace(opts.Branch)
	if branch == "" {
		var err error
		branch, err = g.currentBranch(ctx)
		if err != nil {
			return targetPreflight{}, err
		}
	}
	if err := gitcmd.SanitizeBranchName(branch); err != nil {
		return targetPreflight{}, fmt.Errorf("invalid branch: %w", err)
	}
	remote := ""
	if controller != nil {
		remote = controller.Remote
	} else {
		var err error
		remote, err = detectRemote(ctx, g, branch)
		if err != nil {
			return targetPreflight{}, err
		}
	}
	target := strings.TrimSpace(opts.Target)
	if target == "" {
		var integration Resolution
		if controller != nil {
			integration = Resolution{Participates: true, Name: controller.Integration[0]}
		} else {
			var err error
			integration, err = ResolveIntegrationBranch(ctx, exec, g.dir, opts.IntegrationConfig)
			if err != nil {
				return targetPreflight{}, err
			}
		}
		if integration.Participates {
			target = integration.Name
		} else {
			if remote == "" {
				return targetPreflight{}, fmt.Errorf("no integration branch; pass --target <default> --direct-to-default")
			}
			defaultRef, ok, err := g.symbolicRef(ctx, "refs/remotes/"+remote+"/HEAD")
			if err != nil {
				return targetPreflight{}, err
			}
			if !ok {
				return targetPreflight{}, fmt.Errorf("no local ref for %s/HEAD — pass --target or fetch before integrating", remote)
			}
			target = defaultRef
		}
	}
	name := targetBranchName(target, remote)
	if err := gitcmd.SanitizeBranchName(name); err != nil {
		return targetPreflight{}, fmt.Errorf("invalid --target: %w", err)
	}
	return targetPreflight{branch: branch, remote: remote, targetRef: "refs/heads/" + name}, nil
}
