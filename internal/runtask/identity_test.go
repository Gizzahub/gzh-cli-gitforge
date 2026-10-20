// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package runtask

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestResolveOwnerReadsGlobalGitLikeCleanup pins ISSUE-71: the run lifecycle
// reads ce.workActor by the rule integrate cleanup uses, so an owner it
// records is one cleanup resolves the same way. The integrate side has the
// matching cases in cleanup_owner_identity_test.go.
func TestResolveOwnerReadsGlobalGitLikeCleanup(t *testing.T) {
	for _, tc := range []struct {
		name   string
		config string // "" points GIT_CONFIG_GLOBAL at a directory
		err    string
	}{
		{name: "two values are refused", config: "[ce]\n\tworkActor = alice\n\tworkActor = bob\n", err: "global Git ce.workActor must have exactly one value; found 2"},
		{name: "an empty value is refused", config: "[ce]\n\tworkActor =\n", err: "global Git ce.workActor has an invalid value"},
		// Git exits 1 here as it does for an unset key, but warns on stderr.
		{name: "an unreadable config reports Git's warning", err: "read global Git ce.workActor: exit 1:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, key := range []string{actorEnvAgent, actorEnvHuman} {
				t.Setenv(key, "")
			}
			t.Setenv(hostEnv, "mbp")
			t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
			global := t.TempDir()
			if tc.config != "" {
				global = filepath.Join(global, "gitconfig")
				if err := os.WriteFile(global, []byte(tc.config), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("GIT_CONFIG_GLOBAL", global)
			s := NewService(t.TempDir(), NewExecutor(), NewEngine())
			owner, err := s.resolveOwner(context.Background())
			if err == nil || !strings.Contains(err.Error(), tc.err) || strings.Contains(err.Error(), "not configured") {
				t.Fatalf("resolveOwner = %+v, %v; want an error containing %q", owner, err, tc.err)
			}
		})
	}
}
