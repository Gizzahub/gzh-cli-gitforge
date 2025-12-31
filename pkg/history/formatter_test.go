package history

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestFormatter_FormatCommitStats(t *testing.T) {
	stats := &CommitStats{
		TotalCommits:   100,
		UniqueAuthors:  5,
		TotalAdditions: 1000,
		TotalDeletions: 500,
		FirstCommit:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		LastCommit:     time.Date(2025, 11, 27, 0, 0, 0, 0, time.UTC),
		DateRange:      331 * 24 * time.Hour, // ~331 days
		AvgPerDay:      0.302,
		AvgPerWeek:     2.114,
		AvgPerMonth:    9.06,
		PeakDay:        time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
		PeakCount:      5,
	}

	tests := []struct {
		name         string
		format       OutputFormat
		wantContains []string
		wantErr      error
	}{
		{
			name:   "table format",
			format: FormatTable,
			wantContains: []string{
				"Commit Statistics",
				"Total Commits:",
				"100",
				"Unique Authors:",
				"5",
			},
		},
		{
			name:   "json format",
			format: FormatJSON,
			wantContains: []string{
				`"TotalCommits": 100`,
				`"UniqueAuthors": 5`,
				`"TotalAdditions": 1000`,
			},
		},
		{
			name:   "csv format",
			format: FormatCSV,
			wantContains: []string{
				"Metric,Value",
				"Total Commits,100",
				"Unique Authors,5",
			},
		},
		{
			name:   "markdown format",
			format: FormatMarkdown,
			wantContains: []string{
				"# Commit Statistics",
				"| Metric | Value |",
				"| Total Commits | 100 |",
				"| Unique Authors | 5 |",
			},
		},
		{
			name:    "invalid format",
			format:  "invalid",
			wantErr: ErrInvalidFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := NewFormatter(tt.format)
			got, err := formatter.FormatCommitStats(stats)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("FormatCommitStats() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("FormatCommitStats() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("FormatCommitStats() unexpected error = %v", err)
			}

			output := string(got)
			for _, want := range tt.wantContains {
				if !strings.Contains(output, want) {
					t.Errorf("output missing %q\nGot:\n%s", want, output)
				}
			}
		})
	}
}

func TestFormatter_FormatCommitStats_NilStats(t *testing.T) {
	formatter := NewFormatter(FormatTable)
	_, err := formatter.FormatCommitStats(nil)

	if err == nil {
		t.Error("FormatCommitStats(nil) should return error")
	}
}

func TestFormatter_FormatContributors(t *testing.T) {
	contributors := []*Contributor{
		{
			Rank:           1,
			Name:           "John Doe",
			Email:          "john@example.com",
			TotalCommits:   50,
			LinesAdded:     500,
			LinesDeleted:   200,
			FilesTouched:   25,
			ActiveDays:     30,
			CommitsPerWeek: 10.5,
			FirstCommit:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			LastCommit:     time.Date(2025, 11, 27, 0, 0, 0, 0, time.UTC),
		},
		{
			Rank:           2,
			Name:           "Jane Smith",
			Email:          "jane@example.com",
			TotalCommits:   30,
			LinesAdded:     300,
			LinesDeleted:   150,
			FilesTouched:   15,
			ActiveDays:     20,
			CommitsPerWeek: 6.0,
			FirstCommit:    time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC),
			LastCommit:     time.Date(2025, 11, 27, 0, 0, 0, 0, time.UTC),
		},
	}

	tests := []struct {
		name         string
		format       OutputFormat
		wantContains []string
		wantErr      error
	}{
		{
			name:   "table format",
			format: FormatTable,
			wantContains: []string{
				"Contributors",
				"John Doe",
				"Jane Smith",
				"50",
				"30",
			},
		},
		{
			name:   "json format",
			format: FormatJSON,
			wantContains: []string{
				`"Name": "John Doe"`,
				`"Name": "Jane Smith"`,
				`"TotalCommits": 50`,
			},
		},
		{
			name:   "csv format",
			format: FormatCSV,
			wantContains: []string{
				"Rank,Name,Email",
				"1,John Doe,john@example.com",
				"2,Jane Smith,jane@example.com",
			},
		},
		{
			name:   "markdown format",
			format: FormatMarkdown,
			wantContains: []string{
				"# Contributors",
				"| Rank | Name |",
				"| 1 | John Doe |",
				"| 2 | Jane Smith |",
			},
		},
		{
			name:    "invalid format",
			format:  "invalid",
			wantErr: ErrInvalidFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := NewFormatter(tt.format)
			got, err := formatter.FormatContributors(contributors)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("FormatContributors() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("FormatContributors() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("FormatContributors() unexpected error = %v", err)
			}

			output := string(got)
			for _, want := range tt.wantContains {
				if !strings.Contains(output, want) {
					t.Errorf("output missing %q\nGot:\n%s", want, output)
				}
			}
		})
	}
}

func TestFormatter_FormatContributors_NilContributors(t *testing.T) {
	formatter := NewFormatter(FormatTable)
	_, err := formatter.FormatContributors(nil)

	if err == nil {
		t.Error("FormatContributors(nil) should return error")
	}
}

func TestFormatter_FormatFileHistory(t *testing.T) {
	history := []*FileCommit{
		{
			Hash:         "abc123def456",
			Author:       "John Doe",
			AuthorEmail:  "john@example.com",
			Date:         time.Date(2025, 11, 27, 12, 0, 0, 0, time.UTC),
			Message:      "Initial commit",
			LinesAdded:   100,
			LinesDeleted: 50,
			IsBinary:     false,
			WasRenamed:   false,
		},
		{
			Hash:         "def456abc789",
			Author:       "Jane Smith",
			AuthorEmail:  "jane@example.com",
			Date:         time.Date(2025, 11, 26, 12, 0, 0, 0, time.UTC),
			Message:      "Update file",
			LinesAdded:   20,
			LinesDeleted: 10,
			IsBinary:     false,
			WasRenamed:   false,
		},
	}

	tests := []struct {
		name         string
		format       OutputFormat
		wantContains []string
		wantErr      error
	}{
		{
			name:   "table format",
			format: FormatTable,
			wantContains: []string{
				"File History",
				"abc123d",
				"John Doe",
				"100",
				"50",
			},
		},
		{
			name:   "json format",
			format: FormatJSON,
			wantContains: []string{
				`"Hash": "abc123def456"`,
				`"Author": "John Doe"`,
				`"Message": "Initial commit"`,
			},
		},
		{
			name:   "csv format",
			format: FormatCSV,
			wantContains: []string{
				"Hash,Author,Email",
				"abc123def456,John Doe,john@example.com",
				"def456abc789,Jane Smith,jane@example.com",
			},
		},
		{
			name:   "markdown format",
			format: FormatMarkdown,
			wantContains: []string{
				"# File History",
				"| Hash | Date | Author |",
				"| abc12",
				"2025-11-27",
				"John Doe",
			},
		},
		{
			name:    "invalid format",
			format:  "invalid",
			wantErr: ErrInvalidFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := NewFormatter(tt.format)
			got, err := formatter.FormatFileHistory(history)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("FormatFileHistory() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("FormatFileHistory() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("FormatFileHistory() unexpected error = %v", err)
			}

			output := string(got)
			for _, want := range tt.wantContains {
				if !strings.Contains(output, want) {
					t.Errorf("output missing %q\nGot:\n%s", want, output)
				}
			}
		})
	}
}

func TestFormatter_FormatFileHistory_NilHistory(t *testing.T) {
	formatter := NewFormatter(FormatTable)
	_, err := formatter.FormatFileHistory(nil)

	if err == nil {
		t.Error("FormatFileHistory(nil) should return error")
	}
}

func TestFormatter_JSONFormat_ValidJSON(t *testing.T) {
	stats := &CommitStats{
		TotalCommits:  10,
		UniqueAuthors: 2,
	}

	formatter := NewFormatter(FormatJSON)
	got, err := formatter.FormatCommitStats(stats)
	if err != nil {
		t.Fatalf("FormatCommitStats() unexpected error = %v", err)
	}

	// Verify it's valid JSON
	var decoded CommitStats
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Errorf("output is not valid JSON: %v", err)
	}

	if decoded.TotalCommits != stats.TotalCommits {
		t.Errorf("TotalCommits = %d, want %d", decoded.TotalCommits, stats.TotalCommits)
	}
}

func TestFormatter_HelperFunctions(t *testing.T) {
	t.Run("formatTime", func(t *testing.T) {
		tests := []struct {
			name string
			time time.Time
			want string
		}{
			{"zero time", time.Time{}, "N/A"},
			{"valid time", time.Date(2025, 11, 27, 12, 30, 45, 0, time.UTC), "2025-11-27 12:30:45"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := formatTime(tt.time)
				if got != tt.want {
					t.Errorf("formatTime() = %q, want %q", got, tt.want)
				}
			})
		}
	})

	t.Run("formatDate", func(t *testing.T) {
		tests := []struct {
			name string
			time time.Time
			want string
		}{
			{"zero time", time.Time{}, "N/A"},
			{"valid time", time.Date(2025, 11, 27, 12, 30, 45, 0, time.UTC), "2025-11-27"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := formatDate(tt.time)
				if got != tt.want {
					t.Errorf("formatDate() = %q, want %q", got, tt.want)
				}
			})
		}
	})

	t.Run("formatDuration", func(t *testing.T) {
		tests := []struct {
			name     string
			duration time.Duration
			want     string
		}{
			{"zero", 0, "< 1 day"},
			{"12 hours", 12 * time.Hour, "< 1 day"},
			{"24 hours", 24 * time.Hour, "1 day"},
			{"48 hours", 48 * time.Hour, "2 days"},
			{"7 days", 7 * 24 * time.Hour, "7 days"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := formatDuration(tt.duration)
				if got != tt.want {
					t.Errorf("formatDuration() = %q, want %q", got, tt.want)
				}
			})
		}
	})

	t.Run("truncate", func(t *testing.T) {
		tests := []struct {
			name   string
			input  string
			maxLen int
			want   string
		}{
			{"shorter than max", "hello", 10, "hello"},
			{"equal to max", "hello", 5, "hello"},
			{"longer than max", "hello world", 8, "hello..."},
			{"very short max", "hello", 2, "he"},
			{"max of 3", "hello", 3, "hel"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := truncate(tt.input, tt.maxLen)
				if got != tt.want {
					t.Errorf("truncate() = %q, want %q", got, tt.want)
				}
			})
		}
	})
}

func TestFormatter_EmptyData(t *testing.T) {
	t.Run("empty contributors", func(t *testing.T) {
		formatter := NewFormatter(FormatTable)
		got, err := formatter.FormatContributors([]*Contributor{})
		if err != nil {
			t.Fatalf("FormatContributors([]) unexpected error = %v", err)
		}

		if len(got) == 0 {
			t.Error("output should not be empty")
		}
	})

	t.Run("empty file history", func(t *testing.T) {
		formatter := NewFormatter(FormatMarkdown)
		got, err := formatter.FormatFileHistory([]*FileCommit{})
		if err != nil {
			t.Fatalf("FormatFileHistory([]) unexpected error = %v", err)
		}

		if len(got) == 0 {
			t.Error("output should not be empty")
		}
	})
}
