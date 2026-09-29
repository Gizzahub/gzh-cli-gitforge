package runtask

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type stateStore struct{ dir string }

func newStateStore(commonDir string) stateStore {
	return stateStore{dir: filepath.Join(commonDir, RuntimeStateDir, "task-runtime", "v1")}
}
func (s stateStore) init() error { return os.MkdirAll(s.dir, 0o755) } //nolint:gosec // CE-layout state directory parity; the path is the resolved git common dir
func (s stateStore) load() ([]TaskExecution, error) {
	b, err := os.ReadFile(filepath.Join(s.dir, "executions.json"))
	if os.IsNotExist(err) {
		return []TaskExecution{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read task executions: %w", err)
	}
	var out []TaskExecution
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("decode task executions: %w", err)
	}
	return out, nil
}
func (s stateStore) save(executions []TaskExecution) error {
	if err := s.init(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(executions, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(s.dir, fmt.Sprintf("executions.%d.tmp", time.Now().UnixNano()))
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil { //nolint:gosec // CE-layout state file parity
		return err
	}
	if err := syncFile(tmp); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, filepath.Join(s.dir, "executions.json")); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	syncDir(s.dir)
	return nil
}
func (s stateStore) appendReceipt(receipt TaskReceipt) error {
	if err := s.init(); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(s.dir, "receipts.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644) //nolint:gosec // append-only receipt log, CE-layout parity
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	stat, statErr := f.Stat()
	created := statErr == nil && stat.Size() == 0
	if err := json.NewEncoder(f).Encode(receipt); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if created {
		syncDir(s.dir)
	}
	return nil
}
func (s stateStore) latestReceipts() (map[string]TaskReceipt, error) {
	f, err := os.Open(filepath.Join(s.dir, "receipts.jsonl"))
	if os.IsNotExist(err) {
		return map[string]TaskReceipt{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return parseReceipts(f)
}

func parseReceipts(r io.Reader) (map[string]TaskReceipt, error) {
	out := map[string]TaskReceipt{}
	reader := bufio.NewReader(r)
	for line := 1; ; line++ {
		record, readErr := reader.ReadBytes('\n')
		if len(record) > 0 {
			record = bytes.TrimSuffix(record, []byte("\n"))
			if len(bytes.TrimSpace(record)) == 0 {
				return nil, fmt.Errorf("decode receipt line %d: empty receipt record", line)
			}
			var receipt TaskReceipt
			if err := json.Unmarshal(record, &receipt); err != nil {
				return nil, fmt.Errorf("decode receipt line %d: %w", line, err)
			}
			out[receipt.Task] = receipt
		}
		if errors.Is(readErr, io.EOF) {
			return out, nil
		}
		if readErr != nil {
			return nil, fmt.Errorf("read receipt line %d: %w", line, readErr)
		}
	}
}
func (s stateStore) lock() (func(), error) {
	if err := s.init(); err != nil {
		return nil, err
	}
	path := filepath.Join(s.dir, "mutation.lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644) //nolint:gosec // O_EXCL lock under the resolved state dir, not user input
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("task runtime mutation lock exists: %s", path)
		}
		return nil, err
	}
	failed := func(writeErr error) (func(), error) {
		_ = f.Close()
		_ = os.Remove(path)
		return nil, writeErr
	}
	if _, err := fmt.Fprintf(f, "pid=%d created=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339)); err != nil {
		return failed(fmt.Errorf("write mutation lock metadata: %w", err))
	}
	if err := f.Sync(); err != nil {
		return failed(fmt.Errorf("sync mutation lock: %w", err))
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("close mutation lock: %w", err)
	}
	syncDir(s.dir)
	return func() { _ = os.Remove(path) }, nil
}
func (s stateStore) lockPath() string { return filepath.Join(s.dir, "mutation.lock") }

func (s stateStore) lockInfo(now time.Time) (TaskLockInfo, error) {
	path := s.lockPath()
	b, err := os.ReadFile(path) //nolint:gosec // the path is this store's lock file under the git common dir
	if err != nil {
		return TaskLockInfo{}, err
	}
	info := TaskLockInfo{Path: path}
	for _, field := range strings.Fields(string(b)) {
		key, value, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}
		// A malformed field keeps its zero value and the lock still reports:
		// diagnostics must not fail because one line is unreadable.
		switch key {
		case "pid":
			if pid, convErr := strconv.Atoi(value); convErr == nil {
				info.PID = pid
			}
		case "created":
			if created, parseErr := time.Parse(time.RFC3339, value); parseErr == nil {
				info.CreatedAt = created
			}
		}
	}
	if !info.CreatedAt.IsZero() {
		age := now.UTC().Sub(info.CreatedAt)
		if age > 0 {
			info.AgeSeconds = int64(age.Seconds())
		}
	}
	if info.PID > 0 {
		process, findErr := os.FindProcess(info.PID)
		running := findErr == nil
		if running {
			signalErr := process.Signal(syscall.Signal(0))
			running = signalErr == nil || errors.Is(signalErr, syscall.EPERM)
		}
		info.PIDRunning = &running
	}
	return info, nil
}

func syncFile(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0) //nolint:gosec // syncing a file this store just wrote under its own dir
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return f.Sync()
}

// syncDir flushes a directory entry after a create or rename. Failure here
// only weakens crash durability of an already-written file, so it is
// deliberately best-effort at the single call site pattern: every caller
// treats it as advice, never as evidence.
func syncDir(path string) {
	dir, err := os.Open(path) //nolint:gosec // the path is a directory this store created under the git common dir
	if err != nil {
		return
	}
	_ = dir.Sync()
	_ = dir.Close()
}
