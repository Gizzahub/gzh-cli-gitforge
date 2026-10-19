// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

func TestLegacyCleanupCERuntimeGuard(t *testing.T) {
	for _, mode := range []string{"allow", "malformed", "wrong_guard", "stale_pid", "hash_changed", "fail_after_step"} {
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
func main() { mode:=os.Getenv("LEGACY_GUARD_MODE"); if mode=="fail_after_step" { path:=os.Getenv("LEGACY_GUARD_COUNTER"); n:=0; if raw,e:=os.ReadFile(path);e==nil { n,_=strconv.Atoi(string(raw)) }; n++; _=os.WriteFile(path, []byte(strconv.Itoa(n)), 0600); if n>=3 { fmt.Print("{"); return } }; if mode=="malformed" { fmt.Print("{"); return }; provider:=os.Getppid(); ce:=parent(provider); guard:=json.RawMessage(os.Getenv("LEGACY_GUARD_JSON")); if mode=="wrong_guard" { guard=json.RawMessage([]byte("{}")) }; if mode=="stale_pid" { provider=1 }; result:=map[string]any{"schema_version":1,"capability":"ce-legacy-cleanup-attestation-v1","guard":guard,"plan_digest":value(os.Args,"--plan-digest"),"attempt":value(os.Args,"--attempt"),"ce_pid":ce,"ce_started_at":started(ce),"provider_pid":provider,"provider_started_at":started(os.Getppid()),"allowed":true}; if e:=json.NewEncoder(os.Stdout).Encode(result);e!=nil { panic(e) } }
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
