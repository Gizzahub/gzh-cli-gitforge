// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import "os"

// Windows has no filesystem FIFO. Root-relative access constrains the path;
// descriptor identity and regular-file checks reject observed replacements.
func makeOutcomeReportOpenFlags() int { return os.O_RDONLY }
