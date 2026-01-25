// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/scanner"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/templates"
)

// ConfigKind represents the type of configuration to generate.
type ConfigKind string

const (
	// KindWorkspace is the canonical hierarchical workspace format (default).
	KindWorkspace ConfigKind = "workspace"
	// KindRepositories is the flat repositories list format.
	KindRepositories ConfigKind = "repositories"
)

// Kind aliases (deprecated, will warn).
const (
	// KindWorkspaces is deprecated alias for KindWorkspace.
	KindWorkspaces ConfigKind = "workspaces"
	// KindRepository is deprecated alias for KindRepositories.
	KindRepository ConfigKind = "repository"
)

// ValidStrategies for sync operations.
var ValidStrategies = []string{"reset", "pull", "fetch", "skip"}

// NormalizeKind normalizes kind value and returns canonical form.
// Returns (canonical kind, warning message, error).
func NormalizeKind(kind string) (ConfigKind, string, error) {
	switch ConfigKind(kind) {
	case KindWorkspace:
		return KindWorkspace, "", nil
	case KindWorkspaces:
		return KindWorkspace, "kind 'workspaces' is deprecated, use 'workspace' instead", nil
	case KindRepositories:
		return KindRepositories, "", nil
	case KindRepository:
		return KindRepositories, "kind 'repository' is deprecated, use 'repositories' instead", nil
	case "":
		return "", "", fmt.Errorf("kind is required. Run 'gz-git workspace validate' to check your config")
	default:
		return "", "", fmt.Errorf("invalid kind '%s': must be 'workspace' or 'repositories'", kind)
	}
}

// InitOptions holds options for workspace init command.
type InitOptions struct {
	Path            string
	Output          string
	Depth           int
	ExcludePattern  string
	IncludePattern  string
	NoGitIgnore     bool
	Force           bool
	Template        bool
	Kind            string // repositories or workspaces
	Strategy        string // reset, pull, fetch, skip
	ExplainDefaults bool   // include commented defaults for omitted fields
}

func (f CommandFactory) newInitCmd() *cobra.Command {
	opts := &InitOptions{
		Output:      DefaultConfigFile,
		Depth:       2,
		Kind:        string(KindWorkspace),
		Strategy:    "pull", // Default: safe pull (not reset which destroys local changes)
		NoGitIgnore: true,   // Default: ignore .gitignore (devbox pattern has subprojects in .gitignore)
	}

	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize workspace config by scanning for git repos",
		Long: cliutil.QuickStartHelp(`  # Show usage guide
  gz-git workspace init

  # Scan current directory and create config
  gz-git workspace init .

  # Scan specific directory
  gz-git workspace init ~/mydevbox

  # Scan with depth limit (default: 2)
  gz-git workspace init ~/mydevbox -d 3

  # Exclude patterns
  gz-git workspace init . --exclude "vendor,node_modules,tmp"

  # Overwrite existing config
  gz-git workspace init . --force

  # Create empty template (no scanning)
  gz-git workspace init . --template

  # Choose config kind (workspace or repositories)
  gz-git workspace init . --kind repositories

  # Choose sync strategy (pull, reset, fetch, skip)
  gz-git workspace init . --strategy reset`),
		RunE: func(cmd *cobra.Command, args []string) error {
			// No arguments: show usage guide
			if len(args) == 0 {
				return f.showInitGuide(cmd)
			}

			opts.Path = args[0]
			return f.runInit(cmd, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Output, "output", "o", opts.Output, "Output config file name")
	cmd.Flags().IntVarP(&opts.Depth, "scan-depth", "d", opts.Depth, "Directory scan depth")
	cmd.Flags().StringVar(&opts.ExcludePattern, "exclude", "", "Exclude patterns (comma-separated)")
	cmd.Flags().StringVar(&opts.IncludePattern, "include", "", "Force include patterns (comma-separated)")
	cmd.Flags().BoolVar(&opts.NoGitIgnore, "no-gitignore", true, "Ignore .gitignore patterns (default: true for devbox pattern)")
	cmd.Flags().Lookup("no-gitignore").NoOptDefVal = "true" // --no-gitignore without value means true
	cmd.Flags().BoolVarP(&opts.Force, "force", "f", false, "Overwrite existing config file")
	cmd.Flags().BoolVar(&opts.Template, "template", false, "Create empty template without scanning")
	cmd.Flags().BoolVar(&opts.ExplainDefaults, "explain-defaults", false, "Include commented defaults for omitted fields")
	cmd.Flags().StringVarP(&opts.Kind, "kind", "k", opts.Kind, "Config kind: workspace (hierarchical) or repositories (flat list)")
	cmd.Flags().StringVarP(&opts.Strategy, "strategy", "s", opts.Strategy, "Sync strategy: reset, pull, fetch, skip")

	return cmd
}

func (f CommandFactory) showInitGuide(cmd *cobra.Command) error {
	out := cmd.OutOrStdout()

	fmt.Fprintln(out, "📁 Workspace Init - Create config from existing git repositories")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Usage:")
	fmt.Fprintln(out, "  gz-git workspace init <path>    Scan directory and generate config")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Examples:")
	fmt.Fprintln(out, "  gz-git workspace init .              # Scan current directory")
	fmt.Fprintln(out, "  gz-git workspace init ~/mydevbox     # Scan specific directory")
	fmt.Fprintln(out, "  gz-git workspace init . -d 3         # Scan 3 levels deep")
	fmt.Fprintln(out, "  gz-git workspace init . --exclude vendor,tmp")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Options:")
	fmt.Fprintln(out, "  -d, --scan-depth int   Scan depth (default: 2)")
	fmt.Fprintln(out, "  -o, --output string    Output file (default: .gz-git.yaml)")
	fmt.Fprintln(out, "  -k, --kind string      Config kind: workspace (default) or repositories")
	fmt.Fprintln(out, "  -s, --strategy string  Sync strategy: pull (default), reset, fetch, skip")
	fmt.Fprintln(out, "  -f, --force            Overwrite existing config")
	fmt.Fprintln(out, "      --template         Create empty template (no scanning)")
	fmt.Fprintln(out, "      --explain-defaults Include commented defaults for omitted fields")
	fmt.Fprintln(out, "      --exclude string   Exclude patterns (comma-separated)")
	fmt.Fprintln(out)

	// Check if config already exists in current directory
	if _, err := os.Stat(DefaultConfigFile); err == nil {
		fmt.Fprintf(out, "⚠️  Config file already exists: %s\n", DefaultConfigFile)
		fmt.Fprintln(out, "   Use --force to overwrite:")
		fmt.Fprintln(out, "   gz-git workspace init . --force")
		fmt.Fprintln(out)
	}

	return nil
}

func (f CommandFactory) runInit(cmd *cobra.Command, opts *InitOptions) error {
	out := cmd.OutOrStdout()

	// Validate and normalize kind
	normalizedKind, kindWarning, err := NormalizeKind(opts.Kind)
	if err != nil {
		return err
	}
	if kindWarning != "" {
		fmt.Fprintf(out, "⚠️  %s\n", kindWarning)
	}
	opts.Kind = string(normalizedKind)

	// Validate strategy
	if !isValidStrategy(opts.Strategy) {
		return fmt.Errorf("invalid strategy '%s': must be one of %v", opts.Strategy, ValidStrategies)
	}

	// Resolve output path
	outputPath := opts.Output
	if !filepath.IsAbs(outputPath) {
		outputPath = filepath.Join(opts.Path, outputPath)
	}

	// Check if file exists
	if _, err := os.Stat(outputPath); err == nil {
		if !opts.Force {
			fmt.Fprintf(out, "⚠️  Config file already exists: %s\n\n", outputPath)
			fmt.Fprintf(out, "View:      cat %s\n", outputPath)
			fmt.Fprintf(out, "Overwrite: gz-git workspace init %s --force\n", opts.Path)
			return nil
		}
	}

	// Template mode: create empty template without scanning
	if opts.Template {
		return f.createTemplate(cmd, opts, outputPath)
	}

	// Scan mode: scan directory and generate config
	return f.scanAndGenerate(cmd, opts, outputPath)
}

// isValidStrategy checks if strategy is valid.
func isValidStrategy(strategy string) bool {
	for _, s := range ValidStrategies {
		if s == strategy {
			return true
		}
	}
	return false
}

func (f CommandFactory) createTemplate(cmd *cobra.Command, opts *InitOptions, outputPath string) error {
	var templateName templates.TemplateName

	switch ConfigKind(opts.Kind) {
	case KindWorkspace, KindWorkspaces: // KindWorkspaces is deprecated alias
		templateName = templates.WorkspaceWorkstation
	case KindRepositories, KindRepository: // KindRepository is deprecated alias
		templateName = templates.RepositoriesSample
	default:
		return fmt.Errorf("unknown kind: %s", opts.Kind)
	}

	sampleContent, err := templates.GetRaw(templateName)
	if err != nil {
		return fmt.Errorf("failed to load sample template: %w", err)
	}

	if err := os.WriteFile(outputPath, sampleContent, 0o644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✓ Created empty template (%s): %s\n", opts.Kind, outputPath)
	fmt.Fprintln(cmd.OutOrStdout(), "\nEdit the file and add your repositories, then run:")
	fmt.Fprintln(cmd.OutOrStdout(), "  gz-git workspace sync")

	return nil
}

func (f CommandFactory) scanAndGenerate(cmd *cobra.Command, opts *InitOptions, outputPath string) error {
	ctx := cmd.Context()

	// Parse exclude/include patterns
	var excludePatterns []string
	if opts.ExcludePattern != "" {
		excludePatterns = strings.Split(opts.ExcludePattern, ",")
	}

	var includePatterns []string
	if opts.IncludePattern != "" {
		includePatterns = strings.Split(opts.IncludePattern, ",")
	}

	// Create scanner
	gitScanner := &scanner.GitRepoScanner{
		RootPath:         opts.Path,
		MaxDepth:         opts.Depth,
		RespectGitIgnore: !opts.NoGitIgnore,
		ExcludePatterns:  excludePatterns,
		IncludePatterns:  includePatterns,
	}

	// Scan for repositories
	fmt.Fprintf(cmd.OutOrStdout(), "Scanning %s (depth: %d)...\n", opts.Path, opts.Depth)
	repos, err := gitScanner.Scan(ctx)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	if len(repos) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No git repositories found.")
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), "To create an empty template instead:")
		fmt.Fprintf(cmd.OutOrStdout(), "  gz-git workspace init %s --template\n", opts.Path)
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Found %d git repositories\n\n", len(repos))

	return f.generateConfigFromScan(cmd, opts, repos, outputPath)
}

func (f CommandFactory) generateConfigFromScan(cmd *cobra.Command, opts *InitOptions, repos []*scanner.ScannedRepo, outputPath string) error {
	var content string
	var err error

	baseDir := filepath.Dir(outputPath)

	switch ConfigKind(opts.Kind) {
	case KindWorkspace, KindWorkspaces: // KindWorkspaces is deprecated alias
		content, err = f.renderWorkspaceScanned(opts, repos, baseDir)
	case KindRepositories, KindRepository: // KindRepository is deprecated alias
		content, err = f.renderRepositoriesScanned(opts, repos, baseDir)
	default:
		return fmt.Errorf("unknown kind: %s", opts.Kind)
	}

	if err != nil {
		return fmt.Errorf("render template: %w", err)
	}

	if err := os.WriteFile(outputPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✓ Generated config (%s): %s (%d repositories)\n", opts.Kind, outputPath, len(repos))
	fmt.Fprintln(cmd.OutOrStdout(), "\nNext steps:")
	fmt.Fprintln(cmd.OutOrStdout(), "  gz-git workspace sync        # Clone/update repositories")
	fmt.Fprintln(cmd.OutOrStdout(), "  gz-git workspace status      # Check workspace health")

	return nil
}

// ================================================================================
// Shared Helper Functions
// ================================================================================

// Default values for scanned configs.
const (
	defaultParallel   = 10
	defaultMaxRetries = 3
	defaultCloneProto = "ssh"
)

// processRepoPath processes a repository path for config generation.
// Returns (relPath, pathValue, skip).
// - relPath: the relative path from baseDir
// - pathValue: the path to use in config (empty if equals name for compact mode)
// - skip: true if this repo should be skipped (e.g., root directory ".")
func processRepoPath(baseDir, repoPath, repoName string) (relPath, pathValue string, skip bool) {
	relPath = relativeRepoPath(baseDir, repoPath)

	// Skip the root directory itself - it's the orchestrator, not a target
	// selfSync handles the root directory separately
	if relPath == "." {
		return "", "", true
	}

	// Omit path when it equals the name (compact mode)
	pathValue = relPath
	if relPath == repoName {
		pathValue = ""
	}

	return relPath, pathValue, false
}

// buildCommonConfig creates the shared configuration from scan options and URLs.
func buildCommonConfig(opts *InitOptions, count int, urls []string) templates.CommonScannedConfig {
	return templates.CommonScannedConfig{
		ScannedAt:       time.Now().Format(time.RFC3339),
		Count:           count,
		Strategy:        opts.Strategy,
		Parallel:        defaultParallel,
		MaxRetries:      defaultMaxRetries,
		CloneProto:      defaultCloneProto,
		SSHPort:         extractSSHPortFromURLs(urls),
		ExplainDefaults: opts.ExplainDefaults,
	}
}

// collectURLs extracts URLs from a slice using a getter function.
func collectURLs[T any](items []T, getURL func(T) string) []string {
	urls := make([]string, 0, len(items))
	for _, item := range items {
		if url := getURL(item); url != "" {
			urls = append(urls, url)
		}
	}
	return urls
}

// ================================================================================
// Render Functions
// ================================================================================

// renderWorkspaceScanned renders workspace (hierarchical) format.
func (f CommandFactory) renderWorkspaceScanned(opts *InitOptions, repos []*scanner.ScannedRepo, baseDir string) (string, error) {
	workspaces := make([]templates.WorkspaceScannedEntry, 0, len(repos))
	for _, repo := range repos {
		_, pathValue, skip := processRepoPath(baseDir, repo.Path, repo.Name)
		if skip {
			continue
		}

		workspaces = append(workspaces, templates.WorkspaceScannedEntry{
			Name:   repo.Name,
			Path:   pathValue,
			URL:    extractPrimaryURL(repo.Remotes),
			Branch: repo.Branch,
		})
	}

	// Get workspace name from path
	workspaceName := filepath.Base(opts.Path)
	if workspaceName == "." {
		if cwd, err := os.Getwd(); err == nil {
			workspaceName = filepath.Base(cwd)
		}
	}

	// Build common config with extracted URLs
	urls := collectURLs(workspaces, func(ws templates.WorkspaceScannedEntry) string { return ws.URL })

	data := templates.WorkspaceScannedData{
		CommonScannedConfig: buildCommonConfig(opts, len(workspaces), urls),
		Name:                workspaceName,
		Workspaces:          workspaces,
	}

	return templates.Render(templates.WorkspaceScanned, data)
}

// renderRepositoriesScanned renders repositories (flat list) format.
func (f CommandFactory) renderRepositoriesScanned(opts *InitOptions, repos []*scanner.ScannedRepo, baseDir string) (string, error) {
	repoEntries := make([]templates.ScannedRepoData, 0, len(repos))
	for _, repo := range repos {
		_, pathValue, skip := processRepoPath(baseDir, repo.Path, repo.Name)
		if skip {
			continue
		}

		entry := templates.ScannedRepoData{
			Name:   repo.Name,
			Path:   pathValue,
			Branch: repo.Branch,
		}

		// Handle remotes: origin → URL, others → additionalRemotes
		if len(repo.Remotes) > 0 {
			if originURL, ok := repo.Remotes["origin"]; ok {
				entry.URL = originURL
				if len(repo.Remotes) > 1 {
					entry.AdditionalRemotes = make(map[string]string)
					for name, url := range repo.Remotes {
						if name != "origin" {
							entry.AdditionalRemotes[name] = url
						}
					}
				}
			} else {
				first := true
				for name, url := range repo.Remotes {
					if first {
						entry.URL = url
						first = false
					} else {
						if entry.AdditionalRemotes == nil {
							entry.AdditionalRemotes = make(map[string]string)
						}
						entry.AdditionalRemotes[name] = url
					}
				}
			}
		}

		repoEntries = append(repoEntries, entry)
	}

	// Build common config with extracted URLs
	urls := collectURLs(repoEntries, func(e templates.ScannedRepoData) string { return e.URL })

	data := templates.ScannedData{
		CommonScannedConfig: buildCommonConfig(opts, len(repoEntries), urls),
		BasePath:            ".",
		Repositories:        repoEntries,
	}

	return templates.Render(templates.RepositoriesScanned, data)
}

// extractPrimaryURL extracts the primary URL from remotes (prefers origin).
func extractPrimaryURL(remotes map[string]string) string {
	if len(remotes) == 0 {
		return ""
	}
	if originURL, ok := remotes["origin"]; ok {
		return originURL
	}
	// Return first available
	for _, url := range remotes {
		return url
	}
	return ""
}

// extractSSHPortFromURLs extracts the SSH port from URLs if they use a non-standard port.
// Returns 0 if no custom port is detected or ports are inconsistent.
func extractSSHPortFromURLs(urls []string) int {
	var detectedPort int
	for _, url := range urls {
		port := extractSSHPortFromURL(url)
		if port == 0 {
			continue
		}
		if detectedPort == 0 {
			detectedPort = port
		} else if detectedPort != port {
			// Inconsistent ports - return 0
			return 0
		}
	}
	return detectedPort
}

// extractSSHPortFromURL extracts the SSH port from a single URL.
// Supports formats like:
//   - ssh://git@host:2224/path
//   - ssh://git@host:2224/path.git
//
// Returns 0 for standard port (22) or non-SSH URLs.
func extractSSHPortFromURL(url string) int {
	// Only handle ssh:// URLs with explicit port
	if !strings.HasPrefix(url, "ssh://") {
		return 0
	}

	// Remove ssh:// prefix
	rest := strings.TrimPrefix(url, "ssh://")

	// Find the host:port part (before the first /)
	slashIdx := strings.Index(rest, "/")
	if slashIdx == -1 {
		return 0
	}
	hostPort := rest[:slashIdx]

	// Find port after @user if present
	atIdx := strings.LastIndex(hostPort, "@")
	if atIdx != -1 {
		hostPort = hostPort[atIdx+1:]
	}

	// Find port
	colonIdx := strings.LastIndex(hostPort, ":")
	if colonIdx == -1 {
		return 0
	}

	portStr := hostPort[colonIdx+1:]
	port := 0
	for _, c := range portStr {
		if c < '0' || c > '9' {
			return 0
		}
		port = port*10 + int(c-'0')
	}

	// Skip standard SSH port
	if port == 22 {
		return 0
	}

	return port
}

func relativeRepoPath(baseDir, repoPath string) string {
	if baseDir == "" || repoPath == "" {
		return repoPath
	}
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		absBase = baseDir
	}
	absRepo, err := filepath.Abs(repoPath)
	if err != nil {
		return repoPath
	}
	rel, err := filepath.Rel(absBase, absRepo)
	if err != nil || rel == "" {
		return repoPath
	}
	return rel
}
