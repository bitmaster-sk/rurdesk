package agent

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/githost"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/rs/zerolog/log"
)

// IssueRun is the run of an agent working on an issue: it walks the stage plan, writes its output
// as issue messages and ends with a PR whose merge finishes the run.
type IssueRun struct {
	*runCommon
}

func (r *IssueRun) NextTask(ctx context.Context, run *model.AgentRun, tasks []*model.AgentTask) (*model.AgentTask, error) {
	return r.nextPlannedTask(ctx, run, tasks)
}

func (r *IssueRun) Activate(ctx context.Context, run *model.AgentRun, task *model.AgentTask) (*Dispatch, error) {
	return r.activatePlannedTask(ctx, run, task)
}

// Payload carries the issue, its conversation and the stage artifacts approved so far, the
// repository the agent works in and the branch of the issue's open PR.
func (r *IssueRun) Payload(ctx context.Context, dispatch *Dispatch) (map[string]any, []*model.AgentRunWikiRead, error) {
	run, task := dispatch.Run, dispatch.Task
	bundle, wikiReads, priorTasks, err := r.contextBundle(ctx, run, task, run.IdIssue)
	if err != nil {
		return nil, nil, fmt.Errorf("context bundle build error: %w", err)
	}
	issue, err := r.issueRepo.LoadIssue(ctx, &repository.LoadIssueFilter{IdIssue: run.IdIssue})
	if err != nil {
		return nil, nil, fmt.Errorf("context bundle build error: loading issue: %w", err)
	}
	messages, err := r.messageRepo.LoadIssueMessages(ctx, *run.IdIssue, 0, nil)
	if err != nil {
		messages = nil
	}
	artifacts := stageArtifactContext(task.Stage, priorTasks, messages)
	bundle["issue"] = issue
	bundle["reviewThread"] = reviewThread(messages)
	bundle["approvedDesign"] = artifacts.ApprovedDesign
	bundle["approvedImplPlan"] = artifacts.ApprovedImplPlan
	bundle["rejectedOutput"] = artifacts.RejectedOutput

	repo, err := r.agentRunGitRepo(ctx, run)
	if err != nil {
		return nil, nil, err
	}
	prBranch, err := r.openPrBranch(ctx, run)
	if err != nil {
		log.Warn().Err(err).Int64("idRun", run.IdRun).Str("prBranch", prBranch).Msg("resolving the issue's open PR branch")
	}
	payload, err := r.stagePayload(ctx, run, task, bundle, repo, prBranch)
	return payload, wikiReads, err
}

func (r *IssueRun) Started(ctx context.Context, dispatch *Dispatch) {
	r.broadcastRun(ctx, dispatch.Run)
}

// Complete writes the stage's output as an issue message, opens the PR of an implementation and
// moves the run on. The PR is resolved outside the transaction: it calls the git host, and a host
// failure turns the outcome into errored so the failure path records it.
func (r *IssueRun) Complete(ctx context.Context, run *model.AgentRun, task *model.AgentTask, body *model.CompleteStageReq) (model.CompleteStageRes, error) {
	if err := validateMessageKind(body.Message, body.MessageKind); err != nil {
		return model.CompleteStageRes{}, errs.ErrBadRequest.WithMessage(err.Error())
	}
	isReconcile, isNoOp := r.reconcileGate(ctx, run, task)
	if isNoOp {
		return unchangedCompletion(run, task), nil
	}
	if isImplementationWithoutPr(run, task, body) {
		return model.CompleteStageRes{}, errs.ErrImplementationNoBranch
	}
	pr := r.resolvePrForCompletion(ctx, run, task, body)

	agentUser, _ := extctx.GetUser(ctx)
	var message *model.Message
	var res model.CompleteStageRes
	txErr := extctx.RunInTx(ctx, r.pool, func(ctx context.Context) error {
		var idMessage *int64
		if body.Message != "" {
			var err error
			message, err = r.messageRepo.InsertIssueAgentMessage(ctx, body.Message, &agentUser, *run.IdIssue, constants.MessageKind(body.MessageKind))
			if err != nil {
				return fmt.Errorf("writing message: %w", err)
			}
			if err := r.attachmentLinkSvc.LinkMessageAttachments(ctx, message); err != nil {
				return fmt.Errorf("linking message attachments: %w", err)
			}
			idMessage = &message.IdMessage
		}
		isPrWritten := false
		if pr != nil {
			if err := r.writePrInfo(ctx, run, pr, body.BranchName); err != nil {
				// A concurrent Restart/Continue may have moved the run out of `failed` between the
				// gate and this CAS: a superseded no-op, not a 500.
				if isReconcile && errors.Is(err, repository.ErrPhaseMismatch) {
					return errReconcileSuperseded
				}
				return fmt.Errorf("applying PR info: %w", err)
			}
			isPrWritten = true
		}
		var err error
		res, err = r.finishTask(ctx, run, task, body, idMessage, isPrWritten, isReconcile)
		return err
	})
	if txErr != nil {
		return completionError(txErr, run, task)
	}

	r.thinking.Compact(ctx, task.IdTask)
	if message != nil {
		if idsUser, err := r.projectMembers(ctx, run); err == nil {
			r.notifier.Send(&notify.Notice{
				IdsUser: idsUser,
				Subject: notify.SubjectMessage,
				Action:  notify.ActionCreate,
				Payload: message,
				Source:  "agent",
			})
		}
	}
	if updated, err := r.runRepo.LoadById(ctx, run.IdRun); err == nil {
		r.broadcastRun(context.Background(), updated)
	}
	// The completion changed the issue out of band (PR link, phase→state mirror); push the fresh
	// issue so the detail page updates live.
	BroadcastIssueUpdate(context.Background(), r.notifier, r.issueRepo, r.projectRepo, *run.IdIssue)
	return res, nil
}

func (r *IssueRun) Interrupt(ctx context.Context, run *model.AgentRun, task *model.AgentTask, reason InterruptReason, detail string) error {
	return r.failPlannedRun(ctx, run, task, reason, detail)
}

// ApplyReport has nothing to take: an issue run reports only stats.
func (r *IssueRun) ApplyReport(context.Context, *model.AgentRun, *model.AgentTask, *TaskReport) error {
	return nil
}

// ApplyPrStatus finishes the run when its PR merged (done) or closed without merge (failed). The
// issue's MR badge shows the run's PR.
func (r *IssueRun) ApplyPrStatus(ctx context.Context, run *model.AgentRun, status *githost.Status) (*model.Issue, error) {
	switch status.State {
	case constants.MrStateMerged:
		updated, err := r.runRepo.TransitionPhase(ctx, run.IdRun, constants.PhasePrOpen, constants.PhaseDone, constants.ActorTypeSystem, nil, "PR merged")
		if err != nil {
			return nil, fmt.Errorf("transitioning run %d to done: %w", run.IdRun, err)
		}
		r.broadcastRun(context.Background(), updated)
		r.proposals.RunFinished(ctx, updated)
		return r.prIssue(ctx, run), nil
	case constants.MrStateClosed:
		updated, err := r.runRepo.TransitionPhase(ctx, run.IdRun, constants.PhasePrOpen, constants.PhaseFailed, constants.ActorTypeSystem, nil, "PR closed without merge")
		if err != nil {
			return nil, fmt.Errorf("transitioning run %d to failed: %w", run.IdRun, err)
		}
		if err := r.runRepo.SetErrorMessage(ctx, run.IdRun, "PR closed without merge"); err != nil {
			log.Error().Err(err).Int64("idRun", run.IdRun).Msg("merge poller: failed to set error message")
		}
		r.broadcastRun(context.Background(), updated)
		r.proposals.RunFinished(ctx, updated)
		return r.prIssue(ctx, run), nil
	default:
		return &model.Issue{
			IdIssue:          *run.IdIssue,
			IdProject:        run.IdProject,
			IdGitIntegration: run.IdGitIntegration,
			MrId:             run.PrId,
		}, nil
	}
}

// prIssue loads the issue whose badge shows the run's finished PR; nil when it cannot be loaded.
func (r *IssueRun) prIssue(ctx context.Context, run *model.AgentRun) *model.Issue {
	issue, err := r.issueRepo.LoadIssue(ctx, &repository.LoadIssueFilter{IdIssue: run.IdIssue})
	if err != nil || issue == nil {
		log.Debug().Err(err).Int64("idRun", run.IdRun).Int64("idIssue", *run.IdIssue).
			Msg("merge poller: issue load failed, skipping mr status broadcast")
		return nil
	}
	return issue
}

func (r *IssueRun) Audience(ctx context.Context, run *model.AgentRun) ([]int64, error) {
	return r.projectMembers(ctx, run)
}

func validateMessageKind(message, kind string) error {
	if message == "" {
		return nil
	}
	allowed := strings.Join(constants.AgentMessageKinds, ", ")
	switch {
	case slices.Contains(constants.AgentMessageKinds, kind):
		return nil
	case kind == "":
		return fmt.Errorf("messageKind is required when message is set, use one of: %s", allowed)
	default:
		return fmt.Errorf("unknown messageKind %q, use one of: %s", kind, allowed)
	}
}

// prComputed holds the PR identity resolved (created or found) on the git host
// before the DB transaction.
type prComputed struct {
	prId             string
	prUrl            string
	hostType         string
	idGitIntegration int64
}

func isImplementationWithoutPr(run *model.AgentRun, task *model.AgentTask, body *model.CompleteStageReq) bool {
	if task.Stage != constants.StageImplementation || body.Outcome != constants.StageOutcomeOutputSubmitted {
		return false
	}
	if body.BranchName != "" || body.PrUrl != "" {
		return false
	}
	return run.PrId == nil || *run.PrId == ""
}

// resolvePrForCompletion resolves PR identity for an implementation completion,
// outside any DB transaction (it makes git-host HTTP calls). Returns nil when no
// PR applies; on host failure it converts body to an `errored` outcome so the
// normal failure path records it.
func (r *IssueRun) resolvePrForCompletion(ctx context.Context, run *model.AgentRun, task *model.AgentTask, body *model.CompleteStageReq) *prComputed {
	if task.Stage != constants.StageImplementation || body.Outcome != constants.StageOutcomeOutputSubmitted {
		return nil
	}
	var pr *prComputed
	var err error
	switch {
	case body.PrUrl != "":
		// Deprecated path: agent supplied a URL directly. Guard and use it.
		var hostType, prId string
		hostType, prId, err = derivePrHostAndId(body.PrUrl)
		if err == nil {
			var idGi int64
			if idGi, err = r.resolveIntegrationId(ctx, run); err == nil {
				pr = &prComputed{prId: prId, prUrl: body.PrUrl, hostType: hostType, idGitIntegration: idGi}
			}
		}
	case body.BranchName != "":
		pr, err = r.createOrFindPr(ctx, run, body)
	default:
		return nil // no branch, no URL — nothing to open
	}
	if err != nil {
		log.Error().Err(err).Int64("idRun", run.IdRun).Msg("PR creation failed; marking stage errored")
		body.Outcome = constants.StageOutcomeErrored
		reason := "pr_creation_failed"
		detail := err.Error()
		body.ErrorReason = &reason
		body.ErrorDetail = &detail
		return nil
	}
	return pr
}

// createOrFindPr opens (or reuses, for a re-invoked run) the PR/MR for the run's
// pushed branch. Host I/O — must be called OUTSIDE the DB transaction.
func (r *IssueRun) createOrFindPr(ctx context.Context, run *model.AgentRun, body *model.CompleteStageReq) (*prComputed, error) {
	idGi, err := r.resolveIntegrationId(ctx, run)
	if err != nil {
		return nil, err
	}
	integration, err := r.gitIntRepo.LoadByID(ctx, idGi, run.IdProject)
	if err != nil {
		return nil, err
	}
	if integration == nil {
		return nil, fmt.Errorf("git_integration %d not found", idGi)
	}
	host, err := githost.BuildFromIntegration(integration)
	if err != nil {
		return nil, err
	}
	title := body.PrTitle
	if title == "" {
		title = r.fallbackPrTitle(ctx, run.IdIssue, body.BranchName)
	}
	prId, prUrl, err := openOrReusePr(ctx, host, run.BaseBranch, body.BranchName, title, body.PrBody)
	if err != nil {
		return nil, err
	}
	return &prComputed{prId: prId, prUrl: prUrl, hostType: integration.HostType, idGitIntegration: integration.IdGitIntegration}, nil
}

// openOrReusePr reuses an open PR/MR for branch if one exists, otherwise opens a
// new one against baseBranch, or the repo's default branch for a run dispatched
// before base branches were stamped on runs.
func openOrReusePr(ctx context.Context, host githost.GitHost, baseBranch *string, branch, title, body string) (string, string, error) {
	prId, prUrl, found, err := host.FindOpenPullRequest(ctx, branch)
	if err != nil {
		return "", "", fmt.Errorf("looking up existing PR: %w", err)
	}
	if found {
		return prId, prUrl, nil
	}
	base, err := prTargetBranch(ctx, host, baseBranch)
	if err != nil {
		return "", "", err
	}
	prId, prUrl, err = host.CreatePullRequest(ctx, branch, base, title, body)
	if err != nil {
		return "", "", fmt.Errorf("opening PR: %w", err)
	}
	return prId, prUrl, nil
}

// resolveIntegrationId returns the git_integration to use for PR creation: the
// one stamped on the run at dispatch, or the project's only integration for a run
// dispatched before that.
func (r *IssueRun) resolveIntegrationId(ctx context.Context, run *model.AgentRun) (int64, error) {
	if run.IdGitIntegration != nil {
		return *run.IdGitIntegration, nil
	}
	gitInts, err := r.gitIntRepo.ListByProject(ctx, run.IdProject)
	if err != nil {
		return 0, err
	}
	if len(gitInts) == 1 {
		return gitInts[0].IdGitIntegration, nil
	}
	return 0, fmt.Errorf("no git_integration resolved for the pushed repo (run %d, project has %d integrations)", run.IdRun, len(gitInts))
}

func prTargetBranch(ctx context.Context, host githost.GitHost, baseBranch *string) (string, error) {
	if baseBranch != nil && *baseBranch != "" {
		return *baseBranch, nil
	}
	base, err := host.DefaultBranch(ctx)
	if err != nil {
		return "", fmt.Errorf("resolving base branch: %w", err)
	}
	return base, nil
}

// fallbackPrTitle is used only when the agent did not supply a pr_title.
func (r *IssueRun) fallbackPrTitle(ctx context.Context, idIssue *int64, branchName string) string {
	issue, err := r.issueRepo.LoadIssue(ctx, &repository.LoadIssueFilter{IdIssue: idIssue})
	if err == nil && issue != nil && issue.Title != "" {
		return issue.Title
	}
	return "Agent changes (" + branchName + ")"
}

// writePrInfo persists the resolved PR onto the run (flipping its phase →
// pr_open) and mirrors it onto the issue. DB-only — safe inside the transaction.
// Uses run.Phase as the CAS from-phase: `in_progress` normally, `failed` when
// reconciling a crash-orphaned run, in which case SetPrInfoFrom clears
// the stale failure stamp too.
func (r *IssueRun) writePrInfo(ctx context.Context, run *model.AgentRun, pr *prComputed, branchName string) error {
	dto := model.SetRunPrReq{
		PrUrl:            pr.prUrl,
		PrId:             pr.prId,
		PrHostType:       pr.hostType,
		BranchName:       branchName,
		IdGitIntegration: pr.idGitIntegration,
	}
	if _, err := r.runRepo.SetPrInfoFrom(ctx, run.IdRun, dto, run.Phase); err != nil {
		return err
	}
	// Mirror onto issues.issue.{id_git_integration, mr_id} best-effort.
	_ = r.issueRepo.LinkMr(ctx, *run.IdIssue, dto.IdGitIntegration, dto.PrId)
	return nil
}

// derivePrHostAndId parses a PR URL into (host_type, pr_id). The path tail
// varies by host (/pull/N vs /merge_requests/N), so it checks host first,
// then path.
func derivePrHostAndId(prUrl string) (hostType, prId string, err error) {
	u, parseErr := url.Parse(prUrl)
	if parseErr != nil {
		return "", "", fmt.Errorf("invalid pr_url: %w", parseErr)
	}
	host := strings.ToLower(u.Host)

	switch {
	case strings.Contains(host, "github."):
		hostType = "github"
	case strings.Contains(host, "gitlab."):
		hostType = "gitlab"
	case strings.Contains(host, "gitea."):
		hostType = "gitea"
	default:
		hostType = ""
	}

	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i, seg := range segments {
		if (seg == "pull" || seg == "merge_requests" || seg == "pulls") &&
			i+1 < len(segments) {
			prId = segments[i+1]
			if hostType == "" {
				if seg == "merge_requests" {
					hostType = "gitlab"
				} else {
					hostType = "github"
				}
			}
			break
		}
	}
	if prId == "" {
		return "", "", fmt.Errorf("could not extract pr_id from %s — expected /pull/<n> or /merge_requests/<n>", prUrl)
	}
	if !isAllDigits(prId) {
		return "", "", fmt.Errorf("pr_id %q is not numeric in %s — got a PR-creation/compare URL, not an actual PR", prId, prUrl)
	}
	if hostType == "" {
		return "", "", fmt.Errorf("could not infer pr_host_type from %s — host must contain github/gitlab/gitea", prUrl)
	}
	return hostType, prId, nil
}

// isAllDigits reports whether s is non-empty and consists only of ASCII digits.
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (r *IssueRun) openPrBranch(ctx context.Context, run *model.AgentRun) (string, error) {
	prRun, err := r.runRepo.LoadLatestWithPrByIssue(ctx, *run.IdIssue)
	if err != nil {
		return "", fmt.Errorf("loading the issue's latest PR run: %w", err)
	}
	if prRun == nil {
		return "", nil
	}
	isOpen, err := r.isPrOpen(ctx, prRun)
	if err != nil {
		// A fresh branch here would stick for the whole run and open a duplicate PR.
		return *prRun.BranchName, err
	}
	if !isOpen {
		return "", nil
	}
	return *prRun.BranchName, nil
}

func (r *IssueRun) isPrOpen(ctx context.Context, prRun *model.AgentRun) (bool, error) {
	integration, err := r.gitIntRepo.LoadByID(ctx, *prRun.IdGitIntegration, prRun.IdProject)
	if err != nil {
		return false, fmt.Errorf("loading git integration %d: %w", *prRun.IdGitIntegration, err)
	}
	if integration == nil {
		return false, fmt.Errorf("git integration %d not found", *prRun.IdGitIntegration)
	}
	host, err := githost.BuildFromIntegration(integration)
	if err != nil {
		return false, fmt.Errorf("building git host: %w", err)
	}
	status, err := host.GetMergeRequestStatus(ctx, *prRun.PrId)
	if err != nil {
		return false, fmt.Errorf("loading PR %s status: %w", *prRun.PrId, err)
	}
	return status.State == constants.MrStateOpen, nil
}
