package history

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

func TestFileHistoryTracker_GetHistory(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		output    string
		opts      HistoryOptions
		wantCount int
		wantFirst *FileCommit
		wantErr   error
	}{
		{
			name: "basic file history",
			path: "main.go",
			output: `abc123|John Doe|john@example.com|1700000000|Initial commit

10	5	main.go
def456|Jane Smith|jane@example.com|1700001000|Update main

3	2	main.go`,
			opts:      HistoryOptions{},
			wantCount: 2,
			wantFirst: &FileCommit{
				Hash:         "abc123",
				Author:       "John Doe",
				AuthorEmail:  "john@example.com",
				Message:      "Initial commit",
				LinesAdded:   10,
				LinesDeleted: 5,
			},
		},
		{
			name: "file with rename",
			path: "newfile.go",
			output: `abc123|John Doe|john@example.com|1700000000|Rename file

10	5	oldfile.go => newfile.go`,
			opts:      HistoryOptions{Follow: true},
			wantCount: 1,
			wantFirst: &FileCommit{
				Hash:         "abc123",
				Author:       "John Doe",
				AuthorEmail:  "john@example.com",
				Message:      "Rename file",
				LinesAdded:   10,
				LinesDeleted: 5,
				WasRenamed:   true,
				OldPath:      "oldfile.go",
			},
		},
		{
			name: "binary file",
			path: "image.png",
			output: `abc123|John Doe|john@example.com|1700000000|Add image

-	-	image.png`,
			opts:      HistoryOptions{},
			wantCount: 1,
			wantFirst: &FileCommit{
				Hash:        "abc123",
				Author:      "John Doe",
				AuthorEmail: "john@example.com",
				Message:     "Add image",
				IsBinary:    true,
			},
		},
		{
			name:      "empty path",
			path:      "",
			output:    "",
			opts:      HistoryOptions{},
			wantCount: 0,
			wantErr:   ErrFileNotFound,
		},
		{
			name:      "file not found",
			path:      "nonexistent.go",
			output:    "",
			opts:      HistoryOptions{},
			wantCount: 0,
			wantErr:   ErrFileNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &mockExecutor{
				runFunc: func(ctx context.Context, repoPath string, args ...string) (*gitcmd.Result, error) {
					return &gitcmd.Result{
						Stdout:   tt.output,
						Stderr:   "",
						ExitCode: 0,
					}, nil
				},
			}

			tracker := &fileHistoryTracker{executor: executor}
			repo := &repository.Repository{Path: "/test/repo"}

			got, err := tracker.GetHistory(context.Background(), repo, tt.path, tt.opts)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("GetHistory() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("GetHistory() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("GetHistory() unexpected error = %v", err)
			}

			if len(got) != tt.wantCount {
				t.Errorf("len(commits) = %d, want %d", len(got), tt.wantCount)
			}

			if tt.wantFirst != nil && len(got) > 0 {
				first := got[0]

				if first.Hash != tt.wantFirst.Hash {
					t.Errorf("Hash = %q, want %q", first.Hash, tt.wantFirst.Hash)
				}

				if first.Author != tt.wantFirst.Author {
					t.Errorf("Author = %q, want %q", first.Author, tt.wantFirst.Author)
				}

				if first.AuthorEmail != tt.wantFirst.AuthorEmail {
					t.Errorf("AuthorEmail = %q, want %q", first.AuthorEmail, tt.wantFirst.AuthorEmail)
				}

				if first.Message != tt.wantFirst.Message {
					t.Errorf("Message = %q, want %q", first.Message, tt.wantFirst.Message)
				}

				if first.LinesAdded != tt.wantFirst.LinesAdded {
					t.Errorf("LinesAdded = %d, want %d", first.LinesAdded, tt.wantFirst.LinesAdded)
				}

				if first.LinesDeleted != tt.wantFirst.LinesDeleted {
					t.Errorf("LinesDeleted = %d, want %d", first.LinesDeleted, tt.wantFirst.LinesDeleted)
				}

				if first.IsBinary != tt.wantFirst.IsBinary {
					t.Errorf("IsBinary = %v, want %v", first.IsBinary, tt.wantFirst.IsBinary)
				}

				if first.WasRenamed != tt.wantFirst.WasRenamed {
					t.Errorf("WasRenamed = %v, want %v", first.WasRenamed, tt.wantFirst.WasRenamed)
				}

				if first.OldPath != tt.wantFirst.OldPath {
					t.Errorf("OldPath = %q, want %q", first.OldPath, tt.wantFirst.OldPath)
				}
			}
		})
	}
}

func TestFileHistoryTracker_GetHistoryWithOptions(t *testing.T) {
	tests := []struct {
		name     string
		opts     HistoryOptions
		wantArgs []string
	}{
		{
			name: "with follow",
			opts: HistoryOptions{Follow: true},
			wantArgs: []string{
				"log", "--format=%H|%an|%ae|%ct|%s", "--numstat", "--follow", "--",
				"file.go",
			},
		},
		{
			name: "with max count",
			opts: HistoryOptions{MaxCount: 10},
			wantArgs: []string{
				"log", "--format=%H|%an|%ae|%ct|%s", "--numstat", "--max-count=10",
				"--", "file.go",
			},
		},
		{
			name: "with since",
			opts: HistoryOptions{Since: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
			wantArgs: []string{
				"log", "--format=%H|%an|%ae|%ct|%s", "--numstat",
				"--since=2025-01-01T00:00:00Z", "--", "file.go",
			},
		},
		{
			name: "with until",
			opts: HistoryOptions{Until: time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)},
			wantArgs: []string{
				"log", "--format=%H|%an|%ae|%ct|%s", "--numstat",
				"--until=2025-12-31T23:59:59Z", "--", "file.go",
			},
		},
		{
			name: "with author",
			opts: HistoryOptions{Author: "John Doe"},
			wantArgs: []string{
				"log", "--format=%H|%an|%ae|%ct|%s", "--numstat",
				"--author=John Doe", "--", "file.go",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedArgs []string
			executor := &mockExecutor{
				runFunc: func(ctx context.Context, repoPath string, args ...string) (*gitcmd.Result, error) {
					capturedArgs = args
					return &gitcmd.Result{
						Stdout: `abc123|John Doe|john@example.com|1700000000|Test

10	5	file.go`,
						Stderr:   "",
						ExitCode: 0,
					}, nil
				},
			}

			tracker := &fileHistoryTracker{executor: executor}
			repo := &repository.Repository{Path: "/test/repo"}

			_, err := tracker.GetHistory(context.Background(), repo, "file.go", tt.opts)
			if err != nil {
				t.Fatalf("GetHistory() unexpected error = %v", err)
			}

			if len(capturedArgs) != len(tt.wantArgs) {
				t.Errorf("len(args) = %d, want %d", len(capturedArgs), len(tt.wantArgs))
				t.Errorf("got args: %v", capturedArgs)
				t.Errorf("want args: %v", tt.wantArgs)
				return
			}

			for i, arg := range capturedArgs {
				if arg != tt.wantArgs[i] {
					t.Errorf("args[%d] = %q, want %q", i, arg, tt.wantArgs[i])
				}
			}
		})
	}
}

func TestFileHistoryTracker_GetBlame(t *testing.T) {
	tests := []struct {
		name          string
		path          string
		output        string
		wantLineCount int
		wantFirstLine *BlameLine
		wantErr       error
	}{
		{
			name: "basic blame",
			path: "main.go",
			output: `abc123 (John Doe <john@example.com> 2025-11-27 12:00:00 +0000 1) package main
def456 (Jane Smith <jane@example.com> 2025-11-27 13:00:00 +0000 2) import "fmt"`,
			wantLineCount: 2,
			wantFirstLine: &BlameLine{
				LineNumber:  1,
				Content:     "package main",
				Hash:        "abc123",
				Author:      "John Doe",
				AuthorEmail: "john@example.com",
			},
		},
		{
			name:          "empty path",
			path:          "",
			output:        "",
			wantLineCount: 0,
			wantErr:       ErrFileNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &mockExecutor{
				runFunc: func(ctx context.Context, repoPath string, args ...string) (*gitcmd.Result, error) {
					return &gitcmd.Result{
						Stdout:   tt.output,
						Stderr:   "",
						ExitCode: 0,
					}, nil
				},
			}

			tracker := &fileHistoryTracker{executor: executor}
			repo := &repository.Repository{Path: "/test/repo"}

			got, err := tracker.GetBlame(context.Background(), repo, tt.path)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("GetBlame() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("GetBlame() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("GetBlame() unexpected error = %v", err)
			}

			if got.FilePath != tt.path {
				t.Errorf("FilePath = %q, want %q", got.FilePath, tt.path)
			}

			if len(got.Lines) != tt.wantLineCount {
				t.Errorf("len(Lines) = %d, want %d", len(got.Lines), tt.wantLineCount)
			}

			if tt.wantFirstLine != nil && len(got.Lines) > 0 {
				first := got.Lines[0]

				if first.LineNumber != tt.wantFirstLine.LineNumber {
					t.Errorf("LineNumber = %d, want %d", first.LineNumber, tt.wantFirstLine.LineNumber)
				}

				if first.Content != tt.wantFirstLine.Content {
					t.Errorf("Content = %q, want %q", first.Content, tt.wantFirstLine.Content)
				}

				if first.Hash != tt.wantFirstLine.Hash {
					t.Errorf("Hash = %q, want %q", first.Hash, tt.wantFirstLine.Hash)
				}

				if first.Author != tt.wantFirstLine.Author {
					t.Errorf("Author = %q, want %q", first.Author, tt.wantFirstLine.Author)
				}

				if first.AuthorEmail != tt.wantFirstLine.AuthorEmail {
					t.Errorf("AuthorEmail = %q, want %q", first.AuthorEmail, tt.wantFirstLine.AuthorEmail)
				}
			}
		})
	}
}

func TestFileHistoryTracker_ParseFileHistory(t *testing.T) {
	tracker := &fileHistoryTracker{}

	tests := []struct {
		name      string
		output    string
		wantCount int
		checkFunc func(*testing.T, []*FileCommit)
	}{
		{
			name: "multiple commits",
			output: `abc123|John Doe|john@example.com|1700000000|First commit

10	5	main.go

def456|Jane Smith|jane@example.com|1700001000|Second commit

3	2	main.go`,
			wantCount: 2,
			checkFunc: func(t *testing.T, commits []*FileCommit) {
				if commits[0].Hash != "abc123" {
					t.Errorf("commits[0].Hash = %q, want \"abc123\"", commits[0].Hash)
				}
				if commits[1].Hash != "def456" {
					t.Errorf("commits[1].Hash = %q, want \"def456\"", commits[1].Hash)
				}
			},
		},
		{
			name:      "empty output",
			output:    "",
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tracker.parseFileHistory(tt.output, "main.go")
			if err != nil {
				t.Fatalf("parseFileHistory() unexpected error = %v", err)
			}

			if len(got) != tt.wantCount {
				t.Errorf("len(commits) = %d, want %d", len(got), tt.wantCount)
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, got)
			}
		})
	}
}

func TestFileHistoryTracker_ParseBlame(t *testing.T) {
	tracker := &fileHistoryTracker{}

	tests := []struct {
		name      string
		output    string
		path      string
		wantCount int
		checkFunc func(*testing.T, *BlameInfo)
	}{
		{
			name: "basic blame output",
			path: "main.go",
			output: `abc123 (John Doe <john@example.com> 2025-11-27 12:00:00 +0000 1) package main
def456 (Jane Smith <jane@example.com> 2025-11-27 13:00:00 +0000 2) import "fmt"
abc789 (Bob Jones <bob@example.com> 2025-11-27 14:00:00 +0000 3) func main() {`,
			wantCount: 3,
			checkFunc: func(t *testing.T, info *BlameInfo) {
				if info.FilePath != "main.go" {
					t.Errorf("FilePath = %q, want \"main.go\"", info.FilePath)
				}
				if info.Lines[0].Author != "John Doe" {
					t.Errorf("Lines[0].Author = %q, want \"John Doe\"", info.Lines[0].Author)
				}
				if info.Lines[1].Author != "Jane Smith" {
					t.Errorf("Lines[1].Author = %q, want \"Jane Smith\"", info.Lines[1].Author)
				}
				if info.Lines[2].Author != "Bob Jones" {
					t.Errorf("Lines[2].Author = %q, want \"Bob Jones\"", info.Lines[2].Author)
				}
			},
		},
		{
			name:      "empty output",
			path:      "test.go",
			output:    "",
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tracker.parseBlame(tt.output, tt.path)
			if err != nil {
				t.Fatalf("parseBlame() unexpected error = %v", err)
			}

			if len(got.Lines) != tt.wantCount {
				t.Errorf("len(Lines) = %d, want %d", len(got.Lines), tt.wantCount)
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, got)
			}
		})
	}
}

func TestFileHistoryTracker_ParseBlameMetadata(t *testing.T) {
	tests := []struct {
		name        string
		metadata    string
		wantAuthor  string
		wantEmail   string
		wantHasDate bool
	}{
		{
			name:        "standard format",
			metadata:    "John Doe <john@example.com> 2025-11-27 12:00:00 +0000 1",
			wantAuthor:  "John Doe",
			wantEmail:   "john@example.com",
			wantHasDate: true,
		},
		{
			name:        "with extra spaces",
			metadata:    "  Jane Smith  <jane@example.com>  2025-11-27 13:00:00 +0000  2  ",
			wantAuthor:  "Jane Smith",
			wantEmail:   "jane@example.com",
			wantHasDate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := &fileHistoryTracker{}
			author, email, date := tracker.parseBlameMetadata(tt.metadata)

			if author != tt.wantAuthor {
				t.Errorf("author = %q, want %q", author, tt.wantAuthor)
			}

			if email != tt.wantEmail {
				t.Errorf("email = %q, want %q", email, tt.wantEmail)
			}

			if tt.wantHasDate && date.IsZero() {
				t.Error("date should not be zero")
			}
		})
	}
}

func TestFileHistoryTracker_ParseFileHistory_Timestamps(t *testing.T) {
	tracker := &fileHistoryTracker{}

	baseTime := time.Date(2025, 11, 1, 12, 0, 0, 0, time.UTC).Unix()

	output := fmt.Sprintf(`abc123|John Doe|john@example.com|%d|First commit

10	5	main.go

def456|Jane Smith|jane@example.com|%d|Second commit

3	2	main.go`, baseTime, baseTime+3600)

	commits, err := tracker.parseFileHistory(output, "main.go")
	if err != nil {
		t.Fatalf("parseFileHistory() unexpected error = %v", err)
	}

	if len(commits) != 2 {
		t.Fatalf("len(commits) = %d, want 2", len(commits))
	}

	expectedFirst := time.Unix(baseTime, 0)
	expectedSecond := time.Unix(baseTime+3600, 0)

	if !commits[0].Date.Equal(expectedFirst) {
		t.Errorf("commits[0].Date = %v, want %v", commits[0].Date, expectedFirst)
	}

	if !commits[1].Date.Equal(expectedSecond) {
		t.Errorf("commits[1].Date = %v, want %v", commits[1].Date, expectedSecond)
	}
}

func TestFileHistoryTracker_ParseFileHistory_MultipleFiles(t *testing.T) {
	tracker := &fileHistoryTracker{}

	output := `abc123|John Doe|john@example.com|1700000000|Multi-file commit

10	5	main.go
3	2	util.go
7	1	config.go`

	commits, err := tracker.parseFileHistory(output, "main.go")
	if err != nil {
		t.Fatalf("parseFileHistory() unexpected error = %v", err)
	}

	if len(commits) != 1 {
		t.Fatalf("len(commits) = %d, want 1", len(commits))
	}

	// Should capture the last file's stats (config.go in this case)
	if commits[0].LinesAdded != 7 {
		t.Errorf("LinesAdded = %d, want 7", commits[0].LinesAdded)
	}

	if commits[0].LinesDeleted != 1 {
		t.Errorf("LinesDeleted = %d, want 1", commits[0].LinesDeleted)
	}
}

func TestFileHistoryTracker_ParseBlame_EmptyLines(t *testing.T) {
	tracker := &fileHistoryTracker{}

	output := `abc123 (John Doe <john@example.com> 2025-11-27 12:00:00 +0000 1) package main

def456 (Jane Smith <jane@example.com> 2025-11-27 13:00:00 +0000 3) import "fmt"`

	info, err := tracker.parseBlame(output, "main.go")
	if err != nil {
		t.Fatalf("parseBlame() unexpected error = %v", err)
	}

	// Should skip empty lines
	if len(info.Lines) != 2 {
		t.Errorf("len(Lines) = %d, want 2 (empty lines skipped)", len(info.Lines))
	}
}
