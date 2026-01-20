# Config Profiles Design

## Overview

**Feature**: Per-project and global settings management
**Priority**: P2
**Phase**: 8.2
**Status**: Design

## Problem Statement

### Current Pain Points (v0.4.0)

**Problem 1: Repetitive flags**

```bash
# Work context - every command needs these flags
gz-git sync from-forge \
  --provider gitlab \
  --base-url https://gitlab.company.com \
  --token $WORK_TOKEN \
  --clone-proto ssh \
  --ssh-port 2224 \
  --org backend

# Personal context - different flags
gz-git sync from-forge \
  --provider github \
  --token $PERSONAL_TOKEN \
  --clone-proto https \
  --org my-projects
```

**Problem 2: No project-specific defaults**

```bash
cd ~/work/monorepo/
gz-git status  # Uses global defaults, not project-specific
```

**Problem 3: Configuration scattered**

- Sync config: `sync.yaml` (custom location)
- No global config file
- No profile concept
- No inheritance/precedence rules

## Goals

1. **Reduce typing**: Named profiles eliminate repetitive flags
1. **Context switching**: Easy switching between work/personal/etc.
1. **Project-specific**: Auto-detect `.gz-git.yaml` in projects
1. **Clear precedence**: Flags > Project > Profile > Defaults

## Design

### 1. Profile Structure

**Profile** = Named set of default values for commands

```yaml
# ~/.config/gz-git/profiles/work.yaml
name: work
provider: gitlab
baseURL: https://gitlab.company.com
token: ${WORK_GITLAB_TOKEN}  # Environment variable reference
cloneProto: ssh
sshPort: 2224
parallel: 10
includeSubgroups: true
subgroupMode: flat

# Optional: command-specific overrides
sync:
  strategy: reset
  maxRetries: 3

branch:
  defaultBranch: develop  # Use 'develop' instead of 'main'
```

### 2. File Locations

```
~/.config/gz-git/
├── config.yaml                 # Global config
├── profiles/
│   ├── default.yaml           # Default profile (auto-created)
│   ├── work.yaml              # User-created profiles
│   ├── personal.yaml
│   └── opensource.yaml
└── state/
    └── active-profile.txt     # Currently active profile

# Project-specific config (auto-detected)
~/myproject/.gz-git.yaml
```

**Config File Priority** (highest to lowest):

1. **Command flags** (e.g., `--provider gitlab`)
1. **Project config** (`.gz-git.yaml` in current dir or parent)
1. **Active profile** (`~/.config/gz-git/profiles/{active}.yaml`)
1. **Global config** (`~/.config/gz-git/config.yaml`)
1. **Built-in defaults**

### 3. Global Config Format

```yaml
# ~/.config/gz-git/config.yaml
activeProfile: work  # Default profile to use

# Global defaults (apply to all profiles unless overridden)
defaults:
  parallel: 10
  cloneProto: ssh
  format: default

# Environment-specific settings
environments:
  work:
    gitlabToken: ${WORK_GITLAB_TOKEN}
    githubToken: ${WORK_GITHUB_TOKEN}
  personal:
    githubToken: ${PERSONAL_GITHUB_TOKEN}
```

### 4. Project Config Format

```yaml
# ~/myproject/.gz-git.yaml
# Auto-detected when running commands inside this directory

profile: work  # Use 'work' profile for this project

# Project-specific overrides
sync:
  strategy: pull  # Always pull, never reset
  parallel: 3     # Lower parallelism for this project

branch:
  defaultBranch: main
  protectedBranches: [main, develop, release/*]

# Project metadata (optional)
metadata:
  team: backend
  repository: https://gitlab.company.com/backend/myproject
```

## CLI Interface

### Profile Management

```bash
# List profiles
gz-git config profile list
# Output:
#   default (active)
#   work
#   personal
#   opensource

# Show profile details
gz-git config profile show work
# Output: YAML content of work.yaml

# Create profile (interactive)
gz-git config profile create work
# Prompts for: provider, baseURL, token, etc.

# Create profile (from flags)
gz-git config profile create work \
  --provider gitlab \
  --base-url https://gitlab.company.com \
  --clone-proto ssh \
  --ssh-port 2224

# Set active profile
gz-git config profile use work
# Updates ~/.config/gz-git/state/active-profile.txt

# Delete profile
gz-git config profile delete work

# Edit profile
gz-git config profile edit work
# Opens $EDITOR with profile YAML
```

### Global Config

```bash
# Show effective config (with precedence)
gz-git config show
# Output:
#   Source: Project (.gz-git.yaml)
#   Profile: work
#   provider: gitlab
#   baseURL: https://gitlab.company.com (from profile)
#   parallel: 3 (from project, overrides profile)

# Get specific value
gz-git config get provider
# Output: gitlab

# Set global default
gz-git config set defaults.parallel 10

# Initialize config directory
gz-git config init
# Creates ~/.config/gz-git/ with default profile
```

### Project Config

```bash
# Initialize project config
cd ~/myproject/
gz-git config init --local
# Creates .gz-git.yaml with defaults

# Set project profile
gz-git config set-profile work
# Updates .gz-git.yaml: profile: work

# Get project config
gz-git config show --local
```

## Usage Examples

### Example 1: Work Context

```bash
# One-time setup
gz-git config profile create work \
  --provider gitlab \
  --base-url https://gitlab.company.com \
  --token $WORK_TOKEN \
  --clone-proto ssh \
  --ssh-port 2224

gz-git config profile use work

# Now all commands use work profile
gz-git sync from-forge --org backend
# Automatically uses gitlab, custom SSH port, etc.

gz-git status
# Uses work profile defaults
```

### Example 2: Project-Specific Config

```bash
cd ~/work/important-project/

# Initialize project config
gz-git config init --local
gz-git config set-profile work

# Set project-specific strategy
echo "sync:
  strategy: pull
  parallel: 3" >> .gz-git.yaml

# Now this project always uses pull strategy
gz-git sync from-config -c sync.yaml
# Uses pull strategy from .gz-git.yaml, not work profile
```

### Example 3: Context Switching

```bash
# Switch to personal profile
gz-git config profile use personal

# All commands now use personal settings
gz-git sync from-forge --org my-projects
# Uses GitHub, personal token, etc.

# One-off override
gz-git sync from-forge --profile work --org backend
# Temporarily uses work profile
```

### Example 4: Flag Override

```bash
# Active profile: work (GitLab, SSH)
gz-git config profile use work

# Override protocol for one command
gz-git sync from-forge --org backend --clone-proto https
# Uses GitLab from profile, HTTPS from flag
```

## Implementation Plan

### Phase 1: Basic Profile Management (Week 1)

- [ ] Config directory structure (`~/.config/gz-git/`)
- [ ] Profile YAML format
- [ ] CRUD operations:
  - `config profile create`
  - `config profile list`
  - `config profile show`
  - `config profile delete`
- [ ] Active profile tracking

### Phase 2: Config Loading & Precedence (Week 1-2)

- [ ] Config loader with precedence rules
- [ ] Environment variable expansion (`${VAR}`)
- [ ] Validation (required fields, types)
- [ ] Integration with existing commands
- [ ] `config show` (effective config viewer)

### Phase 3: Project Config (Week 2)

- [ ] `.gz-git.yaml` auto-detection (walk up directory tree)
- [ ] `config init --local`
- [ ] Project config precedence
- [ ] `config show --local`

### Phase 4: Polish & Testing (Week 2)

- [ ] Interactive profile creation wizard
- [ ] Profile templates (GitLab, GitHub, Gitea)
- [ ] Migration guide for existing users
- [ ] Unit tests (precedence logic)
- [ ] Integration tests
- [ ] Documentation

## Data Structures

```go
// Profile represents a named configuration profile
type Profile struct {
    Name            string            `yaml:"name"`
    Provider        string            `yaml:"provider,omitempty"`
    BaseURL         string            `yaml:"baseURL,omitempty"`
    Token           string            `yaml:"token,omitempty"`
    CloneProto      string            `yaml:"cloneProto,omitempty"`
    SSHPort         int               `yaml:"sshPort,omitempty"`
    Parallel        int               `yaml:"parallel,omitempty"`
    IncludeSubgroups bool             `yaml:"includeSubgroups,omitempty"`
    SubgroupMode    string            `yaml:"subgroupMode,omitempty"`

    // Command-specific overrides
    Sync            *SyncConfig       `yaml:"sync,omitempty"`
    Branch          *BranchConfig     `yaml:"branch,omitempty"`
}

// GlobalConfig represents ~/.config/gz-git/config.yaml
type GlobalConfig struct {
    ActiveProfile string                 `yaml:"activeProfile"`
    Defaults      map[string]interface{} `yaml:"defaults"`
    Environments  map[string]Environment `yaml:"environments"`
}

// ProjectConfig represents .gz-git.yaml
type ProjectConfig struct {
    Profile  string                 `yaml:"profile"`
    Sync     *SyncConfig            `yaml:"sync,omitempty"`
    Branch   *BranchConfig          `yaml:"branch,omitempty"`
    Metadata *ProjectMetadata       `yaml:"metadata,omitempty"`
}

// ConfigLoader handles config precedence
type ConfigLoader struct {
    globalConfig  *GlobalConfig
    activeProfile *Profile
    projectConfig *ProjectConfig
}

func (l *ConfigLoader) Get(key string) (interface{}, error) {
    // 1. Check project config
    // 2. Check active profile
    // 3. Check global defaults
    // 4. Return built-in default
}
```

## Precedence Algorithm

```go
func ResolveConfig(flags map[string]interface{}, cwd string) (*EffectiveConfig, error) {
    // 1. Load configs
    globalCfg := loadGlobalConfig()
    activeProfName := globalCfg.ActiveProfile
    activeProf := loadProfile(activeProfName)
    projectCfg := findProjectConfig(cwd) // Walk up directory tree

    // 2. Build effective config (reverse order - lowest priority first)
    effective := &EffectiveConfig{}

    // Layer 1: Built-in defaults
    applyDefaults(effective)

    // Layer 2: Global config
    applyGlobalConfig(effective, globalCfg)

    // Layer 3: Active profile
    if activeProf != nil {
        applyProfile(effective, activeProf)
    }

    // Layer 4: Project config
    if projectCfg != nil {
        // Load profile specified in project config
        if projectCfg.Profile != "" && projectCfg.Profile != activeProfName {
            projProf := loadProfile(projectCfg.Profile)
            applyProfile(effective, projProf)
        }
        applyProjectConfig(effective, projectCfg)
    }

    // Layer 5: Command flags (highest priority)
    applyFlags(effective, flags)

    return effective, nil
}
```

## Testing Strategy

### Unit Tests

- Config parsing (YAML unmarshal)
- Precedence rules (flags > project > profile > global)
- Environment variable expansion
- Validation (invalid values, missing required fields)

### Integration Tests

```go
func TestProfilePrecedence(t *testing.T) {
    // Setup: Create global config, profile, project config
    // Test: Flags override project override profile
    // Assert: Final config has expected values
}

func TestProjectConfigAutoDetection(t *testing.T) {
    // Setup: Create .gz-git.yaml in parent directory
    // Test: Run command in child directory
    // Assert: Project config is loaded
}
```

### User Acceptance Tests

- Create profile → Use in sync command
- Switch profiles → Verify different behavior
- Project config overrides profile
- Environment variable expansion works

## Migration Guide

### For Existing Users

**Before (v0.4.0)**:

```bash
gz-git sync from-forge --provider gitlab --base-url ... --token ...
```

**After (with profiles)**:

```bash
# One-time
gz-git config profile create work --provider gitlab --base-url ...
gz-git config profile use work

# Ongoing
gz-git sync from-forge --org backend
```

**Backward Compatibility**:

- ✅ All existing commands work without profiles
- ✅ Flags always override profiles
- ✅ No breaking changes

## Dependencies

- **Config library**: `github.com/spf13/viper` (YAML, env var expansion)
- **File system**: Standard library (`os`, `filepath`)
- **Home directory**: `os.UserConfigDir()` for `~/.config/gz-git/`

## Security Considerations

1. **Token storage**:

   - ⚠️ Tokens in plain text YAML (like Git config)
   - ✅ Recommend environment variables: `token: ${GITLAB_TOKEN}`
   - ✅ Document secure practices

1. **File permissions**:

   - Profile files: `0600` (user read/write only)
   - Config directory: `0700` (user access only)

1. **Environment variable expansion**:

   - ✅ Only expand `${VAR}` syntax
   - ❌ No shell command execution

## Future Enhancements

- [ ] Encrypted token storage (keyring integration)
- [ ] Profile import/export
- [ ] Profile sharing (team configs)
- [ ] Remote profiles (fetch from URL)
- [ ] Profile validation (`config profile validate`)

## References

- [PHASE8_OVERVIEW.md](PHASE8_OVERVIEW.md) - Overall Phase 8 plan
- [Git config precedence](https://git-scm.com/docs/git-config#_configuration_file) - Inspiration
- [AWS CLI profiles](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-profiles.html) - Similar pattern

______________________________________________________________________

**Version**: 1.0
**Last Updated**: 2026-01-16
**Author**: Design spec for implementation
