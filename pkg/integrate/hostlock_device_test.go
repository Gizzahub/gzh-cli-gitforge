// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

// TestHostLockNamesDevice keeps a copied notice or sidecar attributable: the
// lock path is identical on every device, so only the declared host tells
// two devices' output apart.
func TestHostLockNamesDevice(t *testing.T) {
	fastHostLockPolling(t)
	held, err := acquireHostLock(context.Background(), hostLockHolder{Host: "mst", Branch: "holder"}, time.Second, nil)
	if err != nil {
		t.Fatalf("hold host lock: %v", err)
	}
	defer held.release()

	data, err := os.ReadFile(held.sidecar)
	if err != nil {
		t.Fatalf("read sidecar: %v", err)
	}
	var sidecar hostLockHolder
	if err := json.Unmarshal(data, &sidecar); err != nil || sidecar.Host != "mst" {
		t.Fatalf("sidecar host = %q (err %v), want mst:\n%s", sidecar.Host, err, data)
	}

	for _, tc := range []struct {
		waiter hostLockHolder
		want   string
	}{
		{hostLockHolder{Host: "mst"}, "; host mst\n"},
		{hostLockHolder{}, "; host undeclared\n"},
		{hostLockHolder{hostRejected: true}, "; host invalid\n"},
	} {
		var notice bytes.Buffer
		tc.waiter.Branch = "waiter"
		_, err := acquireHostLockWithSource(context.Background(), tc.waiter, 120*time.Millisecond, "flag", &notice)
		if !errors.Is(err, ErrHostLockWait) {
			t.Fatalf("waiter %+v: err = %v, want ErrHostLockWait", tc.waiter, err)
		}
		line := strings.SplitAfter(notice.String(), "\n")[0]
		if !strings.Contains(line, "; held by pid ") || !strings.HasSuffix(line, tc.want) {
			t.Fatalf("waiter %+v: notice %q, want holder then %q", tc.waiter, line, tc.want)
		}
	}
}

// TestMeasurementLockRecordsDeclaredHost covers the production wiring: a
// declared high-load gate writes the caller's device into the sidecar.
func TestMeasurementLockRecordsDeclaredHost(t *testing.T) {
	t.Setenv("GIT_WORK_HOST", "mst")
	g := newGitRepo(gitcmd.NewExecutor(), t.TempDir())
	_, release, err := holdMeasurementLock(context.Background(), g, CheckOptions{Branch: "gate", LockWait: time.Second})
	if err != nil {
		t.Fatalf("hold measurement lock: %v", err)
	}
	defer release()

	dir, err := hostSlotsDir()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, integrateLockName+".json"))
	if err != nil {
		t.Fatalf("read sidecar: %v", err)
	}
	var sidecar hostLockHolder
	if err := json.Unmarshal(data, &sidecar); err != nil || sidecar.Host != "mst" {
		t.Fatalf("sidecar host = %q (err %v), want mst:\n%s", sidecar.Host, err, data)
	}
}

func TestResolveDeclaredDevice(t *testing.T) {
	global := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(global, []byte("[ce]\n\tworkHost = lab\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	g := newGitRepo(gitcmd.NewExecutor(), t.TempDir())
	ctx := context.Background()

	for _, tc := range []struct {
		env, want string
		rejected  bool
	}{
		{"mst", "mst", false},     // environment wins over global Git config
		{"", "lab", false},        // global ce.workHost
		{"bad host", "", true},    // malformed declaration is not repaired or guessed
		{"-mbp", "", true},        // option-like value is refused
		{"  mbp  ", "mbp", false}, // surrounding space is not part of the name
		{"MBP", "", true},         // outside the device catalog grammar
		{"m;x", "", true},         // a separator would split the notice field
	} {
		t.Setenv("GIT_WORK_HOST", tc.env)
		if got := resolveDeclaredDevice(ctx, g); got.Host != tc.want || got.Rejected != tc.rejected {
			t.Errorf("GIT_WORK_HOST=%q: host = %q rejected = %v, want %q rejected = %v", tc.env, got.Host, got.Rejected, tc.want, tc.rejected)
		}
	}

	t.Setenv("GIT_WORK_HOST", "")
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "missing"))
	if got := resolveDeclaredDevice(ctx, g); got.Host != "" || got.Rejected {
		t.Errorf("undeclared device: host = %q rejected = %v, want empty and not rejected (no hostname fallback)", got.Host, got.Rejected)
	}
}

// TestAdmissionNoticeNamesWaitHeadAndDevice keeps an admission wait
// reconstructable from output: elapsed time, the head ticket and the device.
func TestAdmissionNoticeNamesWaitHeadAndDevice(t *testing.T) {
	fastHostLockPolling(t)
	created := time.Now().UTC().Add(-90 * time.Second)

	for _, tc := range []struct {
		name   string
		head   admissionTicket
		waiter hostLockHolder
		want   []string
	}{
		{
			name:   "full head ticket",
			head:   admissionTicket{Sequence: 7, PID: os.Getpid(), CreatedAt: created, Repository: "/r/head", Branch: "head-branch", Command: "check"},
			waiter: hostLockHolder{Host: "mst", Repository: "/r/waiter", Branch: "waiter"},
			want: []string{
				"integrate: waiting ", " for FIFO resource admission; position 2 of 2; head ticket sequence 7, pid ",
				", repository /r/head, branch head-branch, command check, queued " + created.Format(time.RFC3339) + ", age 1m",
			},
		},
		{
			name:   "old ticket without identity fields",
			head:   admissionTicket{Sequence: 7, PID: os.Getpid(), CreatedAt: created},
			waiter: hostLockHolder{Host: "mst"},
			want:   []string{", repository unknown, branch unknown, command unknown, queued " + created.Format(time.RFC3339)},
		},
		{
			name:   "old ticket without created_at",
			head:   admissionTicket{Sequence: 7, PID: os.Getpid()},
			waiter: hostLockHolder{Host: "mst"},
			want:   []string{", queued unknown, age unknown; host mst\n"},
		},
		{
			name:   "undeclared device",
			head:   admissionTicket{Sequence: 7, PID: os.Getpid(), CreatedAt: created},
			waiter: hostLockHolder{},
			want:   []string{"; host undeclared\n"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.MkdirAll(filepath.Join(dir, admissionTicketsDir), 0o700); err != nil {
				t.Fatal(err)
			}
			if _, err := writeAdmissionTicket(dir, tc.head); err != nil {
				t.Fatal(err)
			}
			var notice bytes.Buffer
			_, err := acquireAdmissionAs(context.Background(), dir, tc.waiter, 120*time.Millisecond, &notice)
			if !errors.Is(err, ErrHostLockWait) {
				t.Fatalf("err = %v, want ErrHostLockWait", err)
			}
			line := strings.SplitAfter(notice.String(), "\n")[0]
			if !strings.HasSuffix(line, "\n") || !strings.Contains(line, "; host ") {
				t.Fatalf("notice %q does not end with a host label", line)
			}
			for _, w := range tc.want {
				if !strings.Contains(line, w) {
					t.Fatalf("notice %q missing %q", line, w)
				}
			}
		})
	}
}

// TestAdmissionTicketBackwardCompatible proves a ticket written before the
// identity fields existed still parses as a queue member.
func TestAdmissionTicketBackwardCompatible(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, admissionTicketsDir), 0o700); err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(dir, admissionTicketsDir, "00000000000000000003-"+strconv.Itoa(os.Getpid())+".json")
	legacy := `{"sequence":3,"pid":` + strconv.Itoa(os.Getpid()) + `,"created_at":"2026-10-18T00:00:00Z"}`
	if err := os.WriteFile(name, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	tickets, err := liveAdmissionTickets(dir)
	if err != nil || len(tickets) != 1 || tickets[0].Repository != "" {
		t.Fatalf("tickets = %+v, err = %v", tickets, err)
	}
	if got := describeAdmissionQueue(tickets, "", time.Date(2026, 10, 18, 0, 1, 0, 0, time.UTC)); !strings.Contains(got, "repository unknown") || !strings.Contains(got, "age 1m0s") {
		t.Fatalf("describe = %q", got)
	}
}

// syncBuffer lets a test read notices while the waiter is still writing.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) lines() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.SplitAfter(b.buf.String(), "\n")
}

// TestAdmissionNoticeKeepsLastQueueWhenLockBusy proves a notice that lands
// while another waiter holds the admission lock still names the head ticket
// seen earlier, and that "not observed" appears only before the first turn.
func TestAdmissionNoticeKeepsLastQueueWhenLockBusy(t *testing.T) {
	fastHostLockPolling(t)
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, admissionTicketsDir), 0o700); err != nil {
		t.Fatal(err)
	}
	created := time.Now().UTC().Add(-time.Minute)
	for _, seq := range []uint64{4, 5} {
		if _, err := writeAdmissionTicket(dir, admissionTicket{Sequence: seq, PID: os.Getpid(), CreatedAt: created, Repository: "/r/head", Branch: "b", Command: "check"}); err != nil {
			t.Fatal(err)
		}
	}
	// Position 3: two live tickets already ahead.
	busy, err := os.OpenFile(filepath.Join(dir, admissionLockName), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	if locked, err := tryLockFile(busy); err != nil || !locked {
		t.Fatalf("hold admission lock: locked=%v err=%v", locked, err)
	}

	var notice syncBuffer
	done := make(chan error, 1)
	go func() {
		_, err := acquireAdmissionAs(context.Background(), dir, hostLockHolder{Host: "mst"}, 1500*time.Millisecond, &notice)
		done <- err
	}()
	deadline := time.Now().Add(2 * time.Second)
	for len(notice.lines()) < 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if first := notice.lines()[0]; !strings.Contains(first, "queue unknown (not observed); host mst\n") {
		t.Fatalf("first notice %q, want fallback before any observation", first)
	}
	if err := unlockFile(busy); err != nil {
		t.Fatal(err)
	}
	// Let the waiter observe the queue once, then hold the lock again.
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		ls := notice.lines()
		if len(ls) > 1 && strings.Contains(ls[len(ls)-2], "position 3 of 3; head ticket sequence 4") {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	// The waiter's turn holds the lock briefly; retry until it is free.
	held := false
	deadline = time.Now().Add(2 * time.Second)
	for !held && time.Now().Before(deadline) {
		var err error
		if held, err = tryLockFile(busy); err != nil {
			t.Fatalf("re-hold admission lock: %v", err)
		}
		if !held {
			time.Sleep(time.Millisecond)
		}
	}
	if !held {
		t.Fatal("re-hold admission lock: not acquired before deadline")
	}
	// A notice after the re-hold must reuse the cached queue. Release before
	// the wait expires so ticket cleanup is not stalled on this lock.
	// lines() ends with an empty element after the last newline, so the next
	// notice lands at index n-1.
	n := len(notice.lines())
	deadline = time.Now().Add(2 * time.Second)
	for len(notice.lines()) <= n && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if ls := notice.lines(); len(ls) <= n || !strings.Contains(ls[n-1], "position 3 of 3; head ticket sequence 4") {
		t.Fatalf("notice after re-hold = %q, want cached position 3 of 3 queue", ls)
	}
	if err := unlockFile(busy); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, ErrHostLockWait) {
		t.Fatalf("err = %v, want ErrHostLockWait", err)
	}
	var observed bool
	for _, l := range notice.lines() {
		switch {
		case strings.Contains(l, "position 3 of 3; head ticket sequence 4"):
			observed = true
		case observed && strings.Contains(l, "queue unknown"):
			t.Fatalf("notice %q lost the observed queue", l)
		}
	}
	if !observed {
		t.Fatalf("no notice named position 3: %q", notice.lines())
	}
}
