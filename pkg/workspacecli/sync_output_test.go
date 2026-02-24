package workspacecli

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
)

func sampleExecutionResult() reposync.ExecutionResult {
	return reposync.ExecutionResult{
		Succeeded: []reposync.ActionResult{
			{
				Action: reposync.Action{
					Repo: reposync.RepoSpec{Name: "repo-ok"},
					Type: reposync.ActionUpdate,
				},
				Message: "updated",
			},
		},
		Failed: []reposync.ActionResult{
			{
				Action: reposync.Action{
					Repo: reposync.RepoSpec{Name: "repo-fail"},
					Type: reposync.ActionUpdate,
				},
				Message: "failed",
				Error:   errors.New("git pull failed"),
			},
		},
		Skipped: []reposync.ActionResult{
			{
				Action: reposync.Action{
					Repo: reposync.RepoSpec{Name: "repo-skip"},
					Type: reposync.ActionSkip,
				},
				Message: "already up-to-date",
			},
		},
	}
}

func TestPrepareSyncResultJSON(t *testing.T) {
	result := prepareSyncResultJSON(sampleExecutionResult(), 1234)

	if result.Total != 3 {
		t.Fatalf("Total = %d, want 3", result.Total)
	}
	if result.Succeeded != 1 {
		t.Fatalf("Succeeded = %d, want 1", result.Succeeded)
	}
	if result.Failed != 1 {
		t.Fatalf("Failed = %d, want 1", result.Failed)
	}
	if result.Duration != 1234 {
		t.Fatalf("Duration = %d, want 1234", result.Duration)
	}
	if len(result.Repos) != 3 {
		t.Fatalf("Repos length = %d, want 3", len(result.Repos))
	}
}

func TestDisplaySyncResultsJSON(t *testing.T) {
	t.Run("compact by default", func(t *testing.T) {
		var buf bytes.Buffer
		err := displaySyncResultsJSON(&buf, sampleExecutionResult(), 100, false)
		if err != nil {
			t.Fatalf("displaySyncResultsJSON returned error: %v", err)
		}

		output := strings.TrimSpace(buf.String())
		if strings.Contains(output, "\n  ") {
			t.Fatalf("expected compact JSON, got pretty JSON: %q", output)
		}

		var parsed SyncResultJSON
		if err := json.Unmarshal([]byte(output), &parsed); err != nil {
			t.Fatalf("failed to parse JSON output: %v", err)
		}
		if parsed.Total != 3 {
			t.Fatalf("parsed total = %d, want 3", parsed.Total)
		}
	})

	t.Run("pretty with verbose", func(t *testing.T) {
		var buf bytes.Buffer
		err := displaySyncResultsJSON(&buf, sampleExecutionResult(), 100, true)
		if err != nil {
			t.Fatalf("displaySyncResultsJSON returned error: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "\n  \"total\"") {
			t.Fatalf("expected pretty JSON with indentation, got: %q", output)
		}
	})
}

func TestDisplaySyncResultsLLM(t *testing.T) {
	var buf bytes.Buffer
	err := displaySyncResultsLLM(&buf, sampleExecutionResult(), 100)
	if err != nil {
		t.Fatalf("displaySyncResultsLLM returned error: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Fatal("expected non-empty LLM output")
	}
	if !strings.Contains(strings.ToLower(output), "total") {
		t.Fatalf("expected LLM output to include summary fields, got: %q", output)
	}
}
