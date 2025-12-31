package repository

import (
	"path/filepath"
	"testing"
)

func TestGetRelativePath(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "tmp", "repo-root")
	target := filepath.Join(root, "child")

	got := getRelativePath(root, target)
	if got != "child" {
		t.Fatalf("expected relative path %q, got %q", "child", got)
	}

	fallback := getRelativePath("", "repo")
	if fallback != "repo" {
		t.Fatalf("expected fallback path %q, got %q", "repo", fallback)
	}
}

func TestCalculateSummary(t *testing.T) {
	results := []RepositoryUpdateResult{
		{Status: StatusUpToDate},
		{Status: StatusError},
		{Status: StatusUpToDate},
	}

	expected := map[string]int{
		StatusUpToDate: 2,
		StatusError:    1,
	}
	assertSummary(t, calculateSummary(results), expected)
}

func TestCalculateFetchSummaryCounts(t *testing.T) {
	results := []RepositoryFetchResult{
		{Status: StatusFetched},
		{Status: StatusUpToDate},
		{Status: StatusFetched},
	}

	expected := map[string]int{
		StatusFetched:  2,
		StatusUpToDate: 1,
	}
	assertSummary(t, calculateFetchSummary(results), expected)
}

func TestCalculatePullSummary(t *testing.T) {
	results := []RepositoryPullResult{
		{Status: StatusPulled},
		{Status: StatusError},
		{Status: StatusPulled},
	}

	expected := map[string]int{
		StatusPulled: 2,
		StatusError:  1,
	}
	assertSummary(t, calculatePullSummary(results), expected)
}

func TestCalculatePushSummaryCounts(t *testing.T) {
	results := []RepositoryPushResult{
		{Status: StatusPushed},
		{Status: StatusUpToDate},
		{Status: StatusPushed},
	}

	expected := map[string]int{
		StatusPushed:   2,
		StatusUpToDate: 1,
	}
	assertSummary(t, calculatePushSummary(results), expected)
}

func TestCalculateStatusSummary(t *testing.T) {
	results := []RepositoryStatusResult{
		{Status: StatusClean},
		{Status: StatusDirty},
		{Status: StatusDirty},
	}

	expected := map[string]int{
		StatusClean: 1,
		StatusDirty: 2,
	}
	assertSummary(t, calculateStatusSummary(results), expected)
}

func assertSummary(t *testing.T, got, expected map[string]int) {
	t.Helper()

	if len(got) != len(expected) {
		t.Fatalf("expected %d statuses, got %d", len(expected), len(got))
	}
	for status, count := range expected {
		if got[status] != count {
			t.Fatalf("status %q: expected %d, got %d", status, count, got[status])
		}
	}
}
