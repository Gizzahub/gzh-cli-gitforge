# Go 라이브러리로 사용하기

gz-git을 Go 프로젝트에서 라이브러리로 사용하는 방법입니다.

---

## 설치

```bash
go get github.com/gizzahub/gzh-cli-gitforge
```

---

## 기본 사용법

### 저장소 상태 확인

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

func main() {
    ctx := context.Background()
    client := repository.NewClient()

    // 저장소 열기
    repo, err := client.Open(ctx, ".")
    if err != nil {
        log.Fatal(err)
    }

    // 정보 가져오기
    info, err := client.GetInfo(ctx, repo)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Branch: %s\n", info.Branch)
    fmt.Printf("Remote: %s\n", info.RemoteURL)

    // 상태 확인
    status, err := client.GetStatus(ctx, repo)
    if err != nil {
        log.Fatal(err)
    }

    if status.IsClean {
        fmt.Println("Status: Clean")
    } else {
        fmt.Printf("Modified: %d, Staged: %d, Untracked: %d\n",
            len(status.ModifiedFiles),
            len(status.StagedFiles),
            len(status.UntrackedFiles))
    }
}
```

---

## 여러 저장소 병렬 처리

```go
package main

import (
    "context"
    "fmt"
    "log"
    "sync"

    "github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

func main() {
    repos := []string{
        "/path/to/repo1",
        "/path/to/repo2",
        "/path/to/repo3",
    }

    ctx := context.Background()
    client := repository.NewClient()

    var wg sync.WaitGroup
    for _, path := range repos {
        wg.Add(1)
        go func(p string) {
            defer wg.Done()
            checkRepo(ctx, client, p)
        }(path)
    }

    wg.Wait()
}

func checkRepo(ctx context.Context, client repository.Client, path string) {
    repo, err := client.Open(ctx, path)
    if err != nil {
        log.Printf("Error: %s - %v", path, err)
        return
    }

    status, err := client.GetStatus(ctx, repo)
    if err != nil {
        log.Printf("Error: %s - %v", path, err)
        return
    }

    if status.IsClean {
        fmt.Printf("Clean: %s\n", path)
    } else {
        fmt.Printf("Dirty: %s\n", path)
    }
}
```

---

## 사용 가능한 패키지

| 패키지 | 용도 |
|--------|------|
| `pkg/repository` | 저장소 관리 (clone, status, info) |
| `pkg/operations` | 일괄 작업 (bulk fetch/pull/push) |
| `pkg/commit` | 커밋 자동화 |
| `pkg/branch` | 브랜치 관리 |
| `pkg/history` | 히스토리 분석 |
| `pkg/merge` | 머지/리베이스 |

---

## API 문서

- [pkg.go.dev](https://pkg.go.dev/github.com/gizzahub/gzh-cli-gitforge) - 전체 API 레퍼런스
- [examples/](../../../examples/) - 예제 코드
