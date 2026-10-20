// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestLoadRepoRootTaskPattern_ThisRepoDeclaration guards this repository's own
// repo-root declaration. The post-integrate RECLAIM machinery stays dormant
// until LoadRepoRootTaskPattern finds a repo-root taskPattern, so deleting or
// loosening the declaration silently disables reclaim. The repo root is
// resolved from this test file's location, never from the working directory.
func TestLoadRepoRootTaskPattern_ThisRepoDeclaration(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")

	decl, err := LoadRepoRootTaskPattern(repoRoot)
	if err != nil {
		t.Fatalf("LoadRepoRootTaskPattern(%s): %v", repoRoot, err)
	}
	if len(decl.Patterns) == 0 {
		t.Fatalf("no taskPattern declaration: facts %v", decl.Facts)
	}

	wantIntegration := []string{"master"}
	if got := []string(decl.IntegrationBranch); !reflect.DeepEqual(got, wantIntegration) {
		t.Fatalf("integrationBranch = %v, want %v", got, wantIntegration)
	}

	mustMatch := []string{
		"feat/x",
		"test/info-branch-cell-colors",
		"agent/task/hermes-01", // agent/* must cover the multi-segment shape
		"dev/mac/fix/reclaim-dev-pattern",
	}
	for _, name := range mustMatch {
		if !MatchesAnyTaskPattern(name, decl.Patterns) {
			t.Errorf("%s must match taskPattern %v", name, decl.Patterns)
		}
	}

	mustNotMatch := []string{"develop", "master", "main"}
	for _, name := range mustNotMatch {
		if MatchesAnyTaskPattern(name, decl.Patterns) {
			t.Errorf("%s must not match taskPattern %v", name, decl.Patterns)
		}
	}
}

func TestRepoRootResourceHighLoad(t *testing.T) {
	writeDecl := func(t *testing.T, name, body string) string {
		t.Helper()
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return root
	}
	t.Run("absent is light", func(t *testing.T) {
		decl, err := LoadRepoRootTaskPattern(writeDecl(t, ".gz-git.yaml", "branch:\n  integrationBranch: master\n"))
		if err != nil || decl.ResourceHighLoad {
			t.Fatalf("load = %+v, %v; want explicit false", decl, err)
		}
	})
	t.Run("yaml true opts in", func(t *testing.T) {
		decl, err := LoadRepoRootTaskPattern(writeDecl(t, ".gz-git.yaml", "branch:\n  resourceHighLoad: true\n"))
		if err != nil || !decl.ResourceHighLoad {
			t.Fatalf("load = %+v, %v; want explicit true", decl, err)
		}
	})
	for _, spelling := range []string{"True", "TRUE", "False", "FALSE"} {
		t.Run("yaml "+spelling, func(t *testing.T) {
			body := "branch:\n  resourceHighLoad: " + spelling + "\n"
			decl, err := LoadRepoRootTaskPattern(writeDecl(t, ".gz-git.yaml", body))
			want := strings.EqualFold(spelling, "true")
			if err != nil || decl.ResourceHighLoad != want {
				t.Fatalf("load = %+v, %v; want high load %v", decl, err, want)
			}
		})
	}
	t.Run("json only accepts boolean", func(t *testing.T) {
		root := writeDecl(t, ".gz-git.json", `{"branch":{"resourceHighLoad":true}}`)
		decl, err := LoadRepoRootTaskPattern(root)
		if err != nil || !decl.ResourceHighLoad {
			t.Fatalf("load = %+v, %v; want explicit true", decl, err)
		}
		root = writeDecl(t, ".gz-git.json", `{"branch":{"resourceHighLoad":"true"}}`)
		if _, err := LoadRepoRootTaskPattern(root); err == nil || !strings.Contains(err.Error(), "resourceHighLoad") {
			t.Fatalf("string resourceHighLoad error = %v", err)
		}
	})
	for name, body := range map[string]string{
		"json null":      `{"branch":{"resourceHighLoad":null}}`,
		"json duplicate": `{"branch":{"resourceHighLoad":false,"resourceHighLoad":true}}`,
		"yaml null":      "branch:\n  resourceHighLoad: null\n",
		"yaml duplicate": "branch:\n  resourceHighLoad: false\n  resourceHighLoad: true\n",
		"yaml alias":     "saved: &branch\n  resourceHighLoad: true\nbranch: *branch\n",
		"yaml merge":     "saved: &branch\n  resourceHighLoad: true\nbranch:\n  <<: *branch\n",
	} {
		t.Run(name, func(t *testing.T) {
			filename := ".gz-git.yaml"
			if strings.HasPrefix(name, "json") {
				filename = ".gz-git.json"
			}
			if _, err := LoadRepoRootTaskPattern(writeDecl(t, filename, body)); err == nil || !strings.Contains(err.Error(), "resourceHighLoad") && !strings.Contains(err.Error(), "duplicate") {
				t.Fatalf("malformed resource declaration error = %v", err)
			}
		})
	}
}

func TestRepoRootLockWait(t *testing.T) {
	write := func(t *testing.T, name, body string) string {
		t.Helper()
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return root
	}
	for _, tt := range []struct {
		name, body string
		wait       time.Duration
		set        bool
	}{
		{"absent", "branch:\n  integrationBranch: master\n", 0, false},
		{"duration", "branch:\n  lockWait: 90m\n", 90 * time.Minute, true},
		{"unbounded", "branch:\n  lockWait: unbounded\n", 0, true},
		{"zero", "branch:\n  lockWait: \"0\"\n", 0, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			decl, err := LoadRepoRootTaskPattern(write(t, ".gz-git.yaml", tt.body))
			if err != nil || decl.LockWait != tt.wait || decl.LockWaitSet != tt.set {
				t.Fatalf("load = (%s, %t), %v; want (%s, %t)", decl.LockWait, decl.LockWaitSet, err, tt.wait, tt.set)
			}
		})
	}
	for name, body := range map[string]string{
		"json null":       `{"branch":{"lockWait":null}}`,
		"json numeric":    `{"branch":{"lockWait":0}}`,
		"json empty":      `{"branch":{"lockWait":""}}`,
		"json duplicate":  `{"branch":{"lockWait":"1m","lockWait":"2m"}}`,
		"yaml null":       "branch:\n  lockWait: null\n",
		"yaml numeric":    "branch:\n  lockWait: 0\n",
		"yaml empty":      "branch:\n  lockWait: \"\"\n",
		"yaml whitespace": "branch:\n  lockWait: \" 1m\"\n",
		"yaml negative":   "branch:\n  lockWait: -1m\n",
		"yaml root merge": "saved: &config\n  branch:\n    lockWait: 1m\n<<: *config\n",
		"yaml alias":      "saved: &branch\n  lockWait: 1m\nbranch: *branch\n",
		"yaml merge":      "saved: &branch\n  lockWait: 1m\nbranch:\n  <<: *branch\n",
	} {
		t.Run(name, func(t *testing.T) {
			file := ".gz-git.yaml"
			if strings.HasPrefix(name, "json") {
				file = ".gz-git.json"
			}
			if _, err := LoadRepoRootTaskPattern(write(t, file, body)); err == nil {
				t.Fatalf("malformed lockWait accepted: %s", body)
			}
		})
	}
	for _, body := range []string{
		"branch:\n  lockWait: 1m\nbranch:\n  lockWait: 2m\n",
		"branch:\n  resourceHighLoad: true\n  lockWait: 1m\n",
	} {
		root := write(t, ".gz-git.yaml", body)
		decl, err := LoadRepoRootTaskPattern(root)
		if strings.Count(body, "branch:") > 1 {
			if err == nil {
				t.Fatalf("duplicate branch accepted: %s", body)
			}
			continue
		}
		if err != nil || !decl.ResourceHighLoad || decl.LockWait != time.Minute {
			t.Fatalf("combined declaration = %+v, %v", decl, err)
		}
	}
	root := write(t, ".gz-git.json", `{"branch":{"lockWait":"unbounded"}}`)
	decl, err := LoadRepoRootTaskPattern(root)
	if err != nil || !decl.LockWaitSet || decl.LockWait != 0 {
		t.Fatalf("json unbounded = %+v, %v", decl, err)
	}
	nested := t.TempDir()
	if err := os.Mkdir(filepath.Join(nested, "child"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "child", ".gz-git.yaml"), []byte("branch:\n  lockWait: 1m\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	decl, err = LoadRepoRootTaskPattern(nested)
	if err != nil || decl.LockWaitSet {
		t.Fatalf("non-root lockWait inherited: %+v, %v", decl, err)
	}
}
