// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

// reclaimDeclaredClones decides what to do with the ignored nested git repos
// found in a task worktree that is about to be removed.
//
// A gate that needs `make prepare` runs `gz-git sync`, which clones the
// repo-root `.gz-git.yaml` `workspaces:` entries into ignored directories; a
// refusal for every nested repo would make such a repository end every
// integration with reclaim incomplete. A nested repo is therefore removable
// only when it is provably a disposable copy of a declared clone:
//
//   - its worktree-relative path is a declared workspace path;
//   - its origin URL is the declared URL;
//   - its working tree is clean, untracked files included;
//   - it holds no stash entries;
//   - every local branch has an upstream and is not ahead of it, and a
//     detached HEAD is contained in a remote-tracking ref.
//
// Anything else keeps the refusal, naming the path and the failed condition,
// and an undeclared nested repo is never removed. Removal is all-or-nothing:
// nothing is deleted unless every nested repo qualifies, and each deletion is
// scoped to that exact verified path. The caller then runs a plain
// `git worktree remove`, so tracked changes still refuse as before.
func reclaimDeclaredClones(ctx context.Context, sg gitRepo, taskWT string, nested []string, out *ReclaimResult) bool {
	declared, declErr := config.DeclaredWorkspaceClones(taskWT)
	var refused []string
	var removable []string
	for _, entry := range nested {
		rel := filepath.ToSlash(filepath.Clean(strings.TrimSuffix(entry, "/")))
		switch {
		case declErr != nil:
			refused = append(refused, fmt.Sprintf("%s (cannot read declared workspaces: %v)", entry, declErr))
			continue
		case declared[rel] == "":
			refused = append(refused, entry+" (not a declared workspace clone)")
			continue
		}
		if failed := verifyDeclaredClone(ctx, sg, filepath.Join(taskWT, filepath.FromSlash(rel)), declared[rel]); len(failed) > 0 {
			refused = append(refused, fmt.Sprintf("%s (%s)", entry, strings.Join(failed, "; ")))
			continue
		}
		removable = append(removable, rel)
	}
	if len(refused) > 0 {
		out.Failed = append(out.Failed, "ignored nested git repo in worktree: "+strings.Join(refused, ", "))
		return false
	}
	for _, rel := range removable {
		path := filepath.Join(taskWT, filepath.FromSlash(rel))
		if err := os.RemoveAll(path); err != nil {
			out.Failed = append(out.Failed, "remove declared workspace clone "+path+": "+err.Error())
			return false
		}
		out.Done = append(out.Done, "workspace-clone("+rel+")")
	}
	return true
}

// verifyDeclaredClone returns the conditions the clone at dir fails; an empty
// result means it holds nothing that exists only there.
func verifyDeclaredClone(ctx context.Context, sg gitRepo, dir, wantURL string) []string {
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() {
		return []string{"not a plain directory"}
	}
	g := newGitRepo(sg.exec, dir)
	var failed []string
	if got, err := g.output(ctx, "config", "--get", "remote.origin.url"); err != nil {
		failed = append(failed, "origin url unreadable")
	} else if normalizeCloneURL(got) != normalizeCloneURL(wantURL) {
		failed = append(failed, fmt.Sprintf("origin url %q is not the declared %q", got, wantURL))
	}
	if status, err := g.output(ctx, "status", "--porcelain", "--untracked-files=all"); err != nil {
		failed = append(failed, "status unreadable")
	} else if status != "" {
		failed = append(failed, "uncommitted or untracked changes")
	}
	if stash, err := g.output(ctx, "stash", "list"); err != nil {
		failed = append(failed, "stash unreadable")
	} else if stash != "" {
		failed = append(failed, "stash entries")
	}
	return append(failed, unpushedWork(ctx, g)...)
}

func unpushedWork(ctx context.Context, g gitRepo) []string {
	refs, err := g.output(ctx, "for-each-ref", "--format=%(refname:short)%09%(upstream)%09%(upstream:track)", "refs/heads")
	if err != nil {
		return []string{"branches unreadable"}
	}
	var failed []string
	for _, line := range splitNonEmpty(refs) {
		fields := strings.Split(line, "\t")
		for len(fields) < 3 {
			fields = append(fields, "")
		}
		name, upstream, track := fields[0], fields[1], fields[2]
		switch {
		case upstream == "":
			failed = append(failed, "branch "+name+" has no upstream")
		case strings.Contains(track, "gone"):
			failed = append(failed, "branch "+name+" upstream is gone")
		case strings.Contains(track, "ahead"):
			failed = append(failed, "branch "+name+" is ahead of its upstream")
		}
	}
	if res, err := g.run(ctx, "symbolic-ref", "--quiet", "HEAD"); err == nil && res != nil && res.ExitCode != 0 {
		// Detached HEAD: its commits are safe only if a remote branch has them.
		contained, err := g.output(ctx, "branch", "--remotes", "--contains", "HEAD")
		if err != nil || contained == "" {
			failed = append(failed, "detached HEAD is not contained in a remote branch")
		}
	}
	return failed
}

// normalizeCloneURL makes the comparison tolerant of a trailing slash or .git
// only; hosts, schemes and paths must otherwise match exactly.
func normalizeCloneURL(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimRight(raw, "/")
	return strings.TrimSuffix(raw, ".git")
}
