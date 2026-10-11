// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

const admissionChildEnv = "GZ_GIT_TEST_ADMISSION_CHILD"

func TestAdmissionChild(t *testing.T) {
	if os.Getenv(admissionChildEnv) == "" {
		t.Skip("admission child")
	}
	dir, err := hostSlotsDir()
	if err != nil {
		t.Fatal(err)
	}
	release, err := acquireAdmission(context.Background(), dir, 0, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	_, _ = os.Stdout.WriteString("admitted\n")
	select {}
}

func TestAdmissionFIFO(t *testing.T) {
	fastHostLockPolling(t)
	holder := holdHostLockInProcess(t, "/holder", "holder")
	t.Cleanup(holder.release)
	type result struct {
		lock *hostLock
		err  error
	}
	first := make(chan result, 1)
	second := make(chan result, 1)
	go func() {
		lock, err := acquireHostLock(context.Background(), hostLockHolder{Branch: "first"}, 5*time.Second, io.Discard)
		first <- result{lock, err}
	}()
	waitAdmissionTickets(t, 1)
	go func() {
		lock, err := acquireHostLock(context.Background(), hostLockHolder{Branch: "second"}, 5*time.Second, io.Discard)
		second <- result{lock, err}
	}()
	waitAdmissionTickets(t, 2)
	holder.release()
	one := <-first
	if one.err != nil {
		t.Fatalf("first admission: %v", one.err)
	}
	select {
	case got := <-second:
		t.Fatalf("second bypassed first: %v", got.err)
	case <-time.After(100 * time.Millisecond):
	}
	one.lock.release()
	two := <-second
	if two.err != nil {
		t.Fatalf("second admission: %v", two.err)
	}
	two.lock.release()
}

func waitAdmissionTickets(t *testing.T, want int) {
	t.Helper()
	dir, err := hostSlotsDir()
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		tickets, err := liveAdmissionTickets(dir)
		if err == nil && len(tickets) == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("admission queue did not reach %d tickets", want)
}

func TestAdmissionRemovesPositivelyDeadTicket(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, admissionTicketsDir), 0o700); err != nil {
		t.Fatal(err)
	}
	path, err := writeAdmissionTicket(dir, admissionTicket{Sequence: 1, PID: 99999999, CreatedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	tickets, err := liveAdmissionTickets(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(tickets) != 0 {
		t.Fatalf("dead ticket remained: %#v", tickets)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("dead ticket file remains: %v", err)
	}
}

func TestAdmissionCancellationRemovesOwnTicket(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, admissionTicketsDir), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := writeAdmissionTicket(dir, admissionTicket{Sequence: 1, PID: os.Getpid(), CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := acquireAdmission(ctx, dir, 0, io.Discard); done <- err }()
	waitAdmissionTicketsAt(t, dir, 2)
	cancel()
	if err := <-done; err == nil {
		t.Fatal("canceled admission succeeded")
	}
	tickets, err := liveAdmissionTickets(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(tickets) != 1 || tickets[0].Sequence != 1 {
		t.Fatalf("tickets after cancellation = %#v", tickets)
	}
}

func TestAdmissionErrorRemovesOwnTicket(t *testing.T) {
	dir := t.TempDir()
	ticketsDir := filepath.Join(dir, admissionTicketsDir)
	if err := os.MkdirAll(ticketsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ticketsDir, "broken.json"), []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := acquireAdmission(context.Background(), dir, time.Second, io.Discard); err == nil {
		t.Fatal("admission unexpectedly accepted malformed queue")
	}
	entries, err := os.ReadDir(ticketsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "broken.json" {
		t.Fatalf("admission error left own ticket: %#v", entries)
	}
}

func TestAdmissionRecoversKilledProcessTicket(t *testing.T) {
	child := exec.Command(os.Args[0], "-test.run=^TestAdmissionChild$") //nolint:noctx // killed explicitly
	child.Env = append(os.Environ(), admissionChildEnv+"=1")
	stdout, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = child.Process.Kill(); _ = child.Wait() })
	if line, err := bufio.NewReader(stdout).ReadString('\n'); err != nil || line != "admitted\n" {
		t.Fatalf("child = %q, %v", line, err)
	}
	dir, err := hostSlotsDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = child.Wait()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		tickets, err := liveAdmissionTickets(dir)
		if err == nil && len(tickets) == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("killed ticket was not recovered")
}

func waitAdmissionTicketsAt(t *testing.T, dir string, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		tickets, err := liveAdmissionTickets(dir)
		if err == nil && len(tickets) == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("admission queue did not reach %d tickets", want)
}
