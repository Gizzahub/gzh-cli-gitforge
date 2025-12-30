#!/bin/bash
# Commit Automation Demo
# Demonstrates gz-git commit features

echo "=== gz-git Commit Automation Demo ==="
echo

# Check if gz-git is installed
if ! command -v gz-git &> /dev/null; then
    echo "Error: gz-git is not installed"
    echo "Install with: go install github.com/gizzahub/gzh-cli-gitforge/cmd/gz-git@latest"
    exit 1
fi

# Example 1: Validate commit messages
echo "1. Validate Commit Messages"
echo "----------------------------"

echo "Valid message:"
gz-git commit validate "feat(cli): add status command"
echo

echo "Invalid message (missing scope):"
gz-git commit validate "feat add status command" || echo "❌ Validation failed (expected)"
echo

# Example 2: List templates
echo "2. List Available Templates"
echo "----------------------------"
gz-git commit template list
echo

# Example 3: Show template details
echo "3. Show Template Details"
echo "------------------------"
gz-git commit template show conventional
echo

# Example 4: Auto-generate (requires staged changes)
echo "4. Auto-Generate Commit Message"
echo "--------------------------------"
echo "This requires staged changes in the repository"
echo "Example usage:"
echo "  git add <files>"
echo "  gz-git commit auto"
echo

echo "Demo complete!"
echo
echo "Try these commands yourself:"
echo "  gz-git commit validate \"<message>\""
echo "  gz-git commit template list"
echo "  gz-git commit auto"
