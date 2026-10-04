// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package benchmarks

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/benchmarkreport"
)

// Exercise the actual Make recipe with a controlled measurement process. The
// metadata encoder and report converter remain real, while stderr and converter
// failure can be injected without emptying the user's module cache.
func TestBenchmarkRecordFidelity(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("POSIX Make recipe")
	}
	root := repositoryRoot(t)
	realGo, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"success", "converter failure", "publication failure"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			shimDir := filepath.Join(dir, "shims")
			if err := os.Mkdir(shimDir, 0o700); err != nil {
				t.Fatal(err)
			}
			scripts := map[string]string{
				"git": `#!/bin/sh
case "$1" in
status) exit 0;;
rev-parse) echo aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa;;
--version) printf '%s\n' 'git version "quoted"';;
*) exit 99;;
esac
`,
				"go": `#!/bin/sh
set -eu
case "$1" in
test)
 test "$GOWORK" = off
 cat "$FIDELITY_ROOT/benchmarks/testdata/report.input.txt"
 echo 'go: downloading example.invalid/module v1.0.0' >&2
 exit 0;;
run)
 if [ "$2" = ./cmd/benchmark-report ] && [ "$FIDELITY_FAIL" = 1 ]; then
  echo 'injected conversion failure' >&2
  exit 13
 fi
 cd "$FIDELITY_ROOT"
 exec "$FIDELITY_REAL_GO" "$@";;
*) exec "$FIDELITY_REAL_GO" "$@";;
esac
`,
			}
			if name == "publication failure" {
				scripts["cp"] = "#!/bin/sh\nexit 17\n"
			}
			for name, script := range scripts {
				if err := os.WriteFile(filepath.Join(shimDir, name), []byte(script), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			outDir := filepath.Join(dir, "results")
			fail := "0"
			if name == "converter failure" {
				fail = "1"
			}
			cmd := exec.CommandContext(t.Context(), "make", "-f", filepath.Join(root, ".make", "test.mk"), "benchmark-record", "OUTPUT_DIR="+outDir)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "PATH="+shimDir+string(os.PathListSeparator)+os.Getenv("PATH"), "FIDELITY_ROOT="+root, "FIDELITY_REAL_GO="+realGo, "FIDELITY_FAIL="+fail)
			output, err := cmd.CombinedOutput()
			if name == "publication failure" {
				if err == nil {
					t.Fatal("publication failure swallowed")
				}
				if _, err := os.Stat(filepath.Join(outDir, "recording.failed")); err != nil {
					t.Fatalf("failure marker missing: %v", err)
				}
				return
			}
			if name == "converter failure" {
				if err == nil || !strings.Contains(string(output), "injected conversion failure") {
					t.Fatalf("expected converter failure: %v\n%s", err, output)
				}
				if _, err := os.Stat(outDir); !os.IsNotExist(err) {
					t.Fatalf("partial output published: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("record: %v\n%s", err, output)
			}
			raw, err := os.ReadFile(filepath.Join(outDir, "bench.txt")) // #nosec G304 -- fixed fixture output in t.TempDir
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(raw), "downloading") {
				t.Fatal("stderr contaminated benchmark input")
			}
			stderr, err := os.ReadFile(filepath.Join(outDir, "bench.stderr.txt")) // #nosec G304 -- fixed fixture output in t.TempDir
			if err != nil || !strings.Contains(string(stderr), "downloading") {
				t.Fatalf("diagnostics lost: %v %s", err, stderr)
			}
			data, err := os.ReadFile(filepath.Join(outDir, "metadata.json")) // #nosec G304 -- fixed fixture output in t.TempDir
			if err != nil {
				t.Fatal(err)
			}
			var metadata benchmarkreport.Metadata
			if err := json.Unmarshal(data, &metadata); err != nil {
				t.Fatal(err)
			}
			if metadata.GitVersion != `git version "quoted"` {
				t.Fatalf("version quoting lost: %q", metadata.GitVersion)
			}
			if metadata.MeasurementCommand != "GOWORK=off go test -run='^$' -bench='^BenchmarkCLIStatus$' -count=3 -benchtime=100ms -benchmem ./benchmarks" {
				t.Fatalf("wrong measurement command: %q", metadata.MeasurementCommand)
			}
			if _, err := os.Stat(filepath.Join(outDir, "report.json")); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(outDir, "recording.failed")); !os.IsNotExist(err) {
				t.Fatalf("success has failure marker: %v", err)
			}
		})
	}
}
