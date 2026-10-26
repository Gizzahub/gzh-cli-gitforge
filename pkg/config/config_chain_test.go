// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigChainEnter(t *testing.T) {
	root := t.TempDir()
	siblingA := filepath.Join(root, "a")
	siblingB := filepath.Join(root, "b")
	for _, dir := range []string{siblingA, siblingB} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	depth10 := make(ConfigChain, 10)
	for i := range depth10 {
		d := filepath.Join(root, "depth10", string(rune('a'+i)))
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		depth10[i] = d
	}
	depth11 := make(ConfigChain, 11)
	copy(depth11, depth10)
	d11 := filepath.Join(root, "depth11")
	if err := os.Mkdir(d11, 0o755); err != nil {
		t.Fatal(err)
	}
	depth11[10] = d11
	fresh := filepath.Join(root, "fresh")
	if err := os.Mkdir(fresh, 0o755); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(t.TempDir(), "link")
	symlinkErr := os.Symlink(root, link)

	t.Chdir(root)

	tests := []struct {
		name    string
		chain   ConfigChain
		dir     string
		wantErr error
		wantLen int
		skip    string
	}{
		{
			name:    "cycle by same path",
			chain:   ConfigChain{root},
			dir:     root,
			wantErr: ErrConfigCycle,
		},
		{
			name:    "cycle by trailing slash",
			chain:   ConfigChain{root},
			dir:     root + string(filepath.Separator),
			wantErr: ErrConfigCycle,
		},
		{
			name:    "cycle by relative spelling",
			chain:   ConfigChain{siblingA},
			dir:     "a",
			wantErr: ErrConfigCycle,
		},
		{
			name:    "cycle by symlink",
			chain:   ConfigChain{root},
			dir:     link,
			wantErr: ErrConfigCycle,
			skip: func() string {
				if symlinkErr != nil {
					return symlinkErr.Error()
				}
				return ""
			}(),
		},
		{
			name:    "depth at len 11 errors",
			chain:   depth11,
			dir:     fresh,
			wantErr: ErrConfigDepth,
		},
		{
			name:    "depth at len 10 succeeds",
			chain:   depth10,
			dir:     fresh,
			wantLen: 11,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip != "" {
				t.Skipf("symlink: %s", tt.skip)
			}
			got, err := tt.chain.Enter(tt.dir)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Enter(%q) err = %v, want %v", tt.dir, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Enter(%q) unexpected err: %v", tt.dir, err)
			}
			if len(got) != tt.wantLen {
				t.Fatalf("len(got) = %d, want %d", len(got), tt.wantLen)
			}
			if got[len(got)-1] != tt.dir {
				t.Errorf("last entry = %q, want %q", got[len(got)-1], tt.dir)
			}
		})
	}

	t.Run("siblings get independent chains", func(t *testing.T) {
		base := ConfigChain{root}
		a, err := base.Enter(siblingA)
		if err != nil {
			t.Fatalf("Enter(a): %v", err)
		}
		b, err := base.Enter(siblingB)
		if err != nil {
			t.Fatalf("Enter(b): %v", err)
		}
		if len(a) != 2 || len(b) != 2 {
			t.Fatalf("len(a)=%d len(b)=%d, want 2", len(a), len(b))
		}
		a[0] = "mutated"
		if base[0] != root {
			t.Errorf("Enter aliased the parent chain")
		}
		if b[0] != root {
			t.Errorf("Enter aliased sibling chains")
		}
		if a[1] != siblingA || b[1] != siblingB {
			t.Errorf("sibling entries: a=%q b=%q", a[1], b[1])
		}
	})
}
