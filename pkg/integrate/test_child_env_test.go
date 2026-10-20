// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

// Process helpers re-executed by readiness and make-process tests.
const (
	readinessNestedHelperEnv = "READINESS_NESTED_HELPER"
	readinessEscapeHelperEnv = "READINESS_ESCAPE_HELPER"
	makeProcessHelperMode    = "GZH_MAKE_PROCESS_HELPER"
)

// testBinaryChildEnvs mark re-executed copies of this test binary. A child
// runs on the environment its parent prepared and never creates a state dir
// of its own: helpers end in os.Exit or a kill, so TestMain's cleanup would
// not run there and every child would leak one (ISSUE-84).
var testBinaryChildEnvs = []string{
	hostLockChildEnv, admissionChildEnv, inheritedResourceChildEnv,
	readinessNestedHelperEnv, readinessEscapeHelperEnv, makeProcessHelperMode,
}
