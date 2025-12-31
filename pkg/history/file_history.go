// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package history

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

// FileHistoryTracker tracks file evolution and changes.
type FileHistoryTracker interface {
	GetHistory(ctx context.Context, repo *repository.Repository, path string, opts HistoryOptions) ([]*FileCommit, error)
	GetBlame(ctx context.Context, repo *repository.Repository, path string) (*BlameInfo, error)
}

type fileHistoryTracker struct {
	executor GitExecutor
}

// NewFileHistoryTracker creates a new file history tracker.
func NewFileHistoryTracker(executor GitExecutor) FileHistoryTracker {
	return &fileHistoryTracker{
		executor: executor,
	}
}

// GetHistory retrieves the commit history for a specific file.
func (f *fileHistoryTracker) GetHistory(ctx context.Context, repo *repository.Repository, path string, opts HistoryOptions) ([]*FileCommit, error) {
	if path == "" {
		return nil, ErrFileNotFound
	}

	// Build git log command
	args := []string{"log", "--format=%H|%an|%ae|%ct|%s", "--numstat"}

	if opts.Follow {
		args = append(args, "--follow")
	}

	if opts.MaxCount > 0 {
		args = append(args, fmt.Sprintf("--max-count=%d", opts.MaxCount))
	}

	if !opts.Since.IsZero() {
		args = append(args, fmt.Sprintf("--since=%s", opts.Since.Format(time.RFC3339)))
	}

	if !opts.Until.IsZero() {
		args = append(args, fmt.Sprintf("--until=%s", opts.Until.Format(time.RFC3339)))
	}

	if opts.Author != "" {
		args = append(args, fmt.Sprintf("--author=%s", opts.Author))
	}

	args = append(args, "--", path)

	// Execute git log
	result, err := f.executor.Run(ctx, repo.Path, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get file history: %w", err)
	}

	// Parse output
	commits, err := f.parseFileHistory(result.Stdout, path)
	if err != nil {
		return nil, err
	}

	if len(commits) == 0 {
		return nil, ErrFileNotFound
	}

	return commits, nil
}

// GetBlame retrieves line-by-line authorship information for a file.
func (f *fileHistoryTracker) GetBlame(ctx context.Context, repo *repository.Repository, path string) (*BlameInfo, error) {
	if path == "" {
		return nil, ErrFileNotFound
	}

	// Build git blame command
	args := []string{"blame", "-e", "--date=iso", path}

	// Execute git blame
	result, err := f.executor.Run(ctx, repo.Path, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get file blame: %w", err)
	}

	// Parse output
	blameInfo, err := f.parseBlame(result.Stdout, path)
	if err != nil {
		return nil, err
	}

	return blameInfo, nil
}

func (f *fileHistoryTracker) parseFileHistory(output, targetPath string) ([]*FileCommit, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return nil, nil
	}

	var commits []*FileCommit
	var currentCommit *FileCommit

	i := 0
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			i++
			continue
		}

		// Check if this is a commit line (format: hash|author|email|timestamp|message)
		if strings.Contains(line, "|") {
			parts := strings.SplitN(line, "|", 5)
			if len(parts) == 5 {
				// Save previous commit if exists
				if currentCommit != nil {
					commits = append(commits, currentCommit)
				}

				// Parse timestamp (default to zero time if malformed)
				timestamp, _ := strconv.ParseInt(parts[3], 10, 64) //nolint:errcheck // malformed input -> 0
				commitTime := time.Unix(timestamp, 0)

				// Create new commit
				currentCommit = &FileCommit{
					Hash:        parts[0],
					Author:      parts[1],
					AuthorEmail: parts[2],
					Date:        commitTime,
					Message:     parts[4],
				}

				i++
				continue
			}
		}

		// Check if this is a numstat line (format: "additions deletions filename")
		// or a rename line (format: "additions deletions oldname => newname")
		fields := strings.Fields(line)
		if len(fields) >= 3 && currentCommit != nil {
			// Check for binary file indicator
			if fields[0] == "-" && fields[1] == "-" {
				currentCommit.IsBinary = true
				i++
				continue
			}

			// Parse additions/deletions (binary files may have non-numeric values)
			additions, _ := strconv.Atoi(fields[0]) //nolint:errcheck // malformed -> 0
			deletions, _ := strconv.Atoi(fields[1]) //nolint:errcheck // malformed -> 0

			// Check for rename (format: "oldpath => newpath")
			filename := strings.Join(fields[2:], " ")
			if strings.Contains(filename, "=>") {
				currentCommit.WasRenamed = true
				parts := strings.Split(filename, "=>")
				if len(parts) == 2 {
					currentCommit.OldPath = strings.TrimSpace(parts[0])
				}
			}

			currentCommit.LinesAdded = additions
			currentCommit.LinesDeleted = deletions
		}

		i++
	}

	// Don't forget the last commit
	if currentCommit != nil {
		commits = append(commits, currentCommit)
	}

	return commits, nil
}

func (f *fileHistoryTracker) parseBlame(output, path string) (*BlameInfo, error) {
	lines := strings.Split(output, "\n")

	blameInfo := &BlameInfo{
		FilePath: path,
		Lines:    make([]*BlameLine, 0),
	}

	for lineNum, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Parse blame line
		// Format: "hash (Author Name <email> date time timezone linenum) content"
		// Example: "abc123 (John Doe <john@example.com> 2025-11-27 12:00:00 +0000 1) package main"

		// Find the opening parenthesis
		openParen := strings.Index(line, "(")
		if openParen == -1 {
			continue
		}

		// Extract hash
		hash := strings.TrimSpace(line[:openParen])

		// Find the closing parenthesis before line number
		closeParen := strings.Index(line[openParen:], ")")
		if closeParen == -1 {
			continue
		}
		closeParen += openParen

		// Extract metadata (author, email, date, linenum)
		metadata := line[openParen+1 : closeParen]
		content := ""
		if closeParen+2 < len(line) {
			content = line[closeParen+2:]
		}

		// Parse author and email from metadata
		author, email, date := f.parseBlameMetadata(metadata)

		blameLine := &BlameLine{
			LineNumber:  lineNum + 1,
			Content:     content,
			Hash:        hash,
			Author:      author,
			AuthorEmail: email,
			Date:        date,
		}

		blameInfo.Lines = append(blameInfo.Lines, blameLine)
	}

	return blameInfo, nil
}

func (f *fileHistoryTracker) parseBlameMetadata(metadata string) (author, email string, date time.Time) {
	// Format: "Author Name <email> YYYY-MM-DD HH:MM:SS +ZZZZ linenum"
	// Extract email first
	emailStart := strings.Index(metadata, "<")
	emailEnd := strings.Index(metadata, ">")

	if emailStart != -1 && emailEnd != -1 {
		author = strings.TrimSpace(metadata[:emailStart])
		email = metadata[emailStart+1 : emailEnd]

		// Parse date after email
		dateStr := strings.TrimSpace(metadata[emailEnd+1:])
		// Remove line number from the end (last token)
		tokens := strings.Fields(dateStr)
		if len(tokens) >= 3 {
			// Reconstruct date string without line number
			dateStr = strings.Join(tokens[:len(tokens)-1], " ")
			// Try to parse ISO date format (returns zero time on parse failure)
			date, _ = time.Parse("2006-01-02 15:04:05 -0700", dateStr) //nolint:errcheck
		}
	}

	return
}
