// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package wizard

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

// SyncSetupOptions holds the results of the sync setup wizard.
type SyncSetupOptions struct {
	Provider         string
	Organization     string
	BaseURL          string
	Token            string
	CloneProto       string
	SSHPort          int
	IncludeSubgroups bool
	SubgroupMode     string
	TargetPath       string
	IncludeArchived  bool
	IncludePrivate   bool
	IncludeForks     bool
	SaveConfig       bool
	ConfigPath       string
	ExecuteNow       bool
	Parallel         int
}

// SyncSetupWizard guides users through sync setup.
type SyncSetupWizard struct {
	printer *Printer
	opts    SyncSetupOptions
}

// NewSyncSetupWizard creates a new sync setup wizard.
func NewSyncSetupWizard() *SyncSetupWizard {
	return &SyncSetupWizard{
		printer: NewPrinter(),
		opts: SyncSetupOptions{
			CloneProto:     "ssh",
			SubgroupMode:   "flat",
			IncludePrivate: true,
			Parallel:       10,
		},
	}
}

// Run executes the sync setup wizard.
func (w *SyncSetupWizard) Run(_ context.Context) (*SyncSetupOptions, error) {
	w.printer.PrintHeader(IconRocket, "Sync Setup Wizard")
	w.printer.PrintInfo("This wizard will help you configure repository synchronization from a Git forge.")
	fmt.Println()

	// Step 1: Provider and basic settings
	if err := w.runProviderStep(); err != nil {
		return nil, err
	}

	// Step 2: Authentication
	if err := w.runAuthStep(); err != nil {
		return nil, err
	}

	// Step 3: Clone options
	if err := w.runCloneOptionsStep(); err != nil {
		return nil, err
	}

	// Step 4: GitLab specific (if applicable)
	if w.opts.Provider == "gitlab" {
		if err := w.runGitLabOptionsStep(); err != nil {
			return nil, err
		}
	}

	// Step 5: Target and filtering
	if err := w.runTargetStep(); err != nil {
		return nil, err
	}

	// Step 6: Config saving options
	if err := w.runSaveOptionsStep(); err != nil {
		return nil, err
	}

	// Show summary
	w.printSummary()

	// Step 7: Execute confirmation
	if err := w.runExecuteStep(); err != nil {
		return nil, err
	}

	return &w.opts, nil
}

func (w *SyncSetupWizard) runProviderStep() error {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Git Forge Provider").
				Description("Select the Git hosting provider").
				Options(
					huh.NewOption("GitLab (recommended for subgroups)", "gitlab"),
					huh.NewOption("GitHub", "github"),
					huh.NewOption("Gitea", "gitea"),
				).
				Value(&w.opts.Provider),

			huh.NewInput().
				Title("Organization/Group Name").
				Description("The organization, group, or username to sync").
				Placeholder("e.g., myorg, parent-group/child").
				Validate(ValidateOrganization).
				Value(&w.opts.Organization),
		),
	).WithTheme(huh.ThemeCharm())

	return form.Run()
}

func (w *SyncSetupWizard) runAuthStep() error {
	// Determine default base URL based on provider
	defaultBaseURL := ""
	baseURLDescription := "API endpoint for self-hosted instances"
	switch w.opts.Provider {
	case "github":
		baseURLDescription = "Leave empty for github.com, or enter URL for GitHub Enterprise"
	case "gitlab":
		defaultBaseURL = "https://gitlab.com"
		baseURLDescription = "GitLab instance URL (default: gitlab.com)"
	case "gitea":
		baseURLDescription = "Gitea instance URL (required for self-hosted)"
	}

	var baseURL, token string
	baseURL = defaultBaseURL

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Base URL").
				Description(baseURLDescription).
				Placeholder(defaultBaseURL).
				Validate(ValidateURL).
				Value(&baseURL),

			huh.NewInput().
				Title("API Token").
				Description("Use ${ENV_VAR} for environment variables (recommended)").
				Placeholder("${GITLAB_TOKEN} or paste token").
				EchoMode(huh.EchoModePassword).
				Validate(ValidateToken).
				Value(&token),
		),
	).WithTheme(huh.ThemeCharm())

	if err := form.Run(); err != nil {
		return err
	}

	w.opts.BaseURL = baseURL
	w.opts.Token = token
	return nil
}

func (w *SyncSetupWizard) runCloneOptionsStep() error {
	var sshPort string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Clone Protocol").
				Description("How to clone repositories").
				Options(
					huh.NewOption("SSH (recommended)", "ssh"),
					huh.NewOption("HTTPS", "https"),
				).
				Value(&w.opts.CloneProto),

			huh.NewInput().
				Title("SSH Port").
				Description("Custom SSH port (leave empty for default 22)").
				Placeholder("22").
				Validate(ValidatePort).
				Value(&sshPort),
		),
	).WithTheme(huh.ThemeCharm())

	if err := form.Run(); err != nil {
		return err
	}

	w.opts.SSHPort = ParsePort(sshPort)
	return nil
}

func (w *SyncSetupWizard) runGitLabOptionsStep() error {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Include Subgroups").
				Description("Also sync repositories from child groups").
				Affirmative("Yes").
				Negative("No").
				Value(&w.opts.IncludeSubgroups),

			huh.NewSelect[string]().
				Title("Subgroup Mode").
				Description("How to organize subgroup repositories").
				Options(
					huh.NewOption("Flat (group-subgroup-repo)", "flat"),
					huh.NewOption("Nested directories", "nested"),
				).
				Value(&w.opts.SubgroupMode),
		),
	).WithTheme(huh.ThemeCharm())

	return form.Run()
}

func (w *SyncSetupWizard) runTargetStep() error {
	// Default target path
	defaultTarget := "."
	if cwd, err := os.Getwd(); err == nil {
		defaultTarget = cwd
	}

	var targetPath, parallel string
	targetPath = defaultTarget
	parallel = strconv.Itoa(repository.DefaultLocalParallel)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Target Directory").
				Description("Where to clone repositories").
				Placeholder(defaultTarget).
				Validate(ValidatePathRequired).
				Value(&targetPath),

			huh.NewInput().
				Title("Parallel Jobs").
				Description("Number of parallel clone/sync operations").
				Placeholder("5").
				Validate(ValidateParallel).
				Value(&parallel),
		),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Include Archived").
				Description("Sync archived/read-only repositories").
				Affirmative("Yes").
				Negative("No").
				Value(&w.opts.IncludeArchived),

			huh.NewConfirm().
				Title("Include Private").
				Description("Sync private repositories").
				Affirmative("Yes").
				Negative("No").
				Value(&w.opts.IncludePrivate),

			huh.NewConfirm().
				Title("Include Forks").
				Description("Sync forked repositories").
				Affirmative("Yes").
				Negative("No").
				Value(&w.opts.IncludeForks),
		),
	).WithTheme(huh.ThemeCharm())

	if err := form.Run(); err != nil {
		return err
	}

	// Expand ~ in path
	if strings.HasPrefix(targetPath, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			targetPath = filepath.Join(home, targetPath[2:])
		}
	}

	w.opts.TargetPath = targetPath
	w.opts.Parallel = ParseParallel(parallel, 10)
	return nil
}

func (w *SyncSetupWizard) runSaveOptionsStep() error {
	var configPath string
	defaultPath := filepath.Join(w.opts.TargetPath, "sync.yaml")
	configPath = defaultPath

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Save Configuration").
				Description("Save settings to a config file for reuse").
				Affirmative("Yes").
				Negative("No").
				Value(&w.opts.SaveConfig),

			huh.NewInput().
				Title("Config File Path").
				Description("Where to save the configuration").
				Placeholder(defaultPath).
				Value(&configPath),
		),
	).WithTheme(huh.ThemeCharm())

	if err := form.Run(); err != nil {
		return err
	}

	w.opts.ConfigPath = configPath
	return nil
}

func (w *SyncSetupWizard) runExecuteStep() error {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Execute Sync Now").
				Description("Start synchronization immediately after setup").
				Affirmative("Yes, sync now").
				Negative("No, just save config").
				Value(&w.opts.ExecuteNow),
		),
	).WithTheme(huh.ThemeCharm())

	return form.Run()
}

func (w *SyncSetupWizard) printSummary() {
	keys := []string{
		"Provider",
		"Organization",
		"Base URL",
		"Token",
		"Clone Protocol",
		"SSH Port",
		"Include Subgroups",
		"Subgroup Mode",
		"Target Directory",
		"Parallel Jobs",
		"Include Archived",
		"Include Private",
		"Include Forks",
	}

	items := map[string]string{
		"Provider":         w.opts.Provider,
		"Organization":     w.opts.Organization,
		"Base URL":         w.opts.BaseURL,
		"Token":            SanitizeTokenForDisplay(w.opts.Token),
		"Clone Protocol":   w.opts.CloneProto,
		"SSH Port":         FormatInt(w.opts.SSHPort, 22),
		"Target Directory": w.opts.TargetPath,
		"Parallel Jobs":    strconv.Itoa(w.opts.Parallel),
		"Include Archived": FormatBool(w.opts.IncludeArchived),
		"Include Private":  FormatBool(w.opts.IncludePrivate),
		"Include Forks":    FormatBool(w.opts.IncludeForks),
	}

	// Add GitLab-specific options
	if w.opts.Provider == "gitlab" {
		items["Include Subgroups"] = FormatBool(w.opts.IncludeSubgroups)
		items["Subgroup Mode"] = w.opts.SubgroupMode
	}

	w.printer.PrintOrderedSummary("Configuration Summary", keys, items)

	if w.opts.SaveConfig {
		fmt.Println()
		w.printer.PrintInfo(fmt.Sprintf("Config will be saved to: %s", w.opts.ConfigPath))
	}
}

// BuildCommand returns the equivalent CLI command for the options.
func (w *SyncSetupWizard) BuildCommand() string {
	parts := []string{"gz-git", "forge", "from"}

	parts = append(parts, "--provider", w.opts.Provider)
	parts = append(parts, "--org", w.opts.Organization)
	parts = append(parts, "--path", w.opts.TargetPath)

	if w.opts.BaseURL != "" {
		parts = append(parts, "--base-url", w.opts.BaseURL)
	}

	if w.opts.Token != "" {
		if strings.HasPrefix(w.opts.Token, "${") {
			parts = append(parts, "--token", w.opts.Token)
		} else {
			parts = append(parts, "--token", "$TOKEN")
		}
	}

	if w.opts.CloneProto != "ssh" {
		parts = append(parts, "--clone-proto", w.opts.CloneProto)
	}

	if w.opts.SSHPort != 0 && w.opts.SSHPort != 22 {
		parts = append(parts, "--ssh-port", strconv.Itoa(w.opts.SSHPort))
	}

	if w.opts.Provider == "gitlab" {
		if w.opts.IncludeSubgroups {
			parts = append(parts, "--include-subgroups")
		}
		if w.opts.SubgroupMode != "flat" {
			parts = append(parts, "--subgroup-mode", w.opts.SubgroupMode)
		}
	}

	if w.opts.Parallel != 10 {
		parts = append(parts, "--parallel", strconv.Itoa(w.opts.Parallel))
	}

	if w.opts.IncludeArchived {
		parts = append(parts, "--include-archived")
	}

	if w.opts.IncludeForks {
		parts = append(parts, "--include-forks")
	}

	if !w.opts.IncludePrivate {
		parts = append(parts, "--include-private=false")
	}

	return strings.Join(parts, " ")
}
