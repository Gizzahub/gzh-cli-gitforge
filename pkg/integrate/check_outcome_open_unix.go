//go:build !windows

// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import "golang.org/x/sys/unix"

// A path replaced after Lstat must neither follow a symlink nor wait for a
// FIFO writer before the descriptor's regular-file check can reject it.
func makeOutcomeReportOpenFlags() int {
	return unix.O_RDONLY | unix.O_NONBLOCK | unix.O_NOFOLLOW
}
