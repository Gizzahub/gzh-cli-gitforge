// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// cargoWorkspacePrepareV1 prepares a Rust workspace so the readiness probes
// measure the branch, not dependency resolution. It runs the two steps a
// Cargo workspace's own provision declaration is expected to name — fetch the
// dependencies, then verify the workspace compiles — before the check and
// lint probes run, against both the target and the source commit.
const cargoWorkspacePrepareV1 = "cargo-workspace-v1"

// cargoPrepareTimeout bounds the whole preparation, both steps combined. It
// is larger than prepareProfileTimeout because a cold workspace compiles its
// entire dependency graph during cargo check --workspace — that compilation
// IS the preparation, and its artifact (the workspace target/ directory) is
// what makes the later probe a measurement of the branch rather than of an
// empty cache.
const cargoPrepareTimeout = 30 * time.Minute

// prepareCargoWorkspace fetches and compiles the workspace dependencies.
//
// The environment is the same recipe the make probe uses — the inherited
// environment plus LC_ALL=C — because the prepared tree only helps if the
// probe later resolves the same registry, toolchain, and target directory
// the preparation built against. Isolating CARGO_HOME here would relocate
// every dependency source path and force the probe to recompile the graph
// it was supposed to inherit.
//
// This is process isolation, not a security sandbox. As with legacy Make,
// the selected task revision is trusted to execute repository-owned code:
// build scripts run during cargo check, exactly as they run during the
// probe itself.
func prepareCargoWorkspace(ctx context.Context, g gitRepo, dir string) error {
	before, err := g.refNames(ctx)
	if err != nil {
		return err
	}
	runCtx, cancel := context.WithTimeout(ctx, cargoPrepareTimeout)
	defer cancel()
	for _, step := range []struct{ name, args string }{
		{"cargo fetch", "fetch"},
		{"cargo check --workspace", "check --workspace"},
	} {
		cmd := exec.CommandContext(runCtx, "cargo", strings.Fields(step.args)...) // #nosec G204 -- fixed closed profile.
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "LC_ALL=C", "CARGO_TERM_COLOR=never", "GIT_TERMINAL_PROMPT=0")
		var out bytes.Buffer
		cmd.Stdout = &limitedWriter{w: &out, n: 128 << 10}
		cmd.Stderr = cmd.Stdout
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s failed: %w: %s", step.name, err, strings.TrimSpace(out.String()))
		}
	}
	if runCtx.Err() != nil {
		return fmt.Errorf("preparation timed out")
	}
	after, err := g.refNames(ctx)
	if err != nil {
		return err
	}
	if strings.Join(before, "\x00") != strings.Join(after, "\x00") {
		return fmt.Errorf("preparation changed git refs")
	}
	return validatePreparedStatus(ctx, dir, "target/")
}
