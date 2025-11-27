package merge

import "errors"

var (
	// ErrInvalidBranch indicates an invalid branch reference
	ErrInvalidBranch = errors.New("invalid branch reference")

	// ErrBranchNotFound indicates the specified branch does not exist
	ErrBranchNotFound = errors.New("branch not found")

	// ErrNoMergeBase indicates no common ancestor found
	ErrNoMergeBase = errors.New("no merge base found between branches")

	// ErrMergeConflict indicates unresolvable merge conflicts
	ErrMergeConflict = errors.New("merge conflicts detected")

	// ErrRebaseInProgress indicates a rebase is already in progress
	ErrRebaseInProgress = errors.New("rebase already in progress")

	// ErrNoRebaseInProgress indicates no rebase to continue/abort
	ErrNoRebaseInProgress = errors.New("no rebase in progress")

	// ErrDirtyWorkingTree indicates uncommitted changes exist
	ErrDirtyWorkingTree = errors.New("working tree has uncommitted changes")

	// ErrInvalidStrategy indicates an unsupported merge strategy
	ErrInvalidStrategy = errors.New("invalid or unsupported merge strategy")

	// ErrBinaryConflict indicates a binary file conflict
	ErrBinaryConflict = errors.New("binary file conflict cannot be auto-resolved")

	// ErrAlreadyUpToDate indicates target is already up to date
	ErrAlreadyUpToDate = errors.New("already up to date")
)
