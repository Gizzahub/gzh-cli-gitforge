// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

// RunOptions configures a fast-forward integrate and reclaim.
type RunOptions struct {
	CheckOptions
	// Execution is the optional durable-evidence context (TASK-302). Absent
	// keeps the legacy run behavior; present, it must be complete and valid,
	// and the run records a receipt before it pushes.
	Execution *ExecutionContext
}

// RunReport is the result of integrate run.
type RunReport struct {
	Check      *CheckReport
	Source     string
	Target     string
	SHA        string
	Reclaim    ReclaimResult
	Printed    []string
	Integrated bool
	// Receipt is the durable evidence recorded for an execution-context run;
	// nil for a legacy run.
	Receipt *Receipt
	// PreMutationOutcome is set only when this invocation durably recorded a
	// verified refusal before it could enter the target push.
	PreMutationOutcome *PreMutationOutcome
}

const preMutationOutcomeWriteTimeout = 5 * time.Second

// recordCurrentPreMutationOutcome gives the evidence write a bounded context
// after the caller's lock-wait context has expired. Reusing the canceled
// context would turn a verified pre-push refusal into an unbound error.
func recordCurrentPreMutationOutcome(ctx context.Context, capture *preMutationCapture, phase string) (*PreMutationOutcome, error) {
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), preMutationOutcomeWriteTimeout)
	defer cancel()
	return recordPreMutationOutcome(writeCtx, capture, phase)
}

// Run fast-forwards the target and reclaims the task branch.
// Reclaim only matches declared taskPattern. No pattern → reclaim nothing.
func Run(ctx context.Context, exec *gitcmd.Executor, opts RunOptions) (*RunReport, error) {
	if exec == nil {
		return nil, fmt.Errorf("git executor is nil")
	}
	// A malformed or partial execution context is rejected before any check
	// or mutation: evidence identity is all-or-nothing.
	if err := opts.Execution.Validate(); err != nil {
		return nil, err
	}
	if opts.Execution != nil {
		opts.ExpectSource = opts.Execution.ExpectSource
	}
	if err := rejectUnconfirmedHeuristicTarget(ctx, exec, opts.CheckOptions); err != nil {
		return nil, err
	}
	taskFence, err := acquireTaskResourceFence(ctx, exec, opts.RepoPath, opts.Branch, opts.LockNotice)
	if err != nil {
		return nil, err
	}
	defer taskFence.release()
	if opts.Execution != nil {
		opts.preMutation = &preMutationCapture{execution: *opts.Execution}
		dir := strings.TrimSpace(opts.RepoPath)
		if dir == "" {
			dir = "."
		}
		g := newGitRepo(exec, dir)
		if !g.isRepo(ctx) {
			return nil, fmt.Errorf("not a git repository: %s", dir)
		}
		common, err := g.output(ctx, "rev-parse", "--path-format=absolute", "--git-common-dir")
		if err != nil || common == "" {
			if err == nil {
				err = fmt.Errorf("git common directory is empty")
			}
			return nil, fmt.Errorf("resolve execution run common directory: %w", err)
		}
		runLock, err := acquireExecutionRunLock(ctx, common, opts.Execution.ExecutionID)
		if err != nil {
			return nil, err
		}
		defer runLock.release()
	}
	// Target ownership survives readiness through the exact lease push and
	// reclaim. Resource slots are acquired only around declared high-load gates.
	ctx, lock := withTargetLockSlot(ctx)
	defer lock.release()
	opts.requireConfirmedTarget = true
	check, err := Check(ctx, exec, opts.CheckOptions)
	if err != nil {
		report := &RunReport{Check: check}
		if recordErr := recordPreMutationCheckFailure(ctx, opts, check, report, err); recordErr != nil {
			return report, recordErr
		}
		return report, err
	}
	report, err := runChecked(ctx, exec, opts, check)
	if err != nil && check != nil && !check.Ready {
		outcome, recordErr := recordCurrentPreMutationOutcome(ctx, opts.preMutation, preMutationPhaseReadinessRejected)
		if recordErr != nil {
			return report, fmt.Errorf("%w; record pre-mutation outcome: %w", err, recordErr)
		}
		if report != nil {
			report.PreMutationOutcome = outcome
		}
	}
	return report, err
}

func recordPreMutationCheckFailure(ctx context.Context, opts RunOptions, check *CheckReport, report *RunReport, checkErr error) error {
	phase := ""
	switch {
	case errors.Is(checkErr, errTargetLockWait):
		phase = preMutationPhaseTargetLockWait
	case opts.Execution != nil && errors.Is(checkErr, ErrHostLockWait) && check != nil && !check.Ready:
		phase = preMutationPhaseReadinessRejected
	default:
		return nil
	}
	outcome, err := recordCurrentPreMutationOutcome(ctx, opts.preMutation, phase)
	if err != nil {
		return fmt.Errorf("%w; record pre-mutation outcome: %w", checkErr, err)
	}
	report.PreMutationOutcome = outcome
	return nil
}

// rejectUnconfirmedHeuristicTarget keeps a remote HEAD fallback observable in
// Check but requires Run callers to confirm it before any readiness gate can
// execute. Explicit targets and controller policies are already confirmation.
func rejectUnconfirmedHeuristicTarget(ctx context.Context, exec *gitcmd.Executor, opts CheckOptions) error {
	if strings.TrimSpace(opts.Target) != "" || strings.TrimSpace(opts.ControllerConfig) != "" {
		return nil
	}
	if err := normalizeExpectedApprovals(&opts); err != nil {
		return err
	}
	g, root, err := openCheckRepo(ctx, exec, opts.RepoPath)
	if err != nil {
		return err
	}
	decl, err := config.LoadRepoRootTaskPattern(root)
	if err != nil {
		return err
	}
	if len(opts.IntegrationConfig) == 0 {
		opts.IntegrationConfig = decl.IntegrationBranch
	}
	integration, err := ResolveIntegrationBranch(ctx, exec, g.dir, opts.IntegrationConfig)
	if err != nil {
		return err
	}
	if integration.Source != SourceHeuristic {
		return nil
	}
	// Check resolves expectation mismatches before readiness gates. Preserve
	// that ordering while still stopping a heuristic run before a gate, merge,
	// or push.
	if opts.ExpectSource != "" || opts.ExpectTarget != "" {
		if _, err := resolveTarget(ctx, g, exec, opts); err != nil {
			return err
		}
	}
	return unconfirmedIntegrationTargetError(integration.Name)
}

// rejectHeuristicRunPlan rechecks the authoritative plan after ownership waits.
// The early precheck alone cannot authorize a declaration that changes later.
func rejectHeuristicRunPlan(plan TargetPlan, opts CheckOptions) error {
	if opts.requireConfirmedTarget && strings.TrimSpace(opts.Target) == "" && strings.TrimSpace(opts.ControllerConfig) == "" && plan.Integration.Source == SourceHeuristic {
		return unconfirmedIntegrationTargetError(plan.Integration.Name)
	}
	return nil
}

func unconfirmedIntegrationTargetError(name string) error {
	return fmt.Errorf("integration target %s is inferred from the remote HEAD; declare branch.integrationBranch in the repository .gz-git.yaml, pass --controller-config, or confirm with --target %s --direct-to-default", name, name)
}

// runChecked continues an integration from an immutable readiness report.
// It deliberately revalidates source, target, and contract provenance before
// any push or reclaim action.
func runChecked(ctx context.Context, exec *gitcmd.Executor, opts RunOptions, check *CheckReport) (*RunReport, error) {
	if exec == nil {
		return nil, fmt.Errorf("git executor is nil")
	}
	if check == nil {
		return nil, fmt.Errorf("readiness report is nil")
	}
	report := &RunReport{Check: check, Source: check.Plan.Branch, Target: check.Plan.Target}
	if !check.Ready {
		return report, fmt.Errorf("not ready")
	}

	dir := strings.TrimSpace(opts.RepoPath)
	if dir == "" {
		dir = "."
	}
	g := newGitRepo(exec, dir)
	root, err := g.toplevel(ctx)
	if err != nil {
		return report, err
	}
	g.dir = root
	if err := revalidateController(ctx, g, check); err != nil {
		return report, err
	}

	// A source that moved while readiness ran is, under --expect-source, a
	// mismatch with the approval, not a generic revalidation failure: compare
	// it first so the caller gets ErrSourceMismatch (exit 4) for it too.
	if err := requireExpectedSource(ctx, g, check.Plan.Branch, opts.ExpectSource); err != nil {
		return report, err
	}
	sourceSHA, targetSHA, targetName, err := revalidateCheckedRefsForExpectedTarget(ctx, g, check, opts.NoFetch, opts.ExpectTarget)
	if err != nil {
		return report, err
	}

	anc, err := g.isAncestor(ctx, targetSHA, sourceSHA)
	if err != nil {
		return report, err
	}
	if !anc {
		return report, fmt.Errorf("%s is not an ancestor of %s; rebase and re-check", check.Plan.Target, check.Plan.Branch)
	}
	if err := revalidateController(ctx, g, check); err != nil {
		return report, err
	}

	pushRemote := check.Plan.Remote
	if check.Controller != nil {
		pushRemote = check.Controller.RemoteURL
	}
	common, err := g.output(ctx, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil || common == "" {
		if err == nil {
			err = fmt.Errorf("git common directory is empty")
		}
		return report, err
	}
	// Intent is durable before the push: a crash past this point leaves
	// evidence cleanup can reconcile against the remote, never folklore.
	if opts.Execution != nil {
		receipt, err := beginRunEvidence(ctx, exec, g, common, opts, check, sourceSHA, targetName)
		if err != nil {
			return report, err
		}
		report.Receipt = receipt
		pushRemote = receipt.PushRemote
	}
	if err := pushFastForwardWithExpectedTarget(ctx, g, pushRemote, check.Plan.Remote, opts.ExpectTarget != "", sourceSHA, targetName, check.Plan.TargetSHA); err != nil {
		return report, err
	}
	if report.Receipt != nil {
		if err := markEvidencePushed(ctx, common, report.Receipt); err != nil {
			return report, err
		}
	}
	report.Integrated = true
	report.SHA = sourceSHA
	report.Printed = append(report.Printed, fmt.Sprintf("INTEGRATED %s (%s) -> %s/%s", check.Plan.Branch, sourceSHA, check.Plan.Remote, targetName))
	if err := ffTargetWorktrees(ctx, exec, g, targetName, sourceSHA); err != nil {
		report.Reclaim.Failed = append(report.Reclaim.Failed, "fast-forward local target worktree: "+err.Error())
		report.Printed = append(report.Printed, "RECLAIM incomplete: "+strings.Join(report.Reclaim.Failed, "; "))
		return report, err
	}
	return finishRunReclaim(ctx, exec, g, root, common, targetName, report, opts.NoFetch)
}

func revalidateCheckedRefs(ctx context.Context, g gitRepo, check *CheckReport, noFetch bool) (sourceSHA, targetSHA, targetName string, err error) {
	return revalidateCheckedRefsForExpectedTarget(ctx, g, check, noFetch, "")
}

func revalidateCheckedRefsForExpectedTarget(ctx context.Context, g gitRepo, check *CheckReport, noFetch bool, expectTarget string) (sourceSHA, targetSHA, targetName string, err error) {
	if check.Plan.Remote != "" && !noFetch {
		if err := g.fetchPrune(ctx, check.Plan.Remote); err != nil {
			return "", "", "", err
		}
	}
	sourceSHA, ok, err := g.revParse(ctx, check.Plan.Branch)
	if err != nil {
		return "", "", "", err
	}
	if !ok || sourceSHA != check.Plan.BranchSHA {
		return "", "", "", fmt.Errorf("source branch changed during readiness; re-run check")
	}
	targetName = targetBranchName(check.Plan.Target, check.Plan.Remote)
	targetRef := check.Plan.Target
	if check.Plan.Remote != "" {
		targetRef = check.Plan.Remote + "/" + targetName
	}
	targetSHA, ok, err = g.revParse(ctx, targetRef)
	if err != nil {
		return "", "", "", err
	}
	if !ok {
		targetSHA, ok, err = g.revParse(ctx, check.Plan.Target)
		if err != nil {
			return "", "", "", err
		}
		if !ok {
			return "", "", "", fmt.Errorf("target ref not found after fetch: %s", check.Plan.Target)
		}
	}
	if targetSHA != check.Plan.TargetSHA {
		if expectTarget == "" {
			return "", "", "", fmt.Errorf("target branch changed during readiness; re-run check")
		}
		return "", "", "", fmt.Errorf("%w: target %s is %s, expected %s", ErrTargetMismatch, targetRef, targetSHA, check.Plan.TargetSHA)
	}
	if check.GateMode != "contract-v1" {
		return sourceSHA, targetSHA, targetName, nil
	}
	targetContract, present, err := loadReadinessContract(ctx, g, targetSHA)
	if err != nil {
		return "", "", "", err
	}
	if !present || targetContract.Digest != check.ContractDigest || targetContract.ManifestOID != check.ManifestOID || targetContract.TreeOID != check.ReadinessTreeOID || targetContract.RunnerOID != check.RunnerOID {
		return "", "", "", fmt.Errorf("target readiness contract changed during readiness; re-run check")
	}
	return sourceSHA, targetSHA, targetName, nil
}

func pushFastForward(ctx context.Context, g gitRepo, remote, sourceSHA, targetName, checkedTargetSHA string) error {
	return pushFastForwardWithExpectedTarget(ctx, g, remote, remote, false, sourceSHA, targetName, checkedTargetSHA)
}

func pushFastForwardWithExpectedTarget(ctx context.Context, g gitRepo, remote, remoteName string, expectTarget bool, sourceSHA, targetName, checkedTargetSHA string) error {
	if remote == "" {
		return fmt.Errorf("no remote; cannot push the fast-forward")
	}
	if expectTarget {
		if err := requireSoleRefPush(ctx, g, remoteName); err != nil {
			return err
		}
	}
	refspec := sourceSHA + ":refs/heads/" + targetName
	lease := "--force-with-lease=refs/heads/" + targetName + ":" + checkedTargetSHA
	args := []string{"push"}
	if expectTarget {
		// An approved source/target pair may update only the leased branch.
		// --porcelain keeps the rejection verdict machine-readable on stdout,
		// and --no-follow-tags suppresses push.followTags so a stale-lease
		// result cannot conceal an accepted tag update behind a retryable
		// target mismatch.
		args = append(args, "--porcelain", "--no-follow-tags")
	}
	args = append(args, lease, remote, refspec)
	push, err := g.run(ctx, args...)
	if err != nil {
		return err
	}
	if push.ExitCode != 0 {
		// The retryable classification is granted only by a complete,
		// unambiguous porcelain record binding the exact leased source and
		// ref, then re-reading the leased destination itself. Stderr prose,
		// mixed or partial output, and any other rejection stay generic.
		if expectTarget && isSolePorcelainStaleLeaseRejection(push, sourceSHA, "refs/heads/"+targetName) {
			moved, refreshErr := targetMovedAtPushEndpoint(ctx, g, remote, remoteName, targetName, checkedTargetSHA, sourceSHA)
			if refreshErr == nil && moved {
				return fmt.Errorf("%w: target %s moved after readiness", ErrTargetMismatch, targetName)
			}
		}
		return fmt.Errorf("git push %s %s failed: %s", remote, refspec, pushFailureDetail(push, expectTarget))
	}
	return nil
}

// pushFailureDetail renders a generic push failure. Porcelain mode moves the
// per-ref record from stderr to stdout, so the opt-in path appends stdout or
// the reason would be invisible to the operator.
func pushFailureDetail(push *gitcmd.Result, expectTarget bool) string {
	detail := strings.TrimSpace(push.Stderr)
	if !expectTarget {
		return detail
	}
	if porcelain := strings.TrimSpace(push.Stdout); porcelain != "" && porcelain != detail {
		detail = strings.TrimSpace(detail + "\n" + porcelain)
	}
	return detail
}

// requireSoleRefPush rejects mirror remotes before an immutable request runs.
// --mirror can expand a refspec into a repository-wide update, which violates
// the approved pair even when the target lease itself later rejects.
func requireSoleRefPush(ctx context.Context, g gitRepo, remote string) error {
	if remote == "" {
		return fmt.Errorf("no remote name for immutable target push")
	}
	res, err := g.run(ctx, "config", "--bool", "--get", "remote."+remote+".mirror")
	if err != nil {
		return err
	}
	if res.ExitCode == 0 && strings.EqualFold(strings.TrimSpace(res.Stdout), "true") {
		return fmt.Errorf("remote %s is mirror-configured; immutable target push requires one explicit ref", remote)
	}
	return nil
}

// porcelainPushRecord is one machine-readable ref-status record printed by
// git push --porcelain: "<flag>\t<from>:<to>\t<summary>" optionally followed
// by " (<reason>)". The grammar and its tab separators are the ones a real
// Git prints for "!	<sha>:refs/heads/<branch>	[rejected] (stale info)".
type porcelainPushRecord struct {
	flag    byte
	from    string
	to      string
	summary string
	reason  string
}

// porcelainPushStatus is the parsed skeleton of one git push --porcelain
// report: its ref-status records plus the counts of the surrounding "To
// <destination>" header and "Done" footer lines.
type porcelainPushStatus struct {
	records []porcelainPushRecord
	headers int
	footers int
}

// parsePorcelainPushStatus parses stdout of git push --porcelain in the exact
// grammar a real Git prints: a final-newline-terminated report whose first
// line is the "To <destination>" header, then ref-status records, then "Done"
// as the last line. ok is false for anything else — partial or malformed
// output, a blank or smuggled line, a header without a destination, or a
// reordered skeleton — because half-trusted output is ambiguous, not proof.
func parsePorcelainPushStatus(stdout string) (porcelainPushStatus, bool) {
	var status porcelainPushStatus
	if stdout == "" {
		return status, true
	}
	if !strings.HasSuffix(stdout, "\n") {
		// A real report ends with a newline; a capture that lost it may have
		// lost a record's tail too.
		return status, false
	}
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	sawRecord, sawFooter := false, false
	for i, line := range lines {
		if line == "" {
			return status, false
		}
		if porcelainToHeader(line) {
			if sawRecord || sawFooter {
				return status, false
			}
			status.headers++
			continue
		}
		if line == "Done" {
			if sawFooter || i != len(lines)-1 {
				return status, false
			}
			sawFooter = true
			status.footers++
			continue
		}
		record, ok := parsePorcelainPushRecord(line)
		if !ok || sawFooter {
			return status, false
		}
		sawRecord = true
		status.records = append(status.records, record)
	}
	return status, true
}

// porcelainToHeader recognizes the "To <destination>" header line. The
// destination is a non-empty URL or path without padding or embedded tabs;
// an empty, padded, or record-smuggling "To " line is not a header a real Git
// prints and leaves the whole report malformed.
func porcelainToHeader(line string) bool {
	const prefix = "To "
	if !strings.HasPrefix(line, prefix) {
		return false
	}
	destination := line[len(prefix):]
	return destination != "" && strings.TrimSpace(destination) == destination && !strings.Contains(destination, "\t")
}

// parsePorcelainPushRecord parses exactly one porcelain record line: flag,
// tab, "<from>:<to>", tab, summary with an optional " (<reason>)" suffix. A
// missing field separator, a superfluous tab, or a ref pair that is not two
// colon-separated parts is malformed.
func parsePorcelainPushRecord(line string) (porcelainPushRecord, bool) {
	var record porcelainPushRecord
	if len(line) < 4 || line[1] != '\t' {
		return record, false
	}
	record.flag = line[0]
	fields := strings.Split(line[2:], "\t")
	if len(fields) != 2 {
		return record, false
	}
	refs := strings.Split(fields[0], ":")
	if len(refs) != 2 {
		return record, false
	}
	record.from, record.to = refs[0], refs[1]
	record.summary = fields[1]
	if strings.HasSuffix(record.summary, ")") {
		if open := strings.LastIndex(record.summary, " ("); open >= 0 {
			record.reason = record.summary[open+2 : len(record.summary)-1]
			record.summary = record.summary[:open]
		}
	}
	return record, true
}

// isSolePorcelainStaleLeaseRejection reports whether a failed push printed
// exactly one complete porcelain ref-status record and that record is the
// stale-lease rejection of precisely sourceSHA:targetRef. Only stdout is
// consulted: stderr never proves a retryable classification, and any second
// record (another ref, an accepted or mixed update), an unrecognized or
// truncated line, a missing header or footer, or no record at all keeps the
// failure generic.
func isSolePorcelainStaleLeaseRejection(push *gitcmd.Result, sourceSHA, targetRef string) bool {
	if push == nil {
		return false
	}
	status, ok := parsePorcelainPushStatus(push.Stdout)
	if !ok || len(status.records) != 1 || status.headers != 1 || status.footers != 1 {
		return false
	}
	rejection := status.records[0]
	return rejection.flag == '!' &&
		rejection.from == sourceSHA &&
		rejection.to == targetRef &&
		rejection.summary == "[rejected]" &&
		rejection.reason == "stale info"
}

// targetMovedAtPushEndpoint proves movement at the exact destination passed to
// git push. Tracking refs may have a different fetch URL, and a remote can
// have multiple push URLs, so either ambiguity deliberately keeps the generic
// push error instead of claiming the request is safely retryable.
func targetMovedAtPushEndpoint(ctx context.Context, g gitRepo, pushRemote, remoteName, targetName, expected, source string) (bool, error) {
	endpoint, err := pushEndpoint(ctx, g, remoteName)
	if err != nil {
		return false, err
	}
	if pushRemote != remoteName {
		actualPush, err := canonicalPushEndpoint(pushRemote)
		if err != nil || actualPush != endpoint {
			return false, fmt.Errorf("push destination differs from verified remote endpoint")
		}
	}
	actual, err := exactRemoteSHA(ctx, g, endpoint, "refs/heads/"+targetName)
	if err != nil {
		return false, err
	}
	// A target equal to the source proves this push (or a concurrent equivalent
	// push) may already have landed. Preserve the generic failure and intent
	// receipt for cleanup/reconciliation; it is not safe to request a rerun.
	return actual != expected && actual != source, nil
}

func ffTargetWorktrees(ctx context.Context, exec *gitcmd.Executor, g gitRepo, targetName, sourceSHA string) error {
	trees, err := g.listWorktrees(ctx)
	if err != nil {
		return fmt.Errorf("integrated but listing worktrees failed: %w", err)
	}
	for _, wt := range trees {
		if wt.Branch != targetName {
			continue
		}
		tg := newGitRepo(exec, wt.Path)
		res, err := tg.run(ctx, "merge", "--ff-only", sourceSHA)
		if err != nil || (res != nil && res.ExitCode != 0) {
			detail := ""
			if res != nil {
				detail = strings.TrimSpace(res.Stderr)
			}
			if err != nil && detail == "" {
				detail = err.Error()
			}
			return fmt.Errorf("remote integrated but local target worktree failed: %s: %s", wt.Path, detail)
		}
		break
	}
	return nil
}

func finishRunReclaim(ctx context.Context, exec *gitcmd.Executor, g gitRepo, root, common, targetName string, report *RunReport, noFetch bool) (*RunReport, error) {
	if err := revalidateController(ctx, g, report.Check); err != nil {
		return report, err
	}
	decl, loadErr := config.LoadRepoRootTaskPattern(root)
	if report.Check.Controller != nil {
		decl.Patterns = append([]string(nil), report.Check.Controller.TaskPattern...)
		decl.Facts = nil
		loadErr = nil
	}
	if loadErr != nil {
		report.Reclaim.Skipped = "reclaim nothing: " + loadErr.Error()
		report.Printed = append(report.Printed, "RECLAIM skipped: "+report.Reclaim.Skipped)
	} else {
		pushRemote := controllerPushRemote(report.Check)
		if report.Receipt != nil {
			pushRemote = report.Receipt.pushRemote()
		}
		report.Reclaim = reclaimAfter(ctx, exec, g, reclaimOpts{
			Branch:       report.Check.Plan.Branch,
			TargetBranch: targetName,
			DefaultName:  defaultBranchName(report.Check.Plan.DefaultRef, report.Check.Plan.Remote),
			Integration:  report.Check.Plan.Integration.Name,
			Remote:       report.Check.Plan.Remote,
			PushRemote:   pushRemote,
			TaskSHA:      report.SHA,
			Patterns:     decl.Patterns,
			Facts:        decl.Facts,
			NoFetch:      noFetch,
			Receipt:      report.Receipt,
		})
		if report.Reclaim.Skipped != "" {
			report.Printed = append(report.Printed, "RECLAIM skipped: "+report.Reclaim.Skipped)
		}
		if len(report.Reclaim.Done) > 0 {
			report.Printed = append(report.Printed, "RECLAIMED "+report.Check.Plan.Branch+" — "+strings.Join(report.Reclaim.Done, " "))
		}
		if report.Reclaim.Incomplete() {
			report.Printed = append(report.Printed, "RECLAIM incomplete: "+strings.Join(report.Reclaim.Failed, "; "))
		}
	}
	// The reclaim results are durable even when incomplete: a crashed
	// reclaim is exactly what cleanup retry resumes from.
	if report.Receipt != nil {
		if err := recordEvidenceReclaim(ctx, common, report.Receipt, report.Reclaim); err != nil {
			return report, err
		}
	}
	return report, nil
}

func controllerPushRemote(check *CheckReport) string {
	if check != nil && check.Controller != nil {
		return check.Controller.RemoteURL
	}
	return ""
}

// FormatRun renders integrate/reclaim lines.
func FormatRun(r *RunReport) string {
	if r == nil {
		return ""
	}
	var b strings.Builder
	if r.Check != nil {
		b.WriteString(FormatCheck(r.Check))
		b.WriteByte('\n')
	}
	for _, line := range r.Printed {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

// requireExpectedSource re-reads the source and holds it to --expect-source.
func requireExpectedSource(ctx context.Context, g gitRepo, branch, raw string) error {
	if raw == "" {
		return nil
	}
	expect, err := NormalizeExpectSource(raw)
	if err != nil {
		return err
	}
	_, err = resolveSource(ctx, g, branch, expect)
	return err
}
