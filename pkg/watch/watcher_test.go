package watch

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

// TestNewWatcher tests watcher creation.
func TestNewWatcher(t *testing.T) {
	client := repository.NewClient()

	tests := []struct {
		name    string
		options WatchOptions
		wantErr bool
	}{
		{
			name:    "default options",
			options: WatchOptions{},
			wantErr: false,
		},
		{
			name: "custom interval",
			options: WatchOptions{
				Interval: 5 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "with debounce",
			options: WatchOptions{
				DebounceDuration: 1 * time.Second,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			watcher, err := NewWatcher(client, tt.options)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewWatcher() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && watcher != nil {
				defer watcher.Stop()
			}
		})
	}
}

// TestEventTypeString tests EventType string representation.
func TestEventTypeString(t *testing.T) {
	tests := []struct {
		eventType EventType
		want      string
	}{
		{EventTypeModified, "modified"},
		{EventTypeStaged, "staged"},
		{EventTypeUntracked, "untracked"},
		{EventTypeDeleted, "deleted"},
		{EventTypeCommit, "commit"},
		{EventTypeBranch, "branch"},
		{EventTypeClean, "clean"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.eventType.String(); got != tt.want {
				t.Errorf("EventType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestWatchOptionsDefaults tests that default options are applied.
func TestWatchOptionsDefaults(t *testing.T) {
	client := repository.NewClient()
	watcher, err := NewWatcher(client, WatchOptions{})
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}
	defer watcher.Stop()

	// We can't directly test the options, but we can verify the watcher was created
	if watcher == nil {
		t.Error("NewWatcher() returned nil watcher")
	}
}

// TestEqualStringSlices tests the equalStringSlices helper function.
func TestEqualStringSlices(t *testing.T) {
	tests := []struct {
		name string
		a    []string
		b    []string
		want bool
	}{
		{
			name: "empty slices",
			a:    []string{},
			b:    []string{},
			want: true,
		},
		{
			name: "equal slices",
			a:    []string{"a", "b", "c"},
			b:    []string{"a", "b", "c"},
			want: true,
		},
		{
			name: "different lengths",
			a:    []string{"a", "b"},
			b:    []string{"a", "b", "c"},
			want: false,
		},
		{
			name: "different values",
			a:    []string{"a", "b", "c"},
			b:    []string{"a", "x", "c"},
			want: false,
		},
		{
			name: "nil vs empty",
			a:    nil,
			b:    []string{},
			want: true, // Both have length 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := equalStringSlices(tt.a, tt.b); got != tt.want {
				t.Errorf("equalStringSlices() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestNoopLogger tests the noop logger implementation.
func TestNoopLogger(t *testing.T) {
	logger := &noopLogger{}

	// These should not panic
	logger.Debug("test %s", "debug")
	logger.Info("test %s", "info")
	logger.Warn("test %s", "warn")
	logger.Error("test %s", "error")
}

// TestWatcherChannels tests that watcher provides access to channels.
func TestWatcherChannels(t *testing.T) {
	client := repository.NewClient()
	watcher, err := NewWatcher(client, WatchOptions{})
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}
	defer watcher.Stop()

	if watcher.Events() == nil {
		t.Error("Events() returned nil channel")
	}

	if watcher.Errors() == nil {
		t.Error("Errors() returned nil channel")
	}
}

// TestWatcherStopWithoutStart tests stopping a watcher that was never started.
func TestWatcherStopWithoutStart(t *testing.T) {
	client := repository.NewClient()
	watcher, err := NewWatcher(client, WatchOptions{})
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}

	// Should be safe to stop without starting
	if err := watcher.Stop(); err != nil {
		t.Errorf("Stop() error = %v", err)
	}
}

// TestDetectChanges tests change detection logic through integration.
// Note: We test through the public interface rather than accessing internal methods.
func TestDetectChanges(t *testing.T) {
	t.Skip("Skipping internal change detection test - requires mock repository")
	// This test would require setting up a mock Git repository
	// and triggering actual changes to test detection.
	// For now, we rely on integration tests.
}

// BenchmarkWatcherCreation benchmarks watcher creation.
func BenchmarkWatcherCreation(b *testing.B) {
	client := repository.NewClient()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		watcher, err := NewWatcher(client, WatchOptions{})
		if err != nil {
			b.Fatalf("NewWatcher() error = %v", err)
		}
		watcher.Stop()
	}
}

// BenchmarkEventDetection benchmarks change detection logic.
func BenchmarkEventDetection(b *testing.B) {
	client := repository.NewClient()
	wtchr, err := NewWatcher(client, WatchOptions{})
	if err != nil {
		b.Fatalf("NewWatcher() error = %v", err)
	}
	defer wtchr.Stop()

	w := wtchr.(*watcher)

	oldStatus := &repository.Status{
		IsClean:       false,
		ModifiedFiles: []string{"file1.go", "file2.go"},
		StagedFiles:   []string{},
	}

	newStatus := &repository.Status{
		IsClean:       false,
		ModifiedFiles: []string{"file1.go", "file2.go", "file3.go"},
		StagedFiles:   []string{"file1.go"},
	}

	state := &repoState{
		path:          "/test/repo",
		lastStatus:    oldStatus,
		currentBranch: "main",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = w.detectChanges(state, newStatus, "main")
	}
}

// BenchmarkEqualStringSlices benchmarks string slice comparison.
func BenchmarkEqualStringSlices(b *testing.B) {
	slice1 := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}
	slice2 := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "k"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = equalStringSlices(slice1, slice2)
	}
}

// BenchmarkWatcherWithMultipleRepos benchmarks watching multiple repositories.
func BenchmarkWatcherWithMultipleRepos(b *testing.B) {
	client := repository.NewClient()

	// Benchmark with different repository counts
	for _, repoCount := range []int{1, 5, 10, 20} {
		b.Run(fmt.Sprintf("repos=%d", repoCount), func(b *testing.B) {
			wtchr, err := NewWatcher(client, WatchOptions{
				Interval: 100 * time.Millisecond,
			})
			if err != nil {
				b.Fatalf("NewWatcher() error = %v", err)
			}
			defer wtchr.Stop()

			w := wtchr.(*watcher)

			// Setup: Create mock repository states
			for i := 0; i < repoCount; i++ {
				path := fmt.Sprintf("/test/repo%d", i)
				w.watching[path] = &repoState{
					path: path,
					lastStatus: &repository.Status{
						IsClean: true,
					},
					currentBranch: "main",
				}
			}

			b.ResetTimer()
			b.ReportAllocs()

			// Memory usage per operation
			for i := 0; i < b.N; i++ {
				// Simulate event generation for all repos
				for path, state := range w.watching {
					newStatus := &repository.Status{
						IsClean:       false,
						ModifiedFiles: []string{fmt.Sprintf("%s/file.go", path)},
					}
					_ = w.detectChanges(state, newStatus, "main")
				}
			}
		})
	}
}

// TestWatcherStartInvalidPath tests starting watcher with invalid path.
func TestWatcherStartInvalidPath(t *testing.T) {
	client := repository.NewClient()
	watcher, err := NewWatcher(client, WatchOptions{})
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}
	defer watcher.Stop()

	ctx := context.Background()

	// Try to watch a non-existent path
	err = watcher.Start(ctx, []string{"/nonexistent/path/to/repo"})
	if err == nil {
		t.Error("Start() with invalid path should return error")
	}
}
