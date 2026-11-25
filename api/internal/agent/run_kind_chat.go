package agent

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/githost"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/rs/zerolog/log"
)

// ChatRun is the run of an agent chat. Each turn is a chat task: a reply becomes an agent message,
// a failed turn an error message or a retry. A turn never fails its run, and every notice goes to
// the conversation owner only.
type ChatRun struct {
	*runCommon
}

// Complete records complete_stage for a chat task: output_submitted stores the reply or task draft,
// errored ends the turn with an error message. A task that is no longer active is a no-op. The
// turn's thinking is compacted before the reply goes out, so the notice tells whether the reply
// has thinking even when only the tail is kept.
func (r *ChatRun) Complete(ctx context.Context, run *model.AgentRun, task *model.AgentTask, body *model.CompleteStageReq) (model.CompleteStageRes, error) {
	unchanged := model.CompleteStageRes{IdTask: task.IdTask, Status: task.Status, NextPhase: run.Phase}
	if body.Outcome == constants.StageOutcomeErrored {
		if task.Status != constants.TaskStatusActive {
			return unchanged, nil
		}
		if err := r.taskRepo.SetResultAndStats(ctx, task.IdTask, nil, body.TokensUsed, body.DurationMs, body.ToolCallsCount); err != nil {
			return model.CompleteStageRes{}, err
		}
		reason, detail := "agent_reported_error", ""
		if body.ErrorReason != nil && *body.ErrorReason != "" {
			reason = *body.ErrorReason
		}
		if body.ErrorDetail != nil {
			detail = *body.ErrorDetail
		}
		nextPhase, err := r.fail(ctx, run, task, turnFailure{reason: reason, detail: detail})
		if err != nil {
			return model.CompleteStageRes{}, err
		}
		r.thinking.Compact(ctx, task.IdTask)
		return model.CompleteStageRes{IdTask: task.IdTask, Status: constants.TaskStatusFailed, NextPhase: nextPhase}, nil
	}

	kind, err := chatMessageKind(body)
	if err != nil {
		return model.CompleteStageRes{}, err
	}
	if task.Status != constants.TaskStatusActive {
		return unchanged, nil
	}

	var message *model.AgentConversationMessage
	var idOwner int64
	nextPhase := run.Phase
	err = extctx.RunInTx(ctx, r.pool, func(ctx context.Context) error {
		// Lock the conversation first, as every turn transaction does, to avoid deadlock.
		conversation, err := r.conversationRepo.LoadConversationByRunForUpdate(ctx, run.IdRun)
		if err != nil {
			return err
		}
		if conversation == nil {
			nextPhase = constants.PhaseCancelled
			return r.cancelOrphan(ctx, run.IdRun)
		}
		if _, err := r.taskRepo.TransitionStatus(ctx, task.IdTask, constants.TaskStatusActive, constants.TaskStatusCompleted); err != nil {
			if errors.Is(err, repository.ErrTaskStatusMismatch) {
				// A concurrent completion (gateway and MCP tool) won the task.
				return nil
			}
			return err
		}
		if err := r.taskRepo.SetResultAndStats(ctx, task.IdTask, nil, body.TokensUsed, body.DurationMs, body.ToolCallsCount); err != nil {
			return err
		}
		message, err = r.conversationRepo.InsertMessage(ctx, conversation.IdConversation,
			constants.ConversationAuthorAgent, kind, body.Message, &run.IdUserAgent, &task.IdTask)
		if err != nil {
			return err
		}
		idOwner = conversation.IdUser
		nextPhase, err = r.endTurn(ctx, run.IdRun, conversation, task)
		return err
	})
	if err != nil {
		return model.CompleteStageRes{}, err
	}
	r.thinking.Compact(ctx, task.IdTask)
	if nextPhase == constants.PhaseCancelled && message == nil {
		return model.CompleteStageRes{IdTask: task.IdTask, Status: constants.TaskStatusCancelled, NextPhase: nextPhase}, nil
	}
	if message == nil {
		return unchanged, nil
	}
	// The tail of an unpersisted turn reaches the task only at compaction.
	if compacted, err := r.conversationRepo.LoadMessage(ctx, message.IdConversation, message.IdMessage); err == nil && compacted != nil {
		message = compacted
	}
	r.notifyOwner(idOwner, message)
	return model.CompleteStageRes{IdTask: task.IdTask, Status: constants.TaskStatusCompleted, NextPhase: nextPhase}, nil
}

// chatMessageKind maps a complete_stage messageKind to the stored message kind.
func chatMessageKind(body *model.CompleteStageReq) (string, error) {
	if body.Outcome != constants.StageOutcomeOutputSubmitted {
		return "", errs.ErrBadRequest.WithMessage(fmt.Sprintf("a chat turn ends with output_submitted or errored, got %s", body.Outcome))
	}
	if body.Message == "" {
		return "", errs.ErrBadRequest.WithMessage("message is required: it is the reply shown in the conversation")
	}
	switch body.MessageKind {
	case constants.ChatMessageKindReply:
		return constants.ConversationMessageText, nil
	case constants.ChatMessageKindTaskDraft:
		return constants.ConversationMessageTaskDraft, nil
	default:
		return "", errs.ErrBadRequest.WithMessage(fmt.Sprintf("unknown messageKind %q for a chat turn, use %s or %s",
			body.MessageKind, constants.ChatMessageKindReply, constants.ChatMessageKindTaskDraft))
	}
}

// NextTask returns the pending turn or a new one. Every chat turn is a stage = chat task.
func (r *ChatRun) NextTask(ctx context.Context, run *model.AgentRun, tasks []*model.AgentTask) (*model.AgentTask, error) {
	if pending := latestPendingTask(tasks); pending != nil {
		return pending, nil
	}
	return r.taskRepo.Insert(ctx, run.IdRun, run.IdUserAgent, constants.StageChat, ResolveNextAttemptNo(tasks, constants.StageChat))
}

// Activate starts a chat turn's attempt: under the conversation and run locks it binds a fresh chat
// token to the run and fixes the turn's mode before the task becomes active. No run snapshot is
// broadcast, as a chat run is private to its owner. A run whose conversation is gone is cancelled
// and not dispatched.
func (r *ChatRun) Activate(ctx context.Context, run *model.AgentRun, task *model.AgentTask) (*Dispatch, error) {
	rawToken, hash, err := MintRunToken()
	if err != nil {
		return nil, err
	}
	var active *model.AgentTask
	var running *model.AgentRun
	isOrphaned := false
	err = extctx.RunInTx(ctx, r.pool, func(ctx context.Context) error {
		conversation, err := r.conversationRepo.LoadConversationByRunForUpdate(ctx, run.IdRun)
		if err != nil {
			return err
		}
		if conversation == nil {
			isOrphaned = true
			return r.cancelOrphan(ctx, run.IdRun)
		}
		running, err = r.runRepo.LoadByIdForUpdate(ctx, run.IdRun)
		if err != nil {
			return err
		}
		if err := r.runRepo.SetRunToken(ctx, run.IdRun, hash, time.Now().Add(constants.RunTokenLifetime)); err != nil {
			return err
		}
		mode := constants.ChatModeReply
		if conversation.TaskDraftRequest != nil && *conversation.TaskDraftRequest == constants.TaskDraftNextTurn {
			mode = constants.ChatModeTaskDraft
		}
		if _, err := r.taskRepo.SetChatMode(ctx, task.IdTask, mode); err != nil {
			return err
		}
		active, err = r.taskRepo.TransitionStatus(ctx, task.IdTask, constants.TaskStatusPending, constants.TaskStatusActive)
		if err != nil {
			return err
		}
		if running.Phase == constants.PhaseQueued {
			running, err = r.runRepo.TransitionPhase(ctx, run.IdRun, constants.PhaseQueued, constants.PhaseInProgress,
				constants.ActorTypeSystem, nil, "dispatch chat turn")
		}
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("activating chat turn %d: %w", task.IdTask, err)
	}
	if isOrphaned {
		log.Warn().Int64("idRun", run.IdRun).Msg("chat run without a conversation cancelled")
		return nil, nil
	}
	return &Dispatch{Run: running, Task: active, RunToken: rawToken}, nil
}

// Payload carries the conversation as history and the turn's run token.
func (r *ChatRun) Payload(ctx context.Context, dispatch *Dispatch) (map[string]any, []*model.AgentRunWikiRead, error) {
	payload, err := r.buildChatPayload(ctx, dispatch.Run, dispatch.Task, dispatch.RunToken)
	return payload, nil, err
}

// cancelOrphan cancels a chat run whose conversation is gone: expires its token and cancels its
// open turns. Call it inside a transaction.
func (r *ChatRun) cancelOrphan(ctx context.Context, idRun int64) error {
	locked, err := r.runRepo.LoadByIdForUpdate(ctx, idRun)
	if err != nil {
		return err
	}
	if err := r.runRepo.ExpireRunToken(ctx, idRun); err != nil {
		return err
	}
	if _, err := r.taskRepo.CancelNonTerminalForRun(ctx, idRun); err != nil {
		return err
	}
	if constants.TerminalPhases[locked.Phase] {
		return nil
	}
	_, err = r.runRepo.TransitionPhase(ctx, idRun, locked.Phase, constants.PhaseCancelled,
		constants.ActorTypeSystem, nil, "conversation deleted")
	return err
}

// turnFailure describes how a turn ends in error: isFailedAccepted lets a task already failed by
// sweep or gateway restart end its turn, taskReason replaces the task's error reason when set.
type turnFailure struct {
	reason           string
	detail           string
	isFailedAccepted bool
	taskReason       string
}

// fail ends the turn of a task with a system error message whose body is the reason code. No-op for
// a task that is no longer active, unless the failure accepts a failed one, or no longer the run's
// latest attempt; returns the run's next phase.
func (r *ChatRun) fail(ctx context.Context, run *model.AgentRun, task *model.AgentTask, failure turnFailure) (string, error) {
	var message *model.AgentConversationMessage
	var idOwner int64
	nextPhase := run.Phase
	err := extctx.RunInTx(ctx, r.pool, func(ctx context.Context) error {
		conversation, err := r.conversationRepo.LoadConversationByRunForUpdate(ctx, run.IdRun)
		if err != nil {
			return err
		}
		if conversation == nil {
			nextPhase = constants.PhaseCancelled
			return r.cancelOrphan(ctx, run.IdRun)
		}
		// The run lock orders this against a re-queue inserting a newer attempt.
		if _, err := r.runRepo.LoadByIdForUpdate(ctx, run.IdRun); err != nil {
			return err
		}
		tasks, err := r.taskRepo.LoadByRun(ctx, run.IdRun)
		if err != nil {
			return err
		}
		latest := latestChatTask(tasks)
		if latest == nil || latest.IdTask != task.IdTask {
			return nil
		}
		isFailed, err := r.failTask(ctx, latest, failure)
		if err != nil || !isFailed {
			return err
		}
		message, err = r.conversationRepo.InsertMessage(ctx, conversation.IdConversation,
			constants.ConversationAuthorSystem, constants.ConversationMessageError, failure.reason, nil, &task.IdTask)
		if err != nil {
			return err
		}
		idOwner = conversation.IdUser
		nextPhase, err = r.endTurn(ctx, run.IdRun, conversation, latest)
		return err
	})
	if err != nil {
		return "", fmt.Errorf("failing chat turn %d: %w", task.IdTask, err)
	}
	r.notifyOwner(idOwner, message)
	return nextPhase, nil
}

// failTask reports whether the task ends failed: an active task fails, an already failed one only
// when the failure allows it, a completed or cancelled one never.
func (r *ChatRun) failTask(ctx context.Context, task *model.AgentTask, failure turnFailure) (bool, error) {
	_, err := r.taskRepo.TransitionStatus(ctx, task.IdTask, constants.TaskStatusActive, constants.TaskStatusFailed)
	switch {
	case err == nil:
		return true, r.taskRepo.SetError(ctx, task.IdTask, cmp.Or(failure.taskReason, failure.reason), failure.detail)
	case !errors.Is(err, repository.ErrTaskStatusMismatch):
		return false, err
	case !failure.isFailedAccepted || task.Status != constants.TaskStatusFailed:
		return false, nil
	case failure.taskReason != "":
		return true, r.taskRepo.SetError(ctx, task.IdTask, failure.taskReason, failure.detail)
	default:
		return true, nil
	}
}

// Interrupt never fails the conversation. A turn the gateway lost on restart is re-queued; any other
// lost turn ends with an error message. An attempt a newer one superseded is left as it is.
func (r *ChatRun) Interrupt(ctx context.Context, run *model.AgentRun, task *model.AgentTask, reason InterruptReason, detail string) error {
	if reason == InterruptGatewayRestart {
		tasks, err := r.taskRepo.LoadByRun(ctx, run.IdRun)
		if err != nil {
			return err
		}
		latest := latestChatTask(tasks)
		if latest == nil || latest.IdTask != task.IdTask {
			return nil
		}
		return r.requeueAfterRestart(ctx, run, tasks, latest)
	}
	_, err := r.fail(ctx, run, task, turnFailure{reason: string(reason), detail: detail, isFailedAccepted: reason.isTaskFailedByCaller()})
	return err
}

// requeueAfterRestart re-queues a chat turn the gateway lost on restart, until ChatTurnMaxRestarts
// restarts in a row hit it; then the turn ends with an error message and its last attempt is marked exhausted.
func (r *ChatRun) requeueAfterRestart(ctx context.Context, run *model.AgentRun, tasks []*model.AgentTask, latest *model.AgentTask) error {
	idRun := run.IdRun
	if consecutiveRestarts(tasks) >= constants.ChatTurnMaxRestarts {
		_, err := r.fail(ctx, run, latest, turnFailure{
			reason:           constants.FailReasonGatewayRestart,
			isFailedAccepted: true,
			taskReason:       constants.FailReasonGatewayRestartExhausted,
		})
		return err
	}
	return extctx.RunInTx(ctx, r.pool, func(ctx context.Context) error {
		locked, err := r.runRepo.LoadByIdForUpdate(ctx, idRun)
		if err != nil {
			return err
		}
		if locked.Phase != constants.PhaseInProgress {
			return nil
		}
		if err := r.runRepo.ExpireRunToken(ctx, idRun); err != nil {
			return err
		}
		retry, err := r.taskRepo.Insert(ctx, idRun, locked.IdUserAgent, constants.StageChat,
			ResolveNextAttemptNo(tasks, constants.StageChat))
		if err != nil {
			return err
		}
		if latest.ChatMode != nil {
			if _, err := r.taskRepo.SetChatMode(ctx, retry.IdTask, *latest.ChatMode); err != nil {
				return err
			}
		}
		_, err = r.runRepo.TransitionPhase(ctx, idRun, locked.Phase, constants.PhaseQueued,
			constants.ActorTypeSystem, nil, constants.FailReasonGatewayRestart)
		return err
	})
}

// ApplyReport stores the commit the turn's worktree is at; a changed commit after the first tells
// the owner with a code_refreshed message.
func (r *ChatRun) ApplyReport(ctx context.Context, run *model.AgentRun, task *model.AgentTask, report *TaskReport) error {
	if report.HeadCommit == "" {
		return nil
	}
	headCommit, idTask := report.HeadCommit, task.IdTask
	var message *model.AgentConversationMessage
	var idOwner int64
	err := extctx.RunInTx(ctx, r.pool, func(ctx context.Context) error {
		conversation, err := r.conversationRepo.LoadConversationByRunForUpdate(ctx, run.IdRun)
		if err != nil || conversation == nil {
			return err
		}
		previous := conversation.HeadCommit
		if previous != nil && *previous == headCommit {
			return nil
		}
		if err := r.conversationRepo.SetHeadCommit(ctx, conversation.IdConversation, headCommit); err != nil {
			return err
		}
		if previous == nil {
			return nil
		}
		message, err = r.conversationRepo.InsertMessage(ctx, conversation.IdConversation,
			constants.ConversationAuthorSystem, constants.ConversationMessageCodeRefreshed, headCommit, nil, &idTask)
		idOwner = conversation.IdUser
		return err
	})
	if err != nil {
		return fmt.Errorf("recording head commit of chat run %d: %w", run.IdRun, err)
	}
	r.notifyOwner(idOwner, message)
	return nil
}

// ApplyPrStatus fails: a chat run opens no PR.
func (r *ChatRun) ApplyPrStatus(_ context.Context, run *model.AgentRun, _ *githost.Status) (*model.Issue, error) {
	return nil, fmt.Errorf("chat run %d: %w", run.IdRun, errRunHasNoPr)
}

// SendFeedback refuses: a chat turn answers each message on its own.
func (r *ChatRun) SendFeedback(context.Context, *model.AgentRun, int64) (*model.AgentRun, error) {
	return nil, errs.ErrRunNotAwaitingFeedback
}

// Audience is the conversation owner alone; nobody when the conversation is gone.
func (r *ChatRun) Audience(ctx context.Context, run *model.AgentRun) ([]int64, error) {
	conversation, err := r.conversationRepo.LoadConversationByRun(ctx, run.IdRun)
	if err != nil || conversation == nil || conversation.IdUser == 0 {
		return nil, err
	}
	return []int64{conversation.IdUser}, nil
}

// endTurn retires the turn's token, settles the owner's task draft request and parks the run:
// queued when a draft is due or the owner wrote after the turn started, else awaiting_input.
// A cancelled run stays cancelled.
func (r *ChatRun) endTurn(ctx context.Context, idRun int64, conversation *model.AgentConversation, task *model.AgentTask) (string, error) {
	if err := r.runRepo.ExpireRunToken(ctx, idRun); err != nil {
		return "", err
	}
	request, idQuoted := conversation.TaskDraftRequest, conversation.IdTaskDraftMessage
	switch {
	case task.ChatMode != nil && *task.ChatMode == constants.ChatModeTaskDraft:
		request, idQuoted = nil, nil
	case request != nil && *request == constants.TaskDraftAfterTurn:
		next := constants.TaskDraftNextTurn
		request = &next
	}
	if !equalStringPtr(request, conversation.TaskDraftRequest) {
		if err := r.conversationRepo.SetTaskDraftRequest(ctx, conversation.IdConversation, request, idQuoted); err != nil {
			return "", err
		}
	}
	locked, err := r.runRepo.LoadByIdForUpdate(ctx, idRun)
	if err != nil {
		return "", err
	}
	if constants.TerminalPhases[locked.Phase] {
		return locked.Phase, nil
	}
	since := task.CreatedAt
	if task.StartedAt != nil {
		since = *task.StartedAt
	}
	hasNewMessage, err := r.conversationRepo.HasUserMessageSince(ctx, conversation.IdConversation, since)
	if err != nil {
		return "", err
	}
	nextPhase := constants.PhaseAwaitingInput
	if hasNewMessage || request != nil {
		nextPhase = constants.PhaseQueued
	}
	if locked.Phase == nextPhase {
		return nextPhase, nil
	}
	if _, err := r.runRepo.TransitionPhase(ctx, idRun, locked.Phase, nextPhase, constants.ActorTypeSystem, nil, "chat turn ended"); err != nil {
		return "", err
	}
	return nextPhase, nil
}

func equalStringPtr(left, right *string) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}

// Started tells the owner that the agent took up the turn, so the conversation shows the agent
// answering before its first thinking arrives.
func (r *ChatRun) Started(ctx context.Context, dispatch *Dispatch) {
	run, task := dispatch.Run, dispatch.Task
	conversation, err := r.conversationRepo.LoadConversationByRun(ctx, run.IdRun)
	if err != nil {
		log.Warn().Err(err).Int64("idRun", run.IdRun).Msg("loading the conversation of a started chat turn")
		return
	}
	if conversation == nil {
		return
	}
	r.notifier.Send(&notify.Notice{
		IdsUser: []int64{conversation.IdUser},
		Subject: notify.SubjectAgentConversationTurn,
		Action:  notify.ActionUpdate,
		Payload: &model.AgentConversationTurnNotice{IdConversation: conversation.IdConversation, IdTask: task.IdTask},
	})
}

// notifyOwner tells the owner about a new message; without an owner nothing is sent, as an
// unaddressed notice would reach every session.
func (r *ChatRun) notifyOwner(idOwner int64, message *model.AgentConversationMessage) {
	if idOwner == 0 || message == nil {
		return
	}
	source := ""
	if message.Author == constants.ConversationAuthorAgent {
		source = "agent"
	}
	r.notifier.Send(&notify.Notice{
		IdsUser: []int64{idOwner},
		Subject: notify.SubjectAgentConversation,
		Action:  notify.ActionCreate,
		Payload: message,
		Source:  source,
	})
}

func latestChatTask(tasks []*model.AgentTask) *model.AgentTask {
	var latest *model.AgentTask
	for _, task := range tasks {
		if latest == nil || task.IdTask > latest.IdTask {
			latest = task
		}
	}
	return latest
}

// consecutiveRestarts counts the run's newest tasks failed by a gateway restart in a row; a
// completed or otherwise failed attempt, or an exhausted one, breaks the chain.
func consecutiveRestarts(tasks []*model.AgentTask) int {
	newestFirst := slices.Clone(tasks)
	slices.SortFunc(newestFirst, func(left, right *model.AgentTask) int {
		return cmp.Compare(right.IdTask, left.IdTask)
	})
	count := 0
	for _, task := range newestFirst {
		if task.Status != constants.TaskStatusFailed || task.ErrorReason == nil ||
			*task.ErrorReason != constants.FailReasonGatewayRestart {
			break
		}
		count++
	}
	return count
}
