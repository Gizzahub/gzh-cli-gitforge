//go:build windows

// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

// Windows has no supported directory fsync. Rename is still atomic locally;
// the file itself is synced before replacement.
func syncStateDirectory(string) error { return nil }
