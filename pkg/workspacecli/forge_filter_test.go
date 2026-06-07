// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"reflect"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

func TestEffectiveForgeWorkspacePatterns(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *config.Config
		ws          *config.Workspace
		wantInclude []string
		wantExclude []string
	}{
		{
			name: "uses defaults filter",
			cfg: &config.Config{
				Defaults: &config.DefaultsConfig{
					Filter: &config.FilterDefaults{
						Include: []string{"api"},
						Exclude: []string{"archive"},
					},
				},
			},
			ws:          &config.Workspace{},
			wantInclude: []string{"api"},
			wantExclude: []string{"archive"},
		},
		{
			name: "workspace patterns override defaults",
			cfg: &config.Config{
				Defaults: &config.DefaultsConfig{
					Filter: &config.FilterDefaults{
						Include: []string{"api"},
						Exclude: []string{"archive"},
					},
				},
			},
			ws: &config.Workspace{
				IncludePatterns: []string{"web"},
				ExcludePatterns: []string{"deprecated"},
			},
			wantInclude: []string{"web"},
			wantExclude: []string{"deprecated"},
		},
		{
			name:        "handles nil config",
			cfg:         nil,
			ws:          &config.Workspace{IncludePatterns: []string{"service"}},
			wantInclude: []string{"service"},
			wantExclude: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotInclude, gotExclude := effectiveForgeWorkspacePatterns(tt.cfg, tt.ws)
			if !reflect.DeepEqual(gotInclude, tt.wantInclude) {
				t.Errorf("include = %v, want %v", gotInclude, tt.wantInclude)
			}
			if !reflect.DeepEqual(gotExclude, tt.wantExclude) {
				t.Errorf("exclude = %v, want %v", gotExclude, tt.wantExclude)
			}
		})
	}
}
