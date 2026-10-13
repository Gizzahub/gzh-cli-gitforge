// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

var errTargetMovedWhileWaiting = errors.New("target branch changed while waiting for resource slot; re-run check")

const (
	checkPass = "PASS"
	checkFail = "FAIL"
	checkWarn = "WARN"
	checkSkip = "SKIP"
)

// CheckOptions configures a read-only readiness check.
type CheckOptions struct {
	RepoPath           string
	Branch             string
	Target             string
	DirectToDefault    bool
	Release            bool
	AllowSkippedChecks bool
	IntegrationConfig  []string
	// ControllerConfig is an explicit devbox/controller file. It is never
	// discovered from ancestors and never inherits repository readiness.
	ControllerConfig string
	// NoFetch skips git fetch of the remote before resolving the target.
	// The target is resolved from the local remote-tracking ref; when that
	// ref is absent the check fails instead of falling back to a stale
	// local integration branch.
	NoFetch bool
	// ExpectSource is the full commit the caller approved. When set, the
	// source is read after the fetch and must be exactly this commit, so the
	// SHA compared is the SHA every readiness row measures.
	ExpectSource string
	// ExpectTarget is the full destination commit the caller approved. It is
	// intentionally separate from ExpectSource: an immutable request names a
	// source/target pair and must never silently adopt a newer target tip.
	ExpectTarget string
	// LockWait bounds one high-load host-slot acquisition. LockWaitSet keeps
	// an explicit zero (unbounded) distinct from an omitted flag.
	LockWait    time.Duration
	LockWaitSet bool
	// LockNotice receives the periodic holder notice while waiting; nil
	// means os.Stderr.
	LockNotice           io.Writer
	resourceWait         *time.Duration
	resourceWaitObserver func(context.Context) error
}

// CheckItem is one readiness row.
type CheckItem struct {
	Name   string
	Status string
	Detail string
}

// CheckReport is the readiness answer. It never pushes or reclaims.
type CheckReport struct {
	Ready     bool
	Plan      TargetPlan
	Items     []CheckItem
	Failures  int
	Warnings  int
	RootFacts []string
	// Gate provenance is recorded so run can repeat the exact checked state.
	GateMode             string
	ManifestPath         string
	ContractDigest       string
	ManifestOID          string
	RunnerPath           string
	RunnerOID            string
	ReadinessTreeOID     string
	ReadinessStatus      string
	ReadinessDuration    time.Duration
	ResourceWaitDuration time.Duration
	PrepareProfile       string
	PrepareInputs        string
	// Baselines records, for each legacy make target, whether the target
	// baseline was measured and why. A source that passed is not compared
	// with anything, so its record says the baseline was not measured
	// instead of leaving the absence unexplained.
	Baselines  []BaselineRecord
	Controller *controllerBinding
}

// BaselineRecord states whether one make target's baseline was measured.
type BaselineRecord struct {
	Target   string
	Measured bool
	Reason   string
}

const (
	baselineSkippedSourcePassed = "not measured: source passed"
	baselineSkippedUndefined    = "not measured: make target undefined"
	baselineSkippedSourceAlone  = "not measured: the source verdict compares nothing"
	baselineMeasuredSourceFail  = "measured: source failed"
	baselineMeasuredOutcomes    = "measured: outcome report declared"
)

// baselineRecordFor names why the baseline was or was not measured for
// probe. needed is the negation of judgeMakeWithoutBaseline's decision, so
// the record cannot disagree with what was actually run.
func baselineRecordFor(probe makeProbe, needed bool) BaselineRecord {
	record := BaselineRecord{Target: probe.Target, Measured: needed}
	switch {
	case needed && probe.Err == nil:
		record.Reason = baselineMeasuredOutcomes
	case needed:
		record.Reason = baselineMeasuredSourceFail
	case !probe.Defined:
		record.Reason = baselineSkippedUndefined
	case probe.Err == nil && !probe.Skipped && !probe.TimedOut && probe.Unavailable == "" && probe.ToolCrash == "" && probe.MissingCD == "":
		record.Reason = baselineSkippedSourcePassed
	default:
		record.Reason = baselineSkippedSourceAlone
	}
	return record
}

// Check answers whether the branch can land on the target.
func Check(ctx context.Context, exec *gitcmd.Executor, opts CheckOptions) (*CheckReport, error) {
	if exec == nil {
		return nil, fmt.Errorf("git executor is nil")
	}
	if opts.LockWait < 0 {
		return nil, fmt.Errorf("--lock-wait must not be negative")
	}
	if opts.ExpectSource != "" {
		expect, err := NormalizeExpectSource(opts.ExpectSource)
		if err != nil {
			return nil, err
		}
		opts.ExpectSource = expect
	}
	if opts.ExpectTarget != "" {
		expect, err := NormalizeExpectTarget(opts.ExpectTarget)
		if err != nil {
			return nil, err
		}
		opts.ExpectTarget = expect
	}
	dir := strings.TrimSpace(opts.RepoPath)
	if dir == "" {
		dir = "."
	}
	g := newGitRepo(exec, dir)
	if !g.isRepo(ctx) {
		return nil, fmt.Errorf("not a git repository: %s", dir)
	}
	root, err := g.toplevel(ctx)
	if err != nil {
		return nil, err
	}
	g.dir = root

	controller, err := resolveCheckController(ctx, g, &opts)
	if err != nil {
		return nil, err
	}
	decl, err := config.LoadRepoRootTaskPattern(root)
	if err != nil {
		return nil, err
	}
	if len(opts.IntegrationConfig) == 0 {
		opts.IntegrationConfig = decl.IntegrationBranch
	}

	preflight, err := preflightTargetRef(ctx, g, exec, opts, controller)
	if err != nil {
		return nil, err
	}
	releaseTarget, err := holdTargetLock(ctx, g, preflight.targetRef, opts.LockNotice)
	if err != nil {
		return nil, err
	}
	defer releaseTarget()

	plan, err := resolveTarget(ctx, g, exec, opts)
	if err != nil {
		return nil, err
	}
	if actual := "refs/heads/" + targetBranchName(plan.Target, plan.Remote); actual != preflight.targetRef || plan.Branch != preflight.branch || plan.Remote != preflight.remote {
		return nil, fmt.Errorf("integration source or target changed while acquiring ownership; re-run check")
	}
	resourceHighLoad, prepareHighLoad, err := resourceDeclarationsForPlan(ctx, g, plan)
	if err != nil {
		return nil, err
	}

	report := &CheckReport{Plan: plan, Controller: controller}
	opts.resourceWait = &report.ResourceWaitDuration
	if opts.LockNotice == nil {
		opts.LockNotice = os.Stderr
	}
	opts.resourceWaitObserver = targetWaitObserver(g, plan, opts.NoFetch, opts.LockNotice)
	add := func(item CheckItem) {
		report.Items = append(report.Items, item)
		switch item.Status {
		case checkFail:
			report.Failures++
		case checkWarn:
			report.Warnings++
		}
	}

	add(checkFreshness(ctx, g, plan))
	_, mergeItem := checkMergeTree(ctx, g, plan)
	add(mergeItem)
	for _, item := range checkOtherBranches(ctx, g, plan) {
		add(item)
	}
	add(checkWorkingTree(ctx, g, plan))
	add(checkPushed(ctx, g, plan))

	contractItem := checkReadinessContractWithResource(ctx, g, plan, report, resourceHighLoad, opts)
	add(contractItem)
	switch report.GateMode {
	case "contract-v1":
		// The contract runner deliberately replaces every head-owned Makefile.
	case "legacy-make":
		checkLegacyMake(ctx, g, plan, controller, opts.AllowSkippedChecks, resourceHighLoad, prepareHighLoad, opts, report, add)
	}
	// A high-load gate may run long enough for either ref to move. The target
	// lock protects local ownership, not a remote writer, so fetch and compare
	// again before reporting READY. Run repeats this check before its lease push.
	if _, _, _, err := revalidateCheckedRefsForExpectedTarget(ctx, g, report, opts.NoFetch, opts.ExpectTarget); err != nil {
		if opts.ExpectTarget != "" && errors.Is(err, ErrTargetMismatch) {
			return report, err
		}
		add(CheckItem{Name: "post-gate freshness", Status: checkFail, Detail: err.Error()})
	} else {
		add(CheckItem{Name: "post-gate freshness", Status: checkPass, Detail: "source and target unchanged"})
	}
	report.Ready = report.Failures == 0
	return report, nil
}

func resourceDeclarationsForPlan(ctx context.Context, g gitRepo, plan TargetPlan) (gate, prepare bool, err error) {
	resourceHighLoad, err := resourceHighLoadForPlan(ctx, g, plan)
	if err != nil {
		return false, false, err
	}
	prepareHighLoad, err := prepareHighLoadForPlan(ctx, g, plan)
	if err != nil {
		return false, false, err
	}
	return resourceHighLoad, prepareHighLoad, nil
}

func checkLegacyMake(ctx context.Context, g gitRepo, plan TargetPlan, controller *controllerBinding, allowSkipped, resourceHighLoad, prepareHighLoad bool, opts CheckOptions, report *CheckReport, add func(CheckItem)) {
	if plan.HeadSHA != plan.BranchSHA && !plan.releasesRemoteRef() {
		add(CheckItem{Name: "make", Status: checkFail, Detail: "HEAD is not the branch; cannot run tests"})
		return
	}
	decl, err := config.LoadRepoRootTaskPattern(g.dir)
	if err != nil {
		add(CheckItem{Name: "make budget", Status: checkFail, Detail: err.Error()})
		return
	}
	budget := resolveMakeBudget(decl.MakeTimeout)
	outcomes, err := resolveMakeOutcomePolicy(ctx, g, plan)
	if err != nil {
		add(CheckItem{Name: "make outcome declaration", Status: checkFail, Detail: err.Error()})
		return
	}
	profile, err := resolvePrepareProfile(ctx, g, plan, controller)
	if err != nil {
		add(CheckItem{Name: "prepare declaration", Status: checkFail, Detail: err.Error()})
		return
	}
	if resourceHighLoad {
		ctx = withResourceScope(ctx, g, opts)
	}
	prepared, err := prepareLegacySourceWithResource(ctx, g, plan, profile, prepareHighLoad, opts)
	if err != nil {
		add(CheckItem{Name: "prepare", Status: checkFail, Detail: err.Error()})
		return
	}
	report.PrepareProfile = profile
	if profile != "" {
		report.PrepareInputs = prepared.evidence()
		add(CheckItem{Name: "prepare profile", Status: checkPass, Detail: report.PrepareInputs})
	}
	// The source is measured first, for every target. The target baseline is
	// then measured only for the make targets whose verdict reads it, so a
	// source that passes never prepares the target at all.
	targets := []string{"check", "lint"}
	probes := make(map[string]makeProbe, len(targets))
	var needed []string
	for _, target := range targets {
		probe := prepared.annotateProbe(ctx, runMakeTargetWithOutcomes(ctx, prepared.source, target, budget, outcomes.source.Includes(target)))
		probe.OutcomesComparable = outcomes.target.Includes(target)
		probes[target] = probe
		_, decided := judgeMakeWithoutBaseline(probe, allowSkipped)
		if !decided {
			needed = append(needed, target)
		}
		report.Baselines = append(report.Baselines, baselineRecordFor(probe, !decided))
	}
	baseline, baselineErr := prepared.measureBaseline(ctx, plan, needed, budget, outcomes.target)
	cleanupErr := prepared.cleanup(ctx)
	declared := 0
	for _, target := range targets {
		probe := probes[target]
		item, decided := judgeMakeWithoutBaseline(probe, allowSkipped)
		switch {
		case decided:
		case baselineErr != nil:
			// A baseline that could not even be prepared is not a verdict
			// about the target commit, and never a pass for the source.
			item = CheckItem{Name: "make " + target, Status: checkFail, Detail: "prepare baseline: " + baselineErr.Error()}
		default:
			item = judgeMakeAgainstProbe(ctx, g, plan, probe, allowSkipped, baseline[target], budget)
		}
		item = saveLegacyMakeDiagnostic(item, probe)
		if item.Status != checkSkip {
			declared++
			add(item)
		}
	}
	if cleanupErr != nil {
		add(CheckItem{Name: "prepare cleanup", Status: checkFail, Detail: cleanupErr.Error()})
	}
	if declared == 0 {
		status := checkFail
		detail := "undefined — this repo declares no integration gate"
		if allowSkipped {
			status = checkWarn
			detail += "; allowed by --allow-skipped-checks"
		}
		add(CheckItem{Name: "make check/lint", Status: status, Detail: detail})
	}
}

// targetWaitObserver refreshes only the resolved destination while a declared
// high-load gate is waiting for capacity. A transient fetch failure is a
// warning because the next interval can still establish the target; an actual
// moved target stops before any gate subprocess begins.
func targetWaitObserver(g gitRepo, plan TargetPlan, noFetch bool, notice io.Writer) func(context.Context) error {
	var fetchWarned bool
	return func(ctx context.Context) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if plan.Remote != "" && !noFetch {
			refreshed, err := refreshWaitingTarget(ctx, g, plan, notice, &fetchWarned)
			if err != nil || !refreshed {
				return err
			}
		}
		targetRef := plan.Target
		if plan.Remote != "" {
			targetRef = plan.Remote + "/" + targetBranchName(plan.Target, plan.Remote)
		}
		sha, ok, err := g.revParse(ctx, targetRef)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err != nil {
			return err
		}
		if !ok || sha != plan.TargetSHA {
			return errTargetMovedWhileWaiting
		}
		return nil
	}
}

// refreshWaitingTarget distinguishes a canceled probe from a missing target.
// A transient refresh failure leaves the request waiting for the next interval.
func refreshWaitingTarget(ctx context.Context, g gitRepo, plan TargetPlan, notice io.Writer, fetchWarned *bool) (bool, error) {
	branch := targetBranchName(plan.Target, plan.Remote)
	exists, existsErr := g.remoteBranchExists(ctx, plan.Remote, branch)
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if existsErr == nil && !exists {
		return false, errTargetMovedWhileWaiting
	}
	fetchErr := g.fetchTargetRef(ctx, plan.Remote, plan.Target)
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if fetchErr != nil {
		if !*fetchWarned && notice != nil {
			fmt.Fprintf(notice, "integrate: warning: refresh target while waiting for resource slot: %v\n", fetchErr)
		}
		*fetchWarned = true
		return false, nil
	}
	return true, nil
}

func resourceWaitFailureDetail(err error) string {
	if errors.Is(err, errTargetMovedWhileWaiting) {
		return err.Error()
	}
	return "resource slot unavailable: " + err.Error()
}

func targetMovedWhileWaiting(err error) bool {
	return errors.Is(err, errTargetMovedWhileWaiting)
}

// saveLegacyMakeDiagnostic preserves branch Make output after the prepared
// worktree is removed. Baseline output is deliberately never passed here: it
// explains the comparison, while this diagnostic is the failed branch run the
// operator needs to inspect.
func saveLegacyMakeDiagnostic(item CheckItem, probe makeProbe) CheckItem {
	if item.Status != checkFail || probe.Output == "" {
		return item
	}
	path, err := writeDiagnostic("make-"+probe.Target, []byte(probe.Output))
	if err != nil {
		item.Detail += "; diagnostic save failed: " + err.Error()
		return item
	}
	item.Detail += "; diagnostic: " + path
	return item
}

func resolveCheckController(ctx context.Context, g gitRepo, opts *CheckOptions) (*controllerBinding, error) {
	if strings.TrimSpace(opts.ControllerConfig) == "" {
		return nil, nil //nolint:nilnil // an omitted optional controller has no binding
	}
	branch := opts.Branch
	if branch == "" {
		var err error
		branch, err = g.currentBranch(ctx)
		if err != nil {
			return nil, err
		}
	}
	controller, err := resolveController(ctx, g, opts.ControllerConfig, branch)
	if err != nil {
		return nil, err
	}
	opts.IntegrationConfig = append([]string(nil), controller.Integration...)
	controllerTarget := targetBranchName(opts.Target, controller.Remote)
	if opts.Target != "" && controllerTarget != controller.Integration[0] {
		return nil, fmt.Errorf("--target must equal controller integration branch %s", controller.Integration[0])
	}
	// The controller is authoritative: a spelling such as origin/develop
	// selects its declared branch, never a same-named repository setting.
	if opts.Target != "" {
		opts.Target = controller.Remote + "/" + controller.Integration[0]
	}
	return controller, nil
}

func checkFreshness(ctx context.Context, g gitRepo, plan TargetPlan) CheckItem {
	mb, err := g.mergeBase(ctx, plan.BranchSHA, plan.TargetSHA)
	if err != nil {
		return CheckItem{Name: "freshness", Status: checkFail, Detail: err.Error()}
	}
	if mb == plan.TargetSHA {
		return CheckItem{Name: "freshness", Status: checkPass, Detail: "based on " + plan.Target + " tip"}
	}
	n, err := g.revCount(ctx, mb+".."+plan.TargetSHA)
	if err != nil {
		return CheckItem{Name: "freshness", Status: checkFail, Detail: err.Error()}
	}
	return CheckItem{Name: "freshness", Status: checkFail, Detail: fmt.Sprintf("%d commits behind %s — git rebase %s", n, plan.Target, plan.Target)}
}

func checkMergeTree(ctx context.Context, g gitRepo, plan TargetPlan) (clean bool, item CheckItem) {
	ok, err := g.mergeTreeClean(ctx, plan.TargetSHA, plan.BranchSHA)
	if err != nil {
		return false, CheckItem{Name: "merge-tree", Status: checkFail, Detail: err.Error()}
	}
	if ok {
		return true, CheckItem{Name: "merge-tree", Status: checkPass, Detail: "no conflict with " + plan.Target}
	}
	return false, CheckItem{Name: "merge-tree", Status: checkFail, Detail: "conflicts with " + plan.Target}
}

func checkOtherBranches(ctx context.Context, g gitRepo, plan TargetPlan) []CheckItem {
	if plan.Remote == "" {
		return []CheckItem{{Name: "cross-merge", Status: checkPass, Detail: "no remote branches to compare"}}
	}
	refs, err := g.shortRefs(ctx, "refs/remotes/"+plan.Remote)
	if err != nil {
		return []CheckItem{{Name: "cross-merge", Status: checkWarn, Detail: err.Error()}}
	}
	var items []CheckItem
	others := 0
	for _, other := range refs {
		if other == plan.Target || other == plan.DefaultRef || other == plan.Remote || other == plan.Remote+"/HEAD" {
			continue
		}
		sha, ok, err := g.revParse(ctx, other)
		if err != nil || !ok || sha == plan.BranchSHA {
			continue
		}
		anc, ancErr := g.isAncestor(ctx, sha, plan.BranchSHA)
		if ancErr != nil || anc {
			continue
		}
		anc, ancErr = g.isAncestor(ctx, plan.BranchSHA, sha)
		if ancErr != nil || anc {
			continue
		}
		others++
		clean, err := g.mergeTreeClean(ctx, plan.BranchSHA, sha)
		if err != nil {
			items = append(items, CheckItem{Name: "cross-merge " + other, Status: checkWarn, Detail: err.Error()})
			continue
		}
		if clean {
			items = append(items, CheckItem{Name: "cross-merge " + other, Status: checkPass, Detail: "no conflict"})
			continue
		}
		items = append(items, CheckItem{Name: "cross-merge " + other, Status: checkWarn, Detail: "cross conflict"})
	}
	if others == 0 {
		return []CheckItem{{Name: "cross-merge", Status: checkPass, Detail: "no other open branches"}}
	}
	return items
}

func checkWorkingTree(ctx context.Context, g gitRepo, plan TargetPlan) CheckItem {
	// A remote release source is never checked out, but the checkout still
	// has to be clean: run fast-forwards the target worktree after the push.
	if plan.HeadSHA != plan.BranchSHA && !plan.releasesRemoteRef() {
		return CheckItem{Name: "working-tree", Status: checkFail, Detail: "HEAD is not the branch"}
	}
	out, err := g.porcelain(ctx)
	if err != nil {
		return CheckItem{Name: "working-tree", Status: checkFail, Detail: err.Error()}
	}
	if strings.TrimSpace(out) != "" {
		return CheckItem{Name: "working-tree", Status: checkFail, Detail: "uncommitted changes"}
	}
	return CheckItem{Name: "working-tree", Status: checkPass, Detail: "clean"}
}

func checkPushed(ctx context.Context, g gitRepo, plan TargetPlan) CheckItem {
	if plan.releasesRemoteRef() {
		return CheckItem{Name: "push", Status: checkPass, Detail: plan.Branch + " is the remote ref"}
	}
	upstream, hasUpstream, err := g.upstreamName(ctx, plan.Branch)
	if err != nil {
		return CheckItem{Name: "upstream", Status: checkFail, Detail: err.Error()}
	}
	if hasUpstream {
		remotes, remotesErr := g.remotes(ctx)
		if remotesErr != nil {
			return CheckItem{Name: "upstream", Status: checkFail, Detail: remotesErr.Error()}
		}
		if UpstreamTargetsIntegration(plan.Branch, upstream, plan.Integration, remotes) {
			remote := plan.Remote
			if remote == "" {
				remote = "origin"
			}
			taskRef := remote + "/" + plan.Branch
			_, taskRefExists, taskRefErr := g.revParse(ctx, taskRef)
			if taskRefErr != nil {
				return CheckItem{Name: "upstream", Status: checkFail, Detail: taskRefErr.Error()}
			}
			remediation := fmt.Sprintf("git push --set-upstream %s HEAD:refs/heads/%s", remote, plan.Branch)
			if taskRefExists {
				remediation = fmt.Sprintf("git branch --set-upstream-to=%s %s", taskRef, plan.Branch)
			}
			return CheckItem{
				Name:   "upstream",
				Status: checkFail,
				Detail: fmt.Sprintf("%s targets integration branch %s — %s", upstream, plan.Integration.Name, remediation),
			}
		}
	}

	up, ok, err := g.upstreamSHA(ctx, plan.Branch)
	if err != nil {
		return CheckItem{Name: "push", Status: checkFail, Detail: err.Error()}
	}
	if !ok {
		remote := plan.Remote
		if remote == "" {
			remote = "origin"
		}
		return CheckItem{Name: "push", Status: checkFail, Detail: fmt.Sprintf("no upstream — git push -u %s %s", remote, plan.Branch)}
	}
	if up != plan.BranchSHA {
		return CheckItem{Name: "push", Status: checkFail, Detail: "upstream differs — git push"}
	}
	return CheckItem{Name: "push", Status: checkPass, Detail: "pushed"}
}

// FormatCheck renders the readiness report.
func FormatCheck(r *CheckReport) string {
	if r == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "integrate check: %s  (target=%s", r.Plan.Branch, r.Plan.Target)
	if r.Plan.DefaultRef != "" {
		fmt.Fprintf(&b, ", default=%s", r.Plan.DefaultRef)
	}
	if r.Plan.Integration.Participates {
		fmt.Fprintf(&b, ", integration=%s", r.Plan.Integration.Name)
	}
	b.WriteString(")\n\n")
	for _, item := range r.Items {
		fmt.Fprintf(&b, "  %s  %s — %s\n", item.Status, item.Name, item.Detail)
	}
	for _, record := range r.Baselines {
		fmt.Fprintf(&b, "  INFO  baseline make %s — %s\n", record.Target, record.Reason)
	}
	if r.ResourceWaitDuration > 0 {
		fmt.Fprintf(&b, "  INFO  resource wait — Duration=%s\n", r.ResourceWaitDuration.Round(time.Millisecond))
	}
	if r.GateMode != "" {
		fmt.Fprintf(&b, "  INFO  readiness provenance — mode=%s source=%s target=%s", r.GateMode, r.Plan.BranchSHA, r.Plan.TargetSHA)
		if r.GateMode == "contract-v1" {
			fmt.Fprintf(&b, " ManifestPath=%s ContractDigest=%s ManifestOID=%s RunnerPath=%s RunnerOID=%s ReadinessTreeOID=%s Status=%s Duration=%s", r.ManifestPath, r.ContractDigest, r.ManifestOID, r.RunnerPath, r.RunnerOID, r.ReadinessTreeOID, r.ReadinessStatus, r.ReadinessDuration.Round(time.Millisecond))
		}
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	if r.Ready {
		fmt.Fprintf(&b, "READY to integrate into %s — %d warning(s)\n", r.Plan.Target, r.Warnings)
	} else {
		fmt.Fprintf(&b, "NOT READY — %d failure(s), %d warning(s)\n", r.Failures, r.Warnings)
	}
	return b.String()
}
