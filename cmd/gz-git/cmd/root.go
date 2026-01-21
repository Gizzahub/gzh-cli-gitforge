// Package cmd implements the CLI commands for gz-git.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// version is set by main.go
	appVersion string

	// Global flags
	verbose         bool
	quiet           bool
	profileOverride string // Override active profile
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gz-git",
	Short: "Advanced Git operations CLI",
	Long: `gz-git is a bulk-first Git CLI that runs safe operations across many repositories in parallel.

[1;36mQuick Start:[0m
  # Initialize workspace and check status
  gz-git config init
  gz-git status

See 'gz-git schema' for configuration reference.`,
	Version: appVersion,
	// Uncomment the following line if your application requires Cobra to
	// check for a config file.
	// PersistentPreRun: initConfig,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(version string) {
	appVersion = version
	rootCmd.Version = version

	// Custom Usage Template with Colors
	rootCmd.SetUsageTemplate(usageTemplate)

	setCommandGroups(rootCmd)
	applyUsageTemplateRecursive(rootCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func setCommandGroups(cmd *cobra.Command) {
	coreGroup := &cobra.Group{ID: "core", Title: colorYellowBold + "Core Git Operations" + colorReset}
	mgmtGroup := &cobra.Group{ID: "mgmt", Title: colorYellowBold + "Management & Configuration" + colorReset}
	toolGroup := &cobra.Group{ID: "tool", Title: colorYellowBold + "Additional Tools" + colorReset}

	cmd.AddGroup(coreGroup, mgmtGroup, toolGroup)

	for _, c := range cmd.Commands() {
		// Skip internal commands
		if c.Name() == "help" || c.Name() == "completion" || c.Name() == "version" {
			continue
		}

		switch c.Name() {
		case "clone", "status", "fetch", "pull", "push", "switch", "commit", "update":
			c.GroupID = coreGroup.ID
		case "workspace", "config", "sync", "schema", "cleanup":
			c.GroupID = mgmtGroup.ID
		default:
			c.GroupID = toolGroup.ID
		}
	}
}

func applyUsageTemplateRecursive(cmd *cobra.Command) {
	cmd.SetUsageTemplate(usageTemplate)
	for _, c := range cmd.Commands() {
		applyUsageTemplateRecursive(c)
	}
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "quiet output (errors only)")
	rootCmd.PersistentFlags().StringVar(&profileOverride, "profile", "", "override active profile (e.g., --profile work)")

	// Version template
	rootCmd.SetVersionTemplate(`{{with .Name}}{{printf "%s " .}}{{end}}{{printf "version %s" .Version}}
`)

	// Custom Usage Template with Colors
	rootCmd.SetUsageTemplate(usageTemplate)
}

const (
	colorCyanBold    = "\033[1;36m"
	colorGreenBold   = "\033[1;32m"
	colorYellowBold  = "\033[1;33m"
	colorMagentaBold = "\033[1;35m"
	colorReset       = "\033[0m"
)

const usageTemplate = `{{if .Runnable}}` + colorGreenBold + `Usage:` + colorReset + `
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}` + colorGreenBold + `Usage:` + colorReset + `
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

` + colorGreenBold + `Examples:` + colorReset + `
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Available Commands:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

` + colorMagentaBold + `Additional Commands:` + colorReset + `{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

` + colorGreenBold + `Flags:` + colorReset + `
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

` + colorGreenBold + `Global Flags:` + colorReset + `
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`

func initConfig() {
	// Configuration file support deferred to Phase 2
	// Will implement with Viper when needed for:
	// - Commit message templates
	// - Default Git options
	// - User preferences
}
