// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const admissionProcessChildEnv = "GZ_GIT_TEST_ADMISSION_PROCESS"

// TestAdmissionProcessChild is re-executed by the process tests below. TestMain
// preserves their private XDG_STATE_HOME because admissionChildEnv is set.
func TestAdmissionProcessChild(t *testing.T) {
	mode := os.Getenv(admissionProcessChildEnv)
	if mode == "" {
		t.Skip("admission process child")
	}
	dir, err := hostSlotsDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	releasePath := os.Getenv("GZ_GIT_TEST_ADMISSION_RELEASE")
	waitRelease := func() {
		for {
			if _, statErr := os.Stat(releasePath); statErr == nil {
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
	}

	switch mode {
	case "legacy-holder":
		file, openErr := os.OpenFile(filepath.Join(dir, integrateLockName), os.O_RDWR|os.O_CREATE, 0o600) // #nosec G304 -- fixed private test state path
		if openErr != nil {
			t.Fatal(openErr)
		}
		defer file.Close()
		for {
			locked, lockErr := tryLockFile(file)
			if lockErr != nil {
				t.Fatal(lockErr)
			}
			if locked {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		fmt.Println("locked")
		waitRelease()
		if err := unlockFile(file); err != nil {
			t.Fatal(err)
		}
	case "admission-holder":
		release, acquireErr := acquireAdmission(context.Background(), dir, 0, io.Discard)
		if acquireErr != nil {
			t.Fatal(acquireErr)
		}
		defer release()
		fmt.Println("admitted")
		waitRelease()
	case "lock-waiter":
		wait, parseErr := time.ParseDuration(os.Getenv("GZ_GIT_TEST_ADMISSION_WAIT"))
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		budget := os.Getenv("GZ_GIT_TEST_ADMISSION_LABEL") == "budget"
		if budget {
			fmt.Println("starting budget")
		}
		started := time.Now()
		lock, acquireErr := acquireHostLock(context.Background(), hostLockHolder{Branch: os.Getenv("GZ_GIT_TEST_ADMISSION_LABEL")}, wait, io.Discard)
		if acquireErr != nil {
			fmt.Println("error " + acquireErr.Error())
			if budget {
				fmt.Println("elapsed " + time.Since(started).String())
			}
			return
		}
		defer lock.release()
		fmt.Println("acquired " + os.Getenv("GZ_GIT_TEST_ADMISSION_LABEL"))
		waitRelease()
	case "cancel-waiter":
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			waitRelease()
			cancel()
		}()
		_, acquireErr := acquireHostLock(ctx, hostLockHolder{Branch: "canceled"}, 0, io.Discard)
		if !errors.Is(acquireErr, context.Canceled) {
			t.Fatalf("acquire err = %v, want canceled", acquireErr)
		}
		fmt.Println("canceled")
	default:
		t.Fatalf("unknown child mode %q", mode)
	}
}

type admissionProcess struct {
	cmd     *exec.Cmd
	stdout  *bufio.Reader
	release string
	cancel  context.CancelFunc
	mu      sync.Mutex
	err     error
	waited  bool
}

func startAdmissionProcess(t *testing.T, mode, label, wait string) *admissionProcess {
	t.Helper()
	release := filepath.Join(t.TempDir(), "release")
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestAdmissionProcessChild$")
	raceOptions := strings.TrimSpace(os.Getenv("GORACE"))
	if raceOptions != "" {
		raceOptions += " "
	}
	raceOptions += "atexit_sleep_ms=0"
	cmd.Env = append(
		os.Environ(),
		admissionChildEnv+"=1",
		admissionProcessChildEnv+"="+mode,
		"GZ_GIT_TEST_ADMISSION_RELEASE="+release,
		"GZ_GIT_TEST_ADMISSION_LABEL="+label,
		"GZ_GIT_TEST_ADMISSION_WAIT="+wait,
		"GORACE="+raceOptions,
	)
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	p := &admissionProcess{cmd: cmd, stdout: bufio.NewReader(pipe), release: release, cancel: cancel}
	t.Cleanup(func() {
		_ = os.WriteFile(p.release, []byte("release\n"), 0o600)
		_ = p.finish(2 * time.Second)
		p.cancel()
	})
	return p
}

func (p *admissionProcess) line(t *testing.T) string {
	t.Helper()
	line, err := p.stdout.ReadString('\n')
	if err != nil {
		t.Fatalf("read child %d: %v", p.cmd.Process.Pid, err)
	}
	return strings.TrimSpace(line)
}

func (p *admissionProcess) signalRelease(t *testing.T) {
	t.Helper()
	if err := os.WriteFile(p.release, []byte("release\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (p *admissionProcess) releaseAndWait(t *testing.T) {
	t.Helper()
	p.signalRelease(t)
	err := p.finish(3 * time.Second)
	if err != nil {
		t.Fatalf("child %d: %v", p.cmd.Process.Pid, err)
	}
}

func (p *admissionProcess) wait(t *testing.T) error {
	t.Helper()
	if err := p.finish(3 * time.Second); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.err
}

// finish starts Cmd.Wait only after every expected stdout line was read. The
// os/exec contract closes a StdoutPipe from Wait, so starting it sooner races
// an otherwise live child's protocol output.
func (p *admissionProcess) finish(timeout time.Duration) error {
	p.mu.Lock()
	if p.waited {
		err := p.err
		p.mu.Unlock()
		return err
	}
	p.mu.Unlock()
	done := make(chan error, 1)
	go func() { done <- p.cmd.Wait() }()
	var err error
	select {
	case err = <-done:
	case <-time.After(timeout):
		p.cancel()
		err = <-done
	}
	p.mu.Lock()
	p.waited = true
	p.err = err
	p.mu.Unlock()
	return err
}

func (p *admissionProcess) readLineAsync() <-chan string {
	line := make(chan string, 1)
	go func() {
		text, err := p.stdout.ReadString('\n')
		if err != nil {
			line <- "read error: " + err.Error()
			return
		}
		line <- strings.TrimSpace(text)
	}()
	return line
}

func TestAdmissionProcessFIFOWithLegacyLockHolder(t *testing.T) {
	fastHostLockPolling(t)
	legacy := startAdmissionProcess(t, "legacy-holder", "", "0")
	if got := legacy.line(t); got != "locked" {
		t.Fatalf("legacy holder = %q", got)
	}
	first := startAdmissionProcess(t, "lock-waiter", "first", "2s")
	waitAdmissionTickets(t, 1)
	second := startAdmissionProcess(t, "lock-waiter", "second", "2s")
	waitAdmissionTickets(t, 2)
	third := startAdmissionProcess(t, "lock-waiter", "third", "2s")
	waitAdmissionTickets(t, 3)

	legacy.releaseAndWait(t)
	if got := first.line(t); got != "acquired first" {
		t.Fatalf("first acquisition = %q", got)
	}
	secondLine := second.readLineAsync()
	thirdLine := third.readLineAsync()
	select {
	case got := <-secondLine:
		t.Fatalf("second acquired while first held the only slot: %q", got)
	case got := <-thirdLine:
		t.Fatalf("third acquired while first held the only slot: %q", got)
	case <-time.After(150 * time.Millisecond):
	}
	first.releaseAndWait(t)
	if got := <-secondLine; got != "acquired second" {
		t.Fatalf("second acquisition = %q", got)
	}
	select {
	case got := <-thirdLine:
		t.Fatalf("third acquired while second held the only slot: %q", got)
	case <-time.After(100 * time.Millisecond):
	}
	second.releaseAndWait(t)
	if got := <-thirdLine; got != "acquired third" {
		t.Fatalf("third acquisition = %q", got)
	}
	third.releaseAndWait(t)
}

func TestAdmissionProcessCanceledMiddleWaiterDoesNotBlockSuccessors(t *testing.T) {
	fastHostLockPolling(t)
	legacy := startAdmissionProcess(t, "legacy-holder", "", "0")
	if got := legacy.line(t); got != "locked" {
		t.Fatalf("legacy holder = %q", got)
	}
	first := startAdmissionProcess(t, "lock-waiter", "first", "2s")
	waitAdmissionTickets(t, 1)
	middle := startAdmissionProcess(t, "cancel-waiter", "", "0")
	waitAdmissionTickets(t, 2)
	last := startAdmissionProcess(t, "lock-waiter", "last", "2s")
	waitAdmissionTickets(t, 3)
	middle.signalRelease(t)
	if got := middle.line(t); got != "canceled" {
		t.Fatalf("middle = %q", got)
	}
	if err := middle.wait(t); err != nil {
		t.Fatalf("middle child: %v", err)
	}
	waitAdmissionTickets(t, 2)

	legacy.releaseAndWait(t)
	if got := first.line(t); got != "acquired first" {
		t.Fatalf("first acquisition = %q", got)
	}
	first.releaseAndWait(t)
	if got := last.line(t); got != "acquired last" {
		t.Fatalf("last acquisition = %q", got)
	}
	last.releaseAndWait(t)
}

func TestAdmissionProcessBudgetIncludesFIFOAndOSWait(t *testing.T) {
	fastHostLockPolling(t)
	head := startAdmissionProcess(t, "admission-holder", "", "0")
	if got := head.line(t); got != "admitted" {
		t.Fatalf("admission holder = %q", got)
	}
	legacy := startAdmissionProcess(t, "legacy-holder", "", "0")
	if got := legacy.line(t); got != "locked" {
		t.Fatalf("legacy holder = %q", got)
	}
	waiter := startAdmissionProcess(t, "lock-waiter", "budget", "2s")
	if got := waiter.line(t); got != "starting budget" {
		t.Fatalf("budget handshake = %q", got)
	}
	waitAdmissionTickets(t, 2)
	time.Sleep(750 * time.Millisecond)
	head.signalRelease(t)
	// The waiter now leaves FIFO admission but the legacy lock consumes its
	// remaining budget. It must fail instead of restarting a second timeout.
	if got := waiter.line(t); !strings.HasPrefix(got, "error ") || !strings.Contains(got, ErrHostLockWait.Error()) || !strings.Contains(got, "after 2s (explicit; per acquisition)") || !strings.Contains(got, "no gate measured") || !strings.Contains(got, " holds ") || strings.Contains(got, "acquire FIFO") || strings.Contains(got, "admission consumed") {
		t.Fatalf("budget waiter = %q", got)
	}
	elapsed, err := time.ParseDuration(strings.TrimPrefix(waiter.line(t), "elapsed "))
	if err != nil {
		t.Fatal(err)
	}
	if elapsed < 1800*time.Millisecond || elapsed > 2400*time.Millisecond {
		t.Fatalf("total acquisition wait = %s, want one 2s budget spanning FIFO and OS wait", elapsed)
	}
	if err := waiter.wait(t); err != nil {
		t.Fatalf("budget waiter child: %v", err)
	}
	if err := head.wait(t); err != nil {
		t.Fatalf("admission holder child: %v", err)
	}
	legacy.releaseAndWait(t)
}
