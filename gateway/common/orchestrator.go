package common

import (
	"context"
	"encoding/json"
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
	// hasStageSlot is set while the task holds the stage slot, the only time an agent process runs for it.
	hasStageSlot bool
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

// cancelledPayload lists the tasks the tracker cancelled. An older tracker sends none: every task of
// the run stops then.
type cancelledPayload struct {
	IdsTask *[]int64 `json:"idsTask"`
}

// handleCancelled stops the cancelled tasks of the run but keeps its worktree. A newer attempt
// of the run, dispatched before this event arrived, keeps running.
func (o *Orchestrator) handleCancelled(event WebhookEvent) error {
	if event.IdRun == 0 {
		return fmt.Errorf("cancelled event missing idRun")
	}
	var payload cancelledPayload
	if len(event.Payload) > 0 {
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return fmt.Errorf("parsing cancelled payload: %w", err)
		}
	}
	toCancel, _ := o.activeTasksOfRun(event.IdRun, false)
	if payload.IdsTask != nil {
		toCancel = slices.DeleteFunc(toCancel, func(at *activeTask) bool {
			return !slices.Contains(*payload.IdsTask, at.task.IdTask)
		})
	}
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

// cancelTasks stops each task. The agent is stopped by run, so only for a task holding the slot: a
// task still waiting for it runs no agent, and stopping the run would kill another attempt of it.
func (o *Orchestrator) cancelTasks(toCancel []*activeTask) {
	for _, at := range toCancel {
		if at.cancel != nil {
			at.cancel()
		}
		if !o.hasStageSlot(at.task.IdTask) {
			continue
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

// repoUse tells what a kind of stage needs a repo for.
type repoUse int

const (
	repoRequired repoUse = iota
	// repoOptional runs in a scratch directory when the tracker sends no repo.
	repoOptional
	repoUnused
)

// stageRules is what differs between the kinds of stage the gateway runs: a stage of an issue run,
// release notes and a chat turn.
type stageRules struct {
	isRunTokenRequired bool
	repo               repoUse
	// prepareWorkspace points the task at the directory the agent runs in. It returns a cleanup
	// only for a scratch directory; a run's worktree outlives the stage attempt.
	prepareWorkspace func(o *Orchestrator, task *Task) (cleanup func(), ok bool)
}

func stageRulesOf(stage string) stageRules {
	switch stage {
	case StageReleaseNotes:
		return stageRules{repo: repoUnused, prepareWorkspace: (*Orchestrator).prepareScratchDir}
	case StageChat:
		return stageRules{isRunTokenRequired: true, repo: repoOptional, prepareWorkspace: (*Orchestrator).prepareChatWorkspace}
	default:
		return stageRules{repo: repoRequired, prepareWorkspace: (*Orchestrator).prepareRunWorkspace}
	}
}

func (r stageRules) isRepoUsed(task Task) bool {
	return r.repo == repoRequired || (r.repo == repoOptional && task.Repo.IsSet())
}

func (o *Orchestrator) runStage(task Task) {
	rules := stageRulesOf(task.Stage)
	forgetSecrets, ok := o.validateTask(&task, rules)
	defer forgetSecrets()
	if !ok {
		return
	}

	ctx, cancel := o.registerActiveTask(task)
	isSlotHeld := false
	// The slot is released only after the task is unregistered: a newer attempt that takes the
	// slot must never find this one still listed as active.
	defer func() {
		o.unregisterActiveTask(task.IdTask, cancel)
		if isSlotHeld {
			<-o.sem
		}
	}()

	// Starts before the concurrency gate so a task waiting for a free slot
	// still reports liveness — otherwise the API stale-sweep would fail it
	// while queued. Covers the whole stage attempt.
	stopHeartbeat := o.startHeartbeat(ctx, &task, cancel)
	defer stopHeartbeat()

	if !o.acquireSlot(ctx, task) {
		return
	}
	isSlotHeld = true
	o.markStageSlotHeld(task.IdTask)
	defer o.removeWorktreeIfDiscarded(task.IdTask, task.IdRun)

	cleanup, ok := rules.prepareWorkspace(o, &task)
	if !ok {
		return
	}
	if cleanup != nil {
		defer cleanup()
	}

	stats, ok := o.runAgent(ctx, task)
	if !ok {
		return
	}
	o.reportStats(task, stats)
}

// validateTask rejects a task the gateway cannot run, applies the stage options and decrypts the
// git token. The task's secrets stay masked in what reaches the tracker until forgetSecrets runs.
func (o *Orchestrator) validateTask(task *Task, rules stageRules) (forgetSecrets func(), ok bool) {
	var secrets []string
	forgetSecrets = func() {
		for _, secret := range secrets {
			o.trackerClient.ForgetSecret(secret)
		}
	}
	redactSecret := func(secret string) {
		o.trackerClient.RedactSecret(secret)
		secrets = append(secrets, secret)
	}

	if rules.isRunTokenRequired && task.RunToken == "" {
		log.Warn().Int64("idTask", task.IdTask).Msg("rejecting a chat turn without its run token")
		o.failTask(*task, ErrCodeTrackerMissingRunToken, "the tracker sent no run token for this chat turn; update the tracker")
		return forgetSecrets, false
	}
	if task.RunToken != "" {
		redactSecret(task.RunToken)
	}
	if rules.repo == repoRequired && !task.Repo.IsSet() {
		log.Warn().Int64("idTask", task.IdTask).Msg("rejecting a stage without a repo")
		o.failTask(*task, ErrCodeTrackerMissingRepo, "the tracker sent no repository for this run; update the tracker")
		return forgetSecrets, false
	}
	if err := o.applyStageOptions(task); err != nil {
		log.Warn().Int64("idTask", task.IdTask).Err(err).Msg("rejecting stage the gateway does not allow")
		reason, detail := agentErrorReason(err)
		o.failTask(*task, reason, detail)
		return forgetSecrets, false
	}
	if rules.isRepoUsed(*task) {
		token, err := DecryptGitToken(o.cfg.WebhookSecret, task.Repo.EncryptedToken)
		if err != nil {
			log.Warn().Int64("idTask", task.IdTask).Err(err).Msg("rejecting a stage whose git token the gateway cannot read")
			o.failTask(*task, ErrCodeGitTokenUnreadable, "the gateway could not decrypt the git token; check that TRACKER_TO_GATEWAY_TOKEN matches the token shown for this gateway in the tracker")
			return forgetSecrets, false
		}
		task.Repo.Token = token
		redactSecret(task.Repo.Token)
	}
	return forgetSecrets, true
}

func (o *Orchestrator) registerActiveTask(task Task) (context.Context, context.CancelCauseFunc) {
	ctx, cancel := context.WithCancelCause(context.Background())
	o.mu.Lock()
	o.activeTasks[task.IdTask] = &activeTask{task: task, cancel: func() { cancel(nil) }}
	o.mu.Unlock()
	return ctx, cancel
}

func (o *Orchestrator) markStageSlotHeld(idTask int64) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if at := o.activeTasks[idTask]; at != nil {
		at.hasStageSlot = true
	}
}

func (o *Orchestrator) hasStageSlot(idTask int64) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	at := o.activeTasks[idTask]
	return at != nil && at.hasStageSlot
}

func (o *Orchestrator) unregisterActiveTask(idTask int64, cancel context.CancelCauseFunc) {
	cancel(nil)
	o.mu.Lock()
	delete(o.activeTasks, idTask)
	o.mu.Unlock()
}

// startHeartbeat reports the task alive until stop, and stops the agent once the tracker no longer
// runs the task, unless the agent already submitted the stage.
func (o *Orchestrator) startHeartbeat(ctx context.Context, task *Task, cancel context.CancelCauseFunc) (stop func()) {
	heartbeatCtx, cancelHeartbeat := context.WithCancel(ctx)
	var isStageSubmitted atomic.Bool
	task.OnStageSubmitted = func() { isStageSubmitted.Store(true) }
	go o.heartbeatLoop.Start(heartbeatCtx, task.IdTask, func() {
		if isStageSubmitted.Load() {
			return
		}
		log.Warn().Int64("idTask", task.IdTask).Msg("tracker no longer runs this task — stopping the agent")
		cancel(errTaskGone)
	})
	return cancelHeartbeat
}

// acquireSlot waits for the one stage slot; the caller releases it. One stage attempt runs the
// agent at a time; respects cancellation so a `cancelled` event releases a queued task.
func (o *Orchestrator) acquireSlot(ctx context.Context, task Task) bool {
	select {
	case o.sem <- struct{}{}:
		return true
	case <-ctx.Done():
		if o.finishCleanup(task.IdTask) {
			go o.removeDiscardedWorktree(task.IdRun)
		}
		return false
	}
}

// runAgent reports a failed attempt itself; ok is false when there are no stats to report.
func (o *Orchestrator) runAgent(ctx context.Context, task Task) (stats RunStats, ok bool) {
	log.Info().
		Int64("idTask", task.IdTask).
		Str("stage", task.Stage).
		Str("branch", task.Branch).
		Msg("dispatching agent")
	start := time.Now()
	stats, err := o.agent.Run(ctx, task)
	// A failed or dropped attempt keeps its worktree for Continue.
	if errors.Is(context.Cause(ctx), errTaskGone) {
		return stats, false
	}
	if err != nil {
		log.Error().Int64("idTask", task.IdTask).Err(err).Msg("agent run failed")
		reason, detail := agentErrorReason(err)
		o.failTask(task, reason, detail)
		return stats, false
	}
	o.state.RecordRunDuration(time.Since(start))
	return stats, true
}

// reportStats backfills what the adapter measured: the agent already posted complete_stage via its
// MCP tool but rarely includes tokens/tool-calls/duration, so the stats panel isn't all zeros.
// A chat turn also reports the commit its worktree is at.
func (o *Orchestrator) reportStats(task Task, stats RunStats) {
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
