// Package coveragegate enforces per-package minimum statement-coverage floors
// against a standard Go coverprofile and fails closed on any drop.
//
// Packages are never merged: every required package is compared on its own
// against its own floor, so a high result in one package can never offset a
// drop in another. Statements are attributed to packages by their module
// prefix, which the caller resolves from go.mod.
//
// The floors recorded in the manifest are a temporary no-regression guard
// pinned to a measured baseline. They are not the product coverage goal; the
// 85% pkg goal in PRODUCT.md stays unchanged and unmet by this gate.
package coveragegate

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// SupportedSchemaVersion is the only manifest schema this checker accepts.
const SupportedSchemaVersion = 1

// maxStatements bounds every parsed and accumulated statement counter so the
// half-up arithmetic in halfUpTenths (2000*covered+total) can never overflow
// int64, even for a hostile profile.
const maxStatements int64 = 1 << 50

// modulePathPattern matches the module directive of a go.mod file.
var modulePathPattern = regexp.MustCompile(`^module\s+(\S+)`)

// Manifest is the on-disk record of the critical coverage floors.
type Manifest struct {
	// The camelCase JSON names are fixed by the TASK-267 card's manifest
	// contract, not a style choice.
	SchemaVersion int            `json:"schemaVersion"` //nolint:tagliatelle // card-mandated manifest field name
	Purpose       string         `json:"purpose"`
	Packages      []PackageFloor `json:"packages"`
}

// PackageFloor pins one package to a minimum coverage percentage and records
// where that floor was measured.
type PackageFloor struct {
	Package                   string  `json:"package"`
	MinimumPercent            float64 `json:"minimumPercent"`            //nolint:tagliatelle // card-mandated manifest field name
	BaselineCoveredStatements int64   `json:"baselineCoveredStatements"` //nolint:tagliatelle // card-mandated manifest field name
	BaselineTotalStatements   int64   `json:"baselineTotalStatements"`   //nolint:tagliatelle // card-mandated manifest field name
	BaselineCommit            string  `json:"baselineCommit"`            //nolint:tagliatelle // card-mandated manifest field name
	MeasurementCommand        string  `json:"measurementCommand"`        //nolint:tagliatelle // card-mandated manifest field name
}

// Result reports the measured coverage of one required package.
type Result struct {
	Package     string
	Covered     int64
	Total       int64
	TenthsOfPct int64 // measured coverage in tenths of a percent, rounded half-up
	FloorTenths int64 // floor in tenths of a percent
	Passed      bool
	Missing     bool // the package had no record in the profile
}

// LoadManifest reads and validates a floors manifest from disk.
func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- the manifest path is an explicit operator-supplied flag.
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest %s: %w", path, err)
	}
	if err := validateManifest(manifest); err != nil {
		return Manifest{}, fmt.Errorf("invalid manifest %s: %w", path, err)
	}
	return manifest, nil
}

// ReadModulePath extracts the module path from a go.mod file.
func ReadModulePath(goModPath string) (string, error) {
	data, err := os.ReadFile(goModPath) // #nosec G304 -- the go.mod path is an explicit operator-supplied flag.
	if err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if match := modulePathPattern.FindStringSubmatch(line); match != nil {
			return match[1], nil
		}
	}
	return "", fmt.Errorf("module path not found in %s", goModPath)
}

// Check parses profile (a standard Go coverprofile), attributes its statements
// to packages under modulePath, and compares every manifest package against
// its own floor. It returns one result per manifest package and a joined error
// listing every violation, so a single run reports all drops at once. Exactly
// meeting a floor passes; anything below it fails.
func Check(manifest Manifest, modulePath string, profile []byte) ([]Result, error) {
	if err := validateManifest(manifest); err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}
	if modulePath == "" {
		return nil, errors.New("module path is empty")
	}
	summaries, err := parseProfile(modulePath, profile)
	if err != nil {
		return nil, err
	}
	results := make([]Result, 0, len(manifest.Packages))
	var failures []error
	for _, floor := range manifest.Packages {
		result, failure := compareFloor(floor, summaries[floor.Package])
		results = append(results, result)
		if failure != nil {
			failures = append(failures, failure)
		}
	}
	return results, errors.Join(failures...)
}

// compareFloor measures one package summary against its floor. A missing
// package, a package with no statements, 0% coverage, and anything below the
// floor all fail; exactly meeting the floor passes.
func compareFloor(floor PackageFloor, summary *packageSummary) (Result, error) {
	result := Result{Package: floor.Package, FloorTenths: percentToTenths(floor.MinimumPercent)}
	if summary == nil {
		result.Missing = true
		return result, fmt.Errorf("required package missing from profile: %s", floor.Package)
	}
	if summary.total == 0 {
		return result, fmt.Errorf("package %s has no instrumented statements", floor.Package)
	}
	result.Covered = summary.covered
	result.Total = summary.total
	result.TenthsOfPct = halfUpTenths(summary.covered, summary.total)
	if summary.covered == 0 {
		return result, fmt.Errorf("package %s coverage is 0%%", floor.Package)
	}
	result.Passed = result.TenthsOfPct >= result.FloorTenths
	if !result.Passed {
		return result, fmt.Errorf("package %s coverage %s%% is below floor %s%%",
			floor.Package, FormatTenths(result.TenthsOfPct), FormatTenths(result.FloorTenths))
	}
	return result, nil
}

// packageSummary accumulates the statement counts of one package: covered
// counts statements in blocks whose execution count is positive, total counts
// statements in every block.
type packageSummary struct {
	covered, total int64
}

// blockKey identifies a coverprofile record by path and start/end coordinates.
type blockKey struct {
	path      string
	startLine int64
	startCol  int64
	endLine   int64
	endCol    int64
}

// parseProfile parses a standard coverprofile ("mode:" header, then
// "file:startLine.startCol,endLine.endCol numStmt count" records) into
// per-package statement summaries.
//
// Records whose path and start/end coordinates repeat are rejected rather
// than merged, whatever their counts: Go's own tooling would OR the counts,
// but here a repeated block means a corrupt or hand-assembled profile, and
// rejecting it guarantees duplicates can never double-count statements.
// Zero-statement blocks ("... 0 1"), which the Go toolchain emits for empty
// regions, are valid and contribute nothing to the totals.
func parseProfile(modulePath string, profile []byte) (map[string]*packageSummary, error) {
	text := strings.TrimSuffix(string(profile), "\n")
	if strings.TrimSpace(text) == "" {
		return nil, errors.New("coverage profile is empty")
	}
	lines := strings.Split(text, "\n")
	if !strings.HasPrefix(lines[0], "mode:") {
		return nil, fmt.Errorf("coverage profile must start with a 'mode:' header, got %q", lines[0])
	}
	mode := strings.TrimSpace(strings.TrimPrefix(lines[0], "mode:"))
	if mode != "set" && mode != "count" && mode != "atomic" {
		return nil, fmt.Errorf("unsupported coverage mode %q", mode)
	}
	seen := make(map[blockKey]struct{})
	summaries := make(map[string]*packageSummary)
	for offset, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		key, numStmt, count, err := parseRecord(line)
		if err != nil {
			return nil, fmt.Errorf("profile line %d: %w", offset+2, err)
		}
		if _, duplicate := seen[key]; duplicate {
			return nil, fmt.Errorf("profile line %d: duplicate coverage record for %s:%d.%d,%d.%d; a block must appear exactly once so duplicates can never double-count statements",
				offset+2, key.path, key.startLine, key.startCol, key.endLine, key.endCol)
		}
		seen[key] = struct{}{}
		pkg, err := packageOf(key.path, modulePath)
		if err != nil {
			return nil, fmt.Errorf("profile line %d: %w", offset+2, err)
		}
		if err := accumulate(summaries, pkg, numStmt, count); err != nil {
			return nil, fmt.Errorf("profile line %d: %w", offset+2, err)
		}
	}
	if len(summaries) == 0 {
		return nil, errors.New("coverage profile has no records")
	}
	return summaries, nil
}

// accumulate folds one record into its package's summary, creating it on
// first sight. The count only decides whether the block's statements count
// as covered; the statement total always grows by numStmt.
func accumulate(summaries map[string]*packageSummary, pkg string, numStmt, count int64) error {
	summary := summaries[pkg]
	if summary == nil {
		summary = &packageSummary{}
		summaries[pkg] = summary
	}
	if numStmt > maxStatements-summary.total {
		return fmt.Errorf("statement total exceeds the %d statement limit", maxStatements)
	}
	summary.total += numStmt
	if count > 0 {
		summary.covered += numStmt
	}
	return nil
}

// parseRecord parses one "file:startLine.startCol,endLine.endCol numStmt
// count" record.
func parseRecord(line string) (key blockKey, numStmt, count int64, err error) {
	fields := strings.Fields(line)
	if len(fields) != 3 {
		return blockKey{}, 0, 0, fmt.Errorf("expected 'file:startLine.startCol,endLine.endCol numStmt count', got %d fields", len(fields))
	}
	colon := strings.LastIndex(fields[0], ":")
	if colon <= 0 {
		return blockKey{}, 0, 0, fmt.Errorf("location %q has no file path", fields[0])
	}
	coords := fields[0][colon+1:]
	comma := strings.Index(coords, ",")
	if comma <= 0 {
		return blockKey{}, 0, 0, fmt.Errorf("location %q is missing a ',' separator", fields[0])
	}
	startLine, startCol, err := parsePoint(coords[:comma])
	if err != nil {
		return blockKey{}, 0, 0, fmt.Errorf("start location: %w", err)
	}
	endLine, endCol, err := parsePoint(coords[comma+1:])
	if err != nil {
		return blockKey{}, 0, 0, fmt.Errorf("end location: %w", err)
	}
	numStmt, err = parseUint(fields[1])
	if err != nil {
		return blockKey{}, 0, 0, fmt.Errorf("numStmt: %w", err)
	}
	count, err = parseUint(fields[2])
	if err != nil {
		return blockKey{}, 0, 0, fmt.Errorf("count: %w", err)
	}
	return blockKey{path: fields[0][:colon], startLine: startLine, startCol: startCol, endLine: endLine, endCol: endCol}, numStmt, count, nil
}

// parsePoint parses one "line.col" half of a location.
func parsePoint(point string) (line, col int64, err error) {
	dot := strings.Index(point, ".")
	if dot <= 0 {
		return 0, 0, fmt.Errorf("%q is not 'line.col'", point)
	}
	line, err = parseUint(point[:dot])
	if err != nil {
		return 0, 0, fmt.Errorf("line: %w", err)
	}
	col, err = parseUint(point[dot+1:])
	if err != nil {
		return 0, 0, fmt.Errorf("column: %w", err)
	}
	if line == 0 || col == 0 {
		return 0, 0, fmt.Errorf("%q has a zero line or column", point)
	}
	return line, col, nil
}

// parseUint parses a non-negative decimal integer. Go's coverprofile writer
// never emits signs, spaces, or underscores, so anything but plain digits is
// malformed input.
func parseUint(text string) (int64, error) {
	if text == "" {
		return 0, errors.New("empty number")
	}
	for i := 0; i < len(text); i++ {
		if text[i] < '0' || text[i] > '9' {
			return 0, fmt.Errorf("%q is not a plain decimal number", text)
		}
	}
	value, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%q is out of range", text)
	}
	if value > maxStatements {
		return 0, fmt.Errorf("%q is unreasonably large", text)
	}
	return value, nil
}

// packageOf maps a coverprofile file path to its Go package import path and
// requires the package to live inside the module.
func packageOf(path, modulePath string) (string, error) {
	slash := strings.LastIndex(path, "/")
	if slash < 0 {
		return "", fmt.Errorf("file path %q is not module-qualified", path)
	}
	pkg := path[:slash]
	if pkg != modulePath && !strings.HasPrefix(pkg, modulePath+"/") {
		return "", fmt.Errorf("file path %q is outside module %s", path, modulePath)
	}
	return pkg, nil
}

// halfUpTenths computes 1000*covered/total in exact integer arithmetic and
// rounds exact halves up: 591.5 tenths (59.15%) becomes 592 (59.2%). Floating
// point is avoided on purpose; x.x5 percentages are not exactly representable
// in binary and could round the wrong way at a floor boundary.
func halfUpTenths(covered, total int64) int64 {
	return (2000*covered + total) / (2 * total)
}

// percentToTenths converts a one-decimal floor percentage such as 59.1 into
// integer tenths of a percent (591).
func percentToTenths(percent float64) int64 {
	return int64(math.Round(percent * 10))
}

// FormatTenths renders tenths of a percent as a fixed one-decimal string.
func FormatTenths(tenths int64) string {
	return strconv.FormatInt(tenths/10, 10) + "." + strconv.FormatInt(tenths%10, 10)
}

// validateManifest rejects anything this checker version cannot enforce, so a
// corrupt or future manifest fails closed instead of checking nothing. The
// baseline and measurement fields are provenance records for humans; the
// fields the gate actually enforces are the package name and its floor.
func validateManifest(manifest Manifest) error {
	if manifest.SchemaVersion != SupportedSchemaVersion {
		return fmt.Errorf("unsupported schemaVersion %d (want %d)", manifest.SchemaVersion, SupportedSchemaVersion)
	}
	if strings.TrimSpace(manifest.Purpose) == "" {
		return errors.New("purpose must describe why the floors exist")
	}
	if len(manifest.Packages) == 0 {
		return errors.New("manifest lists no packages")
	}
	seen := make(map[string]struct{}, len(manifest.Packages))
	for _, floor := range manifest.Packages {
		if err := validateFloor(floor); err != nil {
			return err
		}
		if _, duplicate := seen[floor.Package]; duplicate {
			return fmt.Errorf("package %s is listed twice", floor.Package)
		}
		seen[floor.Package] = struct{}{}
	}
	return nil
}

// validateFloor checks one manifest entry's enforced fields and ranges.
func validateFloor(floor PackageFloor) error {
	if floor.Package == "" {
		return errors.New("package name is empty")
	}
	if floor.MinimumPercent <= 0 || floor.MinimumPercent > 100 {
		return fmt.Errorf("package %s: minimumPercent %g is not in (0, 100]", floor.Package, floor.MinimumPercent)
	}
	if floor.BaselineTotalStatements <= 0 || floor.BaselineCoveredStatements < 0 || floor.BaselineCoveredStatements > floor.BaselineTotalStatements {
		return fmt.Errorf("package %s: baseline statement counts are inconsistent", floor.Package)
	}
	return nil
}
