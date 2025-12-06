# Common Tasks - gzh-cli-gitforge

## Adding New Forge Commands

### Where to add

`cmd/gitforge/` - create new command file

### Example workflow

```go
// cmd/gitforge/repos.go
var reposCmd = &cobra.Command{
    Use:   "repos",
    Short: "Manage repositories",
    RunE:  runRepos,
}

func runRepos(cmd *cobra.Command, args []string) error {
    // Implementation
    return nil
}

func init() {
    rootCmd.AddCommand(reposCmd)
}
```

## Adding Forge Client Logic

### Where to add

`pkg/{forge}/` - GitHub, GitLab, Gitea

### Example

```go
// pkg/github/client.go
type Client struct {
    client *github.Client
}

func (c *Client) ListRepos(ctx context.Context, org string) ([]Repo, error) {
    // Implementation
    return nil, nil
}
```

## Adding Common Forge Operations

### Where to add

`pkg/forge/` - unified interface

### Example

```go
// pkg/forge/interface.go
type Client interface {
    ListRepos(ctx context.Context, org string) ([]Repo, error)
    CreateRepo(ctx context.Context, opts CreateOptions) error
}
```

## Handling Rate Limits

### Use internal/ratelimit package

```go
import "github.com/gizzahub/gzh-cli-gitforge/internal/ratelimit"

limiter := ratelimit.NewLimiter(...)
limiter.Wait(ctx)
```

## Adding Forge-Specific Errors

### Where to add

`internal/errors/errors.go` - categorize by forge

### Example

```go
var (
    ErrGitHubRateLimit = errors.New("GitHub rate limit exceeded")
    ErrGitLabAuth      = errors.New("GitLab authentication failed")
)
```

## Testing with Real APIs

```go
func TestGitHubAPI(t *testing.T) {
    token := os.Getenv("GITHUB_TOKEN")
    if token == "" {
        t.Skip("GITHUB_TOKEN not set")
    }
    // Test with real API
}
```
