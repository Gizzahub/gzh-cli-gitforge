package coveragegate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// testModule is the fixture module the profile records and manifests refer to.
const testModule = "example.com/m"

// TestCriticalCoverageGate covers the checker contract end to end: multi
// package attribution, independent per-package floors, half-up rounding at
// one decimal, and every fail-closed rejection the card requires, plus the
// real cmd/coveragegate binary's non-zero path and its leave-no-trace
// behavior on failure.
func TestCriticalCoverageGate(t *testing.T) {
	t.Run("multi package pass", testMultiPackagePass)
	t.Run("drop is not offset by another package", testDropIsNotOffset)
	t.Run("half up boundaries", testHalfUpBoundaries)
	t.Run("missing required package fails", testMissingPackageFails)
	t.Run("empty profile fails", testEmptyProfileFails)
	t.Run("malformed records fail", testMalformedRecordsFail)
	t.Run("duplicate records fail", testDuplicateRecordsFail)
	t.Run("zero percent fails", testZeroPercentFails)
	t.Run("manifest validation fails closed", testManifestValidationFails)
	t.Run("read module path", testReadModulePath)
	t.Run("cli non zero path leaves no temporary files", testCLINonZeroPath)
}

// floorOf builds a manifest entry whose provenance fields mirror the real
// manifest shape.
func floorOf(pkg string, minimumPercent float64) PackageFloor {
	return PackageFloor{
		Package:                   pkg,
		MinimumPercent:            minimumPercent,
		BaselineCoveredStatements: 139,
		BaselineTotalStatements:   235,
		BaselineCommit:            "1a8b9988a50a4946aa37ee989035d04cd01ccf9a",
		MeasurementCommand:        "go test",
	}
}

// manifestOf builds a schema-current manifest around the given floors.
func manifestOf(purpose string, floors ...PackageFloor) Manifest {
	return Manifest{SchemaVersion: SupportedSchemaVersion, Purpose: purpose, Packages: floors}
}

// pkgPath is a fixture package import path under testModule.
func pkgPath(name string) string {
	return testModule + "/pkg/" + name
}

// fileOf is a fixture file path inside a fixture package.
func fileOf(pkg, name string) string {
	return pkg + "/" + name
}

// record builds one coverprofile record line.
func record(path, coords string, numStmt, count int64) string {
	return fmt.Sprintf("%s:%s %d %d", path, coords, numStmt, count)
}

// profile joins records under a set-mode header.
func profile(records ...string) []byte {
	return []byte("mode: set\n" + strings.Join(records, "\n") + "\n")
}

// testMultiPackagePass checks that statements of packages with different
// block sizes are attributed separately, that zero-statement blocks stay
// legal, and that a clear rise over the floor passes.
func testMultiPackagePass(t *testing.T) {
	t.Helper()
	manifest := manifestOf(
		"test guard",
		floorOf(pkgPath("a"), 80.0),
		floorOf(pkgPath("b"), 63.2),
	)
	prof := profile(
		record(fileOf(pkgPath("a"), "a.go"), "1.1,10.2", 40, 1),
		record(fileOf(pkgPath("a"), "a.go"), "11.1,20.2", 10, 0),
		record(fileOf(pkgPath("a"), "zero.go"), "1.1,1.1", 0, 1),
		record(fileOf(pkgPath("b"), "b.go"), "1.1,5.2", 7, 1),
		record(fileOf(pkgPath("b"), "b.go"), "6.1,10.2", 3, 0),
		record(fileOf(pkgPath("b"), "rise.go"), "1.1,30.2", 10, 1),
	)
	results, err := Check(manifest, testModule, prof)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want one per manifest package", len(results))
	}
	if !results[0].Passed || results[0].Covered != 40 || results[0].Total != 50 || results[0].TenthsOfPct != 800 {
		t.Errorf("package a = %+v, want 40/50 statements = 80.0%% pass", results[0])
	}
	if !results[1].Passed || results[1].Covered != 17 || results[1].Total != 20 || results[1].TenthsOfPct != 850 {
		t.Errorf("package b = %+v, want 17/20 statements = 85.0%% pass", results[1])
	}
}

// testDropIsNotOffset proves a high package never masks a drop in another:
// whichever package falls below its floor fails, in either position.
func testDropIsNotOffset(t *testing.T) {
	t.Helper()
	high := record(fileOf(pkgPath("a"), "a.go"), "1.1,10.2", 50, 1)
	low := record(fileOf(pkgPath("b"), "b.go"), "1.1,5.2", 6, 1) + "\n" + record(fileOf(pkgPath("b"), "b.go"), "6.1,10.2", 4, 0)
	manifest := manifestOf("test guard", floorOf(pkgPath("a"), 100.0), floorOf(pkgPath("b"), 63.2))
	results, err := Check(manifest, testModule, profile(high, low))
	if err == nil {
		t.Fatal("Check passed while package b dropped below its floor")
	}
	if !strings.Contains(err.Error(), pkgPath("b")) || strings.Contains(err.Error(), pkgPath("a")) {
		t.Errorf("error = %v, want a failure naming only package b", err)
	}
	if !results[0].Passed {
		t.Errorf("package a = %+v, want it to pass independently", results[0])
	}
	lowA := record(fileOf(pkgPath("a"), "a.go"), "1.1,10.2", 25, 1) + "\n" + record(fileOf(pkgPath("a"), "a.go"), "11.1,20.2", 25, 0)
	perfectB := record(fileOf(pkgPath("b"), "b.go"), "1.1,5.2", 10, 1)
	reversed := manifestOf("test guard", floorOf(pkgPath("a"), 80.0), floorOf(pkgPath("b"), 60.0))
	results2, err := Check(reversed, testModule, profile(lowA, perfectB))
	if err == nil || !strings.Contains(err.Error(), pkgPath("a")) || strings.Contains(err.Error(), pkgPath("b")) {
		t.Errorf("error = %v, want a failure naming only package a", err)
	}
	if len(results2) != 2 || !results2[1].Passed {
		t.Errorf("package b = %+v, want it to pass independently", results2)
	}
}

// testHalfUpBoundaries pins the one-decimal half-up rounding at both sides of
// an exact x.x5 percentage and at the exactly-equal floor. Each fixture has
// tc.covered covered statements, the rest of tc.total uncovered, and a legal
// zero-statement block.
func testHalfUpBoundaries(t *testing.T) {
	t.Helper()
	cases := []struct {
		name        string
		covered     int64
		total       int64
		floor       float64
		wantTenths  int64
		wantFailure bool
	}{
		{"exact x.x5 rounds up onto the floor", 1183, 2000, 59.2, 592, false},
		{"just below x.x5 stays under the floor", 14787, 25000, 59.2, 591, true},
		{"exactly at the floor passes", 591, 1000, 59.1, 591, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Helper()
			manifest := manifestOf("test guard", floorOf(pkgPath("a"), tc.floor))
			path := fileOf(pkgPath("a"), "a.go")
			prof := profile(
				record(path, "1.1,99.2", tc.covered, 1),
				record(path, "100.1,199.2", tc.total-tc.covered, 0),
				record(path, "200.1,200.2", 0, 1),
			)
			results, err := Check(manifest, testModule, prof)
			if tc.wantFailure {
				if err == nil || results[0].Passed {
					t.Fatalf("Check = %+v, %v; want a below-floor failure", results[0], err)
				}
			} else if err != nil {
				t.Fatalf("Check returned error: %v", err)
			}
			if results[0].TenthsOfPct != tc.wantTenths {
				t.Errorf("TenthsOfPct = %d, want %d", results[0].TenthsOfPct, tc.wantTenths)
			}
		})
	}
}

// testMissingPackageFails proves an absent required package is a failure, not
// a silent pass, while present packages are still reported.
func testMissingPackageFails(t *testing.T) {
	t.Helper()
	manifest := manifestOf("test guard", floorOf(pkgPath("a"), 80.0), floorOf(pkgPath("c"), 50.0))
	prof := profile(record(fileOf(pkgPath("a"), "a.go"), "1.1,10.2", 40, 1))
	results, err := Check(manifest, testModule, prof)
	if err == nil || !strings.Contains(err.Error(), "missing from profile") {
		t.Fatalf("err = %v, want a missing-package failure", err)
	}
	if !results[1].Missing || results[1].Passed {
		t.Errorf("package c = %+v, want Missing with no pass", results[1])
	}
	if !results[0].Passed {
		t.Errorf("package a = %+v, want it to pass", results[0])
	}
}

// testEmptyProfileFails proves nil, blank, headerless, and record-free
// profiles are rejected instead of passing an empty gate.
func testEmptyProfileFails(t *testing.T) {
	t.Helper()
	manifest := manifestOf("test guard", floorOf(pkgPath("a"), 59.1))
	good := record(fileOf(pkgPath("a"), "a.go"), "1.1,2.2", 5, 1)
	cases := map[string][]byte{
		"nil profile":     nil,
		"empty profile":   {},
		"blank profile":   []byte("\n \n"),
		"header only":     []byte("mode: set\n"),
		"no mode header":  []byte("\n" + good + "\n"),
		"unknown mode":    []byte("mode: banana\n" + good + "\n"),
		"foreign package": profile(record("other.com/x/a.go", "1.1,2.2", 5, 1)),
		"relative path":   profile(record("a.go", "1.1,2.2", 5, 1)),
	}
	for name, prof := range cases {
		t.Run(name, func(t *testing.T) {
			t.Helper()
			if _, err := Check(manifest, testModule, prof); err == nil {
				t.Fatalf("Check accepted the %q profile", name)
			}
		})
	}
}

// testMalformedRecordsFail feeds structurally broken records to the parser.
func testMalformedRecordsFail(t *testing.T) {
	t.Helper()
	path := fileOf(pkgPath("a"), "a.go")
	manifest := manifestOf("test guard", floorOf(pkgPath("a"), 50.0))
	cases := []struct {
		name  string
		line  string
		field string
	}{
		{"numStmt is not a number", path + ":1.1,2.2 abc 1", "numStmt"},
		{"negative numStmt", path + ":1.1,2.2 -3 1", "numStmt"},
		{"negative count", path + ":1.1,2.2 5 -1", "count"},
		{"missing count field", path + ":1.1,2.2 5", "fields"},
		{"extra field", path + ":1.1,2.2 5 1 9", "fields"},
		{"malformed coordinates", path + ":1.1.3,2.2 5 1", "not a plain decimal number"},
		{"point without column", path + ":12,2.2 5 1", "not 'line.col'"},
		{"zero line number", path + ":0.1,2.2 5 1", "zero line"},
		{"missing colon", path + "1.1,2.2 5 1", "no file path"},
		{"missing comma", path + ":1.1 5 1", "',' separator"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Helper()
			_, err := Check(manifest, testModule, profile(tc.line))
			if err == nil {
				t.Fatalf("Check accepted %q", tc.line)
			}
			if !strings.Contains(err.Error(), tc.field) {
				t.Errorf("error = %v, want it to mention %q", err, tc.field)
			}
		})
	}
}

// testDuplicateRecordsFail proves a repeated path/start/end block is rejected
// whether its counts agree or not, so duplicates can never double-count
// statements, and that the same profile without the repetition passes.
func testDuplicateRecordsFail(t *testing.T) {
	t.Helper()
	first := record(fileOf(pkgPath("a"), "a.go"), "1.1,10.2", 40, 1)
	second := record(fileOf(pkgPath("a"), "a.go"), "11.1,20.2", 10, 0)
	zeroed := record(fileOf(pkgPath("a"), "a.go"), "1.1,10.2", 40, 0)
	manifest := manifestOf("test guard", floorOf(pkgPath("a"), 80.0))
	cases := []struct {
		name  string
		lines []string
	}{
		{"identical counts", []string{first, second, first}},
		{"differing counts", []string{first, second, first, zeroed}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Helper()
			_, err := Check(manifest, testModule, profile(tc.lines...))
			if err == nil || !strings.Contains(err.Error(), "duplicate coverage record") {
				t.Fatalf("err = %v, want a duplicate-record rejection", err)
			}
		})
	}
	if _, err := Check(manifest, testModule, profile(first, second)); err != nil {
		t.Fatalf("the same profile without the repeat returned error: %v", err)
	}
}

// testZeroPercentFails proves a profile whose blocks all went unexecuted is
// a hard failure.
func testZeroPercentFails(t *testing.T) {
	t.Helper()
	manifest := manifestOf("test guard", floorOf(pkgPath("a"), 59.1))
	prof := profile(
		record(fileOf(pkgPath("a"), "a.go"), "1.1,10.2", 40, 0),
		record(fileOf(pkgPath("a"), "a.go"), "11.1,20.2", 10, 0),
	)
	results, err := Check(manifest, testModule, prof)
	if err == nil || !strings.Contains(err.Error(), "coverage is 0%") {
		t.Fatalf("err = %v, want a 0%% rejection", err)
	}
	if results[0].Covered != 0 || results[0].Passed {
		t.Errorf("result = %+v, want an uncovered fail", results[0])
	}
}

// testManifestValidationFails proves corrupt manifests fail closed at both
// the Check and LoadManifest entry points.
func testManifestValidationFails(t *testing.T) {
	t.Helper()
	valid := floorOf(pkgPath("a"), 59.1)
	wrongVersion := manifestOf("guard", valid)
	wrongVersion.SchemaVersion = 99
	emptyName := floorOf(pkgPath("a"), 59.1)
	emptyName.Package = ""
	inconsistent := floorOf(pkgPath("a"), 59.1)
	inconsistent.BaselineCoveredStatements = 300
	type manifestCase struct {
		name     string
		manifest Manifest
		wantErr  string
	}
	cases := []manifestCase{
		{"wrong schema version", wrongVersion, "schemaVersion"},
		{"empty purpose", manifestOf("  ", valid), "purpose"},
		{"no packages", manifestOf("guard"), "no packages"},
		{"empty package name", manifestOf("guard", emptyName), "package name is empty"},
		{"zero floor", manifestOf("guard", floorOf(pkgPath("a"), 0)), "not in (0, 100]"},
		{"floor above 100", manifestOf("guard", floorOf(pkgPath("a"), 100.1)), "not in (0, 100]"},
		{"duplicate package", manifestOf("guard", valid, valid), "listed twice"},
		{"baseline covered above total", manifestOf("guard", inconsistent), "inconsistent"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Helper()
			_, err := Check(tc.manifest, testModule, profile(record(fileOf(pkgPath("a"), "a.go"), "1.1,2.2", 5, 1)))
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("err = %v, want it to mention %q", err, tc.wantErr)
			}
		})
	}
	t.Run("load manifest from disk", func(t *testing.T) {
		t.Helper()
		dir := t.TempDir()
		data, marshalErr := json.Marshal(manifestOf("test guard", valid))
		if marshalErr != nil {
			t.Fatalf("marshal fixture: %v", marshalErr)
		}
		writeFile(t, dir, "floors.json", string(data))
		loaded, loadErr := LoadManifest(filepath.Join(dir, "floors.json"))
		if loadErr != nil {
			t.Fatalf("LoadManifest: %v", loadErr)
		}
		if len(loaded.Packages) != 1 || loaded.Packages[0].MinimumPercent != 59.1 {
			t.Errorf("loaded = %+v, want the fixture floor", loaded)
		}
		writeFile(t, dir, "future.json", strings.Replace(string(data), `"schemaVersion":1`, `"schemaVersion":99`, 1))
		if _, err := LoadManifest(filepath.Join(dir, "future.json")); err == nil || !strings.Contains(err.Error(), "schemaVersion") {
			t.Errorf("err = %v, want a schemaVersion rejection", err)
		}
	})
}

// testReadModulePath covers go.mod resolution and its failure modes.
func testReadModulePath(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module "+testModule+"\n\ngo 1.26\n\nrequire (\n\tx.y/z v1.0.0\n)\n")
	got, err := ReadModulePath(filepath.Join(dir, "go.mod"))
	if err != nil || got != testModule {
		t.Errorf("ReadModulePath = %q, %v; want %q", got, err, testModule)
	}
	writeFile(t, dir, "bare.txt", "go 1.26\n")
	if _, err := ReadModulePath(filepath.Join(dir, "bare.txt")); err == nil {
		t.Error("ReadModulePath accepted a go.mod without a module directive")
	}
	if _, err := ReadModulePath(filepath.Join(dir, "absent.mod")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("err = %v, want a not-exist error", err)
	}
}

// testCLINonZeroPath builds the real cmd/coveragegate binary and proves it
// exits non-zero on a drop with the violation on stderr, passes exactly at a
// met floor, and leaves the fixture directory byte-for-byte as it found it
// after the failure.
func testCLINonZeroPath(t *testing.T) {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "coveragegate")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", bin, "../../cmd/coveragegate") // #nosec G204 -- builds the in-repo CLI under test.
	build.Env = append(os.Environ(), "GOWORK=off")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build cmd/coveragegate: %v\n%s", err, out)
	}
	fixture := t.TempDir()
	writeFile(t, fixture, "go.mod", "module "+testModule+"\n")
	writeFile(t, fixture, "floors.json", cliFloorsJSON("90.0"))
	path := fileOf(pkgPath("a"), "a.go")
	prof := "mode: set\n" + record(path, "1.1,10.2", 40, 1) + "\n" + record(path, "11.1,20.2", 10, 0) + "\n"
	writeFile(t, fixture, "profile.out", prof)

	stdout, stderr, runErr := runCLI(t, bin, fixture, "floors.json")
	exitErr := &exec.ExitError{}
	if !errors.As(runErr, &exitErr) || exitErr.ExitCode() == 0 {
		t.Fatalf("coveragegate on a dropping profile: err = %v, want non-zero exit", runErr)
	}
	if !strings.Contains(stderr, "below floor 90.0%") {
		t.Errorf("stderr = %q, want the below-floor report", stderr)
	}
	if !strings.Contains(stdout, "FAIL") {
		t.Errorf("stdout = %q, want a FAIL verdict", stdout)
	}
	entries, readErr := os.ReadDir(fixture)
	if readErr != nil {
		t.Fatalf("read fixture dir: %v", readErr)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if want := "floors.json,go.mod,profile.out"; strings.Join(names, ",") != want {
		t.Errorf("fixture dir after failure = %v, want exactly %s (no temporary files left behind)", names, want)
	}

	writeFile(t, fixture, "floors-ok.json", cliFloorsJSON("80.0"))
	stdout, _, runErr = runCLI(t, bin, fixture, "floors-ok.json")
	if runErr != nil {
		t.Errorf("coveragegate exactly at the floor: err = %v, want exit 0", runErr)
	}
	if !strings.Contains(stdout, "floors satisfied") {
		t.Errorf("stdout = %q, want the satisfied banner", stdout)
	}
}

// cliFloorsJSON renders a one-package manifest fixture with the given floor.
func cliFloorsJSON(minimumPercent string) string {
	return fmt.Sprintf(`{
  "schemaVersion": 1,
  "purpose": "test fixture for the CLI contract",
  "packages": [
    {
      "package": "%s/pkg/a",
      "minimumPercent": %s,
      "baselineCoveredStatements": 139,
      "baselineTotalStatements": 235,
      "baselineCommit": "1a8b9988a50a4946aa37ee989035d04cd01ccf9a",
      "measurementCommand": "go test"
    }
  ]
}`, testModule, minimumPercent)
}

// runCLI executes the built binary inside the fixture directory and returns
// its captured output streams and run error.
func runCLI(t *testing.T, bin, dir, manifest string) (stdout, stderr string, err error) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), bin, "-manifest", manifest, "-profile", "profile.out", "-gomod", "go.mod") // #nosec G204 -- fixed argv pointing at the binary built above.
	cmd.Dir = dir
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

// writeFile drops a fixture file into dir.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
