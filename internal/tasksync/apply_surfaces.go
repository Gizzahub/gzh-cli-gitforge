// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package tasksync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Original application writes a different set of fixed surfaces than scratch
// preparation. Refuse unsafe parents/leaves before invoking Git on them.
func safeApplySurfaces(p ApplyPlan) error {
	if err := safeCommonSurfaces(p.CommonDir, ""); err != nil {
		return err
	}
	if err := safeExistingPath(p.TaskWorktree, ".git", false); err != nil {
		return err
	}
	raw, err := os.ReadFile(filepath.Join(p.TaskWorktree, ".git"))
	if err != nil || len(raw) > 4096 || !strings.HasPrefix(string(raw), "gitdir: ") {
		return fmt.Errorf("task administrative identity is unavailable")
	}
	declared := strings.TrimSpace(strings.TrimPrefix(string(raw), "gitdir: "))
	if !filepath.IsAbs(declared) {
		declared = filepath.Join(p.TaskWorktree, declared)
	}
	declared = filepath.Clean(declared)
	if filepath.Dir(declared) != filepath.Join(p.CommonDir, "worktrees") {
		return fmt.Errorf("task admin is outside the common worktree registry")
	}
	// Reconstruct from the trusted common root and one sanitized component;
	// never pass the metadata's arbitrary pathname to later filesystem I/O.
	admin := filepath.Join(p.CommonDir, "worktrees", filepath.Base(declared))
	relative, err := filepath.Rel(p.CommonDir, admin)
	if err != nil {
		return err
	}
	if err := safeExistingPath(p.CommonDir, relative, true); err != nil {
		return err
	}
	for _, name := range []string{"HEAD", "index", "gitdir", "commondir", "logs/HEAD", "config.worktree"} {
		if err := safeExistingPath(admin, name, false); err != nil {
			return err
		}
	}
	adminRoot, err := os.OpenRoot(admin)
	if err != nil {
		return err
	}
	defer adminRoot.Close()
	back, err := adminRoot.ReadFile("gitdir")
	if err != nil || strings.TrimSpace(string(back)) != filepath.Join(p.TaskWorktree, ".git") {
		return fmt.Errorf("task reverse administrative identity differs")
	}
	common, err := adminRoot.ReadFile("commondir")
	if err != nil || filepath.Clean(filepath.Join(admin, strings.TrimSpace(string(common)))) != p.CommonDir {
		return fmt.Errorf("task administrative common directory differs")
	}
	for _, name := range []string{"refs/heads/" + p.TaskBranch, "logs/refs/heads/" + p.TaskBranch} {
		if err := safeExistingPath(p.CommonDir, name, false); err != nil {
			return err
		}
	}
	for _, name := range []string{filepath.Join(admin, "index.lock"), filepath.Join(admin, "HEAD.lock"), filepath.Join(p.CommonDir, "refs/heads/"+p.TaskBranch+".lock")} {
		if _, err := os.Lstat(name); !os.IsNotExist(err) {
			return fmt.Errorf("task metadata lock is present or unreadable")
		}
	}
	return nil
}
