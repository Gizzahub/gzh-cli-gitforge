// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

const (
	readinessTimeout     = config.DefaultReadinessTimeout
	readinessMaxIO       = 1 << 20
	readinessMaxSummary  = 4096
	readinessMaxManifest = 64 << 10
	readinessWaitDelay   = 2 * time.Second
)

type readinessContract struct {
	Decl         config.Readiness
	ManifestPath string
	ManifestOID  string
	Digest       string
	RunnerOID    string
	TreeOID      string
}

type readinessResult struct {
	Version int    `json:"version"`
	Status  string `json:"status"`
	Summary string `json:"summary"`
	stderr  []byte
	failure *ReadinessFailure
}

func loadReadinessContract(ctx context.Context, g gitRepo, sha string) (readinessContract, bool, error) {
	for _, name := range []string{".gz-git.yaml", ".gz-git.yml", ".gz-git.json"} {
		manifest, present, err := g.treeEntry(ctx, sha, name)
		if err != nil {
			return readinessContract{}, false, err
		}
		if !present {
			continue
		}
		return loadReadinessManifest(ctx, g, sha, name, manifest)
	}
	return readinessContract{}, false, nil
}

func loadReadinessManifest(ctx context.Context, g gitRepo, sha, name string, manifest treeEntry) (readinessContract, bool, error) {
	if manifest.Type != "blob" || (manifest.Mode != "100644" && manifest.Mode != "100755") {
		return readinessContract{}, false, nil
	}
	data, present, err := g.showFile(ctx, sha, name)
	if err != nil {
		return readinessContract{}, false, err
	}
	if !present {
		return readinessContract{}, false, nil
	}
	decl, present, err := config.ParseReadinessDocument(data, strings.HasSuffix(name, ".json"))
	if err != nil {
		return readinessContract{}, false, fmt.Errorf("%s: %w", name, err)
	}
	if !present {
		return readinessContract{}, false, nil
	}
	size, err := g.objectSize(ctx, manifest.OID)
	if err != nil {
		return readinessContract{}, false, err
	}
	if size > readinessMaxManifest {
		return readinessContract{}, false, fmt.Errorf("%s exceeds readiness manifest limit", name)
	}
	digest, err := config.ReadinessDigest(decl)
	if err != nil {
		return readinessContract{}, false, err
	}
	runner, ok, err := g.treeEntry(ctx, sha, decl.Runner)
	if err != nil {
		return readinessContract{}, false, err
	}
	if !ok || runner.Type != "blob" || runner.Mode != "100755" {
		return readinessContract{}, false, fmt.Errorf("readiness runner %s must be an executable regular blob (mode 100755)", decl.Runner)
	}
	tree, ok, err := g.treeEntry(ctx, sha, ".gz-git/readiness")
	if err != nil {
		return readinessContract{}, false, err
	}
	if !ok || tree.Type != "tree" {
		return readinessContract{}, false, fmt.Errorf(".gz-git/readiness is missing or not a tree")
	}
	return readinessContract{Decl: decl, ManifestPath: name, ManifestOID: manifest.OID, Digest: digest, RunnerOID: runner.OID, TreeOID: tree.OID}, true, nil
}

func sameReadinessContract(a, b readinessContract) bool {
	return a.Digest == b.Digest && a.TreeOID == b.TreeOID
}

func checkReadinessContract(ctx context.Context, g gitRepo, plan TargetPlan, report *CheckReport) CheckItem {
	return checkReadinessContractWithResource(ctx, g, plan, report, false, CheckOptions{})
}

func checkReadinessContractWithResource(ctx context.Context, g gitRepo, plan TargetPlan, report *CheckReport, resourceHighLoad bool, opts CheckOptions) CheckItem {
	target, targetPresent, err := loadReadinessContract(ctx, g, plan.TargetSHA)
	if err != nil {
		return CheckItem{Name: "readiness contract", Status: checkFail, Detail: err.Error()}
	}
	source, sourcePresent, err := loadReadinessContract(ctx, g, plan.BranchSHA)
	if err != nil {
		return CheckItem{Name: "readiness contract", Status: checkFail, Detail: err.Error()}
	}
	if !targetPresent && !sourcePresent {
		report.GateMode = "legacy-make"
		return CheckItem{Name: "readiness contract", Status: checkWarn, Detail: "legacy head-owned gate"}
	}
	if !targetPresent {
		return CheckItem{Name: "readiness contract", Status: checkFail, Detail: "bootstrap required: target has no readiness contract"}
	}
	if !sourcePresent {
		return CheckItem{Name: "readiness contract", Status: checkFail, Detail: "source is missing target readiness contract"}
	}
	if !sameReadinessContract(target, source) {
		return CheckItem{Name: "readiness contract", Status: checkFail, Detail: "readiness contract changed between target and source; land the change with gz-git integrate readiness update"}
	}
	if !readinessRunnerSupported() {
		return CheckItem{Name: "readiness contract", Status: checkFail, Detail: "measurement unavailable: contract runners are unsupported on Windows"}
	}
	report.GateMode, report.ManifestPath, report.ContractDigest, report.ManifestOID = "contract-v1", target.ManifestPath, target.Digest, target.ManifestOID
	report.RunnerPath, report.RunnerOID, report.ReadinessTreeOID = target.Decl.Runner, target.RunnerOID, target.TreeOID
	var observed readinessObservation
	opts.readinessObservation = &observed
	result, duration, err := runContractWithResource(ctx, g, plan, target, resourceHighLoad, opts)
	report.ReadinessStatus, report.ReadinessDuration = result.Status, duration
	report.PlanDuration, report.PlanResource, report.PlanDigest, report.ResourceClass = observed.PlanDuration, observed.PlanResource, observed.PlanDigest, observed.ResourceClass
	if observed.PrepareInputs != "" {
		report.PrepareProfile, report.PrepareInputs = target.Decl.PrepareProfile, observed.PrepareInputs
	}
	if err != nil {
		if errors.Is(err, errTargetMovedWhileWaiting) {
			return CheckItem{Name: "readiness contract", Status: checkFail, Detail: err.Error()}
		}
		detail, path := readinessFailureDetailWithPath("measurement unavailable: "+err.Error(), result.stderr)
		if result.failure != nil {
			result.failure.DiagnosticPath = path
			report.recordReadinessFailure(result.failure)
		}
		return CheckItem{Name: "readiness contract", Status: checkFail, Detail: detail}
	}
	if result.Status == "ready" {
		return CheckItem{Name: "readiness contract", Status: checkPass, Detail: result.Summary}
	}
	if result.Status == "unavailable" {
		return CheckItem{Name: "readiness contract", Status: checkFail, Detail: readinessFailureDetail("measurement unavailable: "+result.Summary, result.stderr)}
	}
	return CheckItem{Name: "readiness contract", Status: checkFail, Detail: readinessFailureDetail(result.Summary, result.stderr)}
}

func readinessFailureDetail(detail string, stderr []byte) string {
	detail, _ = readinessFailureDetailWithPath(detail, stderr)
	return detail
}

func readinessFailureDetailWithPath(detail string, stderr []byte) (renderedDetail string, diagnosticPath *string) {
	path, err := writeDiagnostic("readiness", stderr)
	if err != nil {
		renderedDetail = detail + "; diagnostic unavailable: " + err.Error()
		return renderedDetail, nil
	}
	if path == "" {
		renderedDetail = detail
		return renderedDetail, nil
	}
	renderedDetail = detail + "; diagnostic: " + path
	diagnosticPath = &path
	return renderedDetail, diagnosticPath
}

func runContractWithTimeout(ctx context.Context, g gitRepo, plan TargetPlan, contract readinessContract, timeout time.Duration) (result readinessResult, duration time.Duration, err error) {
	return runContractWithTimeoutResource(ctx, g, plan, contract, timeout, readinessPlanTimeout, false, CheckOptions{})
}

func runContractWithResource(ctx context.Context, g gitRepo, plan TargetPlan, contract readinessContract, resourceHighLoad bool, opts CheckOptions) (readinessResult, time.Duration, error) {
	timeout, err := contract.Decl.ExecutionTimeout()
	if err != nil {
		return readinessResult{Status: "unavailable"}, 0, err
	}
	return runContractWithTimeoutResource(ctx, g, plan, contract, timeout, readinessPlanTimeout, resourceHighLoad, opts)
}

func runContractWithTimeoutResource(ctx context.Context, g gitRepo, plan TargetPlan, contract readinessContract, timeout, planTimeout time.Duration, resourceHighLoad bool, opts CheckOptions) (result readinessResult, duration time.Duration, err error) {
	return runContractPair(ctx, g, plan, contract, timeout, planTimeout, resourceHighLoad, opts, false)
}

// runContractPair normally executes the target-owned runner. Only the explicit
// globally authorized readiness-update workflow may select the candidate runner.
func runContractPair(ctx context.Context, g gitRepo, plan TargetPlan, contract readinessContract, timeout, planTimeout time.Duration, resourceHighLoad bool, opts CheckOptions, candidate bool) (result readinessResult, duration time.Duration, err error) {
	targetSiblings, sourceSiblings, hasSiblings, err := resolveSiblingContractPair(ctx, g, plan)
	if err != nil {
		return readinessResult{Status: "unavailable"}, 0, err
	}

	root, err := os.MkdirTemp("", "gz-git-readiness-")
	if err != nil {
		return readinessResult{Status: "unavailable"}, 0, fmt.Errorf("create isolated worktrees: %w", err)
	}
	preserveRoot := false
	defer func() {
		if err != nil {
			result.Status = "unavailable"
		}
	}()
	defer func() {
		if preserveRoot {
			err = errors.Join(err, fmt.Errorf("retained readiness workspace after sibling cleanup failure: %s", root))
			return
		}
		if removeErr := os.RemoveAll(root); removeErr != nil {
			err = errors.Join(err, fmt.Errorf("cleanup readiness workspace: %w", removeErr))
		}
	}()
	targetDir, sourceDir := filepath.Join(root, "target"), filepath.Join(root, "source")
	if hasSiblings {
		// Each probe has its own parent so ../dependency cannot alias the other
		// probe's snapshot or collide with its worktree directory.
		targetDir = filepath.Join(root, "target-side", "target")
		sourceDir = filepath.Join(root, "source-side", "source")
		for _, dir := range []string{filepath.Dir(targetDir), filepath.Dir(sourceDir)} {
			if err := os.Mkdir(dir, 0o700); err != nil {
				return readinessResult{Status: "unavailable"}, 0, err
			}
		}
	}
	if err := addPreparedWorktree(ctx, g, targetDir, plan.TargetSHA, contract.Decl.PrepareProfile); err != nil {
		return readinessResult{Status: "unavailable"}, 0, err
	}
	defer func() { err = mergeCleanupError(ctx, err, g, targetDir) }()
	if err := addPreparedWorktree(ctx, g, sourceDir, plan.BranchSHA, contract.Decl.PrepareProfile); err != nil {
		return readinessResult{Status: "unavailable"}, 0, err
	}
	defer func() { err = mergeCleanupError(ctx, err, g, sourceDir) }()
	var siblingEvidence string
	var siblingLeases []SiblingLease
	if hasSiblings {
		leases, evidence, prepareErr := prepareReadinessSiblings(ctx, g, targetSiblings, sourceSiblings, targetDir, sourceDir)
		if prepareErr != nil {
			return readinessResult{Status: "unavailable"}, 0, prepareErr
		}
		siblingLeases = leases
		defer func() {
			cleanupErr := releaseReadinessSiblings(leases)
			preserveRoot = cleanupErr != nil
			err = errors.Join(err, cleanupErr)
		}()
		siblingEvidence = evidence
	}
	observed := opts.observation()
	runnerEnv, err := readinessRunnerEnv(ctx, g, plan, contract.Decl.PrepareProfile, root, targetDir, sourceDir, siblingLeases, opts, observed)
	if err != nil {
		return readinessResult{Status: "unavailable"}, 0, err
	}
	if contract.Decl.PrepareProfile != "" {
		siblingEvidence += "; preparation " + contract.Decl.PrepareProfile
	}
	runnerDir := targetDir
	if candidate {
		runnerDir = sourceDir
	}
	runner := filepath.Join(runnerDir, filepath.FromSlash(contract.Decl.Runner))
	bind, planOut, err := selectReadinessResource(ctx, runner, targetDir, sourceDir, plan, runnerEnv, contract, resourceHighLoad, planTimeout, observed)
	if err != nil {
		return readinessResult{Status: "unavailable", stderr: planOut.stderr, failure: planOut.failure}, 0, err
	}
	if observed.ResourceClass == readinessResourceHigh {
		waitStarted := time.Now()
		runnerCtx, releaseSlot, acquireErr := holdMeasurementLock(ctx, g, opts)
		if opts.resourceWait != nil {
			*opts.resourceWait += time.Since(waitStarted)
		}
		if acquireErr != nil {
			return readinessResult{Status: "unavailable"}, 0, acquireErr
		}
		// Registered after the worktree cleanup defers, so it runs first: Git
		// cleanup remains outside the machine resource slot.
		defer releaseSlot()
		ctx = runnerCtx
	}
	started := time.Now()
	result, err = executeReadinessWithArgs(ctx, runner, targetDir, sourceDir, plan.BranchSHA, plan.TargetSHA, timeout, runnerEnv, bind)
	duration = time.Since(started)
	// ReadinessDuration keeps reporting the whole runner-execution span,
	// while validation time accumulates inside it around the started process
	// alone.
	if siblingEvidence != "" && err == nil {
		result.Summary += "; siblings " + siblingEvidence
	}
	return result, duration, err
}

func mergeCleanupError(parent context.Context, current error, g gitRepo, dir string) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(parent), 30*time.Second)
	defer cancel()
	if err := g.worktreeRemoveForce(cleanupCtx, dir); err != nil {
		removeErr := fmt.Errorf("cleanup readiness worktree: %w", err)
		fsErr := os.RemoveAll(dir)
		return errors.Join(current, removeErr, fsErr)
	}
	return current
}

func executeReadinessWithTimeout(parent context.Context, runner, targetDir, sourceDir, sourceSHA, targetSHA string, timeout time.Duration) (readinessResult, error) {
	return executeReadinessWithEnvironment(parent, runner, targetDir, sourceDir, sourceSHA, targetSHA, timeout, readinessEnv(os.Environ()))
}

func executeReadinessWithEnvironment(parent context.Context, runner, targetDir, sourceDir, sourceSHA, targetSHA string, timeout time.Duration, env []string) (readinessResult, error) {
	return executeReadinessWithArgs(parent, runner, targetDir, sourceDir, sourceSHA, targetSHA, timeout, env, nil)
}

// executeReadinessWithArgs runs the json-v1 execution. extra carries only the
// fixed plan-v1 binding arguments; it is never caller-controlled text.
func executeReadinessWithArgs(parent context.Context, runner, targetDir, sourceDir, sourceSHA, targetSHA string, timeout time.Duration, env, extra []string) (readinessResult, error) {
	args := append([]string{"--source-dir", sourceDir, "--source-sha", sourceSHA, "--target-sha", targetSHA, "--result-format", "json-v1"}, extra...)
	// Validation time is the successfully started runner process alone,
	// recorded immediately after Wait and before the output-limit checks
	// and JSON decode. A Start failure never launched a process and
	// adds exactly zero.
	out, err := runReadinessProcess(parent, runner, targetDir, args, timeout, env, "contract_runner", func(elapsed time.Duration) { accumulateValidation(parent, elapsed) })
	if err != nil {
		return readinessResult{Status: "unavailable", stderr: out.stderr, failure: out.failure}, err
	}
	result, err := parseReadinessResult(out.stdout)
	if err != nil {
		return readinessResult{Status: "unavailable", stderr: out.stderr}, err
	}
	result.stderr = out.stderr
	return result, nil
}

// planReadinessResources runs the slot-free plan-v1 invocation. Its time is
// reported through elapsed, not as validation.
func planReadinessResources(parent context.Context, runner, targetDir, sourceDir, sourceSHA, targetSHA string, timeout time.Duration, env []string, elapsed *time.Duration) (readinessPlanResult, readinessProcessOutput, error) {
	args := []string{"--source-dir", sourceDir, "--source-sha", sourceSHA, "--target-sha", targetSHA, "--result-format", "plan-v1"}
	out, err := runReadinessProcess(parent, runner, targetDir, args, timeout, env, "contract_plan", func(d time.Duration) { *elapsed += d })
	if err != nil {
		return readinessPlanResult{}, out, err
	}
	plan, err := parseReadinessPlan(out.stdout)
	return plan, out, err
}

type readinessProcessOutput struct {
	stdout, stderr []byte
	failure        *ReadinessFailure
}

func runReadinessProcess(parent context.Context, runner, targetDir string, args []string, timeout time.Duration, env []string, stage string, record func(time.Duration)) (readinessProcessOutput, error) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, runner, args...) // #nosec G204 -- runner is target-owned, validated Git blob; fixed arguments
	cmd.Dir = targetDir
	cmd.Env = env
	cmd.Env = hostSlotsHeldForChild(parent, cmd.Env)
	configureReadinessProcess(cmd)
	var cancellation terminationRequestRecorder
	cmd.Cancel = func() error {
		return cancellation.cancel(readinessTerminationRequest(), func() error { return killReadinessProcess(cmd.Process) })
	}
	cmd.WaitDelay = readinessWaitDelay
	var stdout, stderr limitedBuffer
	stdout.limit, stderr.limit = readinessMaxIO, readinessMaxIO
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	started := time.Now()
	runErr := cmd.Start()
	if runErr == nil {
		runErr = cmd.Wait()
		record(time.Since(started))
	}
	elapsed := time.Since(started)
	out := readinessProcessOutput{stderr: bytes.Clone(stderr.Bytes())}
	if ctx.Err() == context.DeadlineExceeded && parent.Err() == nil {
		out.failure = &ReadinessFailure{
			Stage:              stage,
			Class:              "timeout",
			BudgetMS:           int(timeout.Milliseconds()),
			ElapsedMS:          int(elapsed.Milliseconds()),
			LeafExitCode:       nil,
			TerminationRequest: cancellation.snapshot(),
		}
		return out, fmt.Errorf("runner timed out")
	}
	if ctx.Err() != nil {
		return out, ctx.Err()
	}
	if stdout.exceeded || stderr.exceeded {
		return out, fmt.Errorf("runner output exceeded limit")
	}
	if runErr != nil {
		return out, fmt.Errorf("runner failed: %s", boundedDetail(stderr.String()))
	}
	out.stdout = stdout.Bytes()
	return out, nil
}

type limitedBuffer struct {
	bytes.Buffer
	limit    int
	exceeded bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.limit {
		b.exceeded = true
		n := b.limit - b.Len()
		if n > 0 {
			_, _ = b.Buffer.Write(p[:n])
		}
		return len(p), nil
	}
	return b.Buffer.Write(p)
}

func parseReadinessResult(data []byte) (readinessResult, error) {
	if len(data) == 0 || len(data) > readinessMaxIO || !utf8.Valid(data) || bytes.IndexFunc(data, func(r rune) bool { return r < 0x20 && r != '\n' && r != '\r' && r != '\t' }) >= 0 {
		return readinessResult{}, fmt.Errorf("runner emitted invalid JSON")
	}
	raw, err := decodeReadinessResultObject(data)
	if err != nil {
		return readinessResult{}, err
	}
	if len(raw) != 3 || raw["version"] == nil || raw["status"] == nil || raw["summary"] == nil {
		return readinessResult{}, fmt.Errorf("runner result has unknown or missing fields")
	}
	var result readinessResult
	if err := json.Unmarshal(data, &result); err != nil {
		return readinessResult{}, fmt.Errorf("decode runner result: %w", err)
	}
	if result.Version != 1 || (result.Status != "ready" && result.Status != "not_ready" && result.Status != "unavailable") || !validSummary(result.Summary) {
		return readinessResult{}, fmt.Errorf("runner result violates json-v1")
	}
	return result, nil
}

func decodeReadinessResultObject(data []byte) (map[string]json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		return nil, fmt.Errorf("runner result is not a JSON object")
	}
	raw := make(map[string]json.RawMessage, 3)
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("runner result is not valid JSON: %w", err)
		}
		name, ok := key.(string)
		if !ok || raw[name] != nil {
			return nil, fmt.Errorf("runner result has duplicate or invalid fields")
		}
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return nil, fmt.Errorf("runner result is not valid JSON: %w", err)
		}
		raw[name] = value
	}
	if _, err := dec.Token(); err != nil {
		return nil, fmt.Errorf("runner result is not valid JSON: %w", err)
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		return nil, fmt.Errorf("runner emitted trailing data")
	}
	return raw, nil
}

func validSummary(s string) bool {
	if strings.TrimSpace(s) == "" || len(s) > readinessMaxSummary {
		return false
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

func boundedDetail(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, ansiEscape.ReplaceAllString(s, ""))
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return "non-zero exit"
	}
	return string([]rune(s)[:min(len([]rune(s)), 512)])
}

func readinessEnv(env []string) []string {
	blocked := map[string]bool{"BASH_ENV": true, "ENV": true, "CDPATH": true, "PROMPT_COMMAND": true, "GIT_DIR": true, "GIT_WORK_TREE": true, "GIT_INDEX_FILE": true}
	out := make([]string, 0, len(env)+2)
	for _, item := range env {
		key, _, found := strings.Cut(item, "=")
		if found && !blocked[key] && key != "LC_ALL" && key != "LANG" {
			out = append(out, item)
		}
	}
	return append(out, "LC_ALL=C", "LANG=C")
}
