// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

// Command benchmark-report converts one captured `go test -bench` text file
// plus a metadata sidecar into a schema v1 JSON report (see
// internal/benchmarkreport for the contract). It is an offline tool: no
// network calls, no telemetry, no performance thresholds.
//
// Usage:
//
//	benchmark-report --input bench.txt --metadata meta.json --output report.json
//
// All three flags are required. The output file must not already exist; the
// converter refuses to overwrite it and leaves any existing file untouched.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/gizzahub/gzh-cli-gitforge/internal/benchmarkreport"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("benchmark-report", flag.ContinueOnError)
	input := fs.String("input", "", "path to raw go test -bench output text (required)")
	metadata := fs.String("metadata", "", "path to run metadata JSON (required)")
	output := fs.String("output", "", "path for the generated report; must not already exist (required)")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}
	if *input == "" || *metadata == "" || *output == "" {
		return fmt.Errorf("all of --input, --metadata and --output are required")
	}

	inputData, err := os.ReadFile(*input) // #nosec G304 -- explicit operator-supplied input path
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}
	metadataData, err := os.ReadFile(*metadata) // #nosec G304 -- explicit operator-supplied metadata path
	if err != nil {
		return fmt.Errorf("reading metadata: %w", err)
	}

	report, err := benchmarkreport.Convert(inputData, metadataData)
	if err != nil {
		return fmt.Errorf("converting benchmark report: %w", err)
	}
	if err := report.WriteFile(*output); err != nil {
		return err
	}
	fmt.Printf("wrote benchmark report %s (%d samples, %d names, schema v%d)\n",
		*output, len(report.Samples), len(report.Summary), report.SchemaVersion)
	return nil
}
