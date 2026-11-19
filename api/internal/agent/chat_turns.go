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
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// chatThinkingCompactor folds a finished turn's thinking into its task.
type chatThinkingCompactor interface {
	Compact(ctx context.Context, idsTask ...int64)
}

// ChatTurns ends the turns of a chat run: a reply becomes an agent message, a failed turn an error
// message or a retry. A turn never fails its run; every notice goes to the conversation owner only.
type ChatTurns struct {
	pool             *pgxpool.Pool
	conversationRepo *repository.AgentConversationRepository
	runRepo          *repository.AgentRunRepository
	taskRepo         *repository.AgentTaskRepository
	thinking         chatThinkingCompactor
	notifier         notify.INotifier
}

func NewChatTurns(
	pool *pgxpool.Pool,
	conversationRepo *repository.AgentConversationRepository,
	runRepo *repository.AgentRunRepository,
	taskRepo *repository.AgentTaskRepository,
	thinking chatThinkingCompactor,
	notifier notify.INotifier,
) *ChatTurns {
	return &ChatTurns{
		pool:             pool,
		conversationRepo: conversationRepo,
		runRepo:          runRepo,
		taskRepo:         taskRepo,
		thinking:         thinking,
		notifier:         notifier,
	}
}

// Complete records complete_stage for a chat task: output_submitted stores the reply or task draft,
// errored ends the turn with an error message. A task that is no longer active is a no-op. The
// turn's thinking is compacted before the reply goes out, so the notice tells whether the reply
// has thinking even when only the tail is kept.
func (t *ChatTurns) Complete(ctx context.Context, run *model.AgentRun, task *model.AgentTask, body *model.CompleteStageReq) (model.CompleteStageRes, error) {
	unchanged := model.CompleteStageRes{IdTask: task.IdTask, Status: task.Status, NextPhase: run.Phase}
	if body.Outcome == constants.StageOutcomeErrored {
		if task.Status != constants.TaskStatusActive {
			return unchanged, nil
		}
		if err := t.taskRepo.SetResultAndStats(ctx, task.IdTask, nil, body.TokensUsed, body.DurationMs, body.ToolCallsCount); err != nil {
			return model.CompleteStageRes{}, err
		}
		reason, detail := "agent_reported_error", ""
		if body.ErrorReason != nil && *body.ErrorReason != "" {
			reason = *body.ErrorReason
		}
		if body.ErrorDetail != nil {
			detail = *body.ErrorDetail
		}
		nextPhase, err := t.Fail(ctx, run, task, reason, detail)
		if err != nil {
			return model.CompleteStageRes{}, err
		}
		t.thinking.Compact(ctx, task.IdTask)
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
	err = extctx.RunInTx(ctx, t.pool, func(ctx context.Context) error {
		// Lock the conversation first, as every turn transaction does, to avoid deadlock.
		conversation, err := t.conversationRepo.LoadConversationByRunForUpdate(ctx, run.IdRun)
		if err != nil {
			return err
		}
		if conversation == nil {
			nextPhase = constants.PhaseCancelled
			return t.cancelOrphan(ctx, run.IdRun)
		}
		if _, err := t.taskRepo.TransitionStatus(ctx, task.IdTask, constants.TaskStatusActive, constants.TaskStatusCompleted); err != nil {
			if errors.Is(err, repository.ErrTaskStatusMismatch) {
				// A concurrent completion (gateway and MCP tool) won the task.
				return nil
			}
			return err
		}
		if err := t.taskRepo.SetResultAndStats(ctx, task.IdTask, nil, body.TokensUsed, body.DurationMs, body.ToolCallsCount); err != nil {
			return err
		}
		message, err = t.conversationRepo.InsertMessage(ctx, conversation.IdConversation,
			constants.ConversationAuthorAgent, kind, body.Message, &run.IdUserAgent, &task.IdTask)
		if err != nil {
			return err
		}
		idOwner = conversation.IdUser
		nextPhase, err = t.endTurn(ctx, run.IdRun, conversation, task)
		return err
	})
	if err != nil {
		return model.CompleteStageRes{}, err
	}
	t.thinking.Compact(ctx, task.IdTask)
	if nextPhase == constants.PhaseCancelled && message == nil {
		return model.CompleteStageRes{IdTask: task.IdTask, Status: constants.TaskStatusCancelled, NextPhase: nextPhase}, nil
	}
	if message == nil {
		return unchanged, nil
	}
	// The tail of an unpersisted turn reaches the task only at compaction.
	if compacted, err := t.conversationRepo.LoadMessage(ctx, message.IdConversation, message.IdMessage); err == nil && compacted != nil {
		message = compacted
	}
	t.notifyOwner(idOwner, message)
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

// Activate starts a chat turn's attempt: under the conversation and run locks it binds a fresh chat
// token to the run and fixes the turn's mode before the task becomes active. Returns the running
// run, the active task and the raw token, or ErrChatRunOrphaned when the conversation is gone.
func (t *ChatTurns) Activate(ctx context.Context, run *model.AgentRun, task *model.AgentTask) (*model.AgentRun, *model.AgentTask, string, error) {
	rawToken, hash, err := MintRunToken()
	if err != nil {
		return nil, nil, "", err
	}
	var active *model.AgentTask
	var running *model.AgentRun
	isOrphaned := false
	err = extctx.RunInTx(ctx, t.pool, func(ctx context.Context) error {
		conversation, err := t.conversationRepo.LoadConversationByRunForUpdate(ctx, run.IdRun)
		if err != nil {
			return err
		}
		if conversation == nil {
			isOrphaned = true
			return t.cancelOrphan(ctx, run.IdRun)
		}
		running, err = t.runRepo.LoadByIdForUpdate(ctx, run.IdRun)
		if err != nil {
			return err
		}
		if err := t.runRepo.SetRunToken(ctx, run.IdRun, hash, time.Now().Add(constants.RunTokenLifetime)); err != nil {
			return err
		}
		mode := constants.ChatModeReply
		if conversation.TaskDraftRequest != nil && *conversation.TaskDraftRequest == constants.TaskDraftNextTurn {
			mode = constants.ChatModeTaskDraft
		}
		if _, err := t.taskRepo.SetChatMode(ctx, task.IdTask, mode); err != nil {
			return err
		}
		active, err = t.taskRepo.TransitionStatus(ctx, task.IdTask, constants.TaskStatusPending, constants.TaskStatusActive)
		if err != nil {
			return err
		}
		if running.Phase == constants.PhaseQueued {
			running, err = t.runRepo.TransitionPhase(ctx, run.IdRun, constants.PhaseQueued, constants.PhaseInProgress,
				constants.ActorTypeSystem, nil, "dispatch chat turn")
		}
		return err
	})
	if err != nil {
		return nil, nil, "", fmt.Errorf("activating chat turn %d: %w", task.IdTask, err)
	}
	if isOrphaned {
		return nil, nil, "", ErrChatRunOrphaned
	}
	return running, active, rawToken, nil
}

// ErrChatRunOrphaned reports a chat run that was cancelled because its
// conversation is gone.
var ErrChatRunOrphaned = errors.New("chat run has no conversation")

// cancelOrphan cancels a chat run whose conversation is gone: expires its token and cancels its
// open turns. Call it inside a transaction.
func (t *ChatTurns) cancelOrphan(ctx context.Context, idRun int64) error {
	locked, err := t.runRepo.LoadByIdForUpdate(ctx, idRun)
	if err != nil {
		return err
	}
	if err := t.runRepo.ExpireRunToken(ctx, idRun); err != nil {
		return err
	}
	if _, err := t.taskRepo.CancelNonTerminalForRun(ctx, idRun); err != nil {
		return err
	}
	if constants.TerminalPhases[locked.Phase] {
		return nil
	}
	_, err = t.runRepo.TransitionPhase(ctx, idRun, locked.Phase, constants.PhaseCancelled,
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

// Fail ends the turn of an active task with a system error message whose body is the reason code.
// No-op for a task that is no longer active or no longer the run's latest attempt; returns the run's next phase.
func (t *ChatTurns) Fail(ctx context.Context, run *model.AgentRun, task *model.AgentTask, reason, detail string) (string, error) {
	return t.fail(ctx, run, task, turnFailure{reason: reason, detail: detail})
}

func (t *ChatTurns) fail(ctx context.Context, run *model.AgentRun, task *model.AgentTask, failure turnFailure) (string, error) {
	var message *model.AgentConversationMessage
	var idOwner int64
	nextPhase := run.Phase
	err := extctx.RunInTx(ctx, t.pool, func(ctx context.Context) error {
		conversation, err := t.conversationRepo.LoadConversationByRunForUpdate(ctx, run.IdRun)
		if err != nil {
			return err
		}
		if conversation == nil {
			nextPhase = constants.PhaseCancelled
			return t.cancelOrphan(ctx, run.IdRun)
		}
		// The run lock orders this against a re-queue inserting a newer attempt.
		if _, err := t.runRepo.LoadByIdForUpdate(ctx, run.IdRun); err != nil {
			return err
		}
		tasks, err := t.taskRepo.LoadByRun(ctx, run.IdRun)
		if err != nil {
			return err
		}
		latest := latestChatTask(tasks)
		if latest == nil || latest.IdTask != task.IdTask {
			return nil
		}
		isFailed, err := t.failTask(ctx, latest, failure)
		if err != nil || !isFailed {
			return err
		}
		message, err = t.conversationRepo.InsertMessage(ctx, conversation.IdConversation,
			constants.ConversationAuthorSystem, constants.ConversationMessageError, failure.reason, nil, &task.IdTask)
		if err != nil {
			return err
		}
		idOwner = conversation.IdUser
		nextPhase, err = t.endTurn(ctx, run.IdRun, conversation, latest)
		return err
	})
	if err != nil {
		return "", fmt.Errorf("failing chat turn %d: %w", task.IdTask, err)
	}
	t.notifyOwner(idOwner, message)
	return nextPhase, nil
}

// failTask reports whether the task ends failed: an active task fails, an already failed one only
// when the failure allows it, a completed or cancelled one never.
func (t *ChatTurns) failTask(ctx context.Context, task *model.AgentTask, failure turnFailure) (bool, error) {
	_, err := t.taskRepo.TransitionStatus(ctx, task.IdTask, constants.TaskStatusActive, constants.TaskStatusFailed)
	switch {
	case err == nil:
		return true, t.taskRepo.SetError(ctx, task.IdTask, cmp.Or(failure.taskReason, failure.reason), failure.detail)
	case !errors.Is(err, repository.ErrTaskStatusMismatch):
		return false, err
	case !failure.isFailedAccepted || task.Status != constants.TaskStatusFailed:
		return false, nil
	case failure.taskReason != "":
		return true, t.taskRepo.SetError(ctx, task.IdTask, failure.taskReason, failure.detail)
	default:
		return true, nil
	}
}

// FailRun ends the latest turn of a chat run whose task the heartbeat sweep already failed.
func (t *ChatTurns) FailRun(ctx context.Context, idRun int64, reason string) error {
	run, tasks, err := t.loadRunWithTasks(ctx, idRun)
	if err != nil {
		return err
	}
	latest := latestChatTask(tasks)
	if latest == nil {
		return nil
	}
	_, err = t.fail(ctx, run, latest, turnFailure{reason: reason, isFailedAccepted: true})
	return err
}

// GatewayRestarted re-queues a chat turn the gateway lost on restart, until ChatTurnMaxRestarts
// restarts in a row hit it; then the turn ends with an error message and its last attempt is marked exhausted.
func (t *ChatTurns) GatewayRestarted(ctx context.Context, idRun int64) error {
	run, tasks, err := t.loadRunWithTasks(ctx, idRun)
	if err != nil {
		return err
	}
	latest := latestChatTask(tasks)
	if latest == nil {
		return nil
	}
	if consecutiveRestarts(tasks) >= constants.ChatTurnMaxRestarts {
		_, err := t.fail(ctx, run, latest, turnFailure{
			reason:           constants.FailReasonGatewayRestart,
			isFailedAccepted: true,
			taskReason:       constants.FailReasonGatewayRestartExhausted,
		})
		return err
	}
	return extctx.RunInTx(ctx, t.pool, func(ctx context.Context) error {
		locked, err := t.runRepo.LoadByIdForUpdate(ctx, idRun)
		if err != nil {
			return err
		}
		if locked.Phase != constants.PhaseInProgress {
			return nil
		}
		if err := t.runRepo.ExpireRunToken(ctx, idRun); err != nil {
			return err
		}
		retry, err := t.taskRepo.Insert(ctx, idRun, locked.IdUserAgent, constants.StageChat,
			ResolveNextAttemptNo(tasks, constants.StageChat))
		if err != nil {
			return err
		}
		if latest.ChatMode != nil {
			if _, err := t.taskRepo.SetChatMode(ctx, retry.IdTask, *latest.ChatMode); err != nil {
				return err
			}
		}
		_, err = t.runRepo.TransitionPhase(ctx, idRun, locked.Phase, constants.PhaseQueued,
			constants.ActorTypeSystem, nil, constants.FailReasonGatewayRestart)
		return err
	})
}

// ReportHeadCommit stores the commit the turn's worktree is at; a changed commit after the first
// tells the owner with a code_refreshed message.
func (t *ChatTurns) ReportHeadCommit(ctx context.Context, run *model.AgentRun, idTask int64, headCommit string) error {
	var message *model.AgentConversationMessage
	var idOwner int64
	err := extctx.RunInTx(ctx, t.pool, func(ctx context.Context) error {
		conversation, err := t.conversationRepo.LoadConversationByRunForUpdate(ctx, run.IdRun)
		if err != nil || conversation == nil {
			return err
		}
		previous := conversation.HeadCommit
		if previous != nil && *previous == headCommit {
			return nil
		}
		if err := t.conversationRepo.SetHeadCommit(ctx, conversation.IdConversation, headCommit); err != nil {
			return err
		}
		if previous == nil {
			return nil
		}
		message, err = t.conversationRepo.InsertMessage(ctx, conversation.IdConversation,
			constants.ConversationAuthorSystem, constants.ConversationMessageCodeRefreshed, headCommit, nil, &idTask)
		idOwner = conversation.IdUser
		return err
	})
	if err != nil {
		return fmt.Errorf("recording head commit of chat run %d: %w", run.IdRun, err)
	}
	t.notifyOwner(idOwner, message)
	return nil
}

// OwnerOf returns the owner of the chat run's conversation, or 0 when there is none.
func (t *ChatTurns) OwnerOf(ctx context.Context, idRun int64) (int64, error) {
	conversation, err := t.conversationRepo.LoadConversationByRun(ctx, idRun)
	if err != nil || conversation == nil {
		return 0, err
	}
	return conversation.IdUser, nil
}

// endTurn retires the turn's token, settles the owner's task draft request and parks the run:
// queued when a draft is due or the owner wrote after the turn started, else awaiting_input.
// A cancelled run stays cancelled.
func (t *ChatTurns) endTurn(ctx context.Context, idRun int64, conversation *model.AgentConversation, task *model.AgentTask) (string, error) {
	if err := t.runRepo.ExpireRunToken(ctx, idRun); err != nil {
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
		if err := t.conversationRepo.SetTaskDraftRequest(ctx, conversation.IdConversation, request, idQuoted); err != nil {
			return "", err
		}
	}
	locked, err := t.runRepo.LoadByIdForUpdate(ctx, idRun)
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
	hasNewMessage, err := t.conversationRepo.HasUserMessageSince(ctx, conversation.IdConversation, since)
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
	if _, err := t.runRepo.TransitionPhase(ctx, idRun, locked.Phase, nextPhase, constants.ActorTypeSystem, nil, "chat turn ended"); err != nil {
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

func (t *ChatTurns) loadRunWithTasks(ctx context.Context, idRun int64) (*model.AgentRun, []*model.AgentTask, error) {
	run, err := t.runRepo.LoadById(ctx, idRun)
	if err != nil {
		return nil, nil, err
	}
	tasks, err := t.taskRepo.LoadByRun(ctx, idRun)
	if err != nil {
		return nil, nil, err
	}
	return run, tasks, nil
}

// NotifyTurnStarted tells the owner that the agent took up the turn, so the conversation shows the
// agent answering before its first thinking arrives.
func (t *ChatTurns) NotifyTurnStarted(ctx context.Context, run *model.AgentRun, task *model.AgentTask) {
	conversation, err := t.conversationRepo.LoadConversationByRun(ctx, run.IdRun)
	if err != nil {
		log.Warn().Err(err).Int64("idRun", run.IdRun).Msg("loading the conversation of a started chat turn")
		return
	}
	if conversation == nil {
		return
	}
	t.notifier.Send(&notify.Notice{
		IdsUser: []int64{conversation.IdUser},
		Subject: notify.SubjectAgentConversationTurn,
		Action:  notify.ActionUpdate,
		Payload: &model.AgentConversationTurnNotice{IdConversation: conversation.IdConversation, IdTask: task.IdTask},
	})
}

// notifyOwner tells the owner about a new message; without an owner nothing is sent, as an
// unaddressed notice would reach every session.
func (t *ChatTurns) notifyOwner(idOwner int64, message *model.AgentConversationMessage) {
	if idOwner == 0 || message == nil {
		return
	}
	source := ""
	if message.Author == constants.ConversationAuthorAgent {
		source = "agent"
	}
	t.notifier.Send(&notify.Notice{
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
