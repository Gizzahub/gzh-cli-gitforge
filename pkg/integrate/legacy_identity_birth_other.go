//go:build !linux && !darwin && !freebsd && !netbsd

// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

// legacyBirthTime reports no creation time where this package has no portable
// way to read one; the identity then rests on device and inode alone.
func legacyBirthTime(string, uint64) (int64, error) {
	return 0, nil
}
