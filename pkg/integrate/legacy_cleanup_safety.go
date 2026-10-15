// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

// LegacyFileIdentity binds a physical resource, not merely a reusable pathname.
// Hosts without observable device/inode identities are unsupported rather than
// treating size or modification time as a substitute for resource identity.
type LegacyFileIdentity struct {
	Device uint64 `json:"device"`
	Inode  uint64 `json:"inode"`
}

// LegacyWorktreeIdentity is the prospective plan's local resource inventory.
type LegacyWorktreeIdentity struct {
	GitDir              string             `json:"git_dir"`
	PrimaryGitDirectory LegacyFileIdentity `json:"primary_git_directory"`
	Worktree            LegacyFileIdentity `json:"worktree"`
	GitDirectory        LegacyFileIdentity `json:"git_directory"`
	GitFile             LegacyFileIdentity `json:"git_file"`
}

func legacyPhysicalIdentity(path string, directory bool) (LegacyFileIdentity, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return LegacyFileIdentity{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || info.IsDir() != directory || (!directory && !info.Mode().IsRegular()) {
		return LegacyFileIdentity{}, fmt.Errorf("unexpected physical resource type: %s", path)
	}
	stat := reflect.ValueOf(info.Sys())
	if stat.Kind() == reflect.Pointer {
		stat = stat.Elem()
	}
	if stat.Kind() != reflect.Struct {
		return LegacyFileIdentity{}, fmt.Errorf("physical resource identity is unavailable")
	}
	dev, ino := stat.FieldByName("Dev"), stat.FieldByName("Ino")
	device, devOK := legacyStatNumber(dev)
	inode, inoOK := legacyStatNumber(ino)
	if !devOK || !inoOK || inode == 0 {
		return LegacyFileIdentity{}, fmt.Errorf("physical resource identity is unsupported on this host")
	}
	return LegacyFileIdentity{Device: device, Inode: inode}, nil
}

func legacyStatNumber(value reflect.Value) (uint64, bool) {
	if !value.IsValid() {
		return 0, false
	}
	if value.CanUint() {
		return value.Uint(), true
	}
	if value.CanInt() && value.Int() >= 0 {
		return uint64(value.Int()), true // #nosec G115 -- the preceding nonnegative int64 guard bounds the conversion.
	}
	return 0, false
}

func refuseLegacyRuntime(roots ...string) error {
	for _, root := range roots {
		if _, err := os.Lstat(filepath.Join(root, ".gz-git-task.yaml")); err == nil {
			return ErrLegacyCleanupUnsupportedNative
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect native lifecycle declaration: %w", err)
		}
		dir := filepath.Join(root, ".ce")
		info, err := os.Lstat(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect lifecycle directory: %w", err)
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("lifecycle directory has an unsupported physical type: %s", dir)
		}
		for _, name := range []string{"task-runtime.yaml", "task-runtime.yml"} {
			path := filepath.Join(root, ".ce", name)
			if _, err := os.Lstat(path); err == nil {
				return ErrLegacyCleanupUnsupportedNative
			} else if !os.IsNotExist(err) {
				return fmt.Errorf("inspect lifecycle declaration: %w", err)
			}
		}
	}
	return nil
}

// validateLegacyCERuntime permits a CE declaration only when an external CE
// verifier is bound into the prospective plan. Native state is never read.
func validateLegacyCERuntime(roots ...string) error {
	found := false
	for _, root := range roots {
		if _, err := os.Lstat(filepath.Join(root, ".gz-git-task.yaml")); err == nil {
			return ErrLegacyCleanupUnsupportedNative
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect native lifecycle declaration: %w", err)
		}
		dir := filepath.Join(root, ".ce")
		info, err := os.Lstat(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect CE lifecycle directory: %w", err)
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("CE lifecycle directory has an unsupported physical type: %s", dir)
		}
		for _, name := range []string{"task-runtime.yaml", "task-runtime.yml"} {
			path := filepath.Join(dir, name)
			entry, statErr := os.Lstat(path)
			if os.IsNotExist(statErr) {
				continue
			}
			if statErr != nil || !entry.Mode().IsRegular() || entry.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("CE lifecycle declaration has an unsupported physical type: %s", path)
			}
			found = true
		}
	}
	if !found {
		return ErrLegacyCleanupUnsupportedNative
	}
	return nil
}

func refuseLegacyNativeRuntimeState(common string) error {
	if _, err := os.Lstat(filepath.Join(common, "gz-git", "task-runtime")); err == nil {
		return ErrLegacyCleanupUnsupportedNative
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect native lifecycle state boundary: %w", err)
	}
	return nil
}

// State-directory presence is an unsupported lifecycle boundary, not permission
// to interpret or import the host's private execution records.
func refuseLegacyRuntimeState(common string) error {
	for _, owner := range []string{"ce", "gz-git"} {
		if _, err := os.Lstat(filepath.Join(common, owner, "task-runtime")); err == nil {
			return ErrLegacyCleanupUnsupportedNative
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect lifecycle state boundary: %w", err)
		}
	}
	return nil
}

// captureLegacyWorktreeIdentity observes only Git and filesystem facts. It
// issues no execution claim and writes no registration marker or receipt.
func validateLegacyWorktreeRegistration(ctx context.Context, g gitRepo, plan *LegacyCleanupPlan) error {
	trees, err := g.listWorktrees(ctx)
	if err != nil {
		return err
	}
	if len(trees) == 0 || trees[0].Path != plan.RepositoryRoot || plan.TaskWorktree == plan.RepositoryRoot {
		return fmt.Errorf("legacy cleanup requires a stable primary and a distinct task worktree")
	}
	count := 0
	for _, wt := range trees {
		if wt.Path == plan.TaskWorktree || wt.Branch == plan.TaskBranch {
			count++
			if wt.Path != plan.TaskWorktree || wt.Branch != plan.TaskBranch || wt.Head != plan.TaskOID || wt.Locked {
				return fmt.Errorf("legacy task registration changed, is foreign, or is locked")
			}
		}
	}
	if count != 1 {
		return fmt.Errorf("legacy task worktree registration is not exact")
	}
	return nil
}

func captureLegacyWorktreeIdentity(ctx context.Context, exec *gitcmd.Executor, plan *LegacyCleanupPlan) (*LegacyWorktreeIdentity, error) {
	g := newGitRepo(exec, plan.RepositoryRoot)
	if err := validateLegacyWorktreeRegistration(ctx, g, plan); err != nil {
		return nil, err
	}
	canonical, err := filepath.EvalSymlinks(plan.TaskWorktree)
	if err != nil {
		return nil, fmt.Errorf("resolve legacy task path: %w", err)
	}
	if canonical != plan.TaskWorktree || !filepath.IsAbs(plan.TaskWorktree) {
		return nil, fmt.Errorf("legacy task path is not canonical")
	}
	tg := newGitRepo(exec, plan.TaskWorktree)
	common, err := canonicalCommonFromRepo(ctx, tg)
	if err != nil {
		return nil, fmt.Errorf("resolve legacy task repository: %w", err)
	}
	if common != plan.CommonDir {
		return nil, fmt.Errorf("legacy task belongs to a different repository")
	}
	if _, present, err := readRegistrationMarker(ctx, exec, plan.TaskWorktree); err != nil {
		return nil, err
	} else if present {
		return nil, fmt.Errorf("registered execution requires its original lifecycle cleanup")
	}
	status, err := tg.porcelain(ctx)
	if err != nil {
		return nil, fmt.Errorf("observe legacy task status: %w", err)
	}
	if strings.TrimSpace(status) != "" {
		return nil, fmt.Errorf("legacy task is dirty")
	}
	if err := verifyGitOnlyReclaimScope(plan.CommonDir, plan.TaskWorktree); err != nil {
		return nil, err
	}
	gitDir, err := tg.output(ctx, "rev-parse", "--path-format=absolute", "--git-dir")
	if err != nil {
		return nil, err
	}
	canonicalGit, err := filepath.EvalSymlinks(gitDir)
	if err != nil {
		return nil, fmt.Errorf("resolve legacy task git directory: %w", err)
	}
	if canonicalGit != gitDir {
		return nil, fmt.Errorf("legacy task git directory is not canonical")
	}
	identity := &LegacyWorktreeIdentity{GitDir: gitDir}
	if identity.PrimaryGitDirectory, err = legacyPhysicalIdentity(plan.GitDir, true); err != nil {
		return nil, err
	}
	if identity.Worktree, err = legacyPhysicalIdentity(plan.TaskWorktree, true); err != nil {
		return nil, err
	}
	if identity.GitDirectory, err = legacyPhysicalIdentity(gitDir, true); err != nil {
		return nil, err
	}
	if identity.GitFile, err = legacyPhysicalIdentity(filepath.Join(plan.TaskWorktree, ".git"), false); err != nil {
		return nil, err
	}
	return identity, nil
}
