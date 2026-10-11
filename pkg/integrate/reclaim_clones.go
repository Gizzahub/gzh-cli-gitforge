// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"fmt"
	"io/fs"
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
//   - its `.git` is a directory (a gitfile or linked worktree keeps its
//     history elsewhere, so deleting the directory proves nothing);
//   - its origin URL is the declared URL;
//   - its working tree is clean, untracked files included;
//   - it holds no stash entries, no initialized submodule and no linked
//     worktree (their history lives outside the checks below);
//   - no ignored entry inside it contains another git repo, at any depth;
//   - every commit reachable from any ref is also reachable from a
//     remote-tracking ref (`rev-list --all --not --remotes` is empty), and
//     every branch upstream is itself a remote-tracking ref, so a pair of
//     local branches cannot vouch for each other.
//
// Policy, stated rather than implied: ignored files that are not repos (a
// `.env`, build output) are deleted with the clone. That is exactly what
// `git worktree remove` already does to the parent's ignored files, so a
// clone is held to the parent's standard and no stricter.
//
// Known limit: only the repo-root `.gz-git.yaml` is read. Workspace configs in
// subdirectories (for example `deps/.gz-git.yaml`) are out of scope; their
// clones keep the refusal.
//
// Anything else keeps the refusal, naming the path and the failed condition,
// and an undeclared nested repo is never removed. Verification is
// all-or-nothing: nothing is deleted unless every nested repo qualifies.
// Deletion is not atomic: if one removal fails, earlier ones stay removed and
// the failure is reported. Each deletion is re-anchored to the verified path
// right before it happens. The caller then runs a plain `git worktree remove`,
// so tracked changes still refuse as before.
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
		if err := removeVerifiedClone(taskWT, rel, path); err != nil {
			out.Failed = append(out.Failed, "remove declared workspace clone "+path+": "+err.Error())
			return false
		}
		out.Done = append(out.Done, "workspace-clone("+rel+")")
	}
	return true
}

// removeVerifiedClone deletes path only if it still resolves to rel inside the
// resolved task worktree, so a symlink swapped in for a parent directory after
// verification cannot redirect the deletion elsewhere.
func removeVerifiedClone(taskWT, rel, path string) error {
	root, err := filepath.EvalSymlinks(taskWT)
	if err != nil {
		return err
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	if want := filepath.Join(root, filepath.FromSlash(rel)); resolved != want {
		return fmt.Errorf("path resolves to %s, not %s", resolved, want)
	}
	return os.RemoveAll(path)
}

// verifyDeclaredClone returns the conditions the clone at dir fails; an empty
// result means every commit in it is also on a remote and it holds no other
// repo (ignored non-repo files are deleted by policy, see
// reclaimDeclaredClones).
func verifyDeclaredClone(ctx context.Context, sg gitRepo, dir, wantURL string) []string {
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() {
		return []string{"not a plain directory"}
	}
	if gitInfo, err := os.Lstat(filepath.Join(dir, ".git")); err != nil || !gitInfo.IsDir() {
		return []string{".git is not a directory"}
	}
	g := newGitRepo(sg.exec, dir)
	var failed []string
	failed = append(failed, entangledRepos(ctx, g, dir)...)
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
	failed = append(failed, ignoredNestedRepos(ctx, g, dir)...)
	if stash, err := g.output(ctx, "stash", "list"); err != nil {
		failed = append(failed, "stash entries unreadable")
	} else if stash != "" {
		failed = append(failed, "stash entries")
	}
	return append(failed, unpushedWork(ctx, g)...)
}

// entangledRepos reports other repositories whose history lives in or points
// at this clone: initialized submodules (their commits sit in
// `.git/modules`, outside every check below) and linked worktrees (deleting
// the clone would orphan them). Both are refused rather than inspected.
func entangledRepos(ctx context.Context, g gitRepo, dir string) []string {
	var failed []string
	if entries, err := os.ReadDir(filepath.Join(dir, ".git", "modules")); err == nil && len(entries) > 0 {
		failed = append(failed, "has initialized submodules")
	} else if err != nil && !os.IsNotExist(err) {
		failed = append(failed, "submodule directory unreadable")
	}
	// A line starting with "-" is a declared but uninitialized submodule: no
	// .git/config entry and no .git/modules data. It is harmless only while its
	// path holds no repository of its own; a hand-made clone there keeps
	// branches that the clone's own refs and status never see.
	if status, err := g.output(ctx, "submodule", "status", "--recursive"); err != nil {
		failed = append(failed, "submodule status unreadable")
	} else {
		initialized, uninitialized := parseSubmoduleStatus(status)
		if initialized && !containsString(failed, "has initialized submodules") {
			failed = append(failed, "has initialized submodules")
		}
		for _, sub := range uninitialized {
			if _, err := os.Lstat(filepath.Join(dir, filepath.FromSlash(sub), ".git")); err == nil {
				failed = append(failed, "has a repository at uninitialized submodule path "+sub)
			} else if !os.IsNotExist(err) {
				failed = append(failed, "submodule path "+sub+" unreadable")
			}
		}
	}
	if list, err := g.output(ctx, "worktree", "list", "--porcelain"); err != nil {
		failed = append(failed, "worktree list unreadable")
	} else if countWorktrees(list) > 1 {
		failed = append(failed, "has linked worktrees")
	}
	return failed
}

// parseSubmoduleStatus splits `git submodule status` lines into whether any
// submodule is initialized and the paths of the uninitialized ones ("-sha path").
func parseSubmoduleStatus(status string) (initialized bool, uninitialized []string) {
	for _, line := range splitNonEmpty(status) {
		if !strings.HasPrefix(line, "-") {
			initialized = true
			continue
		}
		if _, path, ok := strings.Cut(line, " "); ok {
			uninitialized = append(uninitialized, path)
		}
	}
	return initialized, uninitialized
}

func countWorktrees(porcelain string) int {
	n := 0
	for _, line := range strings.Split(porcelain, "\n") {
		if strings.HasPrefix(line, "worktree ") {
			n++
		}
	}
	return n
}

// ignoredNestedRepos reports ignored entries of the clone that are, or contain
// at any depth, a `.git` directory or file. `status` does not descend into
// them, so the walk is the only thing that sees a repo hiding in there.
func ignoredNestedRepos(ctx context.Context, g gitRepo, dir string) []string {
	res, err := g.run(ctx, "status", "--porcelain", "--ignored", "-z")
	if err != nil || res == nil || res.ExitCode != 0 {
		return []string{"ignored entries unreadable"}
	}
	var failed []string
	for _, record := range strings.Split(res.Stdout, "\x00") {
		if !strings.HasPrefix(record, "!! ") {
			continue
		}
		entry := strings.TrimPrefix(record, "!! ")
		found := false
		walkErr := filepath.WalkDir(filepath.Join(dir, filepath.FromSlash(entry)), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.Name() == ".git" {
				found = true
				return filepath.SkipAll
			}
			return nil
		})
		switch {
		case walkErr != nil:
			failed = append(failed, "ignored entry "+entry+" unreadable")
		case found:
			failed = append(failed, "ignored entry "+entry+" contains a git repo")
		}
	}
	return failed
}

// unpushedWork reports commits that exist only in this clone. One question
// covers branches, detached HEAD, tags and every other ref: is anything
// reachable from any ref but from no remote-tracking ref?
func unpushedWork(ctx context.Context, g gitRepo) []string {
	local, err := g.output(ctx, "rev-list", "--all", "--not", "--remotes", "-n1")
	if err != nil {
		return []string{"commits unreadable"}
	}
	var failed []string
	if local != "" {
		failed = append(failed, "commits not on any remote-tracking ref")
	}
	refs, err := g.output(ctx, "for-each-ref", "--format=%(refname:short)%09%(upstream)", "refs/heads")
	if err != nil {
		return append(failed, "branches unreadable")
	}
	for _, line := range splitNonEmpty(refs) {
		name, upstream, _ := strings.Cut(line, "\t")
		if upstream != "" && !strings.HasPrefix(upstream, "refs/remotes/") {
			failed = append(failed, "branch "+name+" tracks a non-remote ref")
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
