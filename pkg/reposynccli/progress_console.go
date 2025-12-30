package reposynccli

import (
	"fmt"
	"io"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
)

// ConsoleProgressSink prints a simple textual progress log.
type ConsoleProgressSink struct {
	Out io.Writer
}

func (s ConsoleProgressSink) writer() io.Writer {
	if s.Out != nil {
		return s.Out
	}
	return io.Discard
}

// OnStart implements ProgressSink.
func (s ConsoleProgressSink) OnStart(action reposync.Action) {
	fmt.Fprintf(s.writer(), "→ [%s] %s (%s)\n", action.Type, action.Repo.Name, action.Repo.TargetPath)
}

// OnProgress implements ProgressSink.
func (s ConsoleProgressSink) OnProgress(action reposync.Action, message string, progress float64) {
	fmt.Fprintf(s.writer(), "   [%s] %s: %s (%.0f%%)\n", action.Type, action.Repo.Name, message, progress*100)
}

// OnComplete implements ProgressSink.
func (s ConsoleProgressSink) OnComplete(result reposync.ActionResult) {
	if result.Error != nil {
		fmt.Fprintf(s.writer(), "✗ [%s] %s: %v\n", result.Action.Type, result.Action.Repo.Name, result.Error)
		return
	}
	fmt.Fprintf(s.writer(), "✓ [%s] %s: %s\n", result.Action.Type, result.Action.Repo.Name, result.Message)
}
