// Command coveragegate enforces the critical package coverage floors.
//
// It reads a floors manifest and a Go coverprofile produced by
// "go test -coverprofile", attributes the profile statements to packages via
// the module path in go.mod, and fails when any manifest package is missing
// from the profile, malformed input is found, or a package's coverage drops
// below its floor. Exactly meeting a floor passes.
//
// Exit codes: 0 when every required package meets its floor, 1 otherwise.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/gizzahub/gzh-cli-gitforge/internal/coveragegate"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is the testable body of main: it parses args, checks the profile, and
// returns the process exit code.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("coveragegate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	manifestPath := fs.String("manifest", "", "path to the critical coverage floors manifest (required)")
	profilePath := fs.String("profile", "", "path to the Go coverprofile to check (required)")
	gomodPath := fs.String("gomod", "go.mod", "path to go.mod, used to attribute profile records to packages")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "coveragegate: unexpected argument %q\n", fs.Arg(0))
		return 1
	}
	if *manifestPath == "" || *profilePath == "" {
		fmt.Fprintln(stderr, "coveragegate: both -manifest and -profile are required")
		return 1
	}
	manifest, err := coveragegate.LoadManifest(*manifestPath)
	if err != nil {
		fmt.Fprintf(stderr, "coveragegate: %v\n", err)
		return 1
	}
	modulePath, err := coveragegate.ReadModulePath(*gomodPath)
	if err != nil {
		fmt.Fprintf(stderr, "coveragegate: %v\n", err)
		return 1
	}
	profile, err := os.ReadFile(*profilePath) // #nosec G304 -- the profile path is an explicit operator-supplied flag.
	if err != nil {
		fmt.Fprintf(stderr, "coveragegate: %v\n", err)
		return 1
	}
	results, checkErr := coveragegate.Check(manifest, modulePath, profile)
	for _, result := range results {
		printResult(stdout, result)
	}
	if checkErr != nil {
		fmt.Fprintf(stderr, "coveragegate: %v\n", checkErr)
		return 1
	}
	fmt.Fprintln(stdout, "critical coverage floors satisfied")
	return 0
}

// printResult writes one per-package verdict line.
func printResult(w io.Writer, result coveragegate.Result) {
	verdict := "PASS"
	if !result.Passed {
		verdict = "FAIL"
	}
	if result.Missing {
		fmt.Fprintf(w, "%s %s: missing from profile\n", verdict, result.Package)
		return
	}
	fmt.Fprintf(w, "%s %s: %d/%d statements = %s%% (floor %s%%)\n",
		verdict, result.Package, result.Covered, result.Total,
		coveragegate.FormatTenths(result.TenthsOfPct), coveragegate.FormatTenths(result.FloorTenths))
}
