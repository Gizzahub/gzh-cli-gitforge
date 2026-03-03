// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"fmt"
	"io"
)

// TreeNode represents a node in the workspace hierarchy tree.
type TreeNode struct {
	Name     string
	Status   string // "synced", "failed", "skipped"
	Message  string // compact status (e.g. "master|↓2")
	Children []*TreeNode
}

// renderTree writes a tree structure to the writer using Unicode box-drawing characters.
func renderTree(out io.Writer, root *TreeNode) {
	if root == nil {
		return
	}
	// Print root node
	if root.Name != "" {
		label := root.Name
		if root.Message != "" {
			label += "  " + root.Message
		}
		fmt.Fprintln(out, label)
	}
	renderChildren(out, root.Children, "")
}

// renderChildren recursively renders child nodes with proper tree indentation.
func renderChildren(out io.Writer, children []*TreeNode, prefix string) {
	for i, child := range children {
		isLast := i == len(children)-1

		connector := "├── "
		childPrefix := "│   "
		if isLast {
			connector = "└── "
			childPrefix = "    "
		}

		label := child.Name
		if child.Message != "" {
			label = fmt.Sprintf("%-24s %s", child.Name, child.Message)
		}

		fmt.Fprintf(out, "%s%s%s\n", prefix, connector, label)

		if len(child.Children) > 0 {
			renderChildren(out, child.Children, prefix+childPrefix)
		}
	}
}
