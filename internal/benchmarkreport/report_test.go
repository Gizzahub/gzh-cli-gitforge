// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package benchmarkreport

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// fixtureMetadata is the metadata sidecar every fixture-independent case uses.
func fixtureMetadataJSON(t *testing.T) []byte {
	t.Helper()
	doc := map[string]string{
		"sourceCommit":       "81d3543",
		"goVersion":          "go1.26.7",
		"gitVersion":         "git version 2.50.0",
		"os":                 "darwin",
		"arch":               "arm64",
		"workload":           "synthetic-example",
		"observedAt":         "2026-10-02T00:00:00Z",
		"measurementCommand": "go test -bench=. -benchmem -count=3 ./benchmarks (synthetic fixture, not executed)",
	}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshaling metadata fixture: %v", err)
	}
	return data
}

// fixtureInput is one well-formed benchmark line for input-side cases.
const fixtureInput = "BenchmarkFoo-8\t100\t12.5 ns/op\n"

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "benchmarks", "testdata", name)) // #nosec G304 -- fixed fixture path inside the repository
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	return data
}

// canonicalJSON re-encodes arbitrary JSON bytes into a deterministic form so
// golden comparison checks structure and field spelling, not formatting.
func canonicalJSON(t *testing.T, data []byte) []byte {
	t.Helper()
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("parsing JSON: %v", err)
	}
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshaling JSON: %v", err)
	}
	return out
}

// findSummary returns the summary entry for name, failing the test when absent.
func findSummary(t *testing.T, report *Report, name string) NameSummary {
	t.Helper()
	for _, s := range report.Summary {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("no summary entry for %s", name)
	return NameSummary{}
}

func TestBenchmarkReportContract(t *testing.T) {
	input := readFixture(t, "report.input.txt")
	metadata := readFixture(t, "report.metadata.json")

	t.Run("metadata passes through to the report", func(t *testing.T) {
		report, err := Convert(input, metadata)
		if err != nil {
			t.Fatalf("converting fixtures: %v", err)
		}
		want := Metadata{
			SourceCommit:       "81d3543",
			GoVersion:          "go1.26.7",
			GitVersion:         "git version 2.50.0",
			OS:                 "darwin",
			Arch:               "arm64",
			Workload:           "synthetic-example",
			ObservedAt:         "2026-10-02T00:00:00Z",
			MeasurementCommand: "go test -bench=. -benchmem -count=3 ./benchmarks (synthetic fixture, not executed)",
			Note:               "synthetic example fixture for converter tests; not product performance evidence",
		}
		if report.Metadata != want {
			t.Errorf("metadata mismatch\ngot:  %#v\nwant: %#v", report.Metadata, want)
		}
		if report.SchemaVersion != 1 {
			t.Errorf("schemaVersion = %d, want 1", report.SchemaVersion)
		}
	})

	t.Run("golden fixture converts to the expected report", func(t *testing.T) {
		report, err := Convert(input, metadata)
		if err != nil {
			t.Fatalf("converting fixtures: %v", err)
		}
		got, err := report.Marshal()
		if err != nil {
			t.Fatalf("marshaling report: %v", err)
		}
		want := canonicalJSON(t, readFixture(t, "report.expected.json"))
		if gotCanonical := canonicalJSON(t, got); !reflect.DeepEqual(gotCanonical, want) {
			t.Errorf("converted report mismatch\ngot:  %s\nwant: %s", gotCanonical, want)
		}
	})

	t.Run("samples and summary order is deterministic", func(t *testing.T) {
		first, err := Convert(input, metadata)
		if err != nil {
			t.Fatalf("first conversion: %v", err)
		}
		second, err := Convert(input, metadata)
		if err != nil {
			t.Fatalf("second conversion: %v", err)
		}
		firstJSON, err := first.Marshal()
		if err != nil {
			t.Fatalf("marshaling first report: %v", err)
		}
		secondJSON, err := second.Marshal()
		if err != nil {
			t.Fatalf("marshaling second report: %v", err)
		}
		if !reflect.DeepEqual(firstJSON, secondJSON) {
			t.Error("two conversions of the same input differ")
		}

		// Samples: name ascending, input order preserved within a name.
		gotNames := make([]string, 0, len(first.Samples))
		for _, s := range first.Samples {
			gotNames = append(gotNames, s.Name)
		}
		wantNames := []string{
			"BenchmarkCLIBranchList-10",
			"BenchmarkCLIInfo-10",
			"BenchmarkCLIInfo-10",
			"BenchmarkCLIStatus-10",
			"BenchmarkCLIStatus-10",
			"BenchmarkCLIStatus-10",
		}
		if !slices.Equal(gotNames, wantNames) {
			t.Errorf("sample names = %v, want %v", gotNames, wantNames)
		}
		if !slices.IsSorted(gotNames) {
			t.Errorf("sample names not sorted: %v", gotNames)
		}

		// Repeated-name samples keep input order (1000, 1500, 750), not value order.
		statusIterations := make([]int64, 0, 3)
		for _, s := range first.Samples {
			if s.Name == "BenchmarkCLIStatus-10" {
				statusIterations = append(statusIterations, s.Iterations)
			}
		}
		if want := []int64{1000, 1500, 750}; !slices.Equal(statusIterations, want) {
			t.Errorf("repeated-name iterations = %v, want input order %v", statusIterations, want)
		}

		// Summary: same name order as samples, one entry per name.
		summaryNames := make([]string, 0, len(first.Summary))
		for _, s := range first.Summary {
			summaryNames = append(summaryNames, s.Name)
		}
		wantSummary := []string{"BenchmarkCLIBranchList-10", "BenchmarkCLIInfo-10", "BenchmarkCLIStatus-10"}
		if !slices.Equal(summaryNames, wantSummary) {
			t.Errorf("summary names = %v, want %v", summaryNames, wantSummary)
		}
	})

	t.Run("repeated samples aggregate with explicit even and odd medians", func(t *testing.T) {
		report, err := Convert(input, metadata)
		if err != nil {
			t.Fatalf("converting fixtures: %v", err)
		}

		// The fixture keeps both parities available: odd (3) and even (2).
		counts := make([]int, 0, len(report.Summary))
		for _, s := range report.Summary {
			counts = append(counts, s.SampleCount)
		}
		if !slices.Contains(counts, 2) || !slices.Contains(counts, 3) {
			t.Errorf("fixture must keep one even and one odd sample count, got %v", counts)
		}

		status := findSummary(t, report, "BenchmarkCLIStatus-10")
		if status.SampleCount != 3 || status.MinMeanNsPerOp != 2420000 ||
			status.MedianMeanNsPerOp != 2480000 || status.MaxMeanNsPerOp != 2510000 {
			t.Errorf("odd-count summary mismatch: %+v", status)
		}

		info := findSummary(t, report, "BenchmarkCLIInfo-10")
		// Even count: median is the arithmetic mean of the two central values.
		if info.SampleCount != 2 || info.MinMeanNsPerOp != 3860000 ||
			info.MedianMeanNsPerOp != 3940000 || info.MaxMeanNsPerOp != 4020000 {
			t.Errorf("even-count summary mismatch: %+v", info)
		}

		single := findSummary(t, report, "BenchmarkCLIBranchList-10")
		if single.SampleCount != 1 || single.MinMeanNsPerOp != 10740000 ||
			single.MedianMeanNsPerOp != 10740000 || single.MaxMeanNsPerOp != 10740000 {
			t.Errorf("single-sample summary mismatch: %+v", single)
		}

		if got := median([]float64{1, 2, 3, 4}); got != 2.5 {
			t.Errorf("median of 4 sorted values = %v, want 2.5 (mean of central pair)", got)
		}
		if got := median([]float64{1, 2, 100}); got != 2 {
			t.Errorf("median of 3 sorted values = %v, want 2 (middle value)", got)
		}
	})

	t.Run("invalid benchmark input is rejected", func(t *testing.T) {
		cases := []struct {
			name    string
			input   string
			wantErr string
		}{
			{"empty input", "", "no benchmark samples found"},
			{"framework lines only", "goos: darwin\ngoarch: arm64\nPASS\nok  \texample 1s\n", "no benchmark samples found"},
			{"name only", "BenchmarkFoo-8\n", "malformed benchmark line"},
			{"missing iterations and metrics", "BenchmarkFoo-8 100\n", "malformed benchmark line"},
			{"non-numeric iterations", "BenchmarkFoo-8 many 12.5 ns/op\n", "is not an integer"},
			{"zero iterations", "BenchmarkFoo-8 0 12.5 ns/op\n", "iterations must be positive"},
			{"negative iterations", "BenchmarkFoo-8 -3 12.5 ns/op\n", "iterations must be positive"},
			{"negative ns per op", "BenchmarkFoo-8 100 -12.5 ns/op\n", "ns/op must be positive"},
			{"zero ns per op", "BenchmarkFoo-8 100 0 ns/op\n", "ns/op must be positive"},
			{"no ns per op unit", "BenchmarkFoo-8 100 512 B/op 10 allocs/op\n", "no ns/op metric"},
			{"wrong unit", "BenchmarkFoo-8 100 1.5 s/op\n", "no ns/op metric"},
			{"metric without unit", "BenchmarkFoo-8 100 12.5\n", "metrics must be value/unit pairs"},
			{"non-numeric metric value", "BenchmarkFoo-8 100 fast ns/op\n", "is not numeric"},
			{"duplicate ns per op", "BenchmarkFoo-8 100 12.5 ns/op 13.5 ns/op\n", "duplicate ns/op metric"},
			{"unrecognized line", "some random noise\nBenchmarkFoo-8 100 12.5 ns/op\n", "unrecognized line"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if _, err := Convert([]byte(tc.input), fixtureMetadataJSON(t)); err == nil {
					t.Fatal("expected an error, got nil")
				} else if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.wantErr)
				}
			})
		}
	})

	t.Run("invalid metadata is rejected", func(t *testing.T) {
		base := map[string]string{
			"sourceCommit":       "81d3543",
			"goVersion":          "go1.26.7",
			"gitVersion":         "git version 2.50.0",
			"os":                 "darwin",
			"arch":               "arm64",
			"workload":           "synthetic-example",
			"observedAt":         "2026-10-02T00:00:00Z",
			"measurementCommand": "go test -bench=. -benchmem -count=3 ./benchmarks",
		}
		for _, field := range []string{
			"sourceCommit", "goVersion", "gitVersion", "os", "arch",
			"workload", "observedAt", "measurementCommand",
		} {
			t.Run("missing "+field, func(t *testing.T) {
				dropped := maps.Clone(base)
				delete(dropped, field)
				data, err := json.Marshal(dropped)
				if err != nil {
					t.Fatalf("marshaling metadata: %v", err)
				}
				_, convertErr := Convert([]byte(fixtureInput), data)
				if convertErr == nil {
					t.Fatal("expected an error, got nil")
				}
				want := fmt.Sprintf("metadata field %s is required", field)
				if !strings.Contains(convertErr.Error(), want) {
					t.Errorf("error %q does not contain %q", convertErr.Error(), want)
				}
			})
		}

		t.Run("blank value", func(t *testing.T) {
			blank := maps.Clone(base)
			blank["workload"] = "   "
			data, err := json.Marshal(blank)
			if err != nil {
				t.Fatalf("marshaling metadata: %v", err)
			}
			if _, convertErr := Convert([]byte(fixtureInput), data); convertErr == nil ||
				!strings.Contains(convertErr.Error(), "metadata field workload is required") {
				t.Errorf("blank workload not rejected: %v", convertErr)
			}
		})

		t.Run("malformed json", func(t *testing.T) {
			if _, err := Convert([]byte(fixtureInput), []byte("{not json")); err == nil ||
				!strings.Contains(err.Error(), "malformed metadata JSON") {
				t.Errorf("malformed metadata not rejected: %v", err)
			}
		})

		t.Run("non-object json", func(t *testing.T) {
			if _, err := Convert([]byte(fixtureInput), []byte(`["sourceCommit"]`)); err == nil ||
				!strings.Contains(err.Error(), "malformed metadata JSON") {
				t.Errorf("non-object metadata not rejected: %v", err)
			}
		})

		t.Run("unknown field", func(t *testing.T) {
			extra := maps.Clone(base)
			// A key that matches no field even under encoding/json's
			// case-insensitive fallback (a case variant of a real key would
			// be accepted into that field, not rejected as unknown).
			extra["extraKey"] = "not in the schema"
			data, err := json.Marshal(extra)
			if err != nil {
				t.Fatalf("marshaling metadata: %v", err)
			}
			if _, convertErr := Convert([]byte(fixtureInput), data); convertErr == nil ||
				!strings.Contains(convertErr.Error(), "unknown field") {
				t.Errorf("unknown metadata field not rejected: %v", convertErr)
			}
		})

		t.Run("trailing data", func(t *testing.T) {
			data, err := json.Marshal(base)
			if err != nil {
				t.Fatalf("marshaling metadata: %v", err)
			}
			if _, convertErr := Convert([]byte(fixtureInput), append(data, []byte(" {}")...)); convertErr == nil ||
				!strings.Contains(convertErr.Error(), "unexpected trailing data") {
				t.Errorf("trailing metadata data not rejected: %v", convertErr)
			}
		})
	})

	t.Run("write refuses to overwrite an existing output", func(t *testing.T) {
		report, err := Convert(input, metadata)
		if err != nil {
			t.Fatalf("converting fixtures: %v", err)
		}
		path := filepath.Join(t.TempDir(), "report.json")

		if err := report.WriteFile(path); err != nil {
			t.Fatalf("first write: %v", err)
		}
		want, err := report.Marshal()
		if err != nil {
			t.Fatalf("marshaling report: %v", err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading written report: %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Error("written file does not match Marshal output")
		}

		const sentinel = "sentinel-existing-output"
		if err := os.WriteFile(path, []byte(sentinel), 0o600); err != nil {
			t.Fatalf("planting sentinel: %v", err)
		}
		err = report.WriteFile(path)
		if err == nil {
			t.Fatal("expected refusal error, got nil")
		}
		if !strings.Contains(err.Error(), "refusing to overwrite existing output") {
			t.Errorf("error %q does not name the refusal", err.Error())
		}
		if !errors.Is(err, os.ErrExist) && !strings.Contains(err.Error(), "refusing") {
			t.Errorf("refusal error %q is not recognizable", err.Error())
		}
		got, err = os.ReadFile(path)
		if err != nil {
			t.Fatalf("re-reading sentinel: %v", err)
		}
		if string(got) != sentinel {
			t.Errorf("existing output was modified: %q", string(got))
		}
	})
}
