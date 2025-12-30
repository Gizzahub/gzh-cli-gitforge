# Installation Guide

Complete installation instructions for `gz-git`.

## Prerequisites

- **Go**: 1.21 or later
- **Git**: 2.30 or later
- **Operating System**: Linux, macOS, or Windows

## Quick Install

### Option 1: Using Go Install (Recommended)

```bash
go install github.com/gizzahub/gzh-cli-gitforge/cmd/gz-git@latest
```

The binary will be installed to `$GOPATH/bin` (usually `~/go/bin`).

### Option 2: From Source

```bash
# Clone the repository
git clone https://github.com/gizzahub/gzh-cli-gitforge.git
cd gzh-cli-gitforge

# Build
make build

# Install (requires sudo)
sudo make install

# Or install to custom location
make install PREFIX=$HOME/.local
```

### Option 3: Download Binary (Coming Soon)

Pre-built binaries will be available on the [Releases](https://github.com/gizzahub/gzh-cli-gitforge/releases) page.

## Detailed Installation

### Building from Source

#### 1. Clone the Repository

```bash
git clone https://github.com/gizzahub/gzh-cli-gitforge.git
cd gzh-cli-gitforge
```

#### 2. Install Dependencies

```bash
go mod download
```

#### 3. Build

```bash
# Development build
make build

# Production build (optimized)
make build-release

# Build for specific platform
GOOS=linux GOARCH=amd64 make build
GOOS=darwin GOARCH=arm64 make build
GOOS=windows GOARCH=amd64 make build
```

The binary will be created in `build/gz-git`.

#### 4. Install

```bash
# Install to /usr/local/bin (requires sudo)
sudo make install

# Install to custom location
make install PREFIX=$HOME/.local

# Or manually copy
cp build/gz-git /usr/local/bin/
```

#### 5. Verify Installation

```bash
gz-git --version
gz-git --help
```

## Platform-Specific Instructions

### Linux

```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install -y git golang-go

# Fedora/RHEL
sudo dnf install -y git golang

# Arch Linux
sudo pacman -S git go

# Then install gz-git
go install github.com/gizzahub/gzh-cli-gitforge/cmd/gz-git@latest
```

Add Go binaries to your PATH if not already done:

```bash
echo 'export PATH=$PATH:$HOME/go/bin' >> ~/.bashrc
source ~/.bashrc
```

### macOS

```bash
# Install prerequisites with Homebrew
brew install git go

# Install gz-git
go install github.com/gizzahub/gzh-cli-gitforge/cmd/gz-git@latest
```

Add Go binaries to your PATH if needed:

```bash
echo 'export PATH=$PATH:$HOME/go/bin' >> ~/.zshrc
source ~/.zshrc
```

### Windows

```powershell
# Install prerequisites with Chocolatey
choco install git golang

# Install gz-git
go install github.com/gizzahub/gzh-cli-gitforge/cmd/gz-git@latest
```

Add Go binaries to your PATH:

```powershell
$env:Path += ";$env:USERPROFILE\go\bin"
[Environment]::SetEnvironmentVariable("Path", $env:Path, [EnvironmentVariableTarget]::User)
```

## Shell Completion (Optional)

### Bash

```bash
# Generate completion script
gz-git completion bash > /usr/local/etc/bash_completion.d/gz-git

# Or for user-specific:
gz-git completion bash > ~/.bash_completion.d/gz-git
echo 'source ~/.bash_completion.d/gz-git' >> ~/.bashrc
```

### Zsh

```zsh
# Generate completion script
gz-git completion zsh > /usr/local/share/zsh/site-functions/_gz-git

# Or for user-specific:
mkdir -p ~/.zsh/completion
gz-git completion zsh > ~/.zsh/completion/_gz-git
echo 'fpath=(~/.zsh/completion $fpath)' >> ~/.zshrc
echo 'autoload -Uz compinit && compinit' >> ~/.zshrc
```

### Fish

```fish
gz-git completion fish > ~/.config/fish/completions/gz-git.fish
```

## Configuration

### Default Configuration

gz-git works out-of-the-box with sensible defaults.

### Custom Templates

Create custom commit templates:

```bash
# Create templates directory
mkdir -p ~/.config/gz-git/templates

# Copy and customize a template
gz-git commit template show conventional > ~/.config/gz-git/templates/my-template.yaml
```

Edit the template and use it:

```bash
gz-git commit auto --template my-template
```

### Environment Variables

Set default behavior with environment variables:

```bash
# Add to ~/.bashrc or ~/.zshrc
export GZH_GIT_TEMPLATE=conventional
export GZH_GIT_EDITOR=vim
```

## Upgrading

### From Go Install

```bash
go install github.com/gizzahub/gzh-cli-gitforge/cmd/gz-git@latest
```

### From Source

```bash
cd gzh-cli-gitforge
git pull
make build
sudo make install
```

## Uninstallation

### Installed via Go

```bash
rm $(which gz-git)
```

### Installed from Source

```bash
cd gzh-cli-gitforge
sudo make uninstall
```

### Clean All Data

```bash
# Remove binaries
rm $(which gz-git)

# Remove configuration (optional)
rm -rf ~/.config/gz-git
```

## Troubleshooting Installation

### "command not found: gz-git"

The binary is not in your PATH. Check:

```bash
# Find the binary
which gz-git

# Check Go bin directory
ls -la $HOME/go/bin

# Add to PATH
export PATH=$PATH:$HOME/go/bin
```

### "permission denied"

You don't have permission to install to the target directory:

```bash
# Use sudo
sudo make install

# Or install to user directory
make install PREFIX=$HOME/.local
```

### Build Errors

Ensure you have the correct Go version:

```bash
go version  # Should be 1.21 or later

# Update Go if needed
# Visit https://go.dev/dl/
```

### Dependency Issues

```bash
# Clean and reinstall dependencies
go clean -modcache
go mod download
go mod tidy
```

## Verification

After installation, verify everything works:

```bash
# Check version
gz-git --version

# Run help
gz-git --help

# Test with a repository
cd /path/to/git/repo
gz-git status
```

## Next Steps

- Read the [Quick Start Guide](QUICKSTART.md)
- Explore the [Command Reference](commands/README.md)
- Check out [Examples](examples/)

## Getting Help

- **GitHub Issues**: https://github.com/gizzahub/gzh-cli-gitforge/issues
- **Documentation**: https://github.com/gizzahub/gzh-cli-gitforge/tree/main/docs
- **Discussions**: https://github.com/gizzahub/gzh-cli-gitforge/discussions
