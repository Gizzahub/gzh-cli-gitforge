// Copyright (c) 2026 Archmagece
// SPDX-License-Identifier: MIT

package config

import (
	"testing"
)

func TestMemoryTokenStore_CRUD(t *testing.T) {
	s := NewMemoryTokenStore()
	if err := s.Set("GitHub", "tok-1"); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get("github")
	if err != nil || got != "tok-1" {
		t.Fatalf("get=%q err=%v", got, err)
	}
	if err := s.Delete("GITHUB"); err != nil {
		t.Fatal(err)
	}
	got, err = s.Get("github")
	if err != nil || got != "" {
		t.Fatalf("after delete get=%q err=%v", got, err)
	}
}

func TestMemoryTokenStore_Unavailable(t *testing.T) {
	s := NewMemoryTokenStore()
	s.SetAvailable(false)
	if err := s.Set("github", "x"); err == nil {
		t.Fatal("expected unavailable")
	}
}

func TestResolveTokenFromEnv(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "gh-from-env")
	t.Setenv("GZ_GIT_TOKEN", "generic")
	tok, src := ResolveTokenFromEnv("github")
	if tok != "gh-from-env" || src != "env:GITHUB_TOKEN" {
		t.Fatalf("tok=%q src=%q", tok, src)
	}
	t.Setenv("GITHUB_TOKEN", "")
	tok, src = ResolveTokenFromEnv("github")
	if tok != "generic" || src != "env:GZ_GIT_TOKEN" {
		t.Fatalf("generic tok=%q src=%q", tok, src)
	}
}

func TestResolveConfig_KeychainThenEnvThenFlag(t *testing.T) {
	mem := NewMemoryTokenStore()
	prev := DefaultTokenStore
	SetTokenStore(mem)
	t.Cleanup(func() { SetTokenStore(prev) })

	if err := mem.Set("github", "from-keychain"); err != nil {
		t.Fatal(err)
	}

	loader := &ConfigLoader{
		activeProfile: &Profile{Name: "default", Provider: "github", Token: "from-profile"},
	}
	// no env
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GZ_GIT_TOKEN", "")
	eff, err := loader.ResolveConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if eff.Token != "from-keychain" {
		t.Fatalf("want keychain token, got %q (src=%s)", eff.Token, eff.GetSource("token"))
	}
	if eff.GetSource("token") != string(SourceKeychain) {
		t.Fatalf("source=%s", eff.GetSource("token"))
	}

	// env overrides keychain
	t.Setenv("GITHUB_TOKEN", "from-env")
	eff, err = loader.ResolveConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if eff.Token != "from-env" {
		t.Fatalf("want env token, got %q", eff.Token)
	}

	// flag overrides env
	eff, err = loader.ResolveConfig(map[string]any{"token": "from-flag"})
	if err != nil {
		t.Fatal(err)
	}
	if eff.Token != "from-flag" {
		t.Fatalf("want flag token, got %q", eff.Token)
	}
	if eff.GetSource("token") != string(SourceFlag) {
		t.Fatalf("source=%s", eff.GetSource("token"))
	}
}
