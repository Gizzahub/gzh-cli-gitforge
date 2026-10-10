// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

type preparedLegacy struct {
	source, root string
	profile      string
	inputs       []prepareInput
	// sourcePrepared is the kind of tree source is. It is stamped onto every
	// probe measured there so the baseline comparison can see whether the two
	// sides were prepared alike, instead of assuming they were.
	sourcePrepared     PrepareState
	controllerPrepared bool
	g                  gitRepo
}

func (p preparedLegacy) annotateProbe(ctx context.Context, probe makeProbe) makeProbe {
	probe.Prepared = p.sourcePrepared
	if !p.controllerPrepared {
		return probe
	}
	return annotateControllerPreparedProbe(ctx, probe)
}

// evidence is a stable, human-readable record of the closed profile and the
// immutable subproject commits extracted into every prepared tree. The
// source always gets them; the target gets the same ones only when a
// baseline is measured, which the per-target baseline record states.
func (p preparedLegacy) evidence() string {
	if p.profile == "" {
		return ""
	}
	parts := []string{p.profile}
	for _, input := range p.inputs {
		parts = append(parts, input.Name+"@"+input.OID)
	}
	return strings.Join(parts, " ")
}

const prepareProfileTimeout = 5 * time.Minute

func (p preparedLegacy) cleanup(ctx context.Context) error {
	if p.root == "" {
		return nil
	}
	if p.source == "" {
		return removePrepareRoot(ctx, p.g, p.root)
	}
	return removePreparedWorktree(ctx, p.g, p.source, p.root)
}

// prepareLegacySource prepares the tree the source probes run in. With a
// closed preparation profile that is a fresh worktree at the source commit;
// the target is not prepared here at all. Its baseline is read only as the
// non-worsening verdict for a source that failed, so it is measured later,
// by measureBaseline, and only for the make targets that need it.
//
// profile has already been resolved from the commit declarations by the
// caller; this executor never reads a worktree config.
func prepareLegacySource(ctx context.Context, g gitRepo, plan TargetPlan, profile string) (preparedLegacy, error) {
	// No profile means no preparation, and the branch is then measured where
	// the repository already is: the live working directory, carrying deps/,
	// node_modules/ and .venv from earlier runs. The baseline it will be
	// compared against is a pristine worktree carrying none of them. The
	// asymmetry is not fixed here — it is recorded, so the verdict can name it
	// instead of reporting an unmeasurable baseline as a fact about the target
	// commit.
	//
	// A remote release source is the exception: the working directory is the
	// target checkout, not the source, so the source is measured in a
	// pristine detached worktree at the checked SHA.
	if profile == "" {
		if plan.releasesRemoteRef() && plan.HeadSHA != plan.BranchSHA {
			return preparePristineSource(ctx, g, plan)
		}
		return preparedLegacy{source: g.dir, sourcePrepared: PrepareStateWorkingDir}, nil
	}
	inputs, err := snapshotPrepareInputs(ctx, g, profile)
	if err != nil {
		return preparedLegacy{}, fmt.Errorf("snapshot preparation inputs: %w", err)
	}
	root, err := os.MkdirTemp("", "gz-git-integrate-prepare-")
	if err != nil {
		return preparedLegacy{}, err
	}
	source := filepath.Join(root, "source")
	if err := g.worktreeAddDetach(ctx, source, plan.BranchSHA); err != nil {
		_ = os.RemoveAll(root)
		return preparedLegacy{}, fmt.Errorf("prepare source worktree: %w", err)
	}
	if err := runPrepareProfileWithInputs(ctx, g, source, profile, inputs); err != nil {
		cleanupErr := removePreparedWorktree(ctx, g, source, root)
		return preparedLegacy{}, errors.Join(fmt.Errorf("prepare source: %w", err), cleanupErr)
	}
	return preparedLegacy{source: source, root: root, sourcePrepared: PrepareStateProfilePrepared, controllerPrepared: true, profile: profile, inputs: inputs, g: g}, nil
}

// measureBaseline measures the target commit for exactly the named make
// targets, with the profile and input snapshot the source was prepared from.
// Both sides get a fresh worktree and the same profile, so these probes ARE
// prepared alike; the stamp records that symmetry as evidence.
//
// The source worktree is removed first and the target is removed before
// this returns, so the two trees are never alive at the same time and
// repository code in one cannot use the other. Every source probe must
// therefore already have been measured. Without a profile there is nothing
// to measure here: the legacy judge measures its own pristine baseline.
func (p *preparedLegacy) measureBaseline(ctx context.Context, plan TargetPlan, targets []string, budget time.Duration, outcomes *config.MakeOutcomeReport) (map[string]makeProbe, error) {
	if p.profile == "" || len(targets) == 0 {
		return nil, nil //nolint:nilnil // nothing was asked to be measured
	}
	if p.source != "" {
		if err := removePreparedWorktree(ctx, p.g, p.source, ""); err != nil {
			return nil, fmt.Errorf("cleanup prepared source: %w", err)
		}
		p.source = ""
	}
	target := filepath.Join(p.root, "target")
	if err := p.g.worktreeAddDetach(ctx, target, plan.TargetSHA); err != nil {
		return nil, fmt.Errorf("prepare target worktree: %w", err)
	}
	if err := runPrepareProfileWithInputs(ctx, p.g, target, p.profile, p.inputs); err != nil {
		cleanupErr := removePreparedWorktree(ctx, p.g, target, "")
		return nil, errors.Join(fmt.Errorf("prepare target: %w", err), cleanupErr)
	}
	baseline := make(map[string]makeProbe, len(targets))
	for _, name := range targets {
		baseline[name] = p.annotateProbe(ctx, runMakeTargetWithOutcomes(ctx, target, name, budget, outcomes.Includes(name)))
	}
	if err := removePreparedWorktree(ctx, p.g, target, ""); err != nil {
		return nil, fmt.Errorf("cleanup prepared target: %w", err)
	}
	return baseline, nil
}

func preparePristineSource(ctx context.Context, g gitRepo, plan TargetPlan) (preparedLegacy, error) {
	root, err := os.MkdirTemp("", "gz-git-integrate-prepare-")
	if err != nil {
		return preparedLegacy{}, err
	}
	source := filepath.Join(root, "source")
	if err := g.worktreeAddDetach(ctx, source, plan.BranchSHA); err != nil {
		_ = os.RemoveAll(root)
		return preparedLegacy{}, fmt.Errorf("prepare source worktree: %w", err)
	}
	return preparedLegacy{source: source, root: root, sourcePrepared: PrepareStatePristine, g: g}, nil
}

func removePreparedWorktree(parent context.Context, g gitRepo, wt, root string) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), 30*time.Second)
	defer cancel()
	if err := g.worktreeRemoveForce(ctx, wt); err != nil {
		return err
	}
	trees, err := g.listWorktrees(ctx)
	if err != nil {
		return err
	}
	for _, tree := range trees {
		if tree.Path == wt {
			return fmt.Errorf("worktree remains registered: %s", wt)
		}
	}
	if root != "" {
		return removePrepareRoot(ctx, g, root)
	}
	return nil
}

// removePrepareRoot deletes a prepare root only when git registers no
// worktree below it, so a tree that survived its removal is reported instead
// of being deleted out from under its registration.
func removePrepareRoot(parent context.Context, g gitRepo, root string) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), 30*time.Second)
	defer cancel()
	trees, err := g.listWorktrees(ctx)
	if err != nil {
		return err
	}
	for _, tree := range trees {
		rel, relErr := filepath.Rel(root, tree.Path)
		if relErr == nil && (rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))) {
			return fmt.Errorf("worktree remains registered below prepare root: %s", tree.Path)
		}
	}
	return os.RemoveAll(root)
}

// This is process isolation, not a security sandbox. As with legacy Make,
// the selected task revision is trusted to execute repository-owned code.
func runPrepareProfile(parent context.Context, g gitRepo, dir, profile string) error {
	inputs, err := snapshotPrepareInputs(parent, g, profile)
	if err != nil {
		return err
	}
	return runPrepareProfileWithInputs(parent, g, dir, profile, inputs)
}

func runPrepareProfileWithInputs(parent context.Context, g gitRepo, dir, profile string, inputs []prepareInput) error {
	if profile == flowTaskchainLocalSubprojectsV1 {
		return prepareFlowTaskchainLocalSubprojects(parent, dir, inputs)
	}
	if profile == cargoWorkspacePrepareV1 {
		return prepareCargoWorkspace(parent, g, dir)
	}
	if profile == pnpmFrozenLockfilePrepareV1 {
		return preparePnpmFrozenLockfile(parent, g, dir)
	}
	if profile != familybookEntPrepareV1 {
		return fmt.Errorf("unsupported preparation profile %q", profile)
	}
	if err := rejectEntSymlinkChain(dir); err != nil {
		return err
	}
	before, err := g.refNames(parent)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(parent, prepareProfileTimeout)
	defer cancel()
	isolated, err := os.MkdirTemp("", "gz-git-integrate-go-env-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(isolated)
	home := filepath.Join(isolated, "home")
	if err := os.MkdirAll(home, 0o700); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "go", "generate", "./ent") // #nosec G204 -- fixed closed profile.
	cmd.Dir = dir
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home, "XDG_CONFIG_HOME=" + filepath.Join(isolated, "xdg-config"), "XDG_CACHE_HOME=" + filepath.Join(isolated, "xdg-cache"), "GOCACHE=" + filepath.Join(isolated, "go-cache"), "GOMODCACHE=" + filepath.Join(isolated, "go-mod"), "GOWORK=off", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0", "LC_ALL=C"}
	var out bytes.Buffer
	cmd.Stdout = &limitedWriter{w: &out, n: 128 << 10}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go generate ./ent failed: %w: %s", err, strings.TrimSpace(out.String()))
	}
	if ctx.Err() != nil {
		return fmt.Errorf("preparation timed out")
	}
	after, err := g.refNames(parent)
	if err != nil {
		return err
	}
	if strings.Join(before, "\x00") != strings.Join(after, "\x00") {
		return fmt.Errorf("preparation changed git refs")
	}
	return validatePreparedStatus(ctx, dir, "ent/generated/")
}

func rejectEntSymlinkChain(dir string) error {
	for _, part := range []string{"ent", "ent/generated"} {
		info, err := os.Lstat(filepath.Join(dir, part))
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("preparation path is symlink: %s", part)
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// validatePreparedStatus rejects any tree change the preparation introduced
// outside the profile's declared output directory. Ignored ("!!") entries
// under ignoredOutputPrefix are that output directory — a build tool's
// artifacts land there by design — and everything else, untracked included,
// is a forbidden path: the profile must not silently introduce new files.
// An empty prefix allows no ignored entry at all.
func validatePreparedStatus(ctx context.Context, dir, ignoredOutputPrefix string) error {
	return validatePreparedStatusAllowing(ctx, dir, func(path string) bool {
		return strings.HasPrefix(path, ignoredOutputPrefix)
	})
}

// validatePreparedStatusAllowing is validatePreparedStatus for a profile
// whose output is not one directory; allowed decides which ignored paths
// are that output.
func validatePreparedStatusAllowing(ctx context.Context, dir string, allowed func(path string) bool) error {
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignored=matching")
	cmd.Dir = dir
	raw, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("inspect prepared tree: %w", err)
	}
	for _, record := range bytes.Split(raw, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		if len(record) < 4 {
			return fmt.Errorf("invalid preparation status")
		}
		state, path := string(record[:2]), string(record[3:])
		if state != "!!" || !allowed(path) {
			return fmt.Errorf("preparation changed forbidden path: %s", path)
		}
	}
	return nil
}

type limitedWriter struct {
	w *bytes.Buffer
	n int
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	if w.n <= 0 {
		return len(p), nil
	}
	if len(p) > w.n {
		p = p[:w.n]
	}
	w.n -= len(p)
	return w.w.Write(p)
}
