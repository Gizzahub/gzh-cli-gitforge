// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	admissionLockName    = "integrate.admission.lock"
	admissionCounterName = "integrate.admission.counter"
	admissionTicketsDir  = "integrate.admission"
	admissionCleanupWait = 5 * time.Second
)

type admissionTicket struct {
	Sequence  uint64    `json:"sequence"`
	PID       int       `json:"pid"`
	CreatedAt time.Time `json:"created_at"`
}
type queuedTicket struct {
	admissionTicket
	path string
}

func acquireAdmission(ctx context.Context, dir string, wait time.Duration, notice io.Writer) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(dir, admissionTicketsDir), 0o700); err != nil {
		return nil, fmt.Errorf("create admission queue: %w", err)
	}
	lockPath := filepath.Join(dir, admissionLockName)
	lockFile, err := os.OpenFile(lockPath, os.O_RDWR|os.O_CREATE, 0o600) // #nosec G304 -- fixed local state name
	if err != nil {
		return nil, err
	}
	defer lockFile.Close()
	if notice == nil {
		notice = os.Stderr
	}
	start := time.Now()
	var lastNotice time.Time
	var ticketPath string
	acquired := false
	defer func() {
		if !acquired && ticketPath != "" {
			warnAdmissionCleanup(ctx, notice, dir, ticketPath)
		}
	}()
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if wait > 0 && time.Since(start) >= wait {
			return nil, fmt.Errorf("%w after %s; admission ticket canceled", ErrHostLockWait, wait)
		}
		first, err := admissionTurn(lockFile, dir, &ticketPath)
		if err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if wait > 0 && time.Since(start) >= wait {
			return nil, fmt.Errorf("%w after %s; admission ticket canceled", ErrHostLockWait, wait)
		}
		if first {
			acquired = true
			return func() { warnAdmissionCleanup(ctx, notice, dir, ticketPath) }, nil
		}
		now := time.Now()
		if lastNotice.IsZero() || now.Sub(lastNotice) >= hostLockNoticeInterval {
			lastNotice = now
			fmt.Fprintf(notice, "integrate: waiting for FIFO resource admission\n")
		}
		t := time.NewTimer(hostLockPollInterval)
		select {
		case <-ctx.Done():
			t.Stop()
			return nil, ctx.Err()
		case <-t.C:
		}
	}
}

func admissionTurn(lockFile *os.File, dir string, ticketPath *string) (first bool, err error) {
	locked, err := tryLockFile(lockFile)
	if err != nil || !locked {
		return false, err
	}
	defer func() {
		if unlockErr := unlockFile(lockFile); unlockErr != nil && err == nil {
			err = unlockErr
		}
	}()
	if *ticketPath == "" {
		seq, err := nextAdmissionSequence(dir)
		if err != nil {
			return false, err
		}
		*ticketPath, err = writeAdmissionTicket(dir, admissionTicket{Sequence: seq, PID: os.Getpid(), CreatedAt: time.Now().UTC()})
		if err != nil {
			return false, err
		}
	}
	tickets, err := liveAdmissionTickets(dir)
	if err != nil {
		return false, err
	}
	return len(tickets) > 0 && tickets[0].path == *ticketPath, nil
}

func nextAdmissionSequence(dir string) (uint64, error) {
	path := filepath.Join(dir, admissionCounterName)
	data, err := os.ReadFile(path) // #nosec G304 -- fixed queue counter below host state root
	if err != nil && !os.IsNotExist(err) {
		return 0, err
	}
	var value uint64
	if len(data) > 0 {
		value, err = strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse admission counter: %w", err)
		}
	}
	tickets, err := liveAdmissionTickets(dir)
	if err != nil {
		return 0, err
	}
	for _, ticket := range tickets {
		if ticket.Sequence > value {
			value = ticket.Sequence
		}
	}
	if value == math.MaxUint64 {
		return 0, fmt.Errorf("admission counter overflow")
	}
	value++
	if err := atomicStateWrite(path, []byte(strconv.FormatUint(value, 10)+"\n")); err != nil {
		return 0, err
	}
	return value, nil
}

func writeAdmissionTicket(dir string, ticket admissionTicket) (string, error) {
	data, err := json.Marshal(ticket)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, admissionTicketsDir, fmt.Sprintf("%020d-%d.json", ticket.Sequence, ticket.PID))
	if err := atomicStateWrite(path, append(data, '\n')); err != nil {
		return "", err
	}
	return path, nil
}

func liveAdmissionTickets(dir string) ([]queuedTicket, error) {
	entries, err := os.ReadDir(filepath.Join(dir, admissionTicketsDir))
	if err != nil {
		return nil, err
	}
	var tickets []queuedTicket
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, admissionTicketsDir, entry.Name())
		data, err := os.ReadFile(path) // #nosec G304 -- entry originates from the fixed local queue directory
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		var ticket admissionTicket
		if err := json.Unmarshal(data, &ticket); err != nil || ticket.Sequence == 0 || ticket.PID <= 0 || entry.Name() != fmt.Sprintf("%020d-%d.json", ticket.Sequence, ticket.PID) {
			return nil, fmt.Errorf("ambiguous admission ticket %s", path)
		}
		alive, err := processAlive(ticket.PID)
		if err != nil {
			return nil, err
		}
		if !alive {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return nil, err
			}
			continue
		}
		tickets = append(tickets, queuedTicket{admissionTicket: ticket, path: path})
	}
	sort.Slice(tickets, func(i, j int) bool { return tickets[i].Sequence < tickets[j].Sequence })
	return tickets, nil
}

func atomicStateWrite(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	return syncAdmissionDirectory(filepath.Dir(path))
}

func removeAdmissionTicket(ctx context.Context, dir, path string) error {
	if path == "" {
		return nil
	}
	lock, err := os.OpenFile(filepath.Join(dir, admissionLockName), os.O_RDWR|os.O_CREATE, 0o600) // #nosec G304 -- fixed state name
	if err != nil {
		return err
	}
	defer lock.Close()
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("wait to remove admission ticket: %w", err)
		}
		locked, err := tryLockFile(lock)
		if err != nil {
			return err
		}
		if locked {
			break
		}
		timer := time.NewTimer(hostLockPollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("wait to remove admission ticket: %w", ctx.Err())
		case <-timer.C:
		}
	}
	defer func() {
		if err := unlockFile(lock); err != nil {
			fmt.Fprintf(os.Stderr, "integrate: warning: unlock FIFO admission queue: %v\n", err)
		}
	}()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func warnAdmissionCleanup(parent context.Context, notice io.Writer, dir, path string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), admissionCleanupWait)
	defer cancel()
	if err := removeAdmissionTicket(ctx, dir, path); err != nil {
		fmt.Fprintf(notice, "integrate: warning: remove FIFO resource admission ticket %s: %v\n", path, err)
	}
}
