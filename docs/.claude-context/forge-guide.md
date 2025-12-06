# Forge-Specific Guidelines - gzh-cli-gitforge

## GitHub API

### Client Library
- Use `github.com/google/go-github/v66`

### Rate Limits
- 5000/hour authenticated
- Handle secondary rate limits
- Use conditional requests (ETag)

### Best Practices
```go
import "github.com/google/go-github/v66/github"

client := github.NewClient(nil).WithAuthToken(token)

// Use context for all operations
ctx := context.Background()
repos, _, err := client.Repositories.List(ctx, "org", nil)
```

## GitLab API

### Client Library
- Use `github.com/xanzy/go-gitlab`

### Rate Limits
- Vary by plan
- Check headers for remaining quota
- Implement exponential backoff

### Best Practices
```go
import "github.com/xanzy/go-gitlab"

client, err := gitlab.NewClient(token)

// Handle pagination
opt := &gitlab.ListOptions{PerPage: 100}
projects, _, err := client.Projects.ListProjects(opt)
```

## Gitea API

### Considerations
- Similar to GitHub API
- Self-hosted installations
- Version compatibility matters

### Best Practices
```go
// Check Gitea version compatibility
// Use appropriate API endpoints
// Handle self-signed certificates in testing
```

## Common Patterns

### Retry Logic
```go
func retryWithBackoff(ctx context.Context, fn func() error) error {
    backoff := time.Second
    maxRetries := 3

    for i := 0; i < maxRetries; i++ {
        if err := fn(); err != nil {
            if !isRetryable(err) {
                return err
            }
            time.Sleep(backoff)
            backoff *= 2
            continue
        }
        return nil
    }
    return errors.New("max retries exceeded")
}
```

### Pagination
```go
func listAll(ctx context.Context, client Client) ([]Item, error) {
    var allItems []Item
    page := 1

    for {
        items, hasMore, err := client.List(ctx, page)
        if err != nil {
            return nil, err
        }
        allItems = append(allItems, items...)
        if !hasMore {
            break
        }
        page++
    }
    return allItems, nil
}
```

### Error Handling
```go
func handleForgeError(err error) error {
    if rateLimitErr, ok := err.(*github.RateLimitError); ok {
        return fmt.Errorf("GitHub rate limit exceeded, reset at %v: %w",
            rateLimitErr.Rate.Reset, err)
    }
    // Handle other forge-specific errors
    return err
}
```
