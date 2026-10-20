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

// admissionTicket is the queue entry. Repository, Branch and Command are
// display-only and optional: tickets written before they existed parse with
// them empty and are rendered as unknown, never guessed.
type admissionTicket struct {
	Sequence   uint64    `json:"sequence"`
	PID        int       `json:"pid"`
	CreatedAt  time.Time `json:"created_at"`
	Repository string    `json:"repository,omitempty"`
	Branch     string    `json:"branch,omitempty"`
	Command    string    `json:"command,omitempty"`
}
type queuedTicket struct {
	admissionTicket
	path string
}

func acquireAdmission(ctx context.Context, dir string, wait time.Duration, notice io.Writer) (func(), error) {
	return acquireAdmissionAs(ctx, dir, hostLockHolder{}, wait, notice)
}

// acquireAdmissionAs queues holder's identity on its ticket and names it, the
// elapsed wait and the head of the queue in each wait notice, so a log reader
// can reconstruct the wait the way it does for the host measurement lock.
func acquireAdmissionAs(ctx context.Context, dir string, holder hostLockHolder, wait time.Duration, notice io.Writer) (func(), error) {
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
	// lastQueue is the newest queue a turn observed. A turn that finds the
	// admission lock busy sees none, so notices fall back to this snapshot.
	var lastQueue []queuedTicket
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
		if admissionWaitExpired(start, wait) {
			return nil, fmt.Errorf("%w after %s; admission ticket canceled", ErrHostLockWait, wait)
		}
		first, queue, err := admissionTurn(lockFile, dir, holder, &ticketPath)
		if err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if admissionWaitExpired(start, wait) {
			return nil, fmt.Errorf("%w after %s; admission ticket canceled", ErrHostLockWait, wait)
		}
		if queue != nil {
			lastQueue = queue
		}
		if first {
			acquired = true
			return func() { warnAdmissionCleanup(ctx, notice, dir, ticketPath) }, nil
		}
		now := time.Now()
		if lastNotice.IsZero() || now.Sub(lastNotice) >= hostLockNoticeInterval {
			lastNotice = now
			// The device label trails the queue description, as in the host
			// measurement lock notice.
			fmt.Fprintf(notice, "integrate: waiting %s for FIFO resource admission; %s; %s\n",
				now.Sub(start).Round(time.Second), describeAdmissionQueue(lastQueue, ticketPath, now), hostLockDeviceLabel(holder))
			if err := observeAdmissionWait(ctx, start, wait); err != nil {
				return nil, err
			}
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

func admissionWaitExpired(start time.Time, wait time.Duration) bool {
	return wait > 0 && time.Since(start) >= wait
}

func observeAdmissionWait(ctx context.Context, start time.Time, wait time.Duration) error {
	remaining := time.Duration(0)
	if wait > 0 {
		remaining = wait - time.Since(start)
	}
	err := observeResourceWait(ctx, remaining, wait > 0)
	if admissionWaitExpired(start, wait) && !targetMovedWhileWaiting(err) {
		return fmt.Errorf("%w after %s; admission ticket canceled", ErrHostLockWait, wait)
	}
	return err
}

// admissionTurn returns whether this waiter heads the queue, plus the live
// queue it observed, which is nil when the lock was busy.
func admissionTurn(lockFile *os.File, dir string, holder hostLockHolder, ticketPath *string) (first bool, queue []queuedTicket, err error) {
	locked, err := tryLockFile(lockFile)
	if err != nil || !locked {
		return false, nil, err
	}
	defer func() {
		if unlockErr := unlockFile(lockFile); unlockErr != nil && err == nil {
			err = unlockErr
		}
	}()
	if *ticketPath == "" {
		seq, err := nextAdmissionSequence(dir)
		if err != nil {
			return false, nil, err
		}
		*ticketPath, err = writeAdmissionTicket(dir, admissionTicket{
			Sequence: seq, PID: os.Getpid(), CreatedAt: time.Now().UTC(),
			Repository: holder.Repository, Branch: holder.Branch, Command: holder.Command,
		})
		if err != nil {
			return false, nil, err
		}
	}
	tickets, err := liveAdmissionTickets(dir)
	if err != nil {
		return false, nil, err
	}
	return len(tickets) > 0 && tickets[0].path == *ticketPath, tickets, nil
}

// describeAdmissionQueue names this waiter's position and the head ticket.
// Fields an older ticket never recorded read "unknown".
func describeAdmissionQueue(queue []queuedTicket, ticketPath string, now time.Time) string {
	if len(queue) == 0 {
		return "queue unknown (not observed)"
	}
	position := "unknown"
	for i, t := range queue {
		if t.path == ticketPath {
			position = strconv.Itoa(i + 1)
		}
	}
	head := queue[0]
	age := "unknown"
	queued := "unknown"
	if !head.CreatedAt.IsZero() {
		age = now.Sub(head.CreatedAt).Round(time.Second).String()
		queued = head.CreatedAt.UTC().Format(time.RFC3339)
	}
	return fmt.Sprintf("position %s of %d; head ticket sequence %d, pid %d, repository %s, branch %s, command %s, queued %s, age %s",
		position, len(queue), head.Sequence, head.PID,
		admissionField(head.Repository), admissionField(head.Branch), admissionField(head.Command), queued, age)
}

func admissionField(v string) string {
	if v == "" {
		return "unknown"
	}
	return v
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
