# Interactive Mode Design

## Overview

**Feature**: Guided workflows for common tasks
**Priority**: P2
**Phase**: 8.3
**Status**: Design

## Problem Statement

### Current Pain Points (v0.4.0)

**Problem 1: Steep learning curve**
```bash
# New user wants to sync from GitLab
# Must know: all flags, provider name, token location, SSH vs HTTPS

gz-git sync from-forge \
  --provider gitlab \
  --org devbox \
  --target ~/repos \
  --base-url https://gitlab.company.com \
  --token $GITLAB_TOKEN \
  --clone-proto ssh \
  --ssh-port 2224 \
  --include-subgroups \
  --subgroup-mode flat
```

**Issues**:
- ❌ Requires reading docs to know all flags
- ❌ Easy to make mistakes (typos, wrong values)
- ❌ No validation until command runs
- ❌ Unclear which flags are required vs optional

**Problem 2: Trial and error for complex workflows**
```bash
# User wants to clean up old branches
# Must: run cleanup, read output, manually select branches to delete
gz-git cleanup branch --type merged
# ... reads long output ...
gz-git cleanup branch --delete feature/old-1 feature/old-2 ...
# Repeat for 50 repos
```

## Goals

1. **Lower barriers**: New users can complete tasks without reading docs
2. **Guided workflows**: Step-by-step prompts for common tasks
3. **Validation**: Catch errors before execution
4. **Discoverability**: Learn available options through prompts
5. **Opt-in**: Interactive mode doesn't replace traditional CLI

## Design

### 1. Prompt Library Choice

**Decision**: survey (AlecAivazis/survey)

**Why**:
- ✅ Popular, well-maintained
- ✅ Rich prompt types (input, select, multiselect, confirm, password)
- ✅ Validation support
- ✅ Good UX (colors, icons, help text)

**Alternatives**:
- promptui: Less feature-rich
- go-prompt: More complex (autocomplete focus)
- bubbletea: Too heavy for simple wizards

### 2. Wizard Workflows

#### Workflow 1: Sync Setup Wizard

**Command**: `gz-git sync setup` or `gz-git sync from-forge --interactive`

```
$ gz-git sync setup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 🚀 Git Repository Sync Setup
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

? Select Git forge provider: (Use arrow keys)
  ❯ GitLab
    GitHub
    Gitea

? Enter organization/group name: devbox

? Is this a user account (not organization)? (y/N) N

? Enter API base URL (or press Enter for default): https://gitlab.company.com

? Enter API token: (will be hidden) ********************

? Clone protocol:
    SSH (recommended)
  ❯ HTTPS

? SSH port (0 for default 22): 2224

? Include subgroups? (GitLab only)
  ❯ Yes - Flat mode (group-subgroup-repo)
    Yes - Nested mode (group/subgroup/repo)
    No

? Target directory for cloned repositories: ~/repos
  ❯ ~/repos
    ~/projects
    ~/work
    Custom path...

? Additional options:
  [ ] Include archived repositories
  [x] Include private repositories (default)
  [ ] Include forked repositories

? Save as config file for future use? (Y/n) Y

? Config file path: sync.yaml

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 ✓ Configuration Summary
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Provider:       GitLab
Organization:   devbox
Base URL:       https://gitlab.company.com
Clone Protocol: SSH (port 2224)
Subgroups:      Yes (flat mode)
Target:         /home/user/repos
Config saved:   sync.yaml

? Ready to sync now? (Y/n) Y

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 🔄 Syncing repositories...
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✓ Found 25 repositories
⟳ Cloning: backend-api (1/25)
⟳ Cloning: frontend-web (2/25)
...

✓ Sync complete! 25 repositories synced successfully.

Next steps:
  • Run 'gz-git status' to check repo status
  • Run 'gz-git sync from-config -c sync.yaml' to re-sync later
  • See 'gz-git sync --help' for more options
```

**Implementation**:
```go
func runSyncSetup() error {
    // Provider selection
    provider := ""
    survey.Select{
        Message: "Select Git forge provider:",
        Options: []string{"GitLab", "GitHub", "Gitea"},
    }.AskOne(&provider)

    // Organization name
    org := ""
    survey.Input{
        Message: "Enter organization/group name:",
        Validate: survey.Required,
    }.AskOne(&org)

    // API token (hidden input)
    token := ""
    survey.Password{
        Message: "Enter API token:",
    }.AskOne(&token)

    // ... more prompts ...

    // Confirm and execute
    confirm := false
    survey.Confirm{
        Message: "Ready to sync now?",
        Default: true,
    }.AskOne(&confirm)

    if confirm {
        return executeSyncFromForge(opts)
    }
    return nil
}
```

#### Workflow 2: Branch Cleanup Wizard

**Command**: `gz-git cleanup branch --interactive`

```
$ gz-git cleanup branch --interactive

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 🧹 Branch Cleanup Wizard
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Scanning repositories for stale branches...

✓ Found 15 repositories with stale branches

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Repository: backend-api (5 stale branches)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

? Select branches to delete:
  [x] feature/old-login (merged to main, 30 days ago)
  [x] bugfix/temp-fix (merged to develop, 15 days ago)
  [ ] feature/wip (not merged, last commit 2 days ago) ⚠️
  [x] release/v1.0.0 (merged to main, 60 days ago)
  [ ] feature/important (not merged, last commit 1 day ago) ⚠️

  Space: toggle  a: select all merged  n: deselect all

? Delete 3 selected branches from backend-api? (y/N) y

✓ Deleted 3 branches from backend-api

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Repository: frontend-web (3 stale branches)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

? Select branches to delete:
  [x] hotfix/security-patch (merged to main, 45 days ago)
  [x] feature/redesign-old (merged to develop, 20 days ago)
  [ ] feature/new-ui (not merged, last commit 3 days ago) ⚠️

? Delete 2 selected branches from frontend-web? (y/N) y

✓ Deleted 2 branches from frontend-web

... (13 more repositories) ...

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 ✓ Cleanup Complete
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Total branches deleted: 23
Repositories cleaned: 15
Skipped (user choice): 7
```

**Implementation**:
```go
func runBranchCleanupWizard(repos []string) error {
    for _, repo := range repos {
        branches := findStaleBranches(repo)
        if len(branches) == 0 {
            continue
        }

        fmt.Printf("\nRepository: %s (%d stale branches)\n", repo, len(branches))

        // Multiselect for branch deletion
        selected := []string{}
        survey.MultiSelect{
            Message: "Select branches to delete:",
            Options: formatBranchOptions(branches),
            Default: preSelectMergedBranches(branches),
        }.AskOne(&selected)

        if len(selected) == 0 {
            continue
        }

        // Confirm deletion
        confirm := false
        survey.Confirm{
            Message: fmt.Sprintf("Delete %d selected branches from %s?", len(selected), repo),
            Default: false,
        }.AskOne(&confirm)

        if confirm {
            deleteBranches(repo, selected)
        }
    }
    return nil
}
```

#### Workflow 3: Profile Creation Wizard

**Command**: `gz-git config profile create --interactive`

```
$ gz-git config profile create --interactive

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 ⚙️  Create Configuration Profile
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

? Profile name: work

? Description (optional): Work GitLab repositories

? Select Git forge provider:
  ❯ GitLab
    GitHub
    Gitea

? Base URL: https://gitlab.company.com

? API token storage:
  ❯ Environment variable (recommended)
    Store in profile (plain text) ⚠️

? Environment variable name: WORK_GITLAB_TOKEN

? Clone protocol:
  ❯ SSH
    HTTPS

? SSH port (0 for default): 2224

? Default parallelism (concurrent operations): 10

? Include subgroups by default? (Y/n) Y

? Subgroup mode:
  ❯ Flat (group-subgroup-repo)
    Nested (group/subgroup/repo)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 ✓ Profile Configuration
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Name:           work
Provider:       GitLab
Base URL:       https://gitlab.company.com
Token:          $WORK_GITLAB_TOKEN (env var)
Clone Protocol: SSH (port 2224)
Parallelism:    10
Subgroups:      Yes (flat mode)

? Save profile? (Y/n) Y

✓ Profile 'work' created at ~/.config/gz-git/profiles/work.yaml

? Set as active profile? (Y/n) Y

✓ Profile 'work' is now active

Try it:
  gz-git sync from-forge --org backend
  gz-git status
```

### 3. Prompt Types

#### Input (Text Entry)
```go
survey.Input{
    Message: "Enter organization name:",
    Help: "The organization or group name on the Git forge",
    Validate: func(val interface{}) error {
        if str := val.(string); len(str) < 1 {
            return errors.New("organization name is required")
        }
        return nil
    },
}
```

#### Select (Single Choice)
```go
survey.Select{
    Message: "Select provider:",
    Options: []string{"GitLab", "GitHub", "Gitea"},
    Help: "Git forge hosting your repositories",
    Default: "GitLab",
}
```

#### MultiSelect (Multiple Choices)
```go
survey.MultiSelect{
    Message: "Select branches to delete:",
    Options: branchNames,
    Help: "Space to toggle, Enter to confirm",
    PageSize: 10,
}
```

#### Confirm (Yes/No)
```go
survey.Confirm{
    Message: "Delete 5 branches?",
    Default: false,
    Help: "This action cannot be undone",
}
```

#### Password (Hidden Input)
```go
survey.Password{
    Message: "Enter API token:",
    Help: "Find your token at https://gitlab.com/-/profile/personal_access_tokens",
}
```

### 4. Validation

**Real-time validation**:
```go
survey.Input{
    Message: "SSH port:",
    Validate: func(val interface{}) error {
        str := val.(string)
        port, err := strconv.Atoi(str)
        if err != nil {
            return errors.New("port must be a number")
        }
        if port < 0 || port > 65535 {
            return errors.New("port must be between 0 and 65535")
        }
        return nil
    },
}
```

**API validation** (check if token works):
```go
func validateToken(provider, baseURL, token string) error {
    // Try API call with token
    client := createClient(provider, baseURL, token)
    _, err := client.CurrentUser()
    if err != nil {
        return fmt.Errorf("token validation failed: %w", err)
    }
    return nil
}

// Use in wizard
token := ""
survey.Password{
    Message: "Enter API token:",
}.AskOne(&token)

fmt.Println("Validating token...")
if err := validateToken(provider, baseURL, token); err != nil {
    fmt.Println("❌ Invalid token:", err)
    // Prompt again or abort
}
fmt.Println("✓ Token valid")
```

### 5. Help Text & Context

**Inline help**:
```go
survey.Select{
    Message: "Clone protocol:",
    Options: []string{"SSH", "HTTPS"},
    Help: `SSH: Requires SSH key setup, works with custom ports
HTTPS: Username/password or token, standard port 443`,
}
```

**Dynamic help based on choice**:
```go
if provider == "GitLab" {
    survey.Confirm{
        Message: "Include subgroups?",
        Help: "GitLab groups can have nested subgroups. " +
              "Flat mode: group-subgroup-repo, " +
              "Nested mode: group/subgroup/repo",
    }
}
```

### 6. Error Handling

**Graceful errors with recovery**:
```go
func runWizard() error {
    var qs []*survey.Question

    // Add questions...

    answers := struct {
        Provider string
        Org      string
        Token    string
    }{}

    err := survey.Ask(qs, &answers)
    if err != nil {
        if err == terminal.InterruptErr {
            fmt.Println("\n✗ Wizard cancelled")
            return nil
        }
        return fmt.Errorf("wizard error: %w", err)
    }

    // Continue with answers...
}
```

## Implementation Plan

### Week 1: Foundation
- [ ] **Day 1**: Survey library integration
  - Add dependency
  - Basic prompt examples
  - Styling/theming

- [ ] **Day 2**: Sync setup wizard
  - Provider selection
  - Basic prompts (org, token, target)
  - Execute sync from answers

- [ ] **Day 3**: Validation
  - Input validation (required, format)
  - API token validation
  - Error messages

### Week 2: Additional Wizards
- [ ] **Day 1-2**: Branch cleanup wizard
  - Scan repos for stale branches
  - MultiSelect prompt
  - Batch deletion with confirmation

- [ ] **Day 3-4**: Profile creation wizard
  - All profile fields
  - Environment variable option
  - Save and activate

- [ ] **Day 5**: Polish
  - Help text
  - Colors/icons
  - Loading indicators

## Command Integration

**Two approaches**:

### Approach 1: --interactive flag
```bash
gz-git sync from-forge --interactive
gz-git cleanup branch --interactive
gz-git config profile create --interactive
```

### Approach 2: Dedicated wizard commands
```bash
gz-git sync setup        # Wizard for sync setup
gz-git cleanup wizard    # Wizard for cleanup
gz-git config wizard     # Wizard for profile creation
```

**Recommendation**: Use Approach 2 (dedicated commands)
- ✅ Clearer intent
- ✅ Separate help text
- ✅ Can be shorter names (`setup` vs `from-forge --interactive`)

## Testing Strategy

### Unit Tests
```go
func TestSyncSetupWizard(t *testing.T) {
    // Mock survey prompts
    // Verify correct options generated
}
```

### Integration Tests
```go
func TestWizardFlow(t *testing.T) {
    // Simulate user input (automated)
    // Verify correct command execution
}
```

### Manual Testing
- [ ] Complete wizard without errors
- [ ] Test all validation rules
- [ ] Test error recovery (Ctrl+C, invalid input)
- [ ] Test on different terminals

## Dependencies

```go
require (
    github.com/AlecAivazis/survey/v2 v2.3.7
)
```

**Note**: survey already handles terminal detection, colors, etc.

## User Experience Principles

1. **Progressive disclosure**: Show advanced options only if needed
2. **Sensible defaults**: Pre-select recommended options
3. **Clear help**: Explain what each option does
4. **Forgiving**: Allow correction of mistakes
5. **Fast path**: Skip wizard with flags for power users

## Accessibility

- [ ] Keyboard-only (no mouse required)
- [ ] Screen reader compatible (text-based)
- [ ] Clear visual hierarchy
- [ ] Color optional (fallback to symbols)

## Future Enhancements

- [ ] Save wizard answers for repeat use
- [ ] Template wizards (common setups)
- [ ] Multi-step wizards (complex workflows)
- [ ] Wizard history (repeat last wizard)
- [ ] Custom wizards (user-defined)

## References

- [survey Documentation](https://github.com/AlecAivazis/survey)
- [aws configure](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-wizard.html) - Similar wizard pattern
- [helm create](https://helm.sh/docs/helm/helm_create/) - Interactive chart creation

---

**Version**: 1.0
**Last Updated**: 2026-01-16
**Author**: Design spec for implementation
