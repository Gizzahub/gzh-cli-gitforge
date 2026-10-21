// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

const (
	checkPass = "PASS"
	checkFail = "FAIL"
	checkWarn = "WARN"
	checkSkip = "SKIP"
)

// gateModeNone is the only gate mode recorded in CheckReport.GateMode and the
// readiness provenance row: gz-git builds, executes, and lints no repository
// code, and the git checks alone decide readiness. Repository verification
// belongs to the task runtime, which runs it before gz-git is called. The
// former contract-v1 mode is removed.
const gateModeNone = "none"

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
	// SHA compared is the SHA every check row examines.
	ExpectSource string
	// ExpectTarget is the full destination commit the caller approved. It is
	// intentionally separate from ExpectSource: an immutable request names a
	// source/target pair and must never silently adopt a newer target tip.
	ExpectTarget string
	// LockNotice receives the periodic holder notice while waiting; nil
	// means os.Stderr.
	LockNotice io.Writer
	// preMutation is internal Run-only state. Check never writes it durably;
	// Run records it only after a classified refusal before push.
	preMutation *preMutationCapture
	// requireConfirmedTarget is Run-only; standalone Check keeps heuristic WARNs.
	requireConfirmedTarget bool
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
	// GateMode is the readiness provenance mode; always gateModeNone.
	GateMode   string
	Controller *controllerBinding
}

// Check answers whether the branch can land on the target.
func Check(ctx context.Context, exec *gitcmd.Executor, opts CheckOptions) (report *CheckReport, err error) {
	if exec == nil {
		return nil, fmt.Errorf("git executor is nil")
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
		// Freshness is a cheap prerequisite for every check below; a stale
		// source must be refreshed before anything else is worth reporting.
		report.Ready = false
		return report, nil
	}

	_, mergeItem := checkMergeTree(ctx, g, plan)
	add(mergeItem)
	for _, item := range checkOtherBranches(ctx, g, plan) {
		add(item)
	}
	add(checkWorkingTree(ctx, g, plan))
	add(checkPushed(ctx, g, plan))

	report.GateMode = gateModeNone
	add(CheckItem{Name: "readiness contract", Status: checkPass, Detail: "none declared; gz-git runs no repository code, git checks decide readiness"})
	// gz-git runs no repository code: the git checks above and the
	// revalidation below decide READY. The target lock protects local
	// ownership, not a remote writer, so fetch and compare again before
	// reporting READY. Run repeats this check before its lease push.
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
	if r.GateMode != "" {
		fmt.Fprintf(&b, "  INFO  readiness provenance — mode=%s source=%s target=%s\n", r.GateMode, r.Plan.BranchSHA, r.Plan.TargetSHA)
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
