// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package wizard

import (
	"context"
	"fmt"

	"github.com/charmbracelet/huh"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

// ProfileCreateWizard guides users through profile creation.
type ProfileCreateWizard struct {
	printer     *Printer
	profileName string
	profile     *config.Profile
}

// NewProfileCreateWizard creates a new profile creation wizard.
func NewProfileCreateWizard(name string) *ProfileCreateWizard {
	return &ProfileCreateWizard{
		printer:     NewPrinter(),
		profileName: name,
		profile: &config.Profile{
			Name:       name,
			CloneProto: "ssh",
			Parallel:   5,
		},
	}
}

// Run executes the profile creation wizard.
func (w *ProfileCreateWizard) Run(_ context.Context) (*config.Profile, error) {
	w.printer.PrintHeader(IconGear, "Profile Creation Wizard")
	w.printer.PrintInfo(fmt.Sprintf("Creating profile: %s", w.profileName))
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
	if w.profile.Provider == "gitlab" {
		if err := w.runGitLabOptionsStep(); err != nil {
			return nil, err
		}
	}

	// Step 5: Parallelism
	if err := w.runParallelStep(); err != nil {
		return nil, err
	}

	// Show summary and confirm
	w.printSummary()

	var confirm bool
	confirmForm := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Save Profile?").
				Description("Save this profile to your configuration").
				Affirmative("Yes, save").
				Negative("No, cancel").
				Value(&confirm),
		),
	).WithTheme(huh.ThemeCharm())

	if err := confirmForm.Run(); err != nil {
		return nil, err
	}

	if !confirm {
		return nil, fmt.Errorf("profile creation cancelled")
	}

	return w.profile, nil
}

func (w *ProfileCreateWizard) runProviderStep() error {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Git Forge Provider").
				Description("Select the default provider for this profile").
				Options(
					huh.NewOption("GitLab", "gitlab"),
					huh.NewOption("GitHub", "github"),
					huh.NewOption("Gitea", "gitea"),
					huh.NewOption("(No default)", ""),
				).
				Value(&w.profile.Provider),
		),
	).WithTheme(huh.ThemeCharm())

	return form.Run()
}

func (w *ProfileCreateWizard) runAuthStep() error {
	// Skip if no provider selected
	if w.profile.Provider == "" {
		return nil
	}

	// Determine default base URL based on provider
	baseURLDescription := "API endpoint (leave empty for default)"
	switch w.profile.Provider {
	case "github":
		baseURLDescription = "Leave empty for github.com, or enter GitHub Enterprise URL"
	case "gitlab":
		baseURLDescription = "Leave empty for gitlab.com, or enter self-hosted GitLab URL"
	case "gitea":
		baseURLDescription = "Gitea instance URL (required)"
	}

	var tokenMethod string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Base URL").
				Description(baseURLDescription).
				Placeholder("https://gitlab.company.com").
				Validate(ValidateURL).
				Value(&w.profile.BaseURL),

			huh.NewSelect[string]().
				Title("Token Storage").
				Description("How should the API token be stored?").
				Options(
					huh.NewOption("Environment variable (recommended)", "env"),
					huh.NewOption("Store directly in profile", "direct"),
					huh.NewOption("Skip (enter later)", "skip"),
				).
				Value(&tokenMethod),
		),
	).WithTheme(huh.ThemeCharm())

	if err := form.Run(); err != nil {
		return err
	}

	// Handle token based on method
	switch tokenMethod {
	case "env":
		var envVar string
		defaultEnvVar := ""
		switch w.profile.Provider {
		case "github":
			defaultEnvVar = "GITHUB_TOKEN"
		case "gitlab":
			defaultEnvVar = "GITLAB_TOKEN"
		case "gitea":
			defaultEnvVar = "GITEA_TOKEN"
		}

		envForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Environment Variable Name").
					Description("The environment variable containing your token").
					Placeholder(defaultEnvVar).
					Value(&envVar),
			),
		).WithTheme(huh.ThemeCharm())

		if err := envForm.Run(); err != nil {
			return err
		}

		if envVar == "" {
			envVar = defaultEnvVar
		}
		w.profile.Token = "${" + envVar + "}"

	case "direct":
		var token string
		tokenForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("API Token").
					Description("Your personal access token (will be stored in profile)").
					EchoMode(huh.EchoModePassword).
					Validate(ValidateToken).
					Value(&token),
			),
		).WithTheme(huh.ThemeCharm())

		if err := tokenForm.Run(); err != nil {
			return err
		}

		w.profile.Token = token
		w.printer.PrintWarning("Consider using environment variables for better security")
	}

	return nil
}

func (w *ProfileCreateWizard) runCloneOptionsStep() error {
	var sshPort string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Clone Protocol").
				Description("Default protocol for cloning repositories").
				Options(
					huh.NewOption("SSH (recommended)", "ssh"),
					huh.NewOption("HTTPS", "https"),
				).
				Value(&w.profile.CloneProto),

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

	w.profile.SSHPort = ParsePort(sshPort)
	return nil
}

func (w *ProfileCreateWizard) runGitLabOptionsStep() error {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Include Subgroups by Default").
				Description("Automatically include repositories from child groups").
				Affirmative("Yes").
				Negative("No").
				Value(&w.profile.IncludeSubgroups),

			huh.NewSelect[string]().
				Title("Subgroup Mode").
				Description("How to organize subgroup repositories").
				Options(
					huh.NewOption("Flat (group-subgroup-repo)", "flat"),
					huh.NewOption("Nested directories", "nested"),
				).
				Value(&w.profile.SubgroupMode),
		),
	).WithTheme(huh.ThemeCharm())

	return form.Run()
}

func (w *ProfileCreateWizard) runParallelStep() error {
	var parallel string
	parallel = "5"

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Parallel Jobs").
				Description("Default number of parallel operations").
				Placeholder("5").
				Validate(ValidateParallel).
				Value(&parallel),
		),
	).WithTheme(huh.ThemeCharm())

	if err := form.Run(); err != nil {
		return err
	}

	w.profile.Parallel = ParseParallel(parallel, 5)
	return nil
}

func (w *ProfileCreateWizard) printSummary() {
	keys := []string{
		"Name",
		"Provider",
		"Base URL",
		"Token",
		"Clone Protocol",
		"SSH Port",
		"Parallel Jobs",
		"Include Subgroups",
		"Subgroup Mode",
	}

	items := map[string]string{
		"Name":              w.profile.Name,
		"Provider":          w.profile.Provider,
		"Base URL":          w.profile.BaseURL,
		"Token":             SanitizeTokenForDisplay(w.profile.Token),
		"Clone Protocol":    w.profile.CloneProto,
		"SSH Port":          FormatInt(w.profile.SSHPort, 22),
		"Parallel Jobs":     FormatInt(w.profile.Parallel, 5),
		"Include Subgroups": FormatBool(w.profile.IncludeSubgroups),
		"Subgroup Mode":     w.profile.SubgroupMode,
	}

	// Remove empty fields
	if w.profile.Provider == "" {
		delete(items, "Provider")
		delete(items, "Base URL")
		delete(items, "Token")
	}
	if w.profile.Provider != "gitlab" {
		delete(items, "Include Subgroups")
		delete(items, "Subgroup Mode")
	}

	w.printer.PrintOrderedSummary("Profile Summary", keys, items)
}
