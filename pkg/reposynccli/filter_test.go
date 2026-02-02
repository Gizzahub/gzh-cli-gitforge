// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"bytes"
	"testing"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/provider"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"seconds", "30s", 30 * time.Second, false},
		{"minutes", "5m", 5 * time.Minute, false},
		{"hours", "2h", 2 * time.Hour, false},
		{"days", "7d", 7 * 24 * time.Hour, false},
		{"weeks", "2w", 14 * 24 * time.Hour, false},
		{"months", "6M", 180 * 24 * time.Hour, false},
		{"years", "1y", 365 * 24 * time.Hour, false},
		{"zero days", "0d", 0, false},
		{"empty string", "", 0, true},
		{"no unit", "30", 0, true},
		{"invalid unit", "30x", 0, true},
		{"negative", "-5d", 0, true},
		{"invalid number", "abcd", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDuration(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDuration(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseDuration(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseLanguages(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"single", "go", []string{"go"}},
		{"multiple", "go,rust,python", []string{"go", "rust", "python"}},
		{"with spaces", "Go, Rust, Python", []string{"go", "rust", "python"}},
		{"uppercase", "GO,RUST", []string{"go", "rust"}},
		{"empty string", "", nil},
		{"empty parts", "go,,rust", []string{"go", "rust"}},
		{"whitespace only", "  ,  ", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseLanguages(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("ParseLanguages(%q) = %v, want %v", tt.input, got, tt.want)
				return
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("ParseLanguages(%q)[%d] = %q, want %q", tt.input, i, v, tt.want[i])
				}
			}
		})
	}
}

func TestMetadataFilter_Match(t *testing.T) {
	now := time.Now()
	recentPush := now.Add(-7 * 24 * time.Hour)  // 7 days ago
	oldPush := now.Add(-60 * 24 * time.Hour)    // 60 days ago
	cutoff := now.Add(-30 * 24 * time.Hour)     // 30 days ago

	tests := []struct {
		name   string
		filter MetadataFilter
		repo   *provider.Repository
		want   bool
	}{
		{
			name:   "empty filter matches all",
			filter: MetadataFilter{},
			repo:   &provider.Repository{Name: "test", Stars: 100, Language: "go", PushedAt: recentPush},
			want:   true,
		},
		{
			name:   "language match",
			filter: MetadataFilter{Languages: []string{"go", "rust"}},
			repo:   &provider.Repository{Name: "test", Language: "Go"},
			want:   true,
		},
		{
			name:   "language no match",
			filter: MetadataFilter{Languages: []string{"go", "rust"}},
			repo:   &provider.Repository{Name: "test", Language: "python"},
			want:   false,
		},
		{
			name:   "language empty repo",
			filter: MetadataFilter{Languages: []string{"go"}},
			repo:   &provider.Repository{Name: "test", Language: ""},
			want:   false,
		},
		{
			name:   "min stars pass",
			filter: MetadataFilter{MinStars: 100},
			repo:   &provider.Repository{Name: "test", Stars: 150},
			want:   true,
		},
		{
			name:   "min stars fail",
			filter: MetadataFilter{MinStars: 100},
			repo:   &provider.Repository{Name: "test", Stars: 50},
			want:   false,
		},
		{
			name:   "max stars pass",
			filter: MetadataFilter{MaxStars: 1000},
			repo:   &provider.Repository{Name: "test", Stars: 500},
			want:   true,
		},
		{
			name:   "max stars fail",
			filter: MetadataFilter{MaxStars: 1000},
			repo:   &provider.Repository{Name: "test", Stars: 1500},
			want:   false,
		},
		{
			name:   "max stars zero means unlimited",
			filter: MetadataFilter{MaxStars: 0},
			repo:   &provider.Repository{Name: "test", Stars: 999999},
			want:   true,
		},
		{
			name:   "last push recent pass",
			filter: MetadataFilter{LastPushAfter: cutoff},
			repo:   &provider.Repository{Name: "test", PushedAt: recentPush},
			want:   true,
		},
		{
			name:   "last push old fail",
			filter: MetadataFilter{LastPushAfter: cutoff},
			repo:   &provider.Repository{Name: "test", PushedAt: oldPush},
			want:   false,
		},
		{
			name: "combined filters all pass",
			filter: MetadataFilter{
				Languages:     []string{"go"},
				MinStars:      50,
				MaxStars:      500,
				LastPushAfter: cutoff,
			},
			repo: &provider.Repository{Name: "test", Language: "go", Stars: 100, PushedAt: recentPush},
			want: true,
		},
		{
			name: "combined filters one fails",
			filter: MetadataFilter{
				Languages:     []string{"go"},
				MinStars:      50,
				MaxStars:      500,
				LastPushAfter: cutoff,
			},
			repo: &provider.Repository{Name: "test", Language: "rust", Stars: 100, PushedAt: recentPush},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.filter.Match(tt.repo)
			if got != tt.want {
				t.Errorf("MetadataFilter.Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMetadataFilter_IsEmpty(t *testing.T) {
	tests := []struct {
		name   string
		filter MetadataFilter
		want   bool
	}{
		{"empty", MetadataFilter{}, true},
		{"with languages", MetadataFilter{Languages: []string{"go"}}, false},
		{"with min stars", MetadataFilter{MinStars: 100}, false},
		{"with max stars", MetadataFilter{MaxStars: 1000}, false},
		{"with last push", MetadataFilter{LastPushAfter: time.Now()}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.filter.IsEmpty()
			if got != tt.want {
				t.Errorf("MetadataFilter.IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrintWarningsForProvider(t *testing.T) {
	tests := []struct {
		name         string
		provider     string
		filter       *MetadataFilter
		wantContains string
	}{
		{
			name:         "gitlab with language filter",
			provider:     "gitlab",
			filter:       &MetadataFilter{Languages: []string{"go"}},
			wantContains: "GitLab API does not provide language",
		},
		{
			name:         "gitea with language filter",
			provider:     "gitea",
			filter:       &MetadataFilter{Languages: []string{"go"}},
			wantContains: "Gitea SDK does not expose language",
		},
		{
			name:         "github no warning",
			provider:     "github",
			filter:       &MetadataFilter{Languages: []string{"go"}},
			wantContains: "",
		},
		{
			name:         "empty filter no warning",
			provider:     "gitlab",
			filter:       &MetadataFilter{},
			wantContains: "",
		},
		{
			name:         "nil filter no warning",
			provider:     "gitlab",
			filter:       nil,
			wantContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			PrintWarningsForProvider(tt.provider, tt.filter, &buf)
			got := buf.String()

			if tt.wantContains == "" {
				if got != "" {
					t.Errorf("PrintWarningsForProvider() output = %q, want empty", got)
				}
			} else {
				if !bytes.Contains([]byte(got), []byte(tt.wantContains)) {
					t.Errorf("PrintWarningsForProvider() output = %q, want to contain %q", got, tt.wantContains)
				}
			}
		})
	}
}

func TestBuildFilterFromOptions(t *testing.T) {
	tests := []struct {
		name           string
		language       string
		minStars       int
		maxStars       int
		lastPushWithin string
		wantErr        bool
		checkFn        func(*testing.T, *MetadataFilter)
	}{
		{
			name:     "empty options",
			wantErr:  false,
			checkFn: func(t *testing.T, f *MetadataFilter) {
				if !f.IsEmpty() {
					t.Error("expected empty filter")
				}
			},
		},
		{
			name:     "with languages",
			language: "go,rust",
			checkFn: func(t *testing.T, f *MetadataFilter) {
				if len(f.Languages) != 2 {
					t.Errorf("expected 2 languages, got %d", len(f.Languages))
				}
			},
		},
		{
			name:     "with stars",
			minStars: 100,
			maxStars: 1000,
			checkFn: func(t *testing.T, f *MetadataFilter) {
				if f.MinStars != 100 || f.MaxStars != 1000 {
					t.Errorf("stars mismatch: min=%d, max=%d", f.MinStars, f.MaxStars)
				}
			},
		},
		{
			name:           "with valid duration",
			lastPushWithin: "30d",
			checkFn: func(t *testing.T, f *MetadataFilter) {
				if f.LastPushAfter.IsZero() {
					t.Error("expected non-zero LastPushAfter")
				}
			},
		},
		{
			name:           "with invalid duration",
			lastPushWithin: "invalid",
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildFilterFromOptions(tt.language, tt.minStars, tt.maxStars, tt.lastPushWithin)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildFilterFromOptions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.checkFn != nil {
				tt.checkFn(t, got)
			}
		})
	}
}
