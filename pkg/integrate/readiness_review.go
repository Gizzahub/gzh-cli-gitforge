package integrate

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

// ReadinessUpdateReviewOptions selects an in-process review or an explicitly
// opted-in automatic update. Statement is a request, never test evidence.
type ReadinessUpdateReviewOptions struct {
	RepoPath, Branch, Target, Issuer, ExpectSource, Statement string
	Automatic                                                 bool
	Review                                                    func(ReadinessUpdatePlan, string) (bool, error)
	Notice                                                    io.Writer
}

// ReviewReadinessUpdate reviews or measures an exact contract update, then uses
// the existing one-use atomic transaction. A fresh internal plan is issued only
// if the reviewed/measured snapshot is still identical, so time spent reading a
// diff or running checks does not expire the user's approval.
func ReviewReadinessUpdate(ctx context.Context, exec *gitcmd.Executor, opts ReadinessUpdateReviewOptions) (ReadinessUpdatePlan, error) {
	if err := validateReadinessReview(opts); err != nil {
		return ReadinessUpdatePlan{}, err
	}
	if opts.ExpectSource != "" {
		var err error
		opts.ExpectSource, err = NormalizeExpectSource(opts.ExpectSource)
		if err != nil {
			return ReadinessUpdatePlan{}, err
		}
	}
	planOpts := ReadinessUpdateOptions{RepoPath: opts.RepoPath, Branch: opts.Branch, Target: opts.Target, Issuer: opts.Issuer, Expiry: 15 * time.Minute}
	p, err := ReadinessUpdatePlanFor(ctx, exec, planOpts)
	if err != nil {
		return p, err
	}
	if opts.ExpectSource != "" && p.SourceSHA != opts.ExpectSource {
		return p, fmt.Errorf("requested source changed: expected %s, got %s", opts.ExpectSource, p.SourceSHA)
	}
	g := newGitRepo(exec, opts.RepoPath)
	root, err := g.toplevel(ctx)
	if err != nil {
		return p, err
	}
	g.dir = root
	if opts.Automatic {
		if err := measureReadinessUpdate(ctx, g, p, opts.Notice); err != nil {
			return p, err
		}
	} else if err := reviewReadinessDiff(ctx, g, p, opts.Review); err != nil {
		return p, err
	}
	// Keep Git ownership separate from resource ownership; no host resource
	// slot is held while pushing. Exact leases still protect remote writers.
	lockCtx, cancel := context.WithTimeout(ctx, time.Hour)
	defer cancel()
	release, err := holdTargetLock(lockCtx, g, p.DestinationRef, opts.Notice)
	if err != nil {
		return p, err
	}
	defer release()
	planOpts.Branch, planOpts.Target = p.SourceRef, p.TargetRef
	fresh, err := ReadinessUpdatePlanFor(ctx, exec, planOpts)
	if err != nil {
		return p, err
	}
	if !sameReadinessUpdateSnapshot(p, fresh) {
		return p, fmt.Errorf("reviewed readiness update state changed; review again")
	}
	// Keep the measurement and remote audit marker under the same operation ID.
	fresh.OperationID, fresh.OperationRef = p.OperationID, p.OperationRef
	// Recheck the global authority after potentially long measurement/review.
	if err := validateReadinessReview(opts); err != nil {
		return p, err
	}
	if err := ReadinessUpdateApply(ctx, exec, fresh, root, ReadinessUpdatePlanDigest(fresh)); err != nil {
		return fresh, err
	}
	return fresh, nil
}

func validateReadinessReview(opts ReadinessUpdateReviewOptions) error {
	if !opts.Automatic {
		if opts.Review == nil {
			return fmt.Errorf("human readiness review callback is required")
		}
		return nil
	}
	mode, err := config.LoadReadinessApproval()
	if err != nil {
		return err
	}
	if mode != config.ReadinessApprovalAutomatic {
		return fmt.Errorf("automatic readiness update requires global integration.readinessApproval: automatic")
	}
	if opts.Statement != "테스트완료" {
		return fmt.Errorf("automatic readiness update requires --statement 테스트완료 (request only; checks will run)")
	}
	if opts.ExpectSource == "" {
		return fmt.Errorf("automatic readiness update requires --expect-source")
	}
	_, err = NormalizeExpectSource(opts.ExpectSource)
	return err
}

func reviewReadinessDiff(ctx context.Context, g gitRepo, p ReadinessUpdatePlan, review func(ReadinessUpdatePlan, string) (bool, error)) error {
	diff, err := g.output(ctx, "-c", "core.quotePath=true", "diff", "--no-ext-diff", "--no-textconv", "--no-color", p.TargetSHA, p.SourceSHA, "--")
	if err != nil {
		return fmt.Errorf("render readiness update diff: %w", err)
	}
	approved, err := review(p, diff)
	if err != nil {
		return err
	}
	if !approved {
		return fmt.Errorf("readiness update declined")
	}
	return nil
}

func measureReadinessUpdate(ctx context.Context, g gitRepo, p ReadinessUpdatePlan, notice io.Writer) error {
	contract, present, err := updateReadinessContract(ctx, g, p.SourceSHA)
	if err != nil {
		return err
	}
	if !present || !readinessRunnerSupported() {
		return fmt.Errorf("candidate readiness runner is unavailable")
	}
	timeout, err := contract.Decl.ExecutionTimeout()
	if err != nil {
		return err
	}
	plan := TargetPlan{Branch: p.SourceRef, BranchSHA: p.SourceSHA, Remote: p.Remote, Target: p.TargetRef, TargetSHA: p.TargetSHA}
	heavy, err := resourceHighLoadForPlan(ctx, g, plan)
	if err != nil {
		return err
	}
	opts := CheckOptions{RepoPath: g.dir, Branch: p.SourceRef, Target: p.TargetRef, ExpectSource: p.SourceSHA, ExpectTarget: p.TargetSHA, LockNotice: notice}
	opts.resourceWaitObserver = targetWaitObserver(g, plan, false, notice)
	result, duration, runErr := runContractPair(ctx, g, plan, contract, timeout, readinessPlanTimeout, heavy, opts, true)
	// This local observation records actual execution, not an LLM assertion or
	// reusable authorization. The remote one-use operation ref records apply.
	common, err := g.output(ctx, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return err
	}
	path := filepath.Join(common, "gz-git-readiness-updates", p.OperationID+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create readiness measurement directory: %w", err)
	}
	evidence := struct {
		Plan          ReadinessUpdatePlan `json:"plan"`
		Mode          string              `json:"mode"`
		StatementRole string              `json:"statement_role"`
		Status        string              `json:"status"`
		Summary       string              `json:"summary"`
		DurationMS    int64               `json:"duration_ms"`
		ObservedAt    time.Time           `json:"observed_at"`
	}{p, "automatic", "request-only", result.Status, result.Summary, duration.Milliseconds(), time.Now().UTC()}
	if err := atomicStateWrite(path, mustMarshalJSON(evidence)); err != nil {
		return fmt.Errorf("record readiness measurement: %w", err)
	}
	if notice != nil {
		fmt.Fprintf(notice, "READINESS_MEASURED source=%s target=%s runner=%s status=%s evidence=%s\n", p.SourceSHA, p.TargetSHA, p.NewRunnerOID, result.Status, path)
	}
	if runErr != nil {
		return fmt.Errorf("candidate readiness check: %w", runErr)
	}
	if result.Status != "ready" {
		return fmt.Errorf("candidate readiness check %s: %s", result.Status, result.Summary)
	}
	return nil
}
