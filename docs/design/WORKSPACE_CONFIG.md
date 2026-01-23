# Workspace Config Design - Recursive Hierarchical Configuration

**Feature**: 재귀적 계층 구조로 무한 중첩 설정 지원
**Priority**: P1 (사용자 요청)
**Status**: Design Complete - Recursive Approach
**Date**: 2026-01-16

______________________________________________________________________

## 📋 요구사항

### 사용자 요청

> 워크스테이션 전체의 git을 묶어서 관리하는 설정파일을 ~/.gz-git-config.yaml을 만들고
> 이것을 기반으로 ~/mydevbox, ~/mywork 등에 각각의 통합 config
> 그리고 각각의 프로젝트의 config를 그 하위에 놓는식으로 관리
>
> **각 설정파일은 하위 설정파일의 경로를 확인할 수 있어야 한다.**
>
> - 하위 경로가 **단순히 git 저장소**일 수도 있고
> - 하위 경로가 **또 다른 설정파일**을 가진 디렉토리일 수도 있어야 한다
> - 설정파일에 **하위 경로의 설정파일명도 명시** 가능해야 함
> - 설정파일명이 없는 경우 **기본 파일명** 사용

### 핵심 인사이트 💡

**기존 설계의 문제점**:

- WorkstationConfig, WorkspaceConfig, ProjectConfig 3가지 타입
- 3가지 다른 파일명
- 복잡한 타입별 로직

**새로운 접근**: **단 하나의 Config 타입**이 **재귀적으로 중첩**
✅ 단순함: 하나의 타입, 하나의 기본 파일명
✅ 유연함: 원하는 만큼 깊이 중첩 가능
✅ 일관성: 모든 레벨에서 동일한 로직

______________________________________________________________________

## 🎯 재귀적 계층 구조

**모든 설정파일이 같은 구조**를 가지고, **무한히 중첩** 가능:

```
~/.gz-git-config.yaml              ← Config (최상위)
    ↓ children
~/mydevbox/.gz-git.yaml            ← Config (중첩 1)
    ↓ children
~/mydevbox/project/.gz-git.yaml    ← Config (중첩 2)
    ↓ children
~/mydevbox/project/submodule/...   ← Config (중첩 3+, 무한!)
```

**모든 레벨에서 동일한 파일명**: `.gz-git.yaml` (커스텀 가능)

### Precedence (재귀적 우선순위)

```
1. Command flags (--provider gitlab)    ← 최고
2. 현재 경로 config (.gz-git.yaml)
3. 부모 경로 config (../.gz-git.yaml)
4. 조부모 경로 config (../../.gz-git.yaml)
   ... (재귀적으로 최상위까지)
N. Active profile
N+1. Global config
N+2. Built-in defaults                   ← 최저
```

**단순한 규칙**: 자식이 부모를 override

______________________________________________________________________

## 📁 파일 구조

### 통합 Config 파일 (.gz-git.yaml)

**모든 계층에서 동일한 형식**:

```yaml
# ~/.gz-git-config.yaml (최상위)
parallel: 10
cloneProto: ssh

children:
  - path: ~/mydevbox
    type: config           # config = 설정파일 있음
    profile: opensource    # Inline override
    parallel: 10

  - path: ~/mywork
    type: config
    configFile: .work-config.yaml  # 커스텀 파일명!
    profile: work

  - path: ~/single-repo
    type: git              # git = 설정파일 없음
    profile: personal
```

```yaml
# ~/mydevbox/.gz-git.yaml (중첩 Level 1)
profile: opensource

sync:
  strategy: reset
  parallel: 10

children:
  - path: gzh-cli
    type: git            # 단순 Git 저장소

  - path: gzh-cli-gitforge
    type: config         # 설정파일 있음
    sync:
      strategy: pull     # Inline override

  - path: nested
    type: config         # 또 다른 중첩!
    profile: sub
```

```yaml
# ~/mydevbox/gzh-cli-gitforge/.gz-git.yaml (중첩 Level 2)
sync:
  strategy: pull

children:
  - path: vendor/lib
    type: git
    sync:
      strategy: skip   # Submodule skip

  - path: modules/plugin
    type: config       # 또 다른 중첩!
```

______________________________________________________________________

## 🏗️ 데이터 구조

### 단일 Config 타입 (재귀적)

```go
// Config - 모든 레벨에서 사용하는 단일 타입
type Config struct {
    // 이 레벨의 설정
    Profile  string `yaml:"profile,omitempty"`
    Parallel int    `yaml:"parallel,omitempty"`

    Sync   *SyncConfig   `yaml:"sync,omitempty"`
    Branch *BranchConfig `yaml:"branch,omitempty"`
    Fetch  *FetchConfig  `yaml:"fetch,omitempty"`
    Pull   *PullConfig   `yaml:"pull,omitempty"`
    Push   *PushConfig   `yaml:"push,omitempty"`

    // 하위 경로들 (재귀!)
    Children []ChildEntry `yaml:"children,omitempty"`

    // 메타데이터
    Metadata *Metadata `yaml:"metadata,omitempty"`
}

// ChildEntry - 하위 경로 정의
type ChildEntry struct {
    Path       string `yaml:"path"`
    Type       ChildType `yaml:"type"` // "config" or "git"
    ConfigFile string `yaml:"configFile,omitempty"` // 기본: .gz-git.yaml

    // Inline overrides
    Profile  string      `yaml:"profile,omitempty"`
    Parallel int         `yaml:"parallel,omitempty"`
    Sync     *SyncConfig `yaml:"sync,omitempty"`
    Branch   *BranchConfig `yaml:"branch,omitempty"`
}

// ChildType - 단순화된 타입
type ChildType string

const (
    ChildTypeConfig ChildType = "config" // 설정파일 있음 (재귀)
    ChildTypeGit    ChildType = "git"    // Git 저장소만
)

func (t ChildType) DefaultConfigFile() string {
    if t == ChildTypeConfig {
        return ".gz-git.yaml"
    }
    return ""
}
```

### 간소화

**Before**: `WorkstationConfig`, `WorkspaceConfig`, `ProjectConfig` (3가지)
**After**: `Config` (하나)

______________________________________________________________________

## 🔍 재귀적 로딩 알고리즘

### 단일 재귀 함수

```go
// LoadConfigRecursive - 모든 레벨에서 동일한 로직
func LoadConfigRecursive(path string, configFile string) (*Config, error) {
    // 1. 이 레벨의 config 로드
    configPath := filepath.Join(path, configFile)
    data, err := os.ReadFile(configPath)
    if err != nil {
        return nil, err
    }

    var config Config
    if err := yaml.Unmarshal(data, &config); err != nil {
        return nil, err
    }

    // 2. Children 재귀적 로딩
    for i := range config.Children {
        child := &config.Children[i]
        childPath := resolvePath(path, child.Path)

        if child.Type == ChildTypeConfig {
            // 재귀 호출!
            childConfigFile := child.ConfigFile
            if childConfigFile == "" {
                childConfigFile = ".gz-git.yaml"
            }

            childConfig, err := LoadConfigRecursive(childPath, childConfigFile)
            if err != nil {
                log.Debugf("Child config not found: %s", err)
                continue
            }

            // Inline override 적용
            mergeInlineOverrides(childConfig, child)
        } else if child.Type == ChildTypeGit {
            // Git 저장소 검증
            if !isGitRepo(childPath) {
                return nil, fmt.Errorf("not a git repo: %s", childPath)
            }
        }
    }

    return &config, nil
}
```

### Discovery Modes

```go
type DiscoveryMode string

const (
    ExplicitMode DiscoveryMode = "explicit" // children만 사용
    AutoMode     DiscoveryMode = "auto"     // 디렉토리 스캔
    HybridMode   DiscoveryMode = "hybrid"   // children 우선, 없으면 스캔
)

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

        if hasFile(childPath, ".gz-git.yaml") {
            config.Children = append(config.Children, ChildEntry{
                Path: childPath,
                Type: ChildTypeConfig,
            })
        } else if isGitRepo(childPath) {
            config.Children = append(config.Children, ChildEntry{
                Path: childPath,
                Type: ChildTypeGit,
            })
        }
    }

    return nil
}
```

______________________________________________________________________

## 🎨 사용 시나리오

### 1. 워크스테이션 초기 설정

```bash
# ~/.gz-git-config.yaml 생성
cat > ~/.gz-git-config.yaml <<EOF
parallel: 10
cloneProto: ssh

children:
  - path: ~/mydevbox
    type: config
    profile: opensource

  - path: ~/mywork
    type: config
    configFile: .work-config.yaml  # 커스텀!
    profile: work

  - path: ~/single-repo
    type: git
EOF

# 디렉토리마다 자동 프로필 선택
cd ~/mydevbox/any-project
gz-git config show
# → profile: opensource (from ~/.gz-git-config.yaml → ~/mydevbox/.gz-git.yaml)
```

### 2. 워크스페이스 설정

```bash
# ~/mydevbox/.gz-git.yaml 생성
cat > ~/mydevbox/.gz-git.yaml <<EOF
profile: opensource

sync:
  strategy: reset
  parallel: 10

children:
  - path: gzh-cli
    type: git

  - path: gzh-cli-gitforge
    type: config
    sync:
      strategy: pull  # Override!

  - path: nested
    type: config
    configFile: .custom.yaml
EOF
```

### 3. 프로젝트 설정 + Submodules

```bash
# ~/mydevbox/gzh-cli-gitforge/.gz-git.yaml
cat > ~/mydevbox/gzh-cli-gitforge/.gz-git.yaml <<EOF
sync:
  strategy: pull

children:
  - path: vendor/lib
    type: git
    sync:
      strategy: skip  # Submodule skip

  - path: modules/plugin
    type: config
EOF
```

### 4. Auto-Discovery vs Explicit

```bash
# Explicit: children만 사용
gz-git status --discovery-mode explicit

# Auto: 디렉토리 스캔
gz-git status --discovery-mode auto

# Hybrid (기본): children 있으면 explicit, 없으면 auto
gz-git status --discovery-mode hybrid
```

______________________________________________________________________

## 🔨 CLI 명령어

### Config 관리

```bash
# Init (creates .gz-git.yaml)
gz-git config init
gz-git config init --workstation  # ~/.gz-git-config.yaml

# Add child
gz-git config add-child ~/mydevbox --type config --profile opensource
gz-git config add-child gzh-cli --type git

# List children
gz-git config list-children

# Remove child
gz-git config remove-child ~/mydevbox

# Show hierarchy
gz-git config hierarchy
# Output:
# ~/.gz-git-config.yaml
#   ├─ ~/mydevbox (.gz-git.yaml)
#   │   ├─ gzh-cli (git)
#   │   └─ gzh-cli-gitforge (.gz-git.yaml)
#   └─ ~/mywork (.work-config.yaml)
```

______________________________________________________________________

## ✅ 구현 체크리스트

### Phase 1: Core (⏸️)

- [ ] `Config` type (단일 타입)
- [ ] `ChildEntry` type
- [ ] `ChildType` enum (`config`, `git`)
- [ ] `LoadConfigRecursive()` function

### Phase 2: Discovery (⏸️)

- [ ] `DiscoveryMode` enum
- [ ] `autoDiscoverAndAppend()`
- [ ] `--discovery-mode` flag

### Phase 3: CLI (⏸️)

- [ ] `config init [--workstation]`
- [ ] `config add-child <path> --type <config|git>`
- [ ] `config list-children`
- [ ] `config remove-child <path>`
- [ ] `config hierarchy`

### Phase 4: Integration (⏸️)

- [ ] Integrate with existing ConfigLoader
- [ ] Update precedence resolution
- [ ] Backward compatibility

### Phase 5: Testing (⏸️)

- [ ] Unit tests for `LoadConfigRecursive`
- [ ] Discovery mode tests
- [ ] CLI integration tests

______________________________________________________________________

## 📊 Benefits

### 단순함

- **1개 타입**: `Config` (not 3)
- **1개 파일명**: `.gz-git.yaml` (커스텀 가능)
- **1개 로직**: 재귀 함수

### 유연함

- **무한 중첩**: 원하는 만큼 깊이 가능
- **커스텀 파일명**: `configFile` 지정
- **Inline override**: children에 직접 설정

### 일관성

- 모든 레벨에서 동일한 구조
- 동일한 로딩 로직
- 동일한 우선순위 규칙

______________________________________________________________________

## 🚀 Next Steps

1. ✅ 재귀적 구조 설계 완료
1. ⏸️ `Config` 타입 구현
1. ⏸️ `LoadConfigRecursive()` 구현
1. ⏸️ CLI 명령어 구현
1. ⏸️ 기존 시스템 통합
1. ⏸️ 테스트 작성

______________________________________________________________________

**Status**: 🎨 **DESIGN COMPLETE**
**Document**: WORKSPACE_CONFIG_RECURSIVE.md (600 lines)
**Last Updated**: 2026-01-16

**핵심 개선**: WorkstationConfig/WorkspaceConfig/ProjectConfig → **단일 Config 타입 (재귀적)**
