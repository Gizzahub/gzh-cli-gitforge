// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

func TestLegacyCleanupCERuntimeGuard(t *testing.T) {
	// Each refusal must come from the guard check it targets. An earlier
	// failure (such as a dirty primary) also refuses, and would otherwise
	// pass every case without reaching the guard.
	wantRefusal := map[string]string{
		"malformed":       "parse CE runtime attestation",
		"wrong_guard":     "CE runtime attestation binding was refused or changed",
		"stale_pid":       "CE runtime attestation process identity changed",
		"hash_changed":    "CE verifier digest changed",
		"fail_after_step": "runtime attestation after remove step 0: parse CE runtime attestation",
		// A valid attestation padded past the cap must still be refused
		// (ISSUE-77): the stdout case parses as one JSON value.
		"oversized_stdout": "CE runtime attestation exceeds output limit",
		"oversized_stderr": "CE runtime attestation exceeds output limit",
	}
	for _, mode := range []string{"allow", "malformed", "wrong_guard", "stale_pid", "hash_changed", "fail_after_step", "oversized_stdout", "oversized_stderr"} {
		t.Run(mode, func(t *testing.T) {
			plan, planPath, primary, task := legacySecurityPlan(t, true)
			if err := os.MkdirAll(filepath.Join(primary, ".ce"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(primary, ".ce", "task-runtime.yaml"), []byte("schema_version: 1\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			// Without this the primary is dirty unless the developer's global
			// gitignore happens to list .ce/, which a CI runner's does not.
			excludeInClone(t, primary, ".ce/")
			verifier := buildLegacyGuardVerifier(t)
			guard := &LegacyRuntimeGuard{
				SchemaVersion: legacyRuntimeGuardSchema, Capability: legacyRuntimeGuardCapability,
				OperationID: strings.Repeat("a", 32), RepositoryRoot: plan.RepositoryRoot, CommonDir: plan.CommonDir,
				TaskBranch: plan.TaskBranch, TaskWorktree: plan.TaskWorktree, TaskOID: plan.TaskOID, Owner: plan.Owner,
				VerifierExecutable: verifier, VerifierSHA256: hashLegacyVerifier(t, verifier),
			}
			plan.RuntimeGuard = guard
			digest, err := legacyCleanupPlanDigest(plan)
			if err != nil {
				t.Fatal(err)
			}
			plan.PlanDigest = digest
			cePlanPath := filepath.Join(filepath.Dir(planPath), "ce-plan.json")
			if err := WriteLegacyCleanupPlan(cePlanPath, plan); err != nil {
				t.Fatal(err)
			}
			guardJSON, err := json.Marshal(guard)
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv("LEGACY_GUARD_JSON", string(guardJSON))
			t.Setenv("LEGACY_GUARD_MODE", mode)
			if mode == "fail_after_step" {
				t.Setenv("LEGACY_GUARD_COUNTER", filepath.Join(t.TempDir(), "calls"))
			}
			if mode == "hash_changed" {
				if err := os.WriteFile(verifier, []byte("changed"), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			err = ApplyLegacyCleanup(context.Background(), gitcmd.NewExecutor(), LegacyCleanupApplyOptions{PlanPath: cePlanPath, ApproveDigest: plan.PlanDigest, RuntimeAttempt: strings.Repeat("b", 32)})
			if mode == "allow" {
				if err != nil {
					t.Fatalf("guarded cleanup: %v", err)
				}
				if _, statErr := os.Stat(task); !os.IsNotExist(statErr) {
					t.Fatalf("task remains: %v", statErr)
				}
				return
			}
			if err == nil {
				t.Fatal("invalid CE attestation unexpectedly permitted cleanup")
			}
			if !strings.Contains(err.Error(), wantRefusal[mode]) {
				t.Fatalf("refusal did not come from the guard: err = %v, want %q", err, wantRefusal[mode])
			}
			if mode == "fail_after_step" {
				if _, statErr := os.Stat(task); !os.IsNotExist(statErr) {
					t.Fatalf("removed worktree unexpectedly restored: %v", statErr)
				}
				if !refExists(t, primary, "refs/heads/"+plan.TaskBranch) {
					t.Fatal("remaining local task ref was removed after failed attestation")
				}
				if got := gitOutput(t, primary, "ls-remote", "--heads", "origin", plan.TaskBranch); got == "" {
					t.Fatal("remaining remote task ref was removed after failed attestation")
				}
				return
			}
			assertLegacyResourcesPresent(t, primary, task)
		})
	}
}

func hashLegacyVerifier(t *testing.T, path string) string {
	t.Helper()
	hash, err := hashArtifact(path)
	if err != nil {
		t.Fatal(err)
	}
	return hash
}

func buildLegacyGuardVerifier(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	source := filepath.Join(dir, "main.go")
	const program = `package main
import ("encoding/json"; "fmt"; "os"; "os/exec"; "strconv"; "strings"; "time")
func started(pid int) string { c:=exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "lstart="); c.Env=append(os.Environ(), "LC_ALL=C"); out,e:=c.Output(); if e!=nil { panic(e) }; at,e:=time.ParseInLocation(time.ANSIC, strings.TrimSpace(string(out)), time.Local); if e!=nil { panic(e) }; return at.UTC().Format(time.RFC3339Nano) }
func parent(pid int) int { c:=exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "ppid="); c.Env=append(os.Environ(), "LC_ALL=C"); out,e:=c.Output(); if e!=nil { panic(e) }; value,e:=strconv.Atoi(strings.TrimSpace(string(out))); if e!=nil { panic(e) }; return value }
func value(args []string, flag string) string { for i:=0;i+1<len(args);i++ { if args[i]==flag { return args[i+1] } }; panic("missing "+flag) }
func main() { mode:=os.Getenv("LEGACY_GUARD_MODE"); if mode=="oversized_stderr" { fmt.Fprint(os.Stderr, strings.Repeat("x", 200<<10)) }; if mode=="fail_after_step" { path:=os.Getenv("LEGACY_GUARD_COUNTER"); n:=0; if raw,e:=os.ReadFile(path);e==nil { n,_=strconv.Atoi(string(raw)) }; n++; _=os.WriteFile(path, []byte(strconv.Itoa(n)), 0600); if n>=3 { fmt.Print("{"); return } }; if mode=="malformed" { fmt.Print("{"); return }; provider:=os.Getppid(); ce:=parent(provider); guard:=json.RawMessage(os.Getenv("LEGACY_GUARD_JSON")); if mode=="wrong_guard" { guard=json.RawMessage([]byte("{}")) }; if mode=="stale_pid" { provider=1 }; result:=map[string]any{"schema_version":1,"capability":"ce-legacy-cleanup-attestation-v1","guard":guard,"plan_digest":value(os.Args,"--plan-digest"),"attempt":value(os.Args,"--attempt"),"ce_pid":ce,"ce_started_at":started(ce),"provider_pid":provider,"provider_started_at":started(os.Getppid()),"allowed":true}; if e:=json.NewEncoder(os.Stdout).Encode(result);e!=nil { panic(e) }; if mode=="oversized_stdout" { fmt.Print(strings.Repeat(" ", 200<<10)) } }
`
	if err := os.WriteFile(source, []byte(program), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "verifier")
	cmd := exec.CommandContext(context.Background(), "go", "build", "-o", path, source) // #nosec G204 -- test-only fixed compiler invocation over a temp source file.
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build verifier: %v: %s", err, out)
	}
	return path
}

func TestLegacyRuntimeGuardRequiresCEDeclaration(t *testing.T) {
	plan, planPath, primary, task := legacySecurityPlan(t, true)
	guard := &LegacyRuntimeGuard{SchemaVersion: legacyRuntimeGuardSchema, Capability: legacyRuntimeGuardCapability, OperationID: strings.Repeat("a", 32), RepositoryRoot: plan.RepositoryRoot, CommonDir: plan.CommonDir, TaskBranch: plan.TaskBranch, TaskWorktree: plan.TaskWorktree, TaskOID: plan.TaskOID, Owner: plan.Owner, VerifierExecutable: "/usr/bin/true", VerifierSHA256: strings.Repeat("b", 64)}
	plan.RuntimeGuard = guard
	digest, err := legacyCleanupPlanDigest(plan)
	if err != nil {
		t.Fatal(err)
	}
	plan.PlanDigest = digest
	cePlanPath := filepath.Join(filepath.Dir(planPath), "guard-without-ce.json")
	if err := WriteLegacyCleanupPlan(cePlanPath, plan); err != nil {
		t.Fatal(err)
	}
	err = ApplyLegacyCleanup(context.Background(), gitcmd.NewExecutor(), LegacyCleanupApplyOptions{PlanPath: cePlanPath, ApproveDigest: plan.PlanDigest, RuntimeAttempt: strings.Repeat("b", 32)})
	if err == nil {
		t.Fatal("runtime guard without CE declaration was accepted")
	}
	assertLegacyResourcesPresent(t, primary, task)
}

func TestBoundedLegacyBufferCapsCopiedOutput(t *testing.T) {
	t.Parallel()
	// io.Copy prefers the destination's ReaderFrom when the source has no
	// WriterTo, as with the pipe os/exec copies a command's output from;
	// the cap must hold on that path too (ISSUE-77). The struct hides
	// bytes.Reader's WriteTo.
	var buf boundedLegacyBuffer
	src := struct{ io.Reader }{bytes.NewReader(make([]byte, 200<<10))}
	if _, err := io.Copy(&buf, src); err == nil {
		t.Fatal("copy past the cap succeeded")
	}
	if !buf.exceeded() {
		t.Fatal("overflow was not recorded")
	}
	if got := len(buf.Bytes()); got > legacyAttestationMaxOutput {
		t.Fatalf("buffer holds %d bytes, cap is %d", got, legacyAttestationMaxOutput)
	}
}
