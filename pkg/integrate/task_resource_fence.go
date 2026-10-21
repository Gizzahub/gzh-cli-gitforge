// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

// taskFenceDirName keeps the directory and key the removed target lock used,
// so an older gz-git still contends on the same file as this one.
const taskFenceDirName = "gz-git-integrate-targets"

// acquireTaskResourceFence keeps two gz-git mutations of one task branch —
// run reclaim, cleanup retry, legacy cleanup — from deleting the same
// worktree and branches at once, even when callers select different target
// refs or execution IDs. It never waits: a held fence refuses with
// ErrResourceBusy naming the holder. It is cooperative exclusion, never
// authorization to adopt a legacy resource, and it does not order
// integrations of one target; the exact-lease push does that.
func acquireTaskResourceFence(ctx context.Context, exec *gitcmd.Executor, repoPath, branch string) (*resourceLock, error) {
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
	common, err := g.output(ctx, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil || common == "" {
		if err == nil {
			err = fmt.Errorf("git common directory is empty")
		}
		return nil, fmt.Errorf("resolve task resource fence directory: %w", err)
	}
	dir := filepath.Join(common, taskFenceDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil { // #nosec G301 -- Git common directory is repository-owned
		return nil, fmt.Errorf("create task resource fence directory: %w", err)
	}
	digest := sha256.Sum256([]byte("task-resource:" + branch))
	return tryAcquireResourceLock(filepath.Join(dir, fmt.Sprintf("%x.lock", digest[:])), "task resource fence for "+branch)
}
