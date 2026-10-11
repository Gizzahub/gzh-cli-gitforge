// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

const targetLocksDirName = "gz-git-integrate-targets"

// targetLock is local Git mutation ownership. Its path is below the Git
// common directory so every worktree of one repository contends on the same
// inode, while unrelated repositories remain independent.
type targetLock struct{ file *os.File }

func (l *targetLock) release() {
	if l != nil && l.file != nil {
		_ = l.file.Close()
		l.file = nil
	}
}

func acquireTargetLock(ctx context.Context, g gitRepo, targetRef string, notice io.Writer) (*targetLock, error) {
	common, err := g.output(ctx, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil || common == "" {
		if err == nil {
			err = fmt.Errorf("git common directory is empty")
		}
		return nil, fmt.Errorf("resolve target lock directory: %w", err)
	}
	dir := filepath.Join(common, targetLocksDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil { // #nosec G301 -- Git common directory is repository-owned
		return nil, fmt.Errorf("create target lock directory: %w", err)
	}
	digest := sha256.Sum256([]byte(targetRef))
	path := filepath.Join(dir, fmt.Sprintf("%x.lock", digest[:]))
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600) // #nosec G304 -- fixed digest below the Git common directory
	if err != nil {
		return nil, fmt.Errorf("open target lock: %w", err)
	}
	if notice == nil {
		notice = os.Stderr
	}
	var lastNotice time.Time
	for {
		locked, err := tryLockFile(f)
		if err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("lock target %s: %w", targetRef, err)
		}
		if locked {
			return &targetLock{file: f}, nil
		}
		if lastNotice.IsZero() || time.Since(lastNotice) >= hostLockNoticeInterval {
			lastNotice = time.Now()
			fmt.Fprintf(notice, "integrate: waiting for target ownership common-dir=%s destination=%s\n", common, targetRef)
		}
		timer := time.NewTimer(hostLockPollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			_ = f.Close()
			return nil, fmt.Errorf("wait for target lock %s: %w", targetRef, ctx.Err())
		case <-timer.C:
		}
	}
}

// targetLockSlot lets Run retain ownership from readiness through its exact
// lease push and reclaim. A standalone Check releases it before returning.
type (
	targetLockSlot    struct{ lock *targetLock }
	targetLockSlotKey struct{}
)

func withTargetLockSlot(ctx context.Context) (context.Context, *targetLockSlot) {
	slot := &targetLockSlot{}
	return context.WithValue(ctx, targetLockSlotKey{}, slot), slot
}

func (s *targetLockSlot) release() {
	if s != nil {
		s.lock.release()
		s.lock = nil
	}
}

func holdTargetLock(ctx context.Context, g gitRepo, targetRef string, notice io.Writer) (func(), error) {
	if slot, ok := ctx.Value(targetLockSlotKey{}).(*targetLockSlot); ok && slot != nil {
		if slot.lock != nil {
			return func() {}, nil
		}
		lock, err := acquireTargetLock(ctx, g, targetRef, notice)
		if err != nil {
			return nil, err
		}
		slot.lock = lock
		return func() {}, nil
	}
	lock, err := acquireTargetLock(ctx, g, targetRef, notice)
	if err != nil {
		return nil, err
	}
	return lock.release, nil
}

// preflightTargetRef determines destination ownership using only local Git
// reads. Fetch happens only after this lock is held. The returned identity is
// a full destination ref, independent of local remote-tracking spellings.
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
