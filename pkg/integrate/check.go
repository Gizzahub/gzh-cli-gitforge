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
	"sync"
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

// Gate modes recorded in CheckReport.GateMode and the readiness provenance
// row. gateModeNone means neither side declares a readiness contract: gz-git
// builds, executes, and lints no repository code, and the git checks alone
// decide readiness.
const (
	gateModeContractV1 = "contract-v1"
	gateModeNone       = "none"
)

// CheckOptions configures a read-only readiness check.
type CheckOptions struct {
	RepoPath          string
	Branch            string
	Target            string
	DirectToDefault   bool
	Release           bool
	IntegrationConfig []string
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
	readinessObservation *readinessObservation
	resourceWaitObserver func(context.Context) error
	resourceWaitError    *error
	// preMutation is internal Run-only state. Check never writes it durably;
	// Run records it only after a classified refusal before push.
	preMutation *preMutationCapture
	// requireConfirmedTarget is Run-only; standalone Check keeps heuristic WARNs.
	requireConfirmedTarget bool
}

// validationDurationKey carries Check's validation-time accumulator. Unlike
// the resource wait, gate runtime is measured on paths that never see
// CheckOptions (runner helpers), so the accumulator travels on the context
// every gate already receives.
type validationDurationKey struct{}

// withValidationDuration installs the accumulator Check owns for one report.
func withValidationDuration(ctx context.Context, elapsed *time.Duration) context.Context {
	return context.WithValue(ctx, validationDurationKey{}, elapsed)
}

// preparationDurationKey carries the preparation-time accumulator the same
// way, because profiles run below helpers that only receive a context.
type preparationDurationKey struct{}

func withPreparationDuration(ctx context.Context, elapsed *time.Duration) context.Context {
	return context.WithValue(ctx, preparationDurationKey{}, elapsed)
}

func accumulatePreparation(ctx context.Context, elapsed time.Duration) {
	if acc, ok := ctx.Value(preparationDurationKey{}).(*time.Duration); ok && acc != nil {
		*acc += elapsed
	}
}

// accumulateValidation adds one measured gate run to Check's accumulator.
// A context without one (direct runner unit tests) records nothing.
func accumulateValidation(ctx context.Context, elapsed time.Duration) {
	if acc, ok := ctx.Value(validationDurationKey{}).(*time.Duration); ok && acc != nil {
		*acc += elapsed
	}
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
	// ValidationDuration records the time actually spent inside gate
	// subprocesses — contract runner executions — separately from the time
	// spent waiting
	// for a declared high-load resource slot. Gate preparation, Git
	// inspection, cleanup, push, and reclaim are outside it.
	ValidationDuration time.Duration
	// PreparationDuration is time spent inside preparation profiles after any
	// slot was granted; the wait itself stays in ResourceWaitDuration.
	PreparationDuration time.Duration
	// PlanDuration is the slot-free plan-v1 runner span. PlanResource is
	// what the runner asked for, PlanDigest binds it to the execution, and
	// ResourceClass is what gz-git granted (high when the slot was held).
	PlanDuration   time.Duration
	PlanResource   string
	PlanDigest     string
	ResourceClass  string
	PrepareProfile string
	PrepareInputs  string
	// ReadinessFailures contains only typed subprocess timeouts. Human
	// CheckItem.Detail remains diagnostic prose and is never parsed back into
	// protocol data.
	ReadinessFailures []ReadinessFailure
	Controller        *controllerBinding
}

// terminationRequestRecorder observes the Cmd.Cancel callback rather than
// inferring a kill from a deadline. os/exec may finish before cancellation,
// so a timeout can legitimately have a nil termination request.
type terminationRequestRecorder struct {
	mu      sync.Mutex
	request *TerminationRequest
}

func (r *terminationRequestRecorder) cancel(request TerminationRequest, fn func() error) error {
	r.mu.Lock()
	requestCopy := request
	r.request = &requestCopy
	r.mu.Unlock()
	return fn()
}

func (r *terminationRequestRecorder) snapshot() *TerminationRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.request == nil {
		return nil
	}
	observed := *r.request
	return &observed
}

func (r *CheckReport) recordReadinessFailure(failure *ReadinessFailure) {
	if failure == nil {
		return
	}
	r.ReadinessFailures = append(r.ReadinessFailures, *failure)
}

// Check answers whether the branch can land on the target.
func Check(ctx context.Context, exec *gitcmd.Executor, opts CheckOptions) (report *CheckReport, err error) {
	if exec == nil {
		return nil, fmt.Errorf("git executor is nil")
	}
	if opts.LockWait < 0 {
		return nil, fmt.Errorf("--lock-wait must not be negative")
	}
	if err := normalizeExpectedApprovals(&opts); err != nil {
		return nil, err
	}
	g, root, err := openCheckRepo(ctx, exec, opts.RepoPath)
	if err != nil {
		return nil, err
	}

	repositoryTarget := strings.TrimSpace(opts.ControllerConfig) == "" && len(opts.IntegrationConfig) == 0
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
	if err := opts.preMutation.capture(ctx, g, preflight); err != nil {
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
	if err := validateOwnedCheckPlan(ctx, g, root, preflight, plan, controller, decl, repositoryTarget); err != nil {
		return nil, err
	}

	report = &CheckReport{Plan: plan, Controller: controller}
	// Resource-slot expiry is a typed check failure even when the readiness
	// runner turns it into a diagnostic item. Keep the first acquisition error
	// so callers can select the tool-error exit code after cleanup completes.
	opts.resourceWaitError = new(error)
	defer func() { err = applyResourceWaitError(err, *opts.resourceWaitError) }()
	// The accumulator is unconditional: every gate mode records its runtime,
	// so a report always distinguishes "waited for the slot" from "ran the
	// gate", including a zero for a gate that never launched a subprocess.
	ctx = withValidationDuration(ctx, &report.ValidationDuration)
	ctx = withPreparationDuration(ctx, &report.PreparationDuration)
	add := func(item CheckItem) {
		report.Items = append(report.Items, item)
		switch item.Status {
		case checkFail:
			report.Failures++
		case checkWarn:
			report.Warnings++
		}
	}
	addIntegrationTargetWarning(plan, add)

	freshness := checkFreshness(ctx, g, plan)
	add(freshness)
	if freshness.Status == checkFail {
		// Freshness is a cheap prerequisite for every measurement below. Do
		// not execute repository-owned readiness or preparation code
		// against a source that cannot be integrated until it is refreshed.
		report.Ready = false
		return report, nil
	}

	resourceHighLoad, err := resourceHighLoadForPlan(ctx, g, plan)
	if err != nil {
		return nil, err
	}
	opts = configureMeasurementOptions(g, plan, report, opts)

	_, mergeItem := checkMergeTree(ctx, g, plan)
	add(mergeItem)
	for _, item := range checkOtherBranches(ctx, g, plan) {
		add(item)
	}
	add(checkWorkingTree(ctx, g, plan))
	add(checkPushed(ctx, g, plan))

	contractItem := checkReadinessContractWithResource(ctx, g, plan, report, resourceHighLoad, opts)
	add(contractItem)
	// Without a contract (gateModeNone) gz-git runs no repository code: the
	// git checks above and the post-gate revalidation below decide READY.
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

func applyResourceWaitError(err, resourceWaitError error) error {
	if resourceWaitError == nil {
		return err
	}
	if err == nil {
		return resourceWaitError
	}
	if !errors.Is(err, resourceWaitError) {
		return errors.Join(err, resourceWaitError)
	}
	return err
}

// openCheckRepo binds the executor to the requested repository path and
// returns the handle rooted at the discovered toplevel. Check calls it only
// after its argument validation so repo-inspection errors surface in the
// same order as before the extraction.
func openCheckRepo(ctx context.Context, exec *gitcmd.Executor, repoPath string) (gitRepo, string, error) {
	dir := strings.TrimSpace(repoPath)
	if dir == "" {
		dir = "."
	}
	g := newGitRepo(exec, dir)
	if !g.isRepo(ctx) {
		return gitRepo{}, "", fmt.Errorf("not a git repository: %s", dir)
	}
	root, err := g.toplevel(ctx)
	if err != nil {
		return gitRepo{}, "", err
	}
	g.dir = root
	return g, root, nil
}

func configureMeasurementOptions(g gitRepo, plan TargetPlan, report *CheckReport, opts CheckOptions) CheckOptions {
	opts.resourceWait = &report.ResourceWaitDuration
	if opts.LockNotice == nil {
		opts.LockNotice = os.Stderr
	}
	opts.resourceWaitObserver = targetWaitObserver(g, plan, opts.NoFetch, opts.LockNotice)
	return opts
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

func targetMovedWhileWaiting(err error) bool {
	return errors.Is(err, errTargetMovedWhileWaiting)
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
		fmt.Fprintf(&b, ", integration=%s", formatIntegrationResolution(r.Plan.Integration))
	}
	b.WriteString(")\n\n")
	for _, item := range r.Items {
		fmt.Fprintf(&b, "  %s  %s — %s\n", item.Status, item.Name, item.Detail)
	}
	if r.ResourceWaitDuration > 0 {
		fmt.Fprintf(&b, "  INFO  resource wait — Duration=%s\n", r.ResourceWaitDuration.Round(time.Millisecond))
	}
	if r.PreparationDuration > 0 {
		fmt.Fprintf(&b, "  INFO  preparation — Duration=%s\n", r.PreparationDuration.Round(time.Millisecond))
	}
	if r.PlanResource != "" || r.PlanDuration > 0 {
		// A failed plan has spent time but granted nothing; report it with
		// dashes so the human report agrees with plan_duration_ms.
		requested, granted, digest := r.PlanResource, r.ResourceClass, r.PlanDigest
		if requested == "" {
			requested, granted, digest = "-", "-", "-"
		}
		fmt.Fprintf(&b, "  INFO  resource plan — Requested=%s Granted=%s PlanDigest=%s Duration=%s\n", requested, granted, digest, r.PlanDuration.Round(time.Millisecond))
	}
	if r.ValidationDuration > 0 {
		fmt.Fprintf(&b, "  INFO  validation — Duration=%s\n", r.ValidationDuration.Round(time.Millisecond))
	}
	if r.GateMode != "" {
		fmt.Fprintf(&b, "  INFO  readiness provenance — mode=%s source=%s target=%s", r.GateMode, r.Plan.BranchSHA, r.Plan.TargetSHA)
		if r.GateMode == gateModeContractV1 {
			fmt.Fprintf(&b, " ManifestPath=%s ContractDigest=%s ManifestOID=%s RunnerPath=%s RunnerOID=%s ReadinessTreeOID=%s Status=%s Duration=%s", r.ManifestPath, r.ContractDigest, r.ManifestOID, r.RunnerPath, r.RunnerOID, r.ReadinessTreeOID, r.ReadinessStatus, r.ReadinessDuration.Round(time.Millisecond))
			if r.ResourceClass != "" {
				fmt.Fprintf(&b, " ResourceClass=%s", r.ResourceClass)
			}
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

func formatIntegrationResolution(integration Resolution) string {
	source := integration.Source
	if source == SourceHeuristic {
		source += ": remote HEAD"
	}
	return fmt.Sprintf("%s (%s)", integration.Name, source)
}

func addIntegrationTargetWarning(plan TargetPlan, add func(CheckItem)) {
	if plan.Integration.Source != SourceHeuristic {
		return
	}
	add(CheckItem{
		Name:   "integration target",
		Status: checkWarn,
		Detail: fmt.Sprintf("%s is inferred from the remote HEAD; integrate run requires --target %s --direct-to-default, branch.integrationBranch, or --controller-config", plan.Integration.Name, plan.Integration.Name),
	})
}
