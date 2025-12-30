package reposync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// FileStateStore persists run state to a JSON file for resume/audit.
type FileStateStore struct {
	path string
	mu   sync.Mutex
}

// NewFileStateStore creates a new file-backed state store.
func NewFileStateStore(path string) *FileStateStore {
	return &FileStateStore{
		path: filepath.Clean(path),
	}
}

// Save writes state to disk atomically.
func (s *FileStateStore) Save(ctx context.Context, state RunState) error {
	if s.path == "" {
		return errors.New("state file path is empty")
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write temp state: %w", err)
	}

	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("move state file: %w", err)
	}

	return nil
}

// Load reads state from disk. Missing files return an empty state.
func (s *FileStateStore) Load(ctx context.Context) (RunState, error) {
	if s.path == "" {
		return RunState{}, errors.New("state file path is empty")
	}
	if ctx.Err() != nil {
		return RunState{}, ctx.Err()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return RunState{}, nil
		}
		return RunState{}, fmt.Errorf("read state file: %w", err)
	}

	var state RunState
	if err := json.Unmarshal(data, &state); err != nil {
		return RunState{}, fmt.Errorf("unmarshal state: %w", err)
	}

	return state, nil
}
