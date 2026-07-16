// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package merge

// ConflictReport contains detected conflicts from merge analysis.
type ConflictReport struct {
	Source         string
	Target         string
	MergeBase      string
	TotalConflicts int
	Conflicts      []*Conflict
	CanAutoResolve int
	Difficulty     MergeDifficulty
}

// Conflict represents a single merge conflict.
type Conflict struct {
	FilePath       string
	ConflictType   ConflictType
	SourceChange   ChangeType
	TargetChange   ChangeType
	Severity       ConflictSeverity
	AutoResolvable bool
	Description    string
}

// ConflictType defines the type of conflict.
type ConflictType string

// ConflictType values classify the nature of a merge conflict.
const (
	ConflictContent ConflictType = "content" // Content conflicts
	ConflictRename  ConflictType = "rename"  // Rename conflicts
	ConflictDelete  ConflictType = "delete"  // Delete/modify conflicts
	ConflictBinary  ConflictType = "binary"  // Binary file conflicts
)

// ChangeType defines the type of change.
type ChangeType string

// ChangeType values describe how a file was modified.
const (
	ChangeAdded    ChangeType = "added"
	ChangeModified ChangeType = "modified"
	ChangeDeleted  ChangeType = "deleted"
	ChangeRenamed  ChangeType = "renamed"
	ChangeCopied   ChangeType = "copied"
)

// ConflictSeverity indicates conflict severity.
type ConflictSeverity string

// ConflictSeverity values rank how serious a conflict is.
const (
	SeverityLow    ConflictSeverity = "low"
	SeverityMedium ConflictSeverity = "medium"
	SeverityHigh   ConflictSeverity = "high"
)

// MergeDifficulty indicates merge complexity.
type MergeDifficulty string

// MergeDifficulty values estimate the effort required to complete a merge.
const (
	DifficultyTrivial MergeDifficulty = "trivial" // No conflicts
	DifficultyEasy    MergeDifficulty = "easy"    // Auto-resolvable
	DifficultyMedium  MergeDifficulty = "medium"  // Some manual work
	DifficultyHard    MergeDifficulty = "hard"    // Many conflicts
)

// DetectOptions configures conflict detection.
type DetectOptions struct {
	Source        string
	Target        string
	BaseCommit    string
	IncludeBinary bool
}

// MergePreview shows what will happen during merge.
type MergePreview struct {
	Source         string
	Target         string
	CanFastForward bool
	FilesToChange  int
	FilesToAdd     int
	FilesToDelete  int
	Conflicts      []*Conflict
	Difficulty     MergeDifficulty
}

// FileChange represents a change to a file.
type FileChange struct {
	Path       string
	ChangeType ChangeType
	OldPath    string // For renames
}
