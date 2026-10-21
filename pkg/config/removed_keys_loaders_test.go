// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// configDecodeMarker matches a source file that decodes configuration-shaped
// YAML, or hands bytes to one of this package's config document parsers.
var configDecodeMarker = regexp.MustCompile(`yaml\.(Unmarshal|NewDecoder)\(|ParseFlatRepositories\(|ParseSiblingProvisionDocument\(|parseDeclaration\(`)

// unguardedConfigReaders lists the files that read a file and decode YAML but
// do not load a gz-git configuration, with the reason each is exempt.
var unguardedConfigReaders = map[string]string{
	"cmd/gz-git/cmd/commit.go":    "decodes per-repository commit messages, not a configuration",
	"internal/config/config.go":   "loads the forge.yaml client configuration, which has no branch or integration section",
	"internal/runtask/service.go": "also reads .gz-git-task.yaml, whose own schema has no branch or integration section; its .gz-git.yaml read is guarded",
}

// TestEveryConfigLoaderUsesTheRemovedKeyGuard enumerates the module's
// non-test sources that read a file and decode configuration YAML. Each must
// route through ReadConfigFile or RejectRemovedKeys, or be listed in
// unguardedConfigReaders with its reason, so a new loader cannot silently
// accept a removed key that every other loader rejects.
func TestEveryConfigLoaderUsesTheRemovedKeyGuard(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	var unguarded, seenExempt []string
	for _, dir := range []string{"cmd", "pkg", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			src, err := os.ReadFile(path) // #nosec G304 -- walks this module's own sources.
			if err != nil {
				return err
			}
			text := string(src)
			if !strings.Contains(text, "ReadFile(") || !configDecodeMarker.MatchString(text) {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if _, exempt := unguardedConfigReaders[rel]; exempt {
				seenExempt = append(seenExempt, rel)
				return nil
			}
			if rel == "pkg/config/removed_keys.go" {
				return nil
			}
			if !strings.Contains(text, "ReadConfigFile(") && !strings.Contains(text, "RejectRemovedKeys(") {
				unguarded = append(unguarded, rel)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(unguarded) > 0 {
		sort.Strings(unguarded)
		t.Fatalf("config loaders bypass ReadConfigFile/RejectRemovedKeys: %v", unguarded)
	}
	sort.Strings(seenExempt)
	for rel := range unguardedConfigReaders {
		if i := sort.SearchStrings(seenExempt, rel); i == len(seenExempt) || seenExempt[i] != rel {
			t.Errorf("stale exemption %s: the file no longer reads and decodes YAML", rel)
		}
	}
}

// TestRemovedKeyCheckHasOneEntryPoint pins the choke point: the unexported
// checker is called only by RejectRemovedKeys, so every loader shares one
// key list, one message, and one JSON/YAML selection.
func TestRemovedKeyCheckHasOneEntryPoint(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name) // #nosec G304 -- reads this package's own sources.
		if err != nil {
			t.Fatal(err)
		}
		n := strings.Count(string(src), "rejectRemovedConfigKeys(")
		if name != "removed_keys.go" && n > 0 {
			t.Errorf("%s calls rejectRemovedConfigKeys directly; use ReadConfigFile or RejectRemovedKeys", name)
		}
		calls += n
	}
	// One definition and one call from RejectRemovedKeys.
	if calls != 2 {
		t.Fatalf("rejectRemovedConfigKeys appears %d times, want its definition and one call", calls)
	}
}
