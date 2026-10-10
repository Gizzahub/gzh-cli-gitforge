// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The host measurement lock follows ce-devenv ADR-0001 phase 1: one
// integration measurement at a time per host, decided by an OS file lock on
// host-slots/integrate.lock, with a display-only holder sidecar beside it.
// The contended resource is the machine's CPU, so the lock is host-wide and
// lives outside every repository and worktree.
const (
	hostSlotsDirName     = "host-slots"
	integrateLockName    = "integrate.lock"
	hostLockWorkClass    = "integration"
	hostLockCommandCheck = "check"
	hostLockCommandRun   = "run"
)

// ErrHostLockWait reports that --lock-wait expired before the host
// measurement lock was free. Nothing was measured.
var ErrHostLockWait = errors.New("host measurement lock wait expired")

// Tests shorten these; production polls a non-blocking lock so that
// cancellation, --lock-wait, and the periodic holder notice need no
// interruptible blocking lock.
var (
	hostLockPollInterval   = 200 * time.Millisecond
	hostLockNoticeInterval = 30 * time.Second
)

// hostLockHolder is the sidecar content. It is for display only: whoever
// reads it must already have failed to take the lock, or it may describe a
// holder that is gone.
type hostLockHolder struct {
	PID        int       `json:"pid"`
	Repository string    `json:"repository"`
	Worktree   string    `json:"worktree"`
	Branch     string    `json:"branch"`
	Command    string    `json:"command"`
	WorkClass  string    `json:"work_class"`
	StartedAt  time.Time `json:"started_at"`
}

type hostLock struct {
	file    *os.File
	sidecar string
}

// hostSlotsDir resolves ${XDG_STATE_HOME:-$HOME/.local/state}/host-slots. A
// relative XDG_STATE_HOME is ignored, as the XDG spec requires and as
// writeDiagnostic does.
func hostSlotsDir() (string, error) {
	stateRoot := os.Getenv("XDG_STATE_HOME")
	if !filepath.IsAbs(stateRoot) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home for host measurement lock: %w", err)
		}
		stateRoot = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(stateRoot, hostSlotsDirName), nil
}

// acquireHostLock waits for the host measurement lock and then records the
// holder. wait <= 0 waits without bound. While waiting it names the current
// holder on notice at once and then every hostLockNoticeInterval. It never
// returns without the lock unless it returns an error.
func acquireHostLock(ctx context.Context, holder hostLockHolder, wait time.Duration, notice io.Writer) (*hostLock, error) {
	dir, err := hostSlotsDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil { // #nosec G301 -- shared host-slot convention; the lock file holds no data
		return nil, fmt.Errorf("create host measurement lock directory: %w", err)
	}
	path := filepath.Join(dir, integrateLockName)
	// Never O_EXCL and never removed: the file is a stable inode to lock, not
	// a marker. Deleting it would let a newcomer lock a fresh inode while the
	// holder still owns the old one.
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644) // #nosec G302 G304 -- fixed name under the resolved host-slots directory
	if err != nil {
		return nil, fmt.Errorf("open host measurement lock: %w", err)
	}
	if notice == nil {
		notice = os.Stderr
	}
	sidecar := path + ".json"
	start := time.Now()
	var lastNotice time.Time
	for {
		locked, err := tryLockFile(f)
		if err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("lock %s: %w", path, err)
		}
		if locked {
			break
		}
		now := time.Now()
		if wait > 0 && now.Sub(start) >= wait {
			_ = f.Close()
			return nil, fmt.Errorf("%w after %s; %s holds %s; no make target ran", ErrHostLockWait, wait, describeHostLockHolder(sidecar), path)
		}
		if lastNotice.IsZero() || now.Sub(lastNotice) >= hostLockNoticeInterval {
			lastNotice = now
			fmt.Fprintf(notice, "integrate: waiting %s for the host measurement lock %s; %s\n",
				now.Sub(start).Round(time.Second), path, describeHostLockHolder(sidecar))
		}
		timer := time.NewTimer(hostLockPollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			_ = f.Close()
			return nil, fmt.Errorf("wait for host measurement lock: %w", ctx.Err())
		case <-timer.C:
		}
	}
	holder.PID = os.Getpid()
	holder.WorkClass = hostLockWorkClass
	holder.StartedAt = time.Now().UTC()
	if err := writeHostLockSidecar(sidecar, holder); err != nil {
		// The sidecar is display only; the lock is held either way.
		fmt.Fprintf(notice, "integrate: warning: write host measurement lock holder %s: %v\n", sidecar, err)
	}
	return &hostLock{file: f, sidecar: sidecar}, nil
}

// release drops the sidecar while the lock still excludes everyone else, so
// it can only remove this holder's record, then releases the lock by closing
// the file.
func (l *hostLock) release() {
	if l == nil || l.file == nil {
		return
	}
	_ = os.Remove(l.sidecar)
	_ = l.file.Close()
	l.file = nil
}

func writeHostLockSidecar(path string, holder hostLockHolder) error {
	data, err := json.MarshalIndent(holder, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	_, writeErr := tmp.Write(append(data, '\n'))
	closeErr := tmp.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(tmp.Name())
		return firstDiagnosticError(writeErr, closeErr)
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil { // #nosec G302 -- display-only holder record other tools read
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return nil
}

// describeHostLockHolder is only called after a failed lock attempt, so the
// sidecar is read while someone holds the lock. It can still be missing (the
// holder has not written it yet) or stale (a killed holder left it and the
// new holder has not replaced it yet), so it is reported, never trusted.
func describeHostLockHolder(sidecar string) string {
	data, err := os.ReadFile(sidecar) // #nosec G304 -- fixed name beside the host measurement lock
	if err != nil {
		return "holder unknown (no readable " + filepath.Base(sidecar) + ")"
	}
	var h hostLockHolder
	if err := json.Unmarshal(data, &h); err != nil || h.PID == 0 {
		return "holder unknown (unparseable " + filepath.Base(sidecar) + ")"
	}
	parts := []string{fmt.Sprintf("pid %d", h.PID)}
	for _, kv := range [][2]string{
		{"repository", h.Repository},
		{"worktree", h.Worktree},
		{"branch", h.Branch},
		{"command", h.Command},
	} {
		if kv[1] != "" {
			parts = append(parts, kv[0]+" "+kv[1])
		}
	}
	if !h.StartedAt.IsZero() {
		parts = append(parts, "started "+h.StartedAt.Format(time.RFC3339))
	}
	return "held by " + strings.Join(parts, ", ")
}

// measurementLockSlot carries the lock from the Check that acquired it to
// the Run that must keep holding it through the push.
type measurementLockSlot struct{ lock *hostLock }

type measurementLockSlotKey struct{}

// withMeasurementLockSlot marks ctx so that Check hands its lock to the
// caller instead of releasing it on return. The caller releases the slot.
func withMeasurementLockSlot(ctx context.Context) (context.Context, *measurementLockSlot) {
	slot := &measurementLockSlot{}
	return context.WithValue(ctx, measurementLockSlotKey{}, slot), slot
}

func (s *measurementLockSlot) release() {
	if s != nil {
		s.lock.release()
		s.lock = nil
	}
}

// holdMeasurementLock takes the host measurement lock for Check, right before
// the target is fetched and resolved, so waiting precedes both the fetch and
// every make budget. Under Run it parks the lock in Run's slot and returns a
// no-op release.
func holdMeasurementLock(ctx context.Context, g gitRepo, opts CheckOptions) (func(), error) {
	var slot *measurementLockSlot
	if s, ok := ctx.Value(measurementLockSlotKey{}).(*measurementLockSlot); ok {
		slot = s
	}
	if slot != nil && slot.lock != nil {
		return func() {}, nil
	}
	command := hostLockCommandCheck
	if slot != nil {
		command = hostLockCommandRun
	}
	holder := hostLockHolder{Worktree: g.dir, Repository: g.dir, Branch: opts.Branch, Command: command}
	if common, err := g.output(ctx, "rev-parse", "--path-format=absolute", "--git-common-dir"); err == nil && common != "" {
		if filepath.Base(common) == ".git" {
			common = filepath.Dir(common)
		}
		holder.Repository = common
	}
	if holder.Branch == "" {
		if branch, err := g.currentBranch(ctx); err == nil {
			holder.Branch = branch
		}
	}
	lock, err := acquireHostLock(ctx, holder, opts.LockWait, opts.LockNotice)
	if err != nil {
		return nil, err
	}
	if slot != nil {
		slot.lock = lock
		return func() {}, nil
	}
	return lock.release, nil
}
