package common

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

var errTaskGone = errors.New("tracker no longer runs this task")

type activeTask struct {
	task   Task
	cancel context.CancelFunc
	// discarded asks runStage to remove the run's worktree once the agent stops.
	discarded bool
	// isPastCleanup is set once runStage no longer checks discarded, so a
	// later run_discarded has to remove the worktree itself.
	isPastCleanup bool
}

// Orchestrator dispatches stage_execute webhook events to the agent and
// reports failures back to the tracker. The API owns scheduling and marks
// stale tasks failed via its sweep; the gateway accepts every webhook but
// runs one stage attempt at a time (sem), queueing the rest
// in memory. Serialization matters because the CLI adapters share a single
// OAuth credential — concurrent processes would race on that one session.
type Orchestrator struct {
	cfg           *Config
	agent         Agent
	trackerClient *TrackerClient
	state         *State
	heartbeatLoop *HeartbeatLoop

	sem chan struct{} // bounds runStage execution to one at a time

	mu          sync.Mutex
	activeTasks map[int64]*activeTask // idTask -> active
}

func NewOrchestrator(
	cfg *Config,
	agent Agent,
	trackerClient *TrackerClient,
	state *State,
) *Orchestrator {
	return &Orchestrator{
		cfg:           cfg,
		agent:         agent,
		trackerClient: trackerClient,
		state:         state,
		heartbeatLoop: NewHeartbeatLoop(trackerClient),
		sem:           make(chan struct{}, 1),
		activeTasks:   make(map[int64]*activeTask),
	}
}

func (o *Orchestrator) HandleEvent(event WebhookEvent) error {
	switch event.Type {
	case "stage_execute":
		return o.handleStageExecute(event)
	case "cancelled":
		return o.handleCancelled(event)
	case "run_discarded":
		return o.handleRunDiscarded(event)
	default:
		log.Warn().Str("type", event.Type).Str("eventId", event.EventId).Msg("unhandled event type — nothing dispatched")
		return nil
	}
}

func (o *Orchestrator) handleStageExecute(event WebhookEvent) error {
	task, err := parseStageExecutePayload(event)
	if err != nil {
		return fmt.Errorf("parsing stage_execute payload: %w", err)
	}
	log.Info().
		Int64("idTask", task.IdTask).
		Int64("idRun", task.IdRun).
		Str("stage", task.Stage).
		Int("attempt", task.AttemptNo).
		Msg("stage_execute received")

	// Pickup is a no-op stage: no LLM call, no worktree — just confirms
	// receipt so the API can advance.
	if task.Stage == StagePickup {
		go o.completeTaskNoAction(task)
		return nil
	}

	// Other stages need a worktree: first attempt creates the branch,
	// later stages reuse it.
	go o.runStage(task)
	return nil
}

// handleCancelled stops the run's stage but keeps its worktree, so Continue
// can resume the unfinished work.
func (o *Orchestrator) handleCancelled(event WebhookEvent) error {
	if event.IdRun == 0 {
		return fmt.Errorf("cancelled event missing idRun")
	}
	toCancel, _ := o.activeTasksOfRun(event.IdRun, false)
	o.cancelTasks(toCancel)
	return nil
}

// handleRunDiscarded stops the run's stage and removes its worktree after
// Restart replaced the run. Only attempts running now are marked, so a later
// Continue of the same run keeps its new worktree.
func (o *Orchestrator) handleRunDiscarded(event WebhookEvent) error {
	if event.IdRun == 0 {
		return fmt.Errorf("run_discarded event missing idRun")
	}
	toCancel, isWorktreeUnowned := o.activeTasksOfRun(event.IdRun, true)
	o.cancelTasks(toCancel)
	if isWorktreeUnowned {
		go o.removeDiscardedWorktree(event.IdRun)
	}
	return nil
}

// activeTasksOfRun also reports whether no task will remove the run's worktree
// itself.
func (o *Orchestrator) activeTasksOfRun(idRun int64, isDiscarded bool) (tasks []*activeTask, isWorktreeUnowned bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	isWorktreeUnowned = true
	for _, at := range o.activeTasks {
		if at.task.IdRun == idRun {
			at.discarded = at.discarded || isDiscarded
			isWorktreeUnowned = isWorktreeUnowned && at.isPastCleanup
			tasks = append(tasks, at)
		}
	}
	return tasks, isWorktreeUnowned
}

func (o *Orchestrator) cancelTasks(toCancel []*activeTask) {
	for _, at := range toCancel {
		if at.cancel != nil {
			at.cancel()
		}
		runIDKey := RunID(fmt.Sprintf("%d", at.task.IdRun))
		if err := o.agent.Cancel(context.Background(), runIDKey); err != nil {
			log.Warn().Int64("idRun", at.task.IdRun).Err(err).Msg("agent cancel returned error")
		}
	}
}

// removeWorktreeIfDiscarded runs while the stage still holds the semaphore, so
// no later attempt of the run can have created a new worktree yet.
func (o *Orchestrator) removeWorktreeIfDiscarded(idTask, idRun int64) {
	if o.finishCleanup(idTask) {
		o.removeRunWorktree(idRun)
	}
}

// finishCleanup reports whether the task was discarded and hands any later
// run_discarded the removal of the worktree.
func (o *Orchestrator) finishCleanup(idTask int64) (isDiscarded bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	at := o.activeTasks[idTask]
	if at == nil {
		return false
	}
	at.isPastCleanup = true
	return at.discarded
}

// removeDiscardedWorktree waits for a stage slot first. Slots go to waiters
// in arrival order, so a Continue of the run sent after Restart cannot reuse
// the worktree before it is removed.
func (o *Orchestrator) removeDiscardedWorktree(idRun int64) {
	o.sem <- struct{}{}
	defer func() { <-o.sem }()
	o.removeRunWorktree(idRun)
}

func (o *Orchestrator) removeRunWorktree(idRun int64) {
	clones, err := ListClones(o.cfg.WorkspaceBase)
	if err != nil {
		log.Warn().Int64("idRun", idRun).Err(err).Msg("cannot list clones to remove a discarded worktree")
		return
	}
	for _, repoPath := range clones {
		if !WorktreeExists(repoPath, idRun) {
			continue
		}
		if err := RemoveWorktree(repoPath, idRun); err != nil {
			log.Warn().Int64("idRun", idRun).Str("repo", repoPath).Err(err).Msg("failed to remove a discarded worktree")
			continue
		}
		log.Info().Int64("idRun", idRun).Str("repo", repoPath).Msg("removed the worktree of a discarded run")
	}
}

func (o *Orchestrator) completeTaskNoAction(task Task) {
	ctx := context.Background()
	payload := CompleteStagePayload{Outcome: OutcomeNoActionNeeded}
	if err := o.trackerClient.CompleteStage(ctx, task.IdTask, payload); err != nil {
		log.Error().Int64("idTask", task.IdTask).Err(err).Msg("failed to complete pickup stage")
	}
}

func (o *Orchestrator) runStage(task Task) {
	if task.Stage == StageChat && task.RunToken == "" {
		log.Warn().Int64("idTask", task.IdTask).Msg("rejecting a chat turn without its run token")
		o.failTask(task, ErrCodeTrackerMissingRunToken, "the tracker sent no run token for this chat turn; update the tracker")
		return
	}
	if task.RunToken != "" {
		o.trackerClient.RedactSecret(task.RunToken)
		defer o.trackerClient.ForgetSecret(task.RunToken)
	}
	// A chat without a repo runs in a scratch directory like release notes.
	isRepoless := task.Stage == StageReleaseNotes || (task.Stage == StageChat && !task.Repo.IsSet())
	if !isRepoless && !task.Repo.IsSet() {
		log.Warn().Int64("idTask", task.IdTask).Msg("rejecting a stage without a repo")
		o.failTask(task, ErrCodeTrackerMissingRepo, "the tracker sent no repository for this run; update the tracker")
		return
	}
	if err := o.applyStageOptions(&task); err != nil {
		log.Warn().Int64("idTask", task.IdTask).Err(err).Msg("rejecting stage the gateway does not allow")
		reason, detail := agentErrorReason(err)
		o.failTask(task, reason, detail)
		return
	}
	if !isRepoless {
		token, err := DecryptGitToken(o.cfg.WebhookSecret, task.Repo.EncryptedToken)
		if err != nil {
			log.Warn().Int64("idTask", task.IdTask).Err(err).Msg("rejecting a stage whose git token the gateway cannot read")
			o.failTask(task, ErrCodeGitTokenUnreadable, "the gateway could not decrypt the git token; check that TRACKER_TO_GATEWAY_TOKEN matches the token shown for this gateway in the tracker")
			return
		}
		task.Repo.Token = token
		o.trackerClient.RedactSecret(task.Repo.Token)
		defer o.trackerClient.ForgetSecret(task.Repo.Token)
	}

	ctx, cancel := context.WithCancelCause(context.Background())
	o.mu.Lock()
	o.activeTasks[task.IdTask] = &activeTask{task: task, cancel: func() { cancel(nil) }}
	o.mu.Unlock()
	defer func() {
		cancel(nil)
		o.mu.Lock()
		delete(o.activeTasks, task.IdTask)
		o.mu.Unlock()
	}()

	// Starts before the concurrency gate so a task waiting for a free slot
	// still reports liveness — otherwise the API stale-sweep would fail it
	// while queued. Covers the whole stage attempt.
	heartbeatCtx, cancelHeartbeat := context.WithCancel(ctx)
	defer cancelHeartbeat()
	var isStageSubmitted atomic.Bool
	task.OnStageSubmitted = func() { isStageSubmitted.Store(true) }
	go o.heartbeatLoop.Start(heartbeatCtx, task.IdTask, func() {
		if isStageSubmitted.Load() {
			return
		}
		log.Warn().Int64("idTask", task.IdTask).Msg("tracker no longer runs this task — stopping the agent")
		cancel(errTaskGone)
	})

	// One stage attempt runs the agent at a time; respects
	// cancellation so a `cancelled` event releases a queued task.
	select {
	case o.sem <- struct{}{}:
	case <-ctx.Done():
		if o.finishCleanup(task.IdTask) {
			go o.removeDiscardedWorktree(task.IdRun)
		}
		return
	}
	defer func() { <-o.sem }()
	defer o.removeWorktreeIfDiscarded(task.IdTask, task.IdRun)

	cleanup, ok := o.prepareWorkspace(&task)
	if !ok {
		return
	}
	if cleanup != nil {
		defer cleanup()
	}

	log.Info().
		Int64("idTask", task.IdTask).
		Str("stage", task.Stage).
		Str("branch", task.Branch).
		Msg("dispatching agent")
	start := time.Now()
	stats, err := o.agent.Run(ctx, task)
	// A failed or dropped attempt keeps its worktree for Continue.
	if errors.Is(context.Cause(ctx), errTaskGone) {
		return
	}
	if err != nil {
		log.Error().Int64("idTask", task.IdTask).Err(err).Msg("agent run failed")
		reason, detail := agentErrorReason(err)
		o.failTask(task, reason, detail)
		return
	}
	o.state.RecordRunDuration(time.Since(start))
	// The agent already posted complete_stage via its MCP tool but rarely
	// includes tokens/tool-calls/duration — backfill from what the adapter
	// measured so the stats panel isn't all zeros.
	// A chat turn also reports the commit its worktree is at.
	headCommit := ""
	if task.Chat != nil {
		headCommit = task.Chat.WorktreeHead
	}
	if stats.TokensUsed > 0 || stats.DurationMs > 0 || stats.ToolCallsCount > 0 || stats.CostUsd != nil || headCommit != "" {
		if err := o.trackerClient.UpdateTaskStats(context.Background(), task.IdTask, stats, headCommit); err != nil {
			log.Warn().Int64("idTask", task.IdTask).Err(err).Msg("failed to update task stats")
		}
	}
}

func (o *Orchestrator) applyStageOptions(task *Task) error {
	model, err := o.cfg.Models.Resolve(task.RequestedModel)
	if err != nil {
		return err
	}
	task.Model = model

	if task.Effort != "" && !slices.Contains(Efforts, task.Effort) {
		return &AgentError{
			Code:   ErrCodeEffortNotAllowed,
			Detail: fmt.Sprintf("effort %q is not one of %s", task.Effort, strings.Join(Efforts, ", ")),
		}
	}

	limit := o.cfg.MaxTurnsLimit
	if task.MaxTurns == 0 {
		task.MaxTurns = defaultMaxTurns(task.Stage)
		if limit > 0 && task.MaxTurns > limit {
			task.MaxTurns = limit
		}
		return nil
	}
	if limit > 0 && task.MaxTurns > limit {
		return &AgentError{
			Code:   ErrCodeMaxTurnsOverLimit,
			Detail: fmt.Sprintf("%d turns requested, this gateway allows at most %d", task.MaxTurns, limit),
		}
	}
	return nil
}

const (
	defaultMaxTurnsPlan      = 250
	defaultMaxTurnsImplement = 500
)

func defaultMaxTurns(stage string) int {
	if stage == StageImplementation {
		return defaultMaxTurnsImplement
	}
	return defaultMaxTurnsPlan
}

// agentErrorReason extracts the stable error-reason code + detail from an
// adapter error, falling back to the generic "agent_error" with the raw
// message when err isn't a typed *AgentError.
func agentErrorReason(err error) (reason, detail string) {
	var agentErr *AgentError
	if errors.As(err, &agentErr) {
		return agentErr.Code, agentErr.Detail
	}
	return "agent_error", err.Error()
}

func (o *Orchestrator) failTask(task Task, reason, detail string) {
	payload := CompleteStagePayload{
		Outcome:     OutcomeErrored,
		ErrorReason: reason,
		ErrorDetail: detail,
	}
	if err := o.trackerClient.CompleteStage(context.Background(), task.IdTask, payload); err != nil {
		log.Error().Int64("idTask", task.IdTask).Err(err).Msg("failed to post complete_stage on error path")
	}
}

// ActiveTaskCount returns the count of in-flight stage attempts.
func (o *Orchestrator) ActiveTaskCount() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.activeTasks)
}

// ActiveTaskInfo is a loggable snapshot of one in-flight stage attempt.
type ActiveTaskInfo struct {
	IdTask    int64  `json:"idTask"`
	IdRun     int64  `json:"idRun"`
	Stage     string `json:"stage"`
	AttemptNo int    `json:"attemptNo"`
}

// ActiveTasks returns a snapshot of what the gateway is currently executing —
// its whole in-memory "queue" of tasks handed via stage_execute and not yet
// finished. Surfaced on the health check for debugging.
func (o *Orchestrator) ActiveTasks() []ActiveTaskInfo {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]ActiveTaskInfo, 0, len(o.activeTasks))
	for _, at := range o.activeTasks {
		out = append(out, ActiveTaskInfo{
			IdTask:    at.task.IdTask,
			IdRun:     at.task.IdRun,
			Stage:     at.task.Stage,
			AttemptNo: at.task.AttemptNo,
		})
	}
	return out
}

// activeRunIDs returns run ids with an active task, so Retention can skip
// worktrees still in use.
func (o *Orchestrator) activeRunIDs() map[int64]bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make(map[int64]bool, len(o.activeTasks))
	for _, at := range o.activeTasks {
		out[at.task.IdRun] = true
	}
	return out
}
