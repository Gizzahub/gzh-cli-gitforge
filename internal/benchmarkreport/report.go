// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

// Package benchmarkreport converts captured `go test -bench` text output and
// a run-metadata sidecar into a deterministic offline JSON report
// (schemaVersion 1). The converter is purely local: it reads two files,
// performs no network calls, and emits no telemetry.
//
// Contract notes:
//
//   - A benchmark line is a line whose first field begins with "Benchmark".
//     Framework lines `go test` prints around results (goos/goarch/pkg/cpu/
//     coverage headers, PASS/FAIL/ok/?, and -v banners) are ignored; any other
//     line is rejected so corrupted input fails loudly instead of silently
//     dropping samples.
//   - The name is kept verbatim, including the "-N" GOMAXPROCS suffix Go
//     appends (for example BenchmarkCLIStatus-10). Dropping it would merge
//     samples taken at different GOMAXPROCS values into one summary.
//   - ns/op is the mean nanoseconds per operation across that sample's b.N
//     iterations. Summary statistics therefore describe the distribution of
//     sample means, never a per-operation percentile; no p95 is emitted.
//   - samples are sorted by name ascending (byte-wise); samples sharing a
//     name keep input order (stable sort). summary entries are sorted by name.
//   - For a name with an even number of samples the median is the arithmetic
//     mean of the two central values; for an odd count it is the middle value.
package benchmarkreport

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strconv"
	"strings"
)

// SchemaVersion is the report schema this package emits.
const SchemaVersion = 1

// nsPerOpUnit is the only unit the converter extracts from a benchmark line.
const nsPerOpUnit = "ns/op"

// benchmarkPrefix marks a line as a benchmark result sample.
const benchmarkPrefix = "Benchmark"

// ignoredFirstFields lists the first field of every non-benchmark line
// `go test -bench` can emit around its results. Header lines (goos, goarch,
// pkg, cpu, coverage), verdict lines (PASS, FAIL, ok, ?), and -v banners
// (=== RUN, --- PASS) carry no samples, so they are ignored.
var ignoredFirstFields = map[string]struct{}{
	"goos:":     {},
	"goarch:":   {},
	"pkg:":      {},
	"cpu:":      {},
	"coverage:": {},
	"PASS":      {},
	"FAIL":      {},
	"ok":        {},
	"?":         {},
	"===":       {},
	"---":       {},
}

// Metadata is the run sidecar embedded verbatim in the report. Every field
// except Note is required and must be non-empty; unknown keys are rejected so
// a typo fails conversion instead of silently dropping data. observedAt should
// be an RFC 3339 timestamp.
//
//nolint:tagliatelle // camelCase JSON keys are the TASK-269 report contract; the golden fixture pins them.
type Metadata struct {
	SourceCommit       string `json:"sourceCommit"`
	GoVersion          string `json:"goVersion"`
	GitVersion         string `json:"gitVersion"`
	OS                 string `json:"os"`
	Arch               string `json:"arch"`
	Workload           string `json:"workload"`
	ObservedAt         string `json:"observedAt"`
	MeasurementCommand string `json:"measurementCommand"`
	Note               string `json:"note,omitempty"`
}

// Sample is one benchmark result line: name, its b.N iterations, and the
// reported ns/op mean for that run.
//
//nolint:tagliatelle // camelCase JSON keys are the TASK-269 report contract; the golden fixture pins them.
type Sample struct {
	Name       string  `json:"name"`
	Iterations int64   `json:"iterations"`
	NsPerOp    float64 `json:"nsPerOp"`
}

// NameSummary aggregates every sample that shares a name. The Min/Median/Max
// fields are statistics over the samples' nsPerOp means -- the distribution of
// sample means, not per-operation latencies.
//
//nolint:tagliatelle // camelCase JSON keys are the TASK-269 report contract; the golden fixture pins them.
type NameSummary struct {
	Name              string  `json:"name"`
	SampleCount       int     `json:"sampleCount"`
	MinMeanNsPerOp    float64 `json:"minMeanNsPerOp"`
	MedianMeanNsPerOp float64 `json:"medianMeanNsPerOp"`
	MaxMeanNsPerOp    float64 `json:"maxMeanNsPerOp"`
}

// Report is the schema v1 document the converter emits.
//
//nolint:tagliatelle // camelCase JSON keys are the TASK-269 report contract; the golden fixture pins them.
type Report struct {
	SchemaVersion int           `json:"schemaVersion"`
	Metadata      Metadata      `json:"metadata"`
	Samples       []Sample      `json:"samples"`
	Summary       []NameSummary `json:"summary"`
}

// ParseMetadata decodes and validates the metadata sidecar document. It
// rejects malformed JSON, trailing data, unknown keys, and any missing or
// blank required field.
func ParseMetadata(data []byte) (Metadata, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var m Metadata
	if err := dec.Decode(&m); err != nil {
		return Metadata{}, fmt.Errorf("malformed metadata JSON: %w", err)
	}
	if dec.More() {
		return Metadata{}, errors.New("malformed metadata JSON: unexpected trailing data")
	}
	required := []struct {
		field string
		value string
	}{
		{"sourceCommit", m.SourceCommit},
		{"goVersion", m.GoVersion},
		{"gitVersion", m.GitVersion},
		{"os", m.OS},
		{"arch", m.Arch},
		{"workload", m.Workload},
		{"observedAt", m.ObservedAt},
		{"measurementCommand", m.MeasurementCommand},
	}
	for _, r := range required {
		if strings.TrimSpace(r.value) == "" {
			return Metadata{}, fmt.Errorf("metadata field %s is required and must not be empty", r.field)
		}
	}
	return m, nil
}

// ParseBenchmarks parses standard `go test -bench` output and returns one
// Sample per benchmark result line, in input order. It rejects malformed
// benchmark lines, non-positive iterations, non-positive ns/op values, lines
// without an ns/op metric, unrecognized lines, and input with no samples.
func ParseBenchmarks(data []byte) ([]Sample, error) {
	var samples []Sample
	for i, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSuffix(raw, "\r")
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if _, ok := ignoredFirstFields[fields[0]]; ok {
			continue
		}
		if !strings.HasPrefix(fields[0], benchmarkPrefix) {
			return nil, fmt.Errorf("unrecognized line %d (not a benchmark result or a known framework line): %q", i+1, line)
		}
		sample, err := parseBenchmarkLine(line, fields)
		if err != nil {
			return nil, err
		}
		samples = append(samples, sample)
	}
	if len(samples) == 0 {
		return nil, errors.New("no benchmark samples found in input")
	}
	return samples, nil
}

// parseBenchmarkLine parses one benchmark result line of the form
// `BenchmarkName-N  <iterations>  <value> ns/op [other metrics]`. Metric
// tokens after the iterations column must come in value/unit pairs; extra
// metrics such as B/op and allocs/op are accepted but not extracted.
func parseBenchmarkLine(line string, fields []string) (Sample, error) {
	if len(fields) < 3 {
		return Sample{}, fmt.Errorf("malformed benchmark line %q: want name, iterations, and metrics", line)
	}
	iterations, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return Sample{}, fmt.Errorf("malformed benchmark line %q: iterations %q is not an integer", line, fields[1])
	}
	if iterations <= 0 {
		return Sample{}, fmt.Errorf("invalid benchmark line %q: iterations must be positive, got %d", line, iterations)
	}
	if (len(fields)-2)%2 != 0 {
		return Sample{}, fmt.Errorf("malformed benchmark line %q: metrics must be value/unit pairs", line)
	}
	sample := Sample{Name: fields[0], Iterations: iterations}
	for i := 2; i < len(fields); i += 2 {
		value, err := strconv.ParseFloat(fields[i], 64)
		if err != nil {
			return Sample{}, fmt.Errorf("malformed benchmark line %q: metric value %q is not numeric", line, fields[i])
		}
		if fields[i+1] != nsPerOpUnit {
			continue
		}
		if sample.NsPerOp != 0 {
			return Sample{}, fmt.Errorf("malformed benchmark line %q: duplicate %s metric", line, nsPerOpUnit)
		}
		if value <= 0 {
			return Sample{}, fmt.Errorf("invalid benchmark line %q: ns/op must be positive, got %s", line, fields[i])
		}
		sample.NsPerOp = value
	}
	if sample.NsPerOp == 0 {
		return Sample{}, fmt.Errorf("malformed benchmark line %q: no %s metric", line, nsPerOpUnit)
	}
	return sample, nil
}

// Convert parses benchmark text and metadata and assembles the schema v1
// report.
func Convert(input, metadata []byte) (*Report, error) {
	meta, err := ParseMetadata(metadata)
	if err != nil {
		return nil, err
	}
	samples, err := ParseBenchmarks(input)
	if err != nil {
		return nil, err
	}
	return BuildReport(meta, samples)
}

// BuildReport assembles the report from parsed parts. Samples are sorted by
// name ascending (stable, so equal names keep input order) and summarized per
// name in that same order.
func BuildReport(m Metadata, samples []Sample) (*Report, error) {
	if len(samples) == 0 {
		return nil, errors.New("no benchmark samples found in input")
	}
	sorted := slices.Clone(samples)
	slices.SortStableFunc(sorted, func(a, b Sample) int {
		return strings.Compare(a.Name, b.Name)
	})

	summary := make([]NameSummary, 0)
	for start := 0; start < len(sorted); {
		end := start + 1
		for end < len(sorted) && sorted[end].Name == sorted[start].Name {
			end++
		}
		values := make([]float64, 0, end-start)
		for _, s := range sorted[start:end] {
			values = append(values, s.NsPerOp)
		}
		slices.Sort(values)
		summary = append(summary, NameSummary{
			Name:              sorted[start].Name,
			SampleCount:       len(values),
			MinMeanNsPerOp:    values[0],
			MedianMeanNsPerOp: median(values),
			MaxMeanNsPerOp:    values[len(values)-1],
		})
		start = end
	}
	return &Report{
		SchemaVersion: SchemaVersion,
		Metadata:      m,
		Samples:       sorted,
		Summary:       summary,
	}, nil
}

// median returns the median of a non-empty ascending-sorted slice. An even
// count yields the arithmetic mean of the two central values; an odd count
// yields the middle value.
func median(sorted []float64) float64 {
	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

// Marshal renders the report as deterministic two-space indented JSON with a
// trailing newline.
func (r *Report) Marshal() ([]byte, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling report: %w", err)
	}
	return append(data, '\n'), nil
}

// WriteFile writes the report to a new file at path. It refuses to overwrite
// an existing file (the open uses O_CREATE|O_EXCL, so an existing file is
// never touched) and removes the partial file if writing or closing fails.
func (r *Report) WriteFile(path string) error {
	data, err := r.Marshal()
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644) // #nosec G302,G304 -- creates a new report file at an explicit operator-supplied path, mirroring the 0644 receipt files in internal/runtask/store.go
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("refusing to overwrite existing output %s", path)
		}
		return fmt.Errorf("creating output %s: %w", path, err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return fmt.Errorf("writing output %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("closing output %s: %w", path, err)
	}
	return nil
}
