// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

// Offline observation context: the marker and command shaping that keep a
// discard's local-only evidence, and every non-ls-remote finish probe, from
// reading a remote. Network-backed probes are excluded by their callers, not
// by this context.

package runtask

import (
	"context"
	"os"
)

type offlineObservationKey struct{}

func offlineObservationContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, offlineObservationKey{}, true)
}

func isOfflineObservation(ctx context.Context) bool {
	offline, ok := ctx.Value(offlineObservationKey{}).(bool)
	return ok && offline
}

// offlineWorktrunkCommand runs Worktrunk with a config that names no paths of
// its own and git env that forbids lazy fetches, so a local-only discard can
// read the inventory without network access or a Worktrunk config the
// operator may have pointed at another machine.
func (s *Service) offlineWorktrunkCommand(ctx context.Context, source string, args ...string) (CommandResult, error) {
	return s.runner.Run(ctx, CommandRequest{
		Command: "wt",
		Args:    args,
		WorkDir: s.root,
		Env: map[string]string{
			"GIT_CONFIG_COUNT":              "1",
			"GIT_CONFIG_KEY_0":              "worktrunk.default-branch",
			"GIT_CONFIG_VALUE_0":            source,
			"GIT_NO_LAZY_FETCH":             "1",
			"GIT_OPTIONAL_LOCKS":            "0",
			"WORKTRUNK_CONFIG_PATH":         os.DevNull,
			"WORKTRUNK_SYSTEM_CONFIG_PATH":  os.DevNull,
			"WORKTRUNK_PROJECT_CONFIG_PATH": os.DevNull,
		},
	})
}
