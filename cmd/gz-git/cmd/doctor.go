package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/doctor"
)

var (
	doctorSkipForge bool
	doctorSkipRepo  bool
	doctorScanDepth int
	doctorFormat    string
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose system, config, and connectivity health",
	Long: cliutil.QuickStartHelp(`  # Run all checks
  gz-git doctor

  # Skip forge/repo checks
  gz-git doctor --skip-forge
  gz-git doctor --skip-repo

  # Scan repos 2 levels deep
  gz-git doctor -d 2

  # Verbose (per-profile, per-branch details)
  gz-git doctor -v

  # JSON output
  gz-git doctor --format json`),
	RunE: runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)

	doctorCmd.Flags().BoolVar(&doctorSkipForge, "skip-forge", false, "skip forge API connectivity checks")
	doctorCmd.Flags().BoolVar(&doctorSkipRepo, "skip-repo", false, "skip repository health checks")
	doctorCmd.Flags().IntVarP(&doctorScanDepth, "scan-depth", "d", 1, "directory depth to scan for repositories")
	doctorCmd.Flags().StringVar(&doctorFormat, "format", "", "output format (json)")
}

func runDoctor(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	opts := doctor.Options{
		SkipForge: doctorSkipForge,
		SkipRepo:  doctorSkipRepo,
		Verbose:   verbose,
		ScanDepth: doctorScanDepth,
	}

	report := doctor.Run(ctx, opts)

	if doctorFormat == "json" {
		return printDoctorJSON(report)
	}

	printDoctorReport(report)
	return nil
}

func printDoctorJSON(report *doctor.Report) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func printDoctorReport(report *doctor.Report) {
	fmt.Println()
	fmt.Printf("%sgz-git doctor%s\n", cliutil.ColorCyanBold, cliutil.ColorReset)
	fmt.Println()

	currentCategory := doctor.Category("")

	for _, c := range report.Checks {
		// Print category header on change
		if c.Category != currentCategory {
			currentCategory = c.Category
			fmt.Printf("%s%s%s\n", cliutil.ColorYellowBold, categoryTitle(c.Category), cliutil.ColorReset)
		}

		symbol := statusSymbol(c.Status)
		fmt.Printf("  %s %s\n", symbol, c.Message)

		if c.Detail != "" {
			fmt.Printf("    %s\n", c.Detail)
		}
	}

	// Summary
	fmt.Println()
	s := report.Summary
	fmt.Printf("Checks: %d total", s.Total)
	if s.OK > 0 {
		fmt.Printf(", %s%d ok%s", cliutil.ColorGreen, s.OK, cliutil.ColorReset)
	}
	if s.Warning > 0 {
		fmt.Printf(", %s%d warning%s", cliutil.ColorYellow, s.Warning, cliutil.ColorReset)
	}
	if s.Error > 0 {
		fmt.Printf(", %s%d error%s", cliutil.ColorRed, s.Error, cliutil.ColorReset)
	}
	if s.Unreachable > 0 {
		fmt.Printf(", %d unreachable", s.Unreachable)
	}
	fmt.Printf(" (%s)\n", report.Duration.Round(1e6))
	fmt.Println()
}

func statusSymbol(s doctor.Status) string {
	switch s {
	case doctor.StatusOK:
		return cliutil.ColorGreen + "✓" + cliutil.ColorReset
	case doctor.StatusWarning:
		return cliutil.ColorYellow + "⚠" + cliutil.ColorReset
	case doctor.StatusError:
		return cliutil.ColorRed + "✗" + cliutil.ColorReset
	case doctor.StatusUnreachable:
		return cliutil.ColorGray + "⊘" + cliutil.ColorReset
	case doctor.StatusSkipped:
		return cliutil.ColorGray + "-" + cliutil.ColorReset
	default:
		return "?"
	}
}

func categoryTitle(c doctor.Category) string {
	switch c {
	case doctor.CategorySystem:
		return "System"
	case doctor.CategoryConfig:
		return "Configuration"
	case doctor.CategoryAuth:
		return "Authentication"
	case doctor.CategoryForge:
		return "Forge Connectivity"
	case doctor.CategoryRepo:
		return "Repositories"
	default:
		return string(c)
	}
}
