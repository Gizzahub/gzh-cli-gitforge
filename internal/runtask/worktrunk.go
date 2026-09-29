package runtask

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

type worktrunkInventory struct {
	SchemaVersion int
	Paths         map[string]string
	// SkippedNoBranch and SkippedNoPath count schema 2 items dropped from
	// Paths. A silent skip is indistinguishable from a healthy repository
	// with fewer worktrees than expected, so callers that care need a number
	// to compare against rather than an absence -- but one number would not
	// do, because the two losses are not the same size. See the worktrunkList
	// doc comment: a branchless item was never anyone's key, while an item
	// with a branch and no path was. Every caller reports them differently,
	// so they are counted differently here.
	SkippedNoBranch int
	SkippedNoPath   int
	Diagnostic      CommandResult
}

func worktreePathOverride(path string) string {
	return "worktree-path = " + strconv.Quote(filepath.ToSlash(path))
}

// worktrunkList reads the Worktrunk schema 2 inventory and resolves it into
// a branch-to-path map.
//
// A malformed envelope -- wrong schema version, or no items array at all --
// is refused outright: there is no per-item data to salvage, so the only
// honest response is an error. A malformed item inside an otherwise valid
// envelope -- missing branch or worktree.path -- is skipped and counted in
// Skipped instead. That asymmetry is deliberate. Treating a bad item as
// fatal would let one unrelated transient worktree in the repository block
// every task command that reads this inventory.
//
// Skipping is not uniformly lossless, though, and the two halves of the
// predicate differ. Every consumer does a single-key lookup by branch
// (state.go, service_start.go), so an item with no branch could never have
// served as anyone's key: dropping it changes nothing a caller could
// observe. An item that has a branch but no worktree.path does have a key,
// and dropping it is observable -- the lookup misses and that one task is
// blocked by name, instead of the whole repository being refused. That
// downgrade is the point of this change rather than a side effect of it.
//
// Because the two halves differ, they are counted separately rather than
// summed. Callers act on them differently: doctor names the branchless half
// as the ordinary detached-worktree case and the pathless half as a task
// that will be reported missing, and service_start.go distinguishes a branch
// absent from the inventory from one listed at an unexpected path. A single
// total cannot support either distinction.
func (s *Service) worktrunkList(ctx context.Context) (worktrunkInventory, error) {
	result, err := s.command(ctx, s.root, "wt", "--config-set", "list.json-schema = 2", "list", "--format=json")
	if err != nil {
		return worktrunkInventory{}, err
	}
	if result.ExitCode != 0 {
		return worktrunkInventory{Diagnostic: result}, fmt.Errorf("worktrunk list failed: %s", strings.TrimSpace(result.Stderr))
	}
	var payload struct {
		Schema int `json:"schema"`
		Items  []struct {
			Branch   string `json:"branch"`
			Worktree struct {
				Path string `json:"path"`
			} `json:"worktree"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &payload); err != nil {
		return worktrunkInventory{Diagnostic: result}, fmt.Errorf("decode Worktrunk schema 2: %w", err)
	}
	if payload.Schema != 2 || payload.Items == nil {
		return worktrunkInventory{SchemaVersion: payload.Schema, Diagnostic: result}, fmt.Errorf("worktrunk list must return a schema 2 items envelope")
	}
	paths := map[string]string{}
	skippedNoBranch, skippedNoPath := 0, 0
	for _, item := range payload.Items {
		if strings.TrimSpace(item.Branch) == "" {
			// No branch means no key, so no lookup could ever have found this
			// item: dropping it is unobservable. Checked first, so an item
			// missing both fields counts here rather than as a path loss --
			// reporting a cost that was never paid is the thing this split
			// exists to stop.
			skippedNoBranch++
			continue
		}
		if strings.TrimSpace(item.Worktree.Path) == "" {
			// This one has a key. Dropping it is observable: a lookup for
			// that branch misses, and that one task is blocked by name.
			skippedNoPath++
			continue
		}
		absolute, err := filepath.Abs(item.Worktree.Path)
		if err != nil {
			return worktrunkInventory{SchemaVersion: payload.Schema, Diagnostic: result}, fmt.Errorf("resolve Worktrunk path: %w", err)
		}
		paths[item.Branch] = absolute
	}
	return worktrunkInventory{SchemaVersion: payload.Schema, Paths: paths, SkippedNoBranch: skippedNoBranch, SkippedNoPath: skippedNoPath, Diagnostic: result}, nil
}
