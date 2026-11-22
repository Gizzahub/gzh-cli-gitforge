// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func flatCfg(parent, repoName string) string {
	s := ""
	if parent != "" {
		s += "parent: " + parent + "\n"
	}
	return s + fmt.Sprintf("repositories:\n  - url: https://github.com/o/%s.git\n    name: %s\n", repoName, repoName)
}

// captureStderr runs fn and returns what it wrote to os.Stderr.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	fn()
	_ = w.Close()
	os.Stderr = old
	out, _ := io.ReadAll(r)
	return string(out)
}

// TestFlatParentCycles pins that a flat `parent:` cycle terminates and warns
// with the chain of resolved paths (ISSUE-405), like a broken parent chain.
func TestFlatParentCycles(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string // file -> parent ref
		want  []string          // files the warning chain must name, in order
	}{
		{"self", map[string]string{"a.yaml": "a.yaml"}, []string{"a.yaml", "a.yaml"}},
		{"two", map[string]string{"a.yaml": "b.yaml", "b.yaml": "a.yaml"}, []string{"a.yaml", "b.yaml", "a.yaml"}},
		{"three", map[string]string{"a.yaml": "b.yaml", "b.yaml": "c.yaml", "c.yaml": "a.yaml"}, []string{"a.yaml", "b.yaml", "c.yaml", "a.yaml"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for f, parent := range tc.files {
				writeFileAt(t, filepath.Join(dir, f), flatCfg("./"+parent, strings.TrimSuffix(f, ".yaml")))
			}
			var err error
			stderr := captureStderr(t, func() {
				_, err = FileSpecLoader{}.Load(context.Background(), filepath.Join(dir, "a.yaml"))
			})
			if err != nil {
				t.Fatalf("Load err = %v, want warn-and-continue", err)
			}
			if !strings.Contains(stderr, "circular parent config reference detected") {
				t.Fatalf("stderr missing cycle warning: %q", stderr)
			}
			pos := 0
			for _, f := range tc.want {
				i := strings.Index(stderr[pos:], string(filepath.Separator)+f)
				if i < 0 {
					t.Fatalf("chain does not name %s in order: %q", f, stderr)
				}
				pos += i + 1
			}
		})
	}
}

// TestFlatParentDeepChain pins that a long acyclic chain is still followed.
func TestFlatParentDeepChain(t *testing.T) {
	dir := t.TempDir()
	const n = 5
	for i := 0; i < n; i++ {
		parent := ""
		if i < n-1 {
			parent = fmt.Sprintf("./c%d.yaml", i+1)
		}
		body := flatCfg(parent, "deep")
		if i < n-1 {
			// Repositories replace rather than merge, so only the end of the
			// chain declares them; the root inherits them through every link.
			body = "parent: " + parent + "\n"
		}
		writeFileAt(t, filepath.Join(dir, fmt.Sprintf("c%d.yaml", i)), body)
	}
	var data ConfigData
	var err error
	stderr := captureStderr(t, func() {
		data, err = FileSpecLoader{}.Load(context.Background(), filepath.Join(dir, "c0.yaml"))
	})
	if err != nil {
		t.Fatal(err)
	}
	if stderr != "" {
		t.Fatalf("unexpected warning: %q", stderr)
	}
	if got := len(data.Plan.Input.Repos); got != 1 {
		t.Fatalf("repositories = %d, want 1 inherited from the end of the chain", got)
	}
}
