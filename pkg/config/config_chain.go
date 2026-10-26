// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"errors"
	"fmt"
	"os"
)

// ErrConfigCycle reports a config workspace that leads back to a config
// directory already on the current nesting path.
var ErrConfigCycle = errors.New("circular config reference detected")

// ErrConfigDepth reports config nesting deeper than MaxConfigDepth.
var ErrConfigDepth = errors.New("config recursion depth exceeded")

// ConfigChain is the config directories from the root config to the one
// being loaded, in nesting order. Sync and status both use it, so a cycle
// means the same thing in both commands (ISSUE-104).
type ConfigChain []string

// Enter returns the chain for loading the config in dir from the last
// directory of c. dir is a cycle when it names the same directory as any
// entry, compared by file identity so a symlink or a differently spelled
// path back to an ancestor is caught. The copy keeps sibling workspaces
// independent, so siblings may share one nested config.
func (c ConfigChain) Enter(dir string) (ConfigChain, error) {
	if len(c) > MaxConfigDepth {
		return nil, fmt.Errorf("%w (max %d): possible circular or excessively nested config", ErrConfigDepth, MaxConfigDepth)
	}
	for _, e := range c {
		if SameDir(dir, e) {
			return nil, fmt.Errorf("%w: %s", ErrConfigCycle, dir)
		}
	}
	next := make(ConfigChain, len(c)+1)
	copy(next, c)
	next[len(c)] = dir
	return next, nil
}

// IsConfigTreeError reports a config error that refuses the whole workspace
// tree in both sync and status, instead of skipping one workspace with a
// warning (ISSUE-104).
func IsConfigTreeError(err error) bool {
	return errors.Is(err, ErrWorkspaceTypeConflict) ||
		errors.Is(err, ErrConfigCycle) ||
		errors.Is(err, ErrConfigDepth)
}

// SameDir reports whether a and b name the same existing directory, however
// each is spelled: relative, with a trailing slash, or through a symlink.
func SameDir(a, b string) bool {
	ai, err := os.Stat(a)
	if err != nil {
		return false
	}
	bi, err := os.Stat(b)
	if err != nil {
		return false
	}
	return os.SameFile(ai, bi)
}
