// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// readinessWorktreeIdentity binds the short-lived detached worktree to the
// exact Git registration and physical directory created for a readiness run.
// Cleanup must not follow a replacement at the same pathname.
type readinessWorktreeIdentity struct {
	path      string
	head      string
	commonDir string
	directory LegacyFileIdentity
	gitFile   LegacyFileIdentity
	common    LegacyFileIdentity
}

func captureReadinessWorktree(ctx context.Context, g gitRepo, path, head string) (readinessWorktreeIdentity, error) {
	path = filepath.Clean(path)
	if head == "" {
		return readinessWorktreeIdentity{}, fmt.Errorf("capture readiness worktree %s: empty expected HEAD", boundedReadinessPath(path))
	}
	// Reject a symlink at the allocated endpoint before resolving macOS's
	// system-level /var prefix alias. Git may print either spelling.
	entry, err := os.Lstat(path)
	if err != nil {
		return readinessWorktreeIdentity{}, fmt.Errorf("capture readiness worktree %s directory: %w", boundedReadinessPath(path), err)
	}
	if entry.Mode()&os.ModeSymlink != 0 || !entry.IsDir() {
		return readinessWorktreeIdentity{}, fmt.Errorf("capture readiness worktree %s: unsupported directory type", boundedReadinessPath(path))
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return readinessWorktreeIdentity{}, fmt.Errorf("capture readiness worktree %s canonical path: %w", boundedReadinessPath(path), err)
	}
	path = filepath.Clean(resolved)
	directory, err := legacyPhysicalIdentity(path, true)
	if err != nil {
		return readinessWorktreeIdentity{}, fmt.Errorf("capture readiness worktree %s directory: %w", boundedReadinessPath(path), err)
	}
	gitFile, err := legacyPhysicalIdentity(filepath.Join(path, ".git"), false)
	if err != nil {
		return readinessWorktreeIdentity{}, fmt.Errorf("capture readiness worktree %s gitfile: %w", boundedReadinessPath(path), err)
	}
	commonDir, err := g.output(ctx, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return readinessWorktreeIdentity{}, fmt.Errorf("capture readiness common repository: %w", err)
	}
	commonDir, err = filepath.EvalSymlinks(commonDir)
	if err != nil {
		return readinessWorktreeIdentity{}, fmt.Errorf("canonicalize readiness common repository: %w", err)
	}
	common, err := legacyPhysicalIdentity(commonDir, true)
	if err != nil {
		return readinessWorktreeIdentity{}, fmt.Errorf("capture readiness common repository identity: %w", err)
	}
	if err := verifyReadinessWorktree(ctx, g, readinessWorktreeIdentity{path: path, head: head, commonDir: commonDir, directory: directory, gitFile: gitFile, common: common}); err != nil {
		return readinessWorktreeIdentity{}, err
	}
	return readinessWorktreeIdentity{path: path, head: head, commonDir: commonDir, directory: directory, gitFile: gitFile, common: common}, nil
}

func verifyReadinessWorktree(ctx context.Context, g gitRepo, want readinessWorktreeIdentity) error {
	if want.path == "" || want.head == "" || want.commonDir == "" {
		return fmt.Errorf("readiness worktree identity is incomplete")
	}
	if got, err := legacyPhysicalIdentity(want.commonDir, true); err != nil || got != want.common {
		if err != nil {
			return fmt.Errorf("verify readiness common repository: %w", err)
		}
		return fmt.Errorf("verify readiness common repository: physical identity changed")
	}
	if got, err := legacyPhysicalIdentity(want.path, true); err != nil || got != want.directory {
		if err != nil {
			return fmt.Errorf("verify readiness worktree %s directory: %w", boundedReadinessPath(want.path), err)
		}
		return fmt.Errorf("verify readiness worktree %s: directory identity changed", boundedReadinessPath(want.path))
	}
	if got, err := legacyPhysicalIdentity(filepath.Join(want.path, ".git"), false); err != nil || got != want.gitFile {
		if err != nil {
			return fmt.Errorf("verify readiness worktree %s gitfile: %w", boundedReadinessPath(want.path), err)
		}
		return fmt.Errorf("verify readiness worktree %s: gitfile identity changed", boundedReadinessPath(want.path))
	}
	worktree := newGitRepo(g.exec, want.path)
	top, err := worktree.toplevel(ctx)
	if err != nil || filepath.Clean(top) != want.path {
		return fmt.Errorf("verify readiness worktree %s: foreign or replaced top-level", boundedReadinessPath(want.path))
	}
	attached, _, attachedErr := worktree.symbolicRef(ctx, "HEAD")
	if attachedErr != nil {
		return fmt.Errorf("verify readiness worktree %s symbolic HEAD: %w", boundedReadinessPath(want.path), attachedErr)
	}
	if attached != "" {
		return fmt.Errorf("verify readiness worktree %s: expected detached HEAD, found %s", boundedReadinessPath(want.path), attached)
	}
	head, ok, err := worktree.revParse(ctx, "HEAD")
	if err != nil || !ok || head != want.head {
		return fmt.Errorf("verify readiness worktree %s: HEAD changed", boundedReadinessPath(want.path))
	}
	common, err := worktree.output(ctx, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err == nil {
		common, err = filepath.EvalSymlinks(common)
	}
	if err != nil || filepath.Clean(common) != filepath.Clean(want.commonDir) {
		return fmt.Errorf("verify readiness worktree %s: common repository changed", boundedReadinessPath(want.path))
	}
	trees, err := g.listWorktrees(ctx)
	if err != nil {
		return fmt.Errorf("verify readiness worktree registration: %w", err)
	}
	for _, tree := range trees {
		if filepath.Clean(tree.Path) == want.path {
			if tree.Branch != "" || tree.Head != want.head || tree.Locked {
				return fmt.Errorf("verify readiness worktree %s: registration no longer matches detached HEAD", boundedReadinessPath(want.path))
			}
			return nil
		}
	}
	return fmt.Errorf("verify readiness worktree %s: no matching registration", boundedReadinessPath(want.path))
}

func removeReadinessWorktree(parent context.Context, g gitRepo, identity readinessWorktreeIdentity) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(parent), 30*time.Second)
	defer cancel()
	if err := verifyReadinessWorktree(cleanupCtx, g, identity); err != nil {
		return fmt.Errorf("cleanup readiness worktree %s: %w", boundedReadinessPath(identity.path), err)
	}
	if err := g.worktreeRemoveForce(cleanupCtx, identity.path); err != nil {
		return fmt.Errorf("cleanup readiness worktree %s: %w", boundedReadinessPath(identity.path), err)
	}
	return nil
}

func boundedReadinessPath(path string) string {
	clean := filepath.Clean(path)
	clean = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\x00' {
			return -1
		}
		return r
	}, clean)
	return boundedDetail(clean)
}

// removeEmptyReadinessRoot removes only this invocation's known, still-owned
// cache directories after Git proves no worktree remains below the captured
// root. It intentionally never recursively deletes a root after a worktree
// cleanup failure.
func removeEmptyReadinessRoot(parent context.Context, g gitRepo, root string, identity LegacyFileIdentity, siblings bool) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), 30*time.Second)
	defer cancel()
	if got, err := legacyPhysicalIdentity(root, true); err != nil || got != identity {
		if err != nil {
			return fmt.Errorf("verify readiness workspace root: %w", err)
		}
		return fmt.Errorf("verify readiness workspace root: physical identity changed")
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("canonicalize readiness workspace root: %w", err)
	}
	trees, err := g.listWorktrees(ctx)
	if err != nil {
		return fmt.Errorf("list worktrees before readiness root cleanup: %w", err)
	}
	for _, tree := range trees {
		canonicalTree, treeErr := filepath.EvalSymlinks(tree.Path)
		if treeErr != nil {
			if os.IsNotExist(treeErr) && !readinessPathMayBeWithin(root, canonicalRoot, tree.Path) {
				continue // unrelated stale registrations must not block this root.
			}
			return fmt.Errorf("registered worktree below readiness workspace is unresolved: %s", boundedReadinessPath(tree.Path))
		}
		if readinessPathWithin(canonicalRoot, canonicalTree) {
			return fmt.Errorf("registered worktree remains below readiness workspace: %s", boundedReadinessPath(tree.Path))
		}
	}
	for _, name := range []string{"target-cargo", "source-cargo"} {
		if err := removeOwnedReadinessDirectory(filepath.Join(root, name)); err != nil {
			return err
		}
	}
	if siblings {
		for _, name := range []string{"target-side", "source-side"} {
			if err := os.Remove(filepath.Join(root, name)); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return os.Remove(root)
}

func readinessPathWithin(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	return err == nil && (rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))))
}

func readinessPathMayBeWithin(root, canonicalRoot, candidate string) bool {
	return readinessPathWithin(filepath.Clean(root), filepath.Clean(candidate)) || readinessPathWithin(filepath.Clean(canonicalRoot), filepath.Clean(candidate))
}

// removeOwnedReadinessDirectory removes an optional cache directory created
// directly beneath the captured root. A symlink is never traversed.
func removeOwnedReadinessDirectory(path string) error {
	entry, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect readiness cache %s: %w", boundedReadinessPath(path), err)
	}
	if entry.Mode()&os.ModeSymlink != 0 || !entry.IsDir() {
		return fmt.Errorf("refuse unexpected readiness cache type: %s", boundedReadinessPath(path))
	}
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove owned readiness cache %s: %w", boundedReadinessPath(path), err)
	}
	return nil
}
