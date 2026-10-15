package cmd

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestLegacyCleanupPlanJSONMissingFlags(t *testing.T) {
	oldTarget, oldOutput, oldDone, oldReleased, oldJSON := legacyCleanupPlanTarget, legacyCleanupPlanOutput, legacyCleanupPlanDoneArtifact, legacyCleanupPlanReleasedArtifact, legacyCleanupPlanJSON
	t.Cleanup(func() {
		legacyCleanupPlanTarget, legacyCleanupPlanOutput, legacyCleanupPlanDoneArtifact, legacyCleanupPlanReleasedArtifact, legacyCleanupPlanJSON = oldTarget, oldOutput, oldDone, oldReleased, oldJSON
	})
	legacyCleanupPlanTarget, legacyCleanupPlanOutput, legacyCleanupPlanDoneArtifact, legacyCleanupPlanReleasedArtifact, legacyCleanupPlanJSON = "", "", "", "", true
	var out bytes.Buffer
	integrateLegacyCleanupPlanCmd.SetOut(&out)
	err := runIntegrateLegacyCleanupPlan(integrateLegacyCleanupPlanCmd, nil)
	if err == nil {
		t.Fatal("missing flags unexpectedly succeeded")
	}
	var result struct {
		SchemaVersion int    `json:"schema_version"`
		ExitCode      int    `json:"exit_code"`
		Error         string `json:"error"`
	}
	if decodeErr := json.Unmarshal(out.Bytes(), &result); decodeErr != nil {
		t.Fatalf("invalid structured error: %v (%q)", decodeErr, out.String())
	}
	if result.SchemaVersion != 1 || result.ExitCode != 2 || result.Error == "" {
		t.Fatalf("unexpected structured error: %+v", result)
	}
}
