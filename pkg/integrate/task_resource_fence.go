// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

// acquireTaskResourceFence serializes native integration mutations of the same
// task even when callers select different target refs or execution IDs. This is
// cooperative exclusion, never authorization to adopt a legacy resource.
// All callers acquire this fence before target ownership to keep lock ordering
// consistent. Unrelated task branches retain independent target locks.
func acquireTaskResourceFence(ctx context.Context, exec *gitcmd.Executor, repoPath, branch string, notices ...io.Writer) (*targetLock, error) {
	if strings.TrimSpace(repoPath) == "" {
		repoPath = "."
	}
	g := newGitRepo(exec, repoPath)
	branch = strings.TrimSpace(branch)
	if branch == "" {
		var err error
		branch, err = g.currentBranch(ctx)
		if err != nil {
			return nil, fmt.Errorf("resolve task resource fence branch: %w", err)
		}
	}
	branch = strings.TrimPrefix(branch, "refs/heads/")
	if err := gitcmd.SanitizeBranchName(branch); err != nil {
		return nil, fmt.Errorf("task resource fence branch: %w", err)
	}
	var notice io.Writer
	if len(notices) > 0 {
		notice = notices[0]
	}
	return acquireTargetLock(ctx, g, "task-resource:"+branch, notice)
}
