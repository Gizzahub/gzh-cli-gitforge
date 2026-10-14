package cmd

import (
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
)

func TestIntegrateReconcileAcceptsCEShapedInvocationWithoutBranch(t *testing.T) {
	saved := []string{reconcileTarget, reconcileEvidence, reconcileArtifact, reconcileExecution, reconcileGeneration, reconcileActor, reconcileHost, reconcileKind, reconcileSource}
	savedNoFetch, savedJSON := reconcileNoFetch, reconcileJSON
	defer func() {
		reconcileTarget, reconcileEvidence, reconcileArtifact, reconcileExecution, reconcileGeneration, reconcileActor, reconcileHost, reconcileKind, reconcileSource = saved[0], saved[1], saved[2], saved[3], saved[4], saved[5], saved[6], saved[7], saved[8]
		reconcileNoFetch, reconcileJSON = savedNoFetch, savedJSON
	}()
	reconcileTarget, reconcileEvidence, reconcileArtifact = "origin/develop", "/missing/evidence.json", "/missing/artifact"
	reconcileExecution, reconcileGeneration, reconcileActor, reconcileHost, reconcileKind, reconcileSource = "run-1", "g1", "codex", "host", "agent", strings.Repeat("a", 40)
	reconcileJSON = true
	if err := integrateReconcileCmd.Args(integrateReconcileCmd, nil); err != nil {
		t.Fatalf("no-branch CE invocation rejected by Cobra: %v", err)
	}
	err := runIntegrateReconcile(integrateReconcileCmd, nil)
	if cliutil.ExitCodeForError(err) != 2 || err == nil || !strings.Contains(err.Error(), "legacy evidence") {
		t.Fatalf("no-branch invocation = %v, want evidence validation failure", err)
	}
}
