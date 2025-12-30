#!/bin/bash
# Branch Management Demo
# Demonstrates gz-git branch features

echo "=== gz-git Branch Management Demo ==="
echo

# Check if gz-git is installed
if ! command -v gz-git &> /dev/null; then
    echo "Error: gz-git is not installed"
    echo "Install with: go install github.com/gizzahub/gzh-cli-gitforge/cmd/gz-git@latest"
    exit 1
fi

# Example 1: List branches
echo "1. List Local Branches"
echo "----------------------"
gz-git branch list
echo

echo "2. List All Branches (including remote)"
echo "---------------------------------------"
gz-git branch list --all
echo

# Example 2: Branch creation (demo only)
echo "3. Create Branch (Example)"
echo "--------------------------"
echo "To create a new branch:"
echo "  gz-git branch create feature/new-feature"
echo
echo "To create with worktree:"
echo "  gz-git branch create feature/parallel --worktree /tmp/parallel-work"
echo

# Example 3: Branch deletion (demo only)
echo "4. Delete Branch (Example)"
echo "--------------------------"
echo "To delete a branch:"
echo "  gz-git branch delete feature/old-feature"
echo

echo "Demo complete!"
echo
echo "Try these commands yourself:"
echo "  gz-git branch list --all"
echo "  gz-git branch create <name>"
echo "  gz-git branch delete <name>"
