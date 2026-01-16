# Workspace Config Design - Recursive Hierarchical Configuration

**Feature**: 재귀적 계층 구조로 무한 중첩 설정 지원
**Priority**: P1 (사용자 요청)
**Status**: Design Complete - Recursive Approach
**Date**: 2026-01-16
**Updated**: 2026-01-16 (재귀적 구조로 대폭 간소화)

---

## 📋 요구사항 분석

### 사용자 요청

> 워크스테이션 전체의 git을 묶어서 관리하는 설정파일을 ~/.gz-git-config.yaml을 만들고
> 이것을 기반으로 ~/mydevbox, ~/mywork 등에 각각의 통합 config
> 그리고 각각의 프로젝트의 config를 그 하위에 놓는식으로 관리

> **각 설정파일은 하위 설정파일의 경로를 확인할 수 있어야 한다.**
> - 하위 경로가 **단순히 git 저장소**일 수도 있고
> - 하위 경로가 **또 다른 설정파일**을 가진 디렉토리일 수도 있어야 한다
> - 설정파일에 **하위 경로의 설정파일명도 명시** 가능해야 함
> - 설정파일명이 없는 경우 **기본 파일명** 사용

### 핵심 인사이트 💡

**기존 설계의 문제점**: WorkstationConfig, WorkspaceConfig, ProjectConfig 3가지 타입 → 복잡함

**새로운 접근**: **단 하나의 Config 타입**이 **재귀적으로 중첩**되는 구조
- ✅ 단순함: 하나의 타입만 존재
- ✅ 유연함: 원하는 만큼 깊이 중첩 가능
- ✅ 일관성: 모든 레벨에서 동일한 로직

### 기존 한계점

**현재 2-Tier 시스템** (Phase 8.2):
```
~/.config/gz-git/         ← Global profiles
    ↓
~/project/.gz-git.yaml    ← Project config
```

**문제점**:
- ❌ 계층적 설정 미지원
- ❌ ~/mydevbox, ~/mywork 같은 워크스페이스별 설정 불가능
- ❌ 하위 경로 명시적 관리 불가능

---

## 🎯 설계 목표

### 재귀적 계층 구조 (Recursive Hierarchy)

**핵심**: 모든 설정파일이 **같은 구조**를 가지고, **무한히 중첩** 가능

```
~/.gz-git-config.yaml              ← Config (최상위)
    ↓ children
~/mydevbox/.gz-git.yaml            ← Config (중첩 1)
    ↓ children
~/mydevbox/project/.gz-git.yaml    ← Config (중첩 2)
    ↓ children
~/mydevbox/project/submodule/...   ← Config (중첩 3, 무한 중첩 가능!)
```

**모든 레벨에서 동일한 파일 형식**: `.gz-git.yaml`

### Precedence (재귀적 우선순위)

```
1. Command flags (--provider gitlab)                    ← 최고 우선순위
2. 현재 경로의 config (.gz-git.yaml)
3. 부모 경로의 config (../.gz-git.yaml)
4. 조부모 경로의 config (../../.gz-git.yaml)
   ... (재귀적으로 최상위까지)
N. Active profile (~/.config/gz-git/profiles/work.yaml)
N+1. Global config (~/.config/gz-git/config.yaml)
N+2. Built-in defaults                                   ← 최저 우선순위
```

**단순화**: 깊이에 상관없이 **자식이 부모를 override**하는 일관된 규칙

---

## 📁 파일 구조 (재귀적 설계)

### 핵심 개념: 단일 통합 Config

**모든 레벨에서 동일한 구조**:
1. **자신의 설정**: profile, parallel, sync, branch 등
2. **children**: 하위 경로 목록 (재귀적으로 같은 구조 반복)
3. **type**: 하위 경로 타입 (`git` 또는 `config`)

### 통합 Config 파일 (.gz-git.yaml)

**모든 계층에서 동일한 파일 형식 사용**

```yaml
# ~/.gz-git-config.yaml (최상위 설정)
# === 이 레벨의 설정 ===
parallel: 5
cloneProto: ssh
format: default

# === 하위 경로들 (재귀!) ===
children:
  # Child 1: 설정파일이 있는 디렉토리
  - path: ~/mydevbox
    type: config              # config = 설정파일 있음 (재귀적 중첩)
    configFile: .gz-git.yaml  # 기본값이므로 생략 가능
    profile: opensource       # Inline override
    parallel: 10

  # Child 2: 설정파일이 있는 디렉토리 (커스텀 파일명)
  - path: ~/mywork
    type: config
    configFile: .work-config.yaml  # 커스텀 파일명!
    profile: work

  # Child 3: 단순 Git 저장소 (설정파일 없음)
  - path: ~/single-repo
    type: git  # git = 설정파일 없는 Git 저장소
    profile: personal

metadata:
  name: workstation
  owner: archmagece
```

```yaml
# ~/mydevbox/.gz-git.yaml (중첩된 설정 - Level 1)
# === 이 레벨의 설정 ===
profile: opensource

sync:
  strategy: reset
  parallel: 10
  maxRetries: 3

branch:
  defaultBranch: main

# === 하위 경로들 (재귀!) ===
children:
  # Case 1: 단순 Git 저장소
  - path: gzh-cli
    type: git

  - path: gzh-cli-core
    type: git

  # Case 2: 설정파일이 있는 프로젝트
  - path: gzh-cli-gitforge
    type: config
    # configFile 생략 → 기본값 .gz-git.yaml 사용
    sync:
      strategy: pull  # Inline override

  # Case 3: 커스텀 설정파일명
  - path: special-project
    type: config
    configFile: .special-config.yaml  # 커스텀 파일명
    parallel: 1

  # Case 4: 또 다른 중첩 (무한 중첩 가능!)
  - path: nested-workspace
    type: config
    profile: sub-profile

metadata:
  name: mydevbox
  type: development
```

```yaml
# ~/mydevbox/gzh-cli-gitforge/.gz-git.yaml (중첩 Level 2)
# === 이 레벨의 설정 ===
sync:
  strategy: pull
  parallel: 3

branch:
  protectedBranches: [main, develop, release/*]

# === 하위 경로들 (재귀!) ===
children:
  # Submodule 1
  - path: vendor/external-lib
    type: git
    sync:
      strategy: skip

  # Submodule 2 (또 다른 설정파일)
  - path: modules/plugin
    type: config
    sync:
      strategy: reset

metadata:
  name: gzh-cli-gitforge
  owner: gizzahub
```

### 핵심 장점

1. **단순함**: 모든 파일이 `.gz-git.yaml` (통일된 형식)
2. **재귀성**: 같은 구조가 무한히 중첩 가능
3. **타입 단순화**: `git` vs `config` 두 가지만
   - `git`: 설정파일 없는 Git 저장소
   - `config`: 설정파일 있음 (재귀적 중첩 가능)
4. **유연함**: `configFile`로 커스텀 파일명 지정 가능

---

---

## 🏗️ 재귀적 데이터 구조

### 핵심: 단일 Config 타입 (재귀적)

```go
// Config represents a hierarchical configuration that can be nested recursively
// Used at ALL levels: workstation, workspace, project, submodule, etc.
type Config struct {
    // === This level's settings ===
    Profile  string `yaml:"profile,omitempty"`
    Parallel int    `yaml:"parallel,omitempty"`

    // Command-specific overrides
    Sync   *SyncConfig   `yaml:"sync,omitempty"`
    Branch *BranchConfig `yaml:"branch,omitempty"`
    Fetch  *FetchConfig  `yaml:"fetch,omitempty"`
    Pull   *PullConfig   `yaml:"pull,omitempty"`
    Push   *PushConfig   `yaml:"push,omitempty"`

    // === Children (recursive!) ===
    Children []ChildEntry `yaml:"children,omitempty"`

    // === Metadata ===
    Metadata *Metadata `yaml:"metadata,omitempty"`
}

// ChildEntry represents a child path (config or git repo)
type ChildEntry struct {
    // Path is the relative or absolute path to the child
    Path string `yaml:"path"`

    // Type specifies what kind of child this is
    // Values: "config" (has config file), "git" (plain repo)
    Type ChildType `yaml:"type"`

    // ConfigFile specifies the config file name (optional)
    // Default: ".gz-git.yaml"
    // Only used when Type == "config"
    ConfigFile string `yaml:"configFile,omitempty"`

    // Inline overrides (optional)
    Profile  string      `yaml:"profile,omitempty"`
    Parallel int         `yaml:"parallel,omitempty"`
    Sync     *SyncConfig `yaml:"sync,omitempty"`
    Branch   *BranchConfig `yaml:"branch,omitempty"`
}

// ChildType represents the type of child entry
type ChildType string

const (
    ChildTypeConfig ChildType = "config" // Has config file (recursive)
    ChildTypeGit    ChildType = "git"    // Plain Git repo (no config)
)

// DefaultConfigFile returns the default config file name
func (t ChildType) DefaultConfigFile() string {
    if t == ChildTypeConfig {
        return ".gz-git.yaml"
    }
    return "" // Git repos don't have config files
}
```

### 간소화된 설계

**Before (복잡함)**:
- `WorkstationConfig` (174 lines)
- `WorkspaceConfig` (182 lines)
- `ProjectConfig` (167 lines)
- 3가지 타입, 3가지 파일명, 복잡한 로직

**After (단순함)**:
- `Config` (하나의 타입)
- `.gz-git.yaml` (하나의 파일명, 커스텀 가능)
- 재귀적 로딩 (모든 레벨에서 동일한 로직)

---

## 🔍 재귀적 로딩 알고리즘

### 핵심: 단일 재귀 함수

**모든 레벨에서 동일한 로직 사용**

```go
// LoadConfigRecursive loads a config file and recursively loads all children
// This function works at ANY level (workstation, workspace, project, etc.)
func LoadConfigRecursive(path string, configFile string) (*Config, error) {
    // 1. Load this level's config file
    configPath := filepath.Join(path, configFile)
    data, err := os.ReadFile(configPath)
    if err != nil {
        return nil, fmt.Errorf("failed to read config %s: %w", configPath, err)
    }

    var config Config
    if err := yaml.Unmarshal(data, &config); err != nil {
        return nil, fmt.Errorf("failed to parse config %s: %w", configPath, err)
    }

    // 2. Recursively load children
    for i := range config.Children {
        child := &config.Children[i]

        // Resolve child path (handle ~, relative paths)
        childPath := resolvePath(path, child.Path)

        if child.Type == ChildTypeConfig {
            // Child has a config file - recurse!
            childConfigFile := child.ConfigFile
            if childConfigFile == "" {
                childConfigFile = child.Type.DefaultConfigFile() // ".gz-git.yaml"
            }

            // RECURSIVE CALL!
            childConfig, err := LoadConfigRecursive(childPath, childConfigFile)
            if err != nil {
                // Config file not found is OK (use inline overrides only)
                log.Debugf("Child config not found (using inline): %s", err)
                continue
            }

            // Merge inline overrides into loaded config
            mergeInlineOverrides(childConfig, child)

            // Store the loaded child config (could be used for validation)
            // child.LoadedConfig = childConfig
        } else if child.Type == ChildTypeGit {
            // Plain git repo - no config to load
            // Just validate that the path exists and is a git repo
            if !isGitRepo(childPath) {
                return nil, fmt.Errorf("child path is not a git repo: %s", childPath)
            }
        }
    }

    return &config, nil
}

// resolvePath resolves a path relative to a parent directory
// Handles: ~, relative paths (./foo), absolute paths (/foo)
func resolvePath(parentPath string, childPath string) string {
    if strings.HasPrefix(childPath, "~/") {
        home, _ := os.UserHomeDir()
        return filepath.Join(home, childPath[2:])
    }
    if filepath.IsAbs(childPath) {
        return childPath
    }
    return filepath.Join(parentPath, childPath)
}

// mergeInlineOverrides applies inline overrides from ChildEntry to loaded Config
func mergeInlineOverrides(config *Config, entry *ChildEntry) {
    if entry.Profile != "" {
        config.Profile = entry.Profile
    }
    if entry.Parallel > 0 {
        config.Parallel = entry.Parallel
    }
    if entry.Sync != nil {
        config.Sync = entry.Sync
    }
    if entry.Branch != nil {
        config.Branch = entry.Branch
    }
}
```

### Discovery Modes (Simple)

```go
// DiscoveryMode controls how children are discovered
type DiscoveryMode string

const (
    ExplicitMode DiscoveryMode = "explicit" // Use children only
    AutoMode     DiscoveryMode = "auto"     // Scan directories
    HybridMode   DiscoveryMode = "hybrid"   // DEFAULT: children if defined, else scan
)

// LoadChildren loads children based on discovery mode
func LoadChildren(path string, config *Config, mode DiscoveryMode) error {
    switch mode {
    case ExplicitMode:
        // Already loaded by LoadConfigRecursive - nothing to do
        return nil

    case AutoMode:
        // Scan directory and add discovered repos to config.Children
        return autoDiscoverAndAppend(path, config)

    case HybridMode:
        // Use explicit children if defined, otherwise auto-discover
        if len(config.Children) > 0 {
            return nil // Use explicit
        }
        return autoDiscoverAndAppend(path, config)
    }
    return nil
}

// autoDiscoverAndAppend scans directory and appends discovered repos to config
func autoDiscoverAndAppend(path string, config *Config) error {
    entries, err := os.ReadDir(path)
    if err != nil {
        return err
    }

    for _, entry := range entries {
        if !entry.IsDir() {
            continue
        }

        childPath := filepath.Join(path, entry.Name())

        // Check if it has a config file
        if hasFile(childPath, ".gz-git.yaml") {
            config.Children = append(config.Children, ChildEntry{
                Path: childPath,
                Type: ChildTypeConfig,
            })
            continue
        }

        // Check if it's a git repo
        if isGitRepo(childPath) {
            config.Children = append(config.Children, ChildEntry{
                Path: childPath,
                Type: ChildTypeGit,
            })
        }
    }

    return nil
}
```

### 간소화 요약

**Before (복잡함)**:
- `LoadExplicitChildren()` (45 lines)
- `AutoDiscoverChildren()` (43 lines)
- `LoadChildrenWithMode()` (20 lines)
- 다양한 Child 타입 처리

**After (단순함)**:
- `LoadConfigRecursive()` (재귀 한 번!)
- `autoDiscoverAndAppend()` (auto mode 지원)
- 모든 레벨에서 동일한 로직

---

## 💻 구현 상태 (업데이트)

2. **Config Discovery** (`pkg/config/workspace.go`)
   ```go
   func FindWorkstationConfig() (string, error)
   func FindWorkspaceConfig() (string, error)
   func FindAllConfigs() (workstation, workspace, project string, err error)
   ```

3. **Manager Extensions** (`pkg/config/manager.go`)
   ```go
   func LoadWorkstationConfig() (*WorkstationConfig, error)
   func LoadWorkspaceConfig() (*WorkspaceConfig, error)
   func SaveWorkstationConfig(config *WorkstationConfig) error
   func SaveWorkspaceConfig(config *WorkspaceConfig) error
   ```

### ⏸️ 진행 중

4. **Loader Update** - 7-layer precedence 구현
5. **CLI Commands** - workspace init, show 명령어
6. **Tests** - 계층 config 테스트

---

## 🔄 Precedence 알고리즘

### 업데이트된 Load() 함수

```go
func (l *ConfigLoader) Load() error {
    // 1. Load workstation config (NEW!)
    workstationConfig, err := l.manager.LoadWorkstationConfig()
    if err != nil {
        return fmt.Errorf("failed to load workstation config: %w", err)
    }
    l.workstationConfig = workstationConfig

    // 2. Load workspace config (NEW!)
    workspaceConfig, err := l.manager.LoadWorkspaceConfig()
    if err != nil {
        return fmt.Errorf("failed to load workspace config: %w", err)
    }
    l.workspaceConfig = workspaceConfig

    // 3. Load global config
    globalConfig, err := l.manager.LoadGlobalConfig()
    if err != nil {
        return fmt.Errorf("failed to load global config: %w", err)
    }
    l.globalConfig = globalConfig

    // 4. Determine active profile
    // Priority: workspace config > workstation mapping > global active profile
    activeProfileName := l.determineActiveProfile(workspaceConfig, workstationConfig, globalConfig)

    // 5. Load active profile
    if activeProfileName != "" && l.manager.ProfileExists(activeProfileName) {
        activeProfile, err := l.manager.LoadProfile(activeProfileName)
        if err != nil {
            return fmt.Errorf("failed to load active profile '%s': %w", activeProfileName, err)
        }
        l.activeProfile = activeProfile
    }

    // 6. Load project config
    projectConfig, err := l.manager.LoadProjectConfig()
    if err != nil {
        return fmt.Errorf("failed to load project config: %w", err)
    }
    l.projectConfig = projectConfig

    return nil
}
```

### 업데이트된 ResolveConfig() 함수

```go
func (l *ConfigLoader) ResolveConfig(flags map[string]interface{}) (*EffectiveConfig, error) {
    effective := &EffectiveConfig{
        Sources: make(map[string]string),
    }

    // Layer 1: Built-in defaults
    l.applyDefaults(effective)

    // Layer 2: Global config
    if l.globalConfig != nil {
        l.applyGlobalConfig(effective)
    }

    // Layer 3: Active profile
    if l.activeProfile != nil {
        l.applyProfile(effective)
    }

    // Layer 4: Workstation config (NEW!)
    if l.workstationConfig != nil {
        l.applyWorkstationConfig(effective)
    }

    // Layer 5: Workspace config (NEW!)
    if l.workspaceConfig != nil {
        l.applyWorkspaceConfig(effective)
    }

    // Layer 6: Project config
    if l.projectConfig != nil {
        l.applyProjectConfig(effective)
    }

    // Layer 7: Command flags (highest priority)
    l.applyFlags(effective, flags)

    return effective, nil
}
```

---

## 🎨 사용 시나리오

### 시나리오 1: 워크스테이션 초기 설정 (명시적 children)

```bash
# 워크스테이션 전체 기본값 설정 + 명시적 children 정의
cat > ~/.gz-git-config.yaml <<EOF
defaults:
  parallel: 5
  cloneProto: ssh
  format: default

# 명시적 하위 워크스페이스 정의 (NEW!)
children:
  - path: ~/mydevbox
    type: workspace
    configFile: .gz-git-workspace.yaml  # 기본 파일명
    profile: opensource
    parallel: 10

  - path: ~/mywork
    type: workspace
    configFile: .work-config.yaml  # 커스텀 파일명!
    profile: work
    parallel: 3

  - path: ~/personal
    type: workspace
    # configFile 생략 → 기본값 .gz-git-workspace.yaml 사용
    profile: personal

  - path: ~/single-repo
    type: git  # 단일 Git 저장소 (workspace 아님)
    profile: personal
EOF

# 이제 디렉토리마다 자동으로 프로필이 선택됨!
cd ~/mydevbox/any-project
gz-git config show
# → profile: opensource (from workstation → workspace)

cd ~/mywork/company-repo
gz-git config show
# → profile: work (from workstation → workspace)
# → workspace config file: .work-config.yaml (커스텀)
```

**개선점**:
- ✅ **명시적 children 정의**: 어떤 워크스페이스가 있는지 명확
- ✅ **커스텀 파일명 지원**: ~/mywork는 `.work-config.yaml` 사용
- ✅ **타입 구분**: workspace vs git 구분
- ✅ **기본값 지원**: configFile 생략 시 기본 파일명 사용

### 시나리오 2: 워크스페이스 설정 (명시적 children)

```bash
# mydevbox 워크스페이스 전체 설정 + 명시적 children 정의
cd ~/mydevbox
cat > .gz-git-workspace.yaml <<EOF
profile: opensource

sync:
  strategy: reset
  parallel: 10
  maxRetries: 3

branch:
  defaultBranch: main

# 명시적 하위 프로젝트 정의 (NEW!)
children:
  # Case 1: Git 저장소만 (설정파일 없음)
  - path: gzh-cli
    type: git

  - path: gzh-cli-core
    type: git

  # Case 2: 프로젝트 + 설정파일 (기본 파일명)
  - path: gzh-cli-gitforge
    type: project
    # configFile 생략 → .gz-git.yaml 사용
    sync:
      strategy: pull  # 이 프로젝트만 pull 사용

  # Case 3: 프로젝트 + 커스텀 설정파일명
  - path: special-project
    type: project
    configFile: .special-config.yaml  # 커스텀 파일명!
    parallel: 1

  # Case 4: Nested workspace
  - path: subworkspace
    type: workspace
    configFile: .gz-git-workspace.yaml
    profile: sub-profile

metadata:
  workspace: mydevbox
  type: development
  owner: archmagece
EOF

# 이제 mydevbox 내 모든 프로젝트는 이 설정을 공유
cd ~/mydevbox/gzh-cli
gz-git status
# → parallel: 10, strategy: reset (from workspace)
# → type: git (설정파일 없음)

cd ~/mydevbox/gzh-cli-gitforge
gz-git status
# → parallel: 10 (from workspace)
# → strategy: pull (from child override)
# → type: project (설정파일 .gz-git.yaml 존재)

cd ~/mydevbox/special-project
gz-git status
# → parallel: 1 (from child override)
# → configFile: .special-config.yaml (커스텀)
```

**개선점**:
- ✅ **명시적 프로젝트 목록**: 어떤 프로젝트가 있는지 명확
- ✅ **타입별 처리**: git(설정 없음) vs project(설정 있음)
- ✅ **Inline override**: children에 직접 sync, parallel 지정
- ✅ **Nested workspace 지원**: 워크스페이스 안에 워크스페이스 가능

### 시나리오 3: 프로젝트별 override + submodules

```bash
# 특정 프로젝트만 다른 설정 + submodule 관리
cd ~/mydevbox/gzh-cli-gitforge
cat > .gz-git.yaml <<EOF
sync:
  strategy: pull    # workspace의 reset 대신 pull 사용
  parallel: 3       # workspace의 10 대신 3 사용

branch:
  protectedBranches: [main, develop, release/*]

# 하위 Git 저장소 (submodules) 관리 (NEW!)
children:
  # Submodule 1: Skip sync
  - path: vendor/external-lib
    type: git
    sync:
      strategy: skip

  # Submodule 2: 설정파일 있음
  - path: modules/plugin
    type: project
    configFile: .gz-git.yaml
    sync:
      strategy: reset

metadata:
  project: gzh-cli-gitforge
  owner: gizzahub
EOF

# 이 프로젝트만 pull 전략 사용
gz-git sync from-config -c sync.yaml
# → strategy: pull, parallel: 3 (from project, overrides workspace)

# Submodule도 자동으로 처리됨
gz-git status
# → vendor/external-lib: skipped (from child override)
# → modules/plugin: reset (from child override)
```

**개선점**:
- ✅ **Submodule 관리**: Git submodule도 계층에 포함
- ✅ **Submodule별 전략**: 각 submodule마다 다른 sync 전략 지정
- ✅ **Nested repo 지원**: 프로젝트 내부의 Git 저장소 관리

### 시나리오 4: Config 계층 확인 (with children)

```bash
cd ~/mydevbox/gzh-cli-gitforge
gz-git config show --sources

# 출력:
# Configuration Hierarchy:
#   Workstation: ~/.gz-git-config.yaml
#     Children (explicit):
#       - ~/mydevbox (workspace, .gz-git-workspace.yaml)
#       - ~/mywork (workspace, .work-config.yaml)
#       - ~/personal (workspace, .gz-git-workspace.yaml)
#
#   Workspace:   ~/mydevbox/.gz-git-workspace.yaml
#     Children (explicit):
#       - gzh-cli (git, no config)
#       - gzh-cli-gitforge (project, .gz-git.yaml)
#       - special-project (project, .special-config.yaml)
#
#   Project:     ~/mydevbox/gzh-cli-gitforge/.gz-git.yaml
#     Children (explicit):
#       - vendor/external-lib (git, skip)
#       - modules/plugin (project, .gz-git.yaml)
#
#   Profile:     opensource (from workspace)
#
# Effective Config:
#   Provider: gitlab (from profile:opensource)
#   Parallel: 3 (from project, overrides workspace:10)
#   Strategy: pull (from project, overrides workspace:reset)
#   CloneProto: ssh (from workstation defaults)
```

### 시나리오 5: Auto-Discovery vs Explicit Children

```bash
# Auto-Discovery Mode (기존 방식)
cd ~/mydevbox
gz-git status --discovery-mode auto
# → 디렉토리를 스캔하여 모든 git repo 자동 탐색
# → .gz-git-workspace.yaml, .gz-git.yaml 자동 탐지
# → children 정의 무시

# Explicit Mode (명시적 children만 사용)
cd ~/mydevbox
gz-git status --discovery-mode explicit
# → children에 정의된 경로만 탐색
# → gzh-cli, gzh-cli-gitforge, special-project만 처리
# → 정의되지 않은 디렉토리는 무시

# Hybrid Mode (기본값)
cd ~/mydevbox
gz-git status --discovery-mode hybrid
# → children 정의되어 있으면 explicit mode
# → children 없으면 auto-discovery mode
```

**사용 케이스**:
- **Explicit Mode**: 일부 프로젝트만 선택적으로 관리
- **Auto Mode**: 모든 Git 저장소 자동 탐지
- **Hybrid Mode**: 유연하게 두 가지 방식 혼용

### 시나리오 6: 마이그레이션 (2-tier → 3-tier)

```bash
# Step 1: 기존 2-tier 시스템 확인
cd ~/mydevbox/gzh-cli-gitforge
gz-git config show
# → profile: work (from global active profile)
# → parallel: 5 (from built-in defaults)

# Step 2: Workstation config 생성
cat > ~/.gz-git-config.yaml <<EOF
defaults:
  parallel: 5
  cloneProto: ssh

children:
  - path: ~/mydevbox
    type: workspace
    profile: opensource
EOF

# Step 3: Workspace config 생성
cd ~/mydevbox
cat > .gz-git-workspace.yaml <<EOF
profile: opensource

sync:
  strategy: reset
  parallel: 10

children:
  - path: gzh-cli-gitforge
    type: project
    sync:
      strategy: pull
EOF

# Step 4: 확인
cd ~/mydevbox/gzh-cli-gitforge
gz-git config show --sources
# → profile: opensource (from workspace)
# → parallel: 10 (from workspace)
# → strategy: pull (from workspace child override)

# 기존 2-tier는 여전히 작동!
cd ~/other-project  # workstation/workspace config 없는 경로
gz-git config show
# → profile: work (from global active profile)
# → parallel: 5 (from built-in defaults)
```

**호환성**:
- ✅ **기존 시스템 유지**: workstation/workspace config 없으면 기존대로 동작
- ✅ **점진적 마이그레이션**: 필요한 계층만 추가
- ✅ **Zero breaking changes**: 모든 기존 명령어 그대로 작동

---

## 🔨 CLI 명령어 추가

### Workstation Config

```bash
# Initialize workstation config
gz-git config init --workstation
# → Creates ~/.gz-git-config.yaml with defaults and children template

# Show workstation config
gz-git config show --workstation
# → Display workstation config with children list

# Edit workstation config
gz-git config edit --workstation
# → Open ~/.gz-git-config.yaml in $EDITOR

# Add workspace to workstation (NEW!)
gz-git config workstation add-workspace ~/mydevbox \
  --profile opensource \
  --parallel 10 \
  --config-file .gz-git-workspace.yaml  # Optional, defaults to .gz-git-workspace.yaml

# Add single git repo to workstation (NEW!)
gz-git config workstation add-workspace ~/single-repo \
  --type git \
  --profile personal

# List all workspaces in workstation (NEW!)
gz-git config workstation list
# Output:
#   ~/mydevbox (workspace, .gz-git-workspace.yaml, profile: opensource)
#   ~/mywork (workspace, .work-config.yaml, profile: work)
#   ~/single-repo (git, profile: personal)

# Remove workspace from workstation (NEW!)
gz-git config workstation remove-workspace ~/mydevbox
```

### Workspace Config

```bash
# Initialize workspace config (in current directory)
cd ~/mydevbox
gz-git config init --workspace
# → Creates .gz-git-workspace.yaml with defaults and children template

# Initialize with custom file name (NEW!)
gz-git config init --workspace --config-file .custom-workspace.yaml

# Show workspace config
gz-git config show --workspace
# → Display workspace config with children list

# Set workspace profile
gz-git config workspace set-profile opensource

# Add project to workspace (NEW!)
cd ~/mydevbox
gz-git config workspace add-child gzh-cli-gitforge \
  --type project \
  --config-file .gz-git.yaml \
  --profile opensource

# Add git repo to workspace (NEW!)
gz-git config workspace add-child gzh-cli \
  --type git

# Add nested workspace (NEW!)
gz-git config workspace add-child subworkspace \
  --type workspace \
  --config-file .gz-git-workspace.yaml \
  --profile sub-profile

# List all children in workspace (NEW!)
gz-git config workspace list
# Output:
#   gzh-cli (git, no config)
#   gzh-cli-gitforge (project, .gz-git.yaml)
#   special-project (project, .special-config.yaml)
#   subworkspace (workspace, .gz-git-workspace.yaml)

# Remove child from workspace (NEW!)
gz-git config workspace remove-child gzh-cli
```

### Project Config (with children)

```bash
# Add submodule to project (NEW!)
cd ~/mydevbox/gzh-cli-gitforge
gz-git config project add-child vendor/external-lib \
  --type git \
  --sync-strategy skip

# Add nested repo with config (NEW!)
gz-git config project add-child modules/plugin \
  --type project \
  --config-file .gz-git.yaml \
  --sync-strategy reset

# List all children in project (NEW!)
gz-git config project list
# Output:
#   vendor/external-lib (git, sync: skip)
#   modules/plugin (project, .gz-git.yaml, sync: reset)
```

### Hierarchy View

```bash
# Show all config files in hierarchy
gz-git config hierarchy

# Output:
# Config Hierarchy (highest to lowest priority):
#   1. Command flags
#   2. Project:      ~/mydevbox/gzh-cli-gitforge/.gz-git.yaml ✓
#      Children (explicit):
#        - vendor/external-lib (git, skip)
#        - modules/plugin (project, .gz-git.yaml)
#   3. Workspace:    ~/mydevbox/.gz-git-workspace.yaml ✓
#      Children (explicit):
#        - gzh-cli (git)
#        - gzh-cli-gitforge (project, .gz-git.yaml)
#        - special-project (project, .special-config.yaml)
#   4. Workstation:  ~/.gz-git-config.yaml ✓
#      Children (explicit):
#        - ~/mydevbox (workspace, .gz-git-workspace.yaml)
#        - ~/mywork (workspace, .work-config.yaml)
#   5. Profile:      opensource (active)
#   6. Global:       ~/.config/gz-git/config.yaml ✓
#   7. Defaults:     (built-in)

# Show hierarchy with discovery mode info (NEW!)
gz-git config hierarchy --verbose
# → Shows discovery mode for each level (explicit/auto/hybrid)

# Validate hierarchy (NEW!)
gz-git config hierarchy --validate
# → Check if all config files are valid
# → Check if all children paths exist
# → Check if all custom config files exist
```

### Discovery Mode Control

```bash
# Set discovery mode for current workspace (NEW!)
cd ~/mydevbox
gz-git config workspace set-discovery-mode explicit
# → Only use children defined in .gz-git-workspace.yaml

# Set discovery mode for specific command
gz-git status --discovery-mode auto
# → Scan directories, ignore children definitions

# Set default discovery mode in workspace config (NEW!)
cat >> .gz-git-workspace.yaml <<EOF
discovery:
  mode: hybrid  # explicit, auto, or hybrid
EOF
```

---

## ✅ 구현 체크리스트

### Phase 1: Core Data Structures (✅ DONE)

- [x] **ChildEntry type** (`pkg/config/types.go`)
  - [x] Path, Type, ConfigFile fields
  - [x] Inline override fields (Profile, Parallel, Sync, Branch)
  - [x] ChildType enum (workspace, project, git)
  - [x] DefaultConfigFile() method

- [x] **Updated Config types** (`pkg/config/types.go`)
  - [x] WorkstationConfig with Children []ChildEntry
  - [x] WorkspaceConfig with Children []ChildEntry
  - [x] ProjectConfig with Children []ChildEntry

- [x] **Config discovery** (`pkg/config/workspace.go`)
  - [x] FindWorkstationConfig() (~/.gz-git-config.yaml)
  - [x] FindWorkspaceConfig() (walk up from current dir)
  - [x] FindAllConfigs() (workstation → workspace → project)

- [x] **Manager extensions** (`pkg/config/manager.go`)
  - [x] LoadWorkstationConfig(), SaveWorkstationConfig()
  - [x] LoadWorkspaceConfig(), SaveWorkspaceConfig()

### Phase 2: Children Loading & Discovery (🔨 IN PROGRESS)

- [ ] **DiscoveryMode type** (`pkg/config/types.go`)
  - [ ] DiscoveryMode enum (explicit, auto, hybrid)
  - [ ] Add to WorkstationConfig, WorkspaceConfig, ProjectConfig

- [ ] **Child type** (`pkg/config/workspace.go`)
  - [ ] Child struct (Path, Type, ConfigFile, Config, Entry)
  - [ ] LoadChildrenWithMode(parentPath, config, mode)

- [ ] **Explicit children loading** (`pkg/config/workspace.go`)
  - [ ] LoadExplicitChildren(parentPath, entries []ChildEntry)
  - [ ] resolvePath() for ~, relative paths
  - [ ] loadChildConfig() for each child type
  - [ ] Handle missing config files gracefully

- [ ] **Auto-discovery** (`pkg/config/workspace.go`)
  - [ ] AutoDiscoverChildren(parentPath)
  - [ ] hasFile() helper
  - [ ] isGitRepo() helper
  - [ ] Detect workspace/project/git by config file presence

- [ ] **Hybrid discovery** (`pkg/config/workspace.go`)
  - [ ] Use children if len(children) > 0
  - [ ] Otherwise auto-discover

### Phase 3: 7-Layer Precedence (⏸️ PENDING)

- [ ] **ConfigLoader update** (`pkg/config/loader.go`)
  - [ ] Load() - add workstation and workspace config loading
  - [ ] ResolveConfig() - add layers 4 and 5
  - [ ] applyWorkstationConfig(effective)
  - [ ] applyWorkspaceConfig(effective)
  - [ ] determineActiveProfile() - check workspace → workstation → global

- [ ] **Profile selection logic** (`pkg/config/loader.go`)
  - [ ] Workspace config profile override
  - [ ] Workstation mapping by current path
  - [ ] Fallback to global active profile

### Phase 4: CLI Commands - Workstation (⏸️ PENDING)

- [ ] **`config init --workstation`** (`cmd/gz-git/cmd/config.go`)
  - [ ] Create ~/.gz-git-config.yaml
  - [ ] Interactive mode: prompt for defaults
  - [ ] Template with children example

- [ ] **`config show --workstation`** (`cmd/gz-git/cmd/config.go`)
  - [ ] Display workstation config
  - [ ] Show children list with types

- [ ] **`config edit --workstation`** (`cmd/gz-git/cmd/config.go`)
  - [ ] Open ~/.gz-git-config.yaml in $EDITOR

- [ ] **`config workstation add-workspace`** (`cmd/gz-git/cmd/config_workstation.go` - NEW)
  - [ ] Add child to workstation config
  - [ ] Flags: --profile, --parallel, --config-file, --type
  - [ ] Validate path exists
  - [ ] Create config file if not exists

- [ ] **`config workstation remove-workspace`** (`cmd/gz-git/cmd/config_workstation.go`)
  - [ ] Remove child from workstation config
  - [ ] Optionally delete config file (--delete-config flag)

- [ ] **`config workstation list`** (`cmd/gz-git/cmd/config_workstation.go`)
  - [ ] List all children with types and config files

### Phase 5: CLI Commands - Workspace (⏸️ PENDING)

- [ ] **`config init --workspace`** (`cmd/gz-git/cmd/config.go`)
  - [ ] Create .gz-git-workspace.yaml in current dir
  - [ ] Support --config-file for custom name
  - [ ] Interactive mode: prompt for profile, sync strategy

- [ ] **`config show --workspace`** (`cmd/gz-git/cmd/config.go`)
  - [ ] Display workspace config
  - [ ] Show children list with types

- [ ] **`config workspace add-child`** (`cmd/gz-git/cmd/config_workspace.go` - NEW)
  - [ ] Add child to workspace config
  - [ ] Flags: --type, --config-file, --profile, --sync-strategy, --parallel
  - [ ] Validate path exists
  - [ ] Create config file if not exists

- [ ] **`config workspace remove-child`** (`cmd/gz-git/cmd/config_workspace.go`)
  - [ ] Remove child from workspace config

- [ ] **`config workspace list`** (`cmd/gz-git/cmd/config_workspace.go`)
  - [ ] List all children with types

- [ ] **`config workspace set-discovery-mode`** (`cmd/gz-git/cmd/config_workspace.go`)
  - [ ] Set discovery mode (explicit, auto, hybrid)

### Phase 6: CLI Commands - Project (⏸️ PENDING)

- [ ] **`config project add-child`** (`cmd/gz-git/cmd/config_project.go` - NEW)
  - [ ] Add child (submodule, nested repo) to project config
  - [ ] Flags: --type, --config-file, --sync-strategy

- [ ] **`config project remove-child`** (`cmd/gz-git/cmd/config_project.go`)
  - [ ] Remove child from project config

- [ ] **`config project list`** (`cmd/gz-git/cmd/config_project.go`)
  - [ ] List all children with sync strategies

### Phase 7: CLI Commands - Hierarchy (⏸️ PENDING)

- [ ] **`config hierarchy`** (`cmd/gz-git/cmd/config.go`)
  - [ ] Display all 7 layers with file paths
  - [ ] Show which layers are active (✓)
  - [ ] Show children for each layer
  - [ ] Flag: --verbose (show discovery modes)
  - [ ] Flag: --validate (check config validity)

- [ ] **Hierarchy validation** (`pkg/config/validator.go`)
  - [ ] ValidateHierarchy() function
  - [ ] Check all config files are valid YAML
  - [ ] Check all children paths exist
  - [ ] Check all custom config files exist
  - [ ] Report errors with file:line

### Phase 8: Global Flags (⏸️ PENDING)

- [ ] **`--discovery-mode`** flag (all bulk commands)
  - [ ] Add to status, fetch, pull, push, sync, etc.
  - [ ] Values: explicit, auto, hybrid
  - [ ] Override config file setting

### Phase 9: Testing (⏸️ PENDING)

- [ ] **Unit tests** (`pkg/config/`)
  - [ ] ChildEntry.DefaultConfigFile()
  - [ ] LoadExplicitChildren()
  - [ ] AutoDiscoverChildren()
  - [ ] LoadChildrenWithMode()
  - [ ] Workstation/Workspace config loading
  - [ ] 7-layer precedence resolution

- [ ] **Integration tests** (`pkg/config/`)
  - [ ] Full hierarchy (workstation → workspace → project)
  - [ ] Profile selection from workspace
  - [ ] Children loading with custom config files
  - [ ] Discovery mode switching

- [ ] **CLI tests** (`cmd/gz-git/cmd/`)
  - [ ] config init --workstation
  - [ ] config workstation add-workspace
  - [ ] config workspace add-child
  - [ ] config hierarchy
  - [ ] config hierarchy --validate

### Phase 10: Documentation (⏸️ PENDING)

- [ ] **CLAUDE.md** update
  - [ ] Add 3-tier hierarchy section
  - [ ] Update precedence order (7 layers)
  - [ ] Add workspace config examples

- [ ] **Migration guide** (`docs/guides/MIGRATION_2TIER_TO_3TIER.md` - NEW)
  - [ ] Step-by-step migration process
  - [ ] Backward compatibility notes
  - [ ] Common migration scenarios

- [ ] **User guide** (`docs/guides/WORKSPACE_CONFIG_GUIDE.md` - NEW)
  - [ ] How to set up workstation config
  - [ ] How to organize workspaces
  - [ ] Children management best practices
  - [ ] Discovery mode selection guide

### Phase 11: Polish (⏸️ PENDING)

- [ ] **Error messages**
  - [ ] Clear error when config file not found
  - [ ] Clear error when child path doesn't exist
  - [ ] Suggestions for fixing hierarchy issues

- [ ] **Performance**
  - [ ] Cache config file reads
  - [ ] Lazy load children configs
  - [ ] Parallel children loading

- [ ] **Security**
  - [ ] Validate file paths (no ../ escaping)
  - [ ] Check file permissions (warn on 644 for sensitive configs)
  - [ ] Sanitize custom config file names

---

## 📊 Benefits

### For Users

✅ **워크스테이션 전체 관리** - 한 곳에서 모든 워크스페이스 설정
✅ **워크스페이스별 설정** - ~/mydevbox, ~/mywork 각각 다른 설정
✅ **자동 프로필 선택** - 디렉토리 기반 자동 프로필 전환
✅ **계층적 override** - 필요한 부분만 override
✅ **일관된 설정** - 워크스페이스 내 모든 프로젝트 일관성

### Backward Compatibility

✅ **100% 호환** - 기존 2-tier 시스템 그대로 작동
✅ **점진적 적용** - 필요한 계층만 추가
✅ **Zero breaking changes**

---

## 🚀 Implementation Roadmap

### Completed (Phase 1)
1. ✅ WorkstationConfig, WorkspaceConfig 타입 정의
2. ✅ Config discovery 함수 구현
3. ✅ Manager에 load/save 함수 추가
4. ✅ **설계 문서 개선 완료** (명시적 children 관리)

### Next Steps (Phase 2-4)
5. 🔨 **Phase 2**: Children loading & discovery 구현
   - DiscoveryMode enum
   - LoadExplicitChildren(), AutoDiscoverChildren()
   - Child type 정의

6. ⏸️ **Phase 3**: 7-layer precedence 구현
   - ConfigLoader 업데이트
   - applyWorkstationConfig(), applyWorkspaceConfig()
   - Automatic profile selection

7. ⏸️ **Phase 4-7**: CLI commands 구현
   - Workstation commands (init, add-workspace, list)
   - Workspace commands (init, add-child, list)
   - Project commands (add-child, list)
   - Hierarchy command (show, validate)

8. ⏸️ **Phase 8-9**: Global flags & testing
   - --discovery-mode flag for all bulk commands
   - Unit tests, integration tests, CLI tests

9. ⏸️ **Phase 10-11**: Documentation & polish
   - CLAUDE.md update
   - Migration guide
   - User guide

---

## 📝 Design Summary

### 핵심 개선사항

이 설계는 사용자의 요구사항을 완벽하게 반영합니다:

✅ **"각 설정파일은 하위 설정파일의 경로를 확인할 수 있어야 한다"**
- → `children: []ChildEntry` 배열로 명시적 정의

✅ **"하위 경로가 단순히 git일수도 있고 하위 설정파일일 수도 있어야한다"**
- → `type: workspace | project | git` 구분

✅ **"하위 경로의 설정파일명도 명시 가능해야 함"**
- → `configFile: string` 필드로 커스텀 파일명 지정

✅ **"파일명 없는경우 기본파일명 사용"**
- → `configFile` 생략 시 `DefaultConfigFile()` 사용

### 주요 설계 특징

1. **명시적 계층 관리** (Explicit Hierarchy)
   - 각 config 파일에 children 명시
   - 타입별 구분 (workspace, project, git)
   - 커스텀 config 파일명 지원

2. **유연한 탐색 모드** (Flexible Discovery)
   - Explicit: children만 사용
   - Auto: 디렉토리 스캔
   - Hybrid: children 우선, 없으면 스캔 (기본값)

3. **7-Layer Precedence**
   - Command flags → Project → Workspace → Workstation → Profile → Global → Defaults
   - 각 layer마다 children 정의 가능

4. **100% Backward Compatibility**
   - 기존 2-tier 시스템 그대로 작동
   - 점진적 마이그레이션 가능
   - Zero breaking changes

### File Structure Example

```
~/.gz-git-config.yaml           # Workstation config
  children:
    - ~/mydevbox                # Workspace 1
    - ~/mywork                  # Workspace 2
    - ~/single-repo             # Single git repo

~/mydevbox/.gz-git-workspace.yaml  # Workspace config
  children:
    - gzh-cli                   # Git repo (no config)
    - gzh-cli-gitforge          # Project (with .gz-git.yaml)
    - special-project           # Project (custom .special-config.yaml)

~/mydevbox/gzh-cli-gitforge/.gz-git.yaml  # Project config
  children:
    - vendor/external-lib       # Submodule (skip sync)
    - modules/plugin            # Nested repo (with config)
```

### Use Cases

| Use Case | Solution |
|----------|----------|
| Workstation-wide defaults | `~/.gz-git-config.yaml` |
| Workspace-specific settings | `.gz-git-workspace.yaml` |
| Project-specific overrides | `.gz-git.yaml` |
| Custom config file names | `configFile: .custom.yaml` |
| Mixed git/project repos | `type: git \| project` |
| Selective repo management | `discovery: explicit` |
| Auto-detect all repos | `discovery: auto` |
| Submodule management | Project config children |

---

**Status**: 🎨 **DESIGN COMPLETE** → Ready for Phase 2 implementation
**Target**: Phase 8.2 확장 (Workspace Config)
**Priority**: P1 (사용자 요청)
**Document**: WORKSPACE_CONFIG_DESIGN.md (1300+ lines)
**Last Updated**: 2026-01-16
