package merge

// ConflictReport contains detected conflicts from merge analysis
type ConflictReport struct {
	Source         string
	Target         string
	MergeBase      string
	TotalConflicts int
	Conflicts      []*Conflict
	CanAutoResolve int
	Difficulty     MergeDifficulty
}

// Conflict represents a single merge conflict
type Conflict struct {
	FilePath       string
	ConflictType   ConflictType
	SourceChange   ChangeType
	TargetChange   ChangeType
	Severity       ConflictSeverity
	AutoResolvable bool
	Description    string
}

// ConflictType defines the type of conflict
type ConflictType string

const (
	ConflictContent ConflictType = "content" // Content conflicts
	ConflictRename  ConflictType = "rename"  // Rename conflicts
	ConflictDelete  ConflictType = "delete"  // Delete/modify conflicts
	ConflictBinary  ConflictType = "binary"  // Binary file conflicts
)

// ChangeType defines the type of change
type ChangeType string

const (
	ChangeAdded    ChangeType = "added"
	ChangeModified ChangeType = "modified"
	ChangeDeleted  ChangeType = "deleted"
	ChangeRenamed  ChangeType = "renamed"
	ChangeCopied   ChangeType = "copied"
)

// ConflictSeverity indicates conflict severity
type ConflictSeverity string

const (
	SeverityLow    ConflictSeverity = "low"
	SeverityMedium ConflictSeverity = "medium"
	SeverityHigh   ConflictSeverity = "high"
)

// MergeDifficulty indicates merge complexity
type MergeDifficulty string

const (
	DifficultyTrivial MergeDifficulty = "trivial" // No conflicts
	DifficultyEasy    MergeDifficulty = "easy"    // Auto-resolvable
	DifficultyMedium  MergeDifficulty = "medium"  // Some manual work
	DifficultyHard    MergeDifficulty = "hard"    // Many conflicts
)

// DetectOptions configures conflict detection
type DetectOptions struct {
	Source        string
	Target        string
	BaseCommit    string
	IncludeBinary bool
}

// MergePreview shows what will happen during merge
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

// MergeStrategy defines merge approach
type MergeStrategy string

const (
	StrategyFastForward MergeStrategy = "fast-forward"
	StrategyRecursive   MergeStrategy = "recursive"
	StrategyOurs        MergeStrategy = "ours"
	StrategyTheirs      MergeStrategy = "theirs"
	StrategyOctopus     MergeStrategy = "octopus"
)

// MergeOptions configures merge operation
type MergeOptions struct {
	Source           string
	Target           string
	Strategy         MergeStrategy
	AllowFastForward bool
	CommitMessage    string
	NoCommit         bool
	Squash           bool
}

// MergeResult contains merge outcome
type MergeResult struct {
	Success      bool
	Strategy     MergeStrategy
	CommitHash   string
	Conflicts    []*Conflict
	FilesChanged int
	Additions    int
	Deletions    int
	Message      string
}

// RebaseOptions configures rebase operation
type RebaseOptions struct {
	Branch         string
	Onto           string
	Interactive    bool
	AutoSquash     bool
	PreserveMerges bool
	UpstreamName   string
}

// RebaseResult contains rebase outcome
type RebaseResult struct {
	Success        bool
	CommitsRebased int
	ConflictsFound int
	CurrentCommit  string
	Status         RebaseStatus
	Message        string
}

// RebaseStatus indicates rebase state
type RebaseStatus string

const (
	RebaseComplete   RebaseStatus = "complete"
	RebaseInProgress RebaseStatus = "in_progress"
	RebaseConflict   RebaseStatus = "conflict"
	RebaseAborted    RebaseStatus = "aborted"
)

// RebaseAction defines interactive rebase action
type RebaseAction string

const (
	ActionPick   RebaseAction = "pick"
	ActionReword RebaseAction = "reword"
	ActionEdit   RebaseAction = "edit"
	ActionSquash RebaseAction = "squash"
	ActionFixup  RebaseAction = "fixup"
	ActionDrop   RebaseAction = "drop"
)

// ResolutionResult contains resolution outcome
type ResolutionResult struct {
	TotalConflicts int
	Resolved       int
	Failed         int
	Resolutions    []*Resolution
}

// Resolution represents a single conflict resolution
type Resolution struct {
	FilePath     string
	ConflictType ConflictType
	Strategy     ResolutionStrategy
	Success      bool
	Error        error
}

// ResolutionStrategy defines how to resolve
type ResolutionStrategy string

const (
	StrategyKeepBoth   ResolutionStrategy = "keep_both"
	StrategyKeepOurs   ResolutionStrategy = "keep_ours"
	StrategyKeepTheirs ResolutionStrategy = "keep_theirs"
	StrategyMergeLines ResolutionStrategy = "merge_lines"
	StrategyNormalize  ResolutionStrategy = "normalize"
)

// FileChange represents a change to a file
type FileChange struct {
	Path       string
	ChangeType ChangeType
	OldPath    string // For renames
}
