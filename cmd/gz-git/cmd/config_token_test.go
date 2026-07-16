// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

func TestConfigTokenSetGetDelete(t *testing.T) {
	mem := config.NewMemoryTokenStore()
	prev := config.DefaultTokenStore
	config.SetTokenStore(mem)
	t.Cleanup(func() { config.SetTokenStore(prev) })

	out := captureStdout(t, func() {
		if err := runConfigTokenSet(configTokenSetCmd, []string{"github", "secret-token-value"}); err != nil {
			t.Fatalf("set: %v", err)
		}
	})
	if !strings.Contains(out, "Stored") {
		t.Fatalf("set out: %q", out)
	}

	tokenShowFull = false
	out = captureStdout(t, func() {
		if err := runConfigTokenGet(configTokenGetCmd, []string{"github"}); err != nil {
			t.Fatalf("get: %v", err)
		}
	})
	if strings.Contains(out, "secret-token-value") {
		t.Fatalf("masked get leaked full token: %q", out)
	}

	tokenShowFull = true
	out = captureStdout(t, func() {
		if err := runConfigTokenGet(configTokenGetCmd, []string{"github"}); err != nil {
			t.Fatalf("get full: %v", err)
		}
	})
	if !strings.Contains(out, "secret-token-value") {
		t.Fatalf("full get: %q", out)
	}

	out = captureStdout(t, func() {
		if err := runConfigTokenDelete(configTokenDeleteCmd, []string{"github"}); err != nil {
			t.Fatalf("delete: %v", err)
		}
	})
	if !strings.Contains(out, "Deleted") {
		t.Fatalf("delete out: %q", out)
	}
}

func TestConfigTokenSet_UnavailableFallback(t *testing.T) {
	mem := config.NewMemoryTokenStore()
	mem.SetAvailable(false)
	prev := config.DefaultTokenStore
	config.SetTokenStore(mem)
	t.Cleanup(func() { config.SetTokenStore(prev) })

	// Should not error (warn + fallback)
	if err := runConfigTokenSet(configTokenSetCmd, []string{"gitlab", "x"}); err != nil {
		t.Fatalf("set unavailable: %v", err)
	}
}
