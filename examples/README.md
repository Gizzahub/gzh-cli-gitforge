# gzh-cli-git Examples

This directory contains runnable examples demonstrating how to use the gzh-cli-git library in your Go applications.

## Examples

### Basic Operations

Demonstrates basic repository operations: opening a repository, getting info, and checking status.

```bash
cd examples/basic
go run main.go
```

**What it does:**
- Opens the gzh-cli-git repository
- Displays branch, remote URL, and upstream information
- Shows working tree status (clean/dirty, file counts)

### Clone Repository

Demonstrates cloning a repository with options.

```bash
cd examples/clone
go run main.go <repository-url> [destination]
```

**Examples:**
```bash
# Clone to default location (/tmp/cloned-repo)
go run main.go https://github.com/golang/example.git

# Clone to specific location
go run main.go https://github.com/golang/example.git /tmp/my-repo
```

**What it does:**
- Clones a repository with shallow clone (depth=1) for speed
- Uses single-branch mode to clone only the default branch
- Displays clone progress and repository information

## Running All Examples

You can test all examples with:

```bash
# Basic example (in current repository)
cd examples/basic && go run main.go && cd ../..

# Clone example (requires internet)
cd examples/clone && go run main.go https://github.com/golang/example.git /tmp/test-clone && cd ../..
```

## Building Examples

To build standalone binaries:

```bash
# Build basic example
go build -o bin/basic examples/basic/main.go

# Build clone example
go build -o bin/clone examples/clone/main.go
```

## Learning More

For complete API documentation, see:
- [Repository Client API](../pkg/repository/interfaces.go)
- [Main README](../README.md)

## Adding More Examples

Want to contribute an example? Create a new directory under `examples/` with:
- `main.go` - Your example code
- `README.md` - Description and usage (optional)

Make sure your example:
- Has a clear purpose
- Includes error handling
- Uses `context.Context` properly
- Has helpful output messages
