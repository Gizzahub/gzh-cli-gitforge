// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import "fmt"

const (
	// ReadinessApprovalManual requires an interactive human acknowledgement.
	ReadinessApprovalManual = "manual"
	// ReadinessApprovalAutomatic permits the statement-bound automatic path.
	ReadinessApprovalAutomatic = "automatic"
)

// LoadReadinessApproval loads the global-only readiness approval mode. Project
// and profile configuration are deliberately excluded from this decision.
func LoadReadinessApproval() (string, error) {
	mgr, err := NewManager()
	if err != nil {
		return "", fmt.Errorf("initialize global configuration: %w", err)
	}
	global, err := mgr.LoadGlobalConfig()
	if err != nil {
		return "", err
	}
	if global.Integration == nil || global.Integration.ReadinessApproval == "" {
		return ReadinessApprovalManual, nil
	}
	return global.Integration.ReadinessApproval, nil
}
