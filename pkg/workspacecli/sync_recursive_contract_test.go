// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
)

// TestRecursiveSyncCompatibility pins the recursive child workspace sync
// contract while the block lives in sync_recursive.go: max depth must stop
// child invocation entirely, visited paths must be de-duplicated, and a
// missing child config must emit the existing warning text without stopping
// the parent.
func TestRecursiveSyncCompatibility(t *testing.T) {
	ctx := context.Background()

	t.Run("max depth stops child invocation", func(t *testing.T) {
		// Real temp config fixture: a loadable child config.
		tmpDir := t.TempDir()
		child := filepath.Join(tmpDir, "child")
		if err := os.MkdirAll(child, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(child, DefaultConfigFile), []byte("repositories: []\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		var buf bytes.Buffer
		opts := recursiveSyncOpts{
			MaxDepth: 1,
			Visited:  map[string]bool{},
			Out:      &buf,
			Depth:    1, // already at max depth
		}
		result := reposync.ExecutionResult{
			Succeeded: []reposync.ActionResult{
				{Action: reposync.Action{Repo: reposync.RepoSpec{Name: "child", TargetPath: child}}},
			},
		}

		syncChildWorkspaces(ctx, result, opts)
		output := buf.String()

		if !strings.Contains(output, "[recursive] Max depth 1 reached, stopping.") {
			t.Errorf("expected max depth message, got:\n%s", output)
		}
		if strings.Contains(output, "Syncing child workspace") {
			t.Errorf("child must not be invoked at max depth, got:\n%s", output)
		}
		if strings.Contains(output, "Found") {
			t.Errorf("no child scan should run at max depth, got:\n%s", output)
		}
	})

	t.Run("visited path is deduplicated", func(t *testing.T) {
		// Real temp config fixture. The recursion discovers the child at one
		// level, then meets the same path again at a deeper level; the shared
		// Visited map (as passed down via childOpts) must suppress the
		// re-processing entirely.
		tmpDir := t.TempDir()
		child := filepath.Join(tmpDir, "duplicate-child")
		if err := os.MkdirAll(child, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(child, DefaultConfigFile), []byte("repositories: []\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		absChild, err := filepath.Abs(child)
		if err != nil {
			t.Fatal(err)
		}

		result := reposync.ExecutionResult{
			Succeeded: []reposync.ActionResult{
				{Action: reposync.Action{Repo: reposync.RepoSpec{Name: "duplicate-child", TargetPath: child}}},
			},
		}
		visited := map[string]bool{}
		var first, second bytes.Buffer

		opts := recursiveSyncOpts{MaxDepth: 3, Visited: visited, Out: &first, Depth: 0}
		syncChildWorkspaces(ctx, result, opts)
		firstOut := first.String()

		if !strings.Contains(firstOut, "Found 1 child workspace(s) at depth 1") {
			t.Errorf("expected the child discovered once, got:\n%s", firstOut)
		}
		if got := strings.Count(firstOut, "Syncing child workspace: "+absChild); got != 1 {
			t.Errorf("child %s synced %d times on first visit, want exactly 1, output:\n%s", absChild, got, firstOut)
		}

		// Same child discovered again at a later recursion level: the visited
		// path must be skipped, producing no output and no second sync.
		opts2 := recursiveSyncOpts{MaxDepth: 3, Visited: visited, Out: &second, Depth: 1}
		syncChildWorkspaces(ctx, result, opts2)
		if secondOut := second.String(); secondOut != "" {
			t.Errorf("visited child must not be re-synced, got:\n%s", secondOut)
		}
	})

	t.Run("missing child config warns and parent continues", func(t *testing.T) {
		tmpDir := t.TempDir()
		missing := filepath.Join(tmpDir, "no-config-child")
		if err := os.MkdirAll(missing, 0o755); err != nil {
			t.Fatal(err)
		}
		good := filepath.Join(tmpDir, "good-child")
		if err := os.MkdirAll(good, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(good, DefaultConfigFile), []byte("repositories: []\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		absGood, err := filepath.Abs(good)
		if err != nil {
			t.Fatal(err)
		}

		// 1) Direct runChildSync call: a child dir with no .gz-git.yaml at all.
		// The existing warning text must appear and control must return to the
		// caller without propagating an error.
		var direct bytes.Buffer
		runChildSync(ctx, missing, recursiveSyncOpts{
			MaxDepth: 3,
			Visited:  map[string]bool{},
			Out:      &direct,
			Depth:    0,
		})
		directOut := direct.String()
		if !strings.Contains(directOut, "Failed to load config:") {
			t.Errorf("expected existing load-failure warning text, got:\n%s", directOut)
		}
		if !strings.Contains(directOut, "Syncing child workspace: "+missing) {
			t.Errorf("expected child header for %s, got:\n%s", missing, directOut)
		}

		// 2) Parent loop: a child whose config exists at scan time but cannot
		// be loaded must not stop the parent from syncing the next child.
		// bad-child's .gz-git.yaml is a directory: os.Stat passes at scan time,
		// FileSpecLoader.Load fails at load time — the same warning branch as a
		// config deleted between scan and load.
		bad := filepath.Join(tmpDir, "bad-child")
		if err := os.MkdirAll(filepath.Join(bad, DefaultConfigFile), 0o755); err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		opts := recursiveSyncOpts{
			MaxDepth: 3,
			Visited:  map[string]bool{},
			Out:      &buf,
			Depth:    0,
		}
		result := reposync.ExecutionResult{
			Succeeded: []reposync.ActionResult{
				{Action: reposync.Action{Repo: reposync.RepoSpec{Name: "bad-child", TargetPath: bad}}},
				{Action: reposync.Action{Repo: reposync.RepoSpec{Name: "good-child", TargetPath: good}}},
			},
		}

		syncChildWorkspaces(ctx, result, opts)
		output := buf.String()

		if !strings.Contains(output, "Syncing child workspace: "+bad) {
			t.Errorf("bad child should be attempted, got:\n%s", output)
		}
		if !strings.Contains(output, "Failed to load config:") {
			t.Errorf("expected existing load-failure warning for bad child, got:\n%s", output)
		}
		if !strings.Contains(output, "Syncing child workspace: "+absGood) {
			t.Errorf("parent must continue to the good child %s, got:\n%s", absGood, output)
		}
		if !strings.Contains(output, "No repositories found in child workspace.") {
			t.Errorf("good child should complete its (empty) sync, got:\n%s", output)
		}
	})
}
