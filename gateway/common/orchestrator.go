package common

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

type activeTask struct {
	task   Task
	cancel context.CancelFunc
}

// Orchestrator dispatches stage_execute webhook events to the agent and
// reports failures back to the tracker. The API owns scheduling and marks
// stale tasks failed via its sweep; the gateway accepts every webhook but
// runs at most MaxConcurrent stage attempts at once (sem), queueing the rest
// in memory. Serialization matters because the CLI adapters share a single
// OAuth credential — concurrent processes would race on that one session.
type Orchestrator struct {
	cfg           *Config
	agent         Agent
	trackerClient *TrackerClient
	state         *State
	heartbeatLoop *HeartbeatLoop

	sem chan struct{} // bounds concurrent runStage execution to MaxConcurrent

	mu          sync.Mutex
	activeTasks map[int64]*activeTask // idTask -> active
}

func NewOrchestrator(
	cfg *Config,
	agent Agent,
	trackerClient *TrackerClient,
	state *State,
) *Orchestrator {
	maxConcurrent := cfg.MaxConcurrent
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	return &Orchestrator{
		cfg:           cfg,
		agent:         agent,
		trackerClient: trackerClient,
		state:         state,
		heartbeatLoop: NewHeartbeatLoop(trackerClient),
		sem:           make(chan struct{}, maxConcurrent),
		activeTasks:   make(map[int64]*activeTask),
	}
}

func (o *Orchestrator) HandleEvent(event WebhookEvent) error {
	switch event.Type {
	case "stage_execute":
		return o.handleStageExecute(event)
	case "cancelled":
		return o.handleCancelled(event)
	default:
		log.Warn().Str("type", event.Type).Str("eventId", event.EventId).Msg("unhandled event type — nothing dispatched")
		return nil
	}
}

func (o *Orchestrator) handleStageExecute(event WebhookEvent) error {
	task, err := parseStageExecutePayload(event.Payload)
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

func (o *Orchestrator) handleCancelled(event WebhookEvent) error {
	idRun := int64Field(event.Payload, "idRun")
	if idRun == 0 {
		return fmt.Errorf("cancelled event missing idRun")
	}
	o.mu.Lock()
	var toCancel []*activeTask
	for _, at := range o.activeTasks {
		if at.task.IdRun == idRun {
			toCancel = append(toCancel, at)
		}
	}
	o.mu.Unlock()

	for _, at := range toCancel {
		if at.cancel != nil {
			at.cancel()
		}
		runIDKey := RunID(fmt.Sprintf("%d", at.task.IdRun))
		if err := o.agent.Cancel(context.Background(), runIDKey); err != nil {
			log.Warn().Int64("idRun", at.task.IdRun).Err(err).Msg("agent cancel returned error")
		}
	}
	return nil
}

func (o *Orchestrator) completeTaskNoAction(task Task) {
	ctx := context.Background()
	payload := CompleteStagePayload{Outcome: OutcomeNoActionNeeded}
	if err := o.trackerClient.CompleteStage(ctx, task.IdTask, payload); err != nil {
		log.Error().Int64("idTask", task.IdTask).Err(err).Msg("failed to complete pickup stage")
	}
}

func (o *Orchestrator) runStage(task Task) {
	ctx, cancel := context.WithCancel(context.Background())
	o.mu.Lock()
	o.activeTasks[task.IdTask] = &activeTask{task: task, cancel: cancel}
	o.mu.Unlock()
	defer func() {
		cancel()
		o.mu.Lock()
		delete(o.activeTasks, task.IdTask)
		o.mu.Unlock()
	}()

	// Starts before the concurrency gate so a task waiting for a free slot
	// still reports liveness — otherwise the API stale-sweep would fail it
	// while queued. Covers the whole stage attempt.
	heartbeatCtx, cancelHeartbeat := context.WithCancel(ctx)
	defer cancelHeartbeat()
	go o.heartbeatLoop.Start(heartbeatCtx, task.IdTask)

	// At most MaxConcurrent stage attempts run the agent at once; respects
	// cancellation so a `cancelled` event releases a queued task.
	select {
	case o.sem <- struct{}{}:
	case <-ctx.Done():
		return
	}
	defer func() { <-o.sem }()

	// New branch on the first Implementation attempt, reused for every later
	// stage of the same run. Branch name is deterministic from
	// idUserBot+idIssue, so a redispatched run with the same idRun lands on
	// the same worktree.
	repoPath := RepoPathFromURL(o.cfg.WorkspaceBase, o.cfg.RepoUrl)
	if WorktreeExists(repoPath, task.IdRun) {
		task.WorktreePath = WorktreePath(repoPath, task.IdRun)
		if branch, err := branchOfWorktree(task.WorktreePath); err == nil {
			task.Branch = branch
		}
	} else {
		branch := GenerateBranchName(task.IdUserBot, task.IdIssue)
		task.Branch = branch
		path, err := CreateWorktree(repoPath, branch, task.IdRun)
		if err != nil {
			log.Error().Int64("idTask", task.IdTask).Err(err).Msg("failed to create worktree")
			o.failTask(task, "worktree_error", err.Error())
			return
		}
		task.WorktreePath = path
	}

	// Tells the tracker which repo this run pushes to, so it can resolve the
	// matching git_integration before complete_stage (the API opens the
	// PR/MR). Best-effort — a failure just leaves the run unresolved and the
	// complete path errors clearly.
	if err := o.trackerClient.ReportRunRepo(ctx, task.IdRun, RepoSlugFromURL(o.cfg.RepoUrl)); err != nil {
		log.Warn().Int64("idRun", task.IdRun).Err(err).Msg("failed to report run repo to tracker")
	}

	log.Info().
		Int64("idTask", task.IdTask).
		Str("stage", task.Stage).
		Str("branch", task.Branch).
		Msg("dispatching agent")
	start := time.Now()
	stats, err := o.agent.Run(ctx, task)
	if err != nil {
		log.Error().Int64("idTask", task.IdTask).Err(err).Msg("agent run failed")
		reason, detail := agentErrorReason(err)
		o.failTask(task, reason, detail)
		// Worktree is removed on failure so the next attempt starts fresh.
		_ = RemoveWorktree(repoPath, task.IdRun)
		return
	}
	o.state.RecordRunDuration(time.Since(start))
	// The agent already posted complete_stage via its MCP tool but rarely
	// includes tokens/tool-calls/duration — backfill from what the adapter
	// measured so the stats panel isn't all zeros.
	if stats.TokensUsed > 0 || stats.DurationMs > 0 || stats.ToolCallsCount > 0 {
		if err := o.trackerClient.UpdateTaskStats(context.Background(), task.IdTask, stats); err != nil {
			log.Warn().Int64("idTask", task.IdTask).Err(err).Msg("failed to update task stats")
		}
	}
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

func parseStageExecutePayload(payload map[string]any) (Task, error) {
	if payload == nil {
		return Task{}, fmt.Errorf("nil payload")
	}
	task := Task{
		IdRun:     int64Field(payload, "idRun"),
		IdIssue:   int64Field(payload, "idIssue"),
		IdUserBot: int64Field(payload, "idUserBot"),
		IdProject: int64Field(payload, "idProject"),
	}
	if task.IdRun == 0 {
		return Task{}, fmt.Errorf("missing idRun")
	}

	inner, _ := payload["payload"].(map[string]any)
	if inner != nil {
		task.IdTask = int64Field(inner, "idTask")
		task.Stage, _ = inner["stage"].(string)
		task.AttemptNo = int(int64Field(inner, "attemptNo"))

		if ctx, ok := inner["contextBundle"].(map[string]any); ok {
			if issue, ok := ctx["issue"].(map[string]any); ok {
				task.IdIssuePublic = int64Field(issue, "idIssuePublic")
				task.IssueTitle = stringField(issue, "title")
				task.IssueDesc = stringField(issue, "description")
			}
			if design, ok := ctx["approvedDesign"].(map[string]any); ok {
				task.ApprovedDesign = stringField(design, "message")
			}
			if plan, ok := ctx["approvedImplPlan"].(map[string]any); ok {
				task.ApprovedImplPlan = stringField(plan, "message")
			}
			if rejected, ok := ctx["rejectedOutput"].(map[string]any); ok {
				task.RejectedOutput = stringField(rejected, "message")
			}
			if ref, ok := ctx["approvedMockupRef"].(string); ok {
				task.ApprovedMockupRef = ref
			}
			if comments, ok := ctx["pendingComments"].([]any); ok {
				for _, raw := range comments {
					m, ok := raw.(map[string]any)
					if !ok {
						continue
					}
					creator, _ := m["creator"].(map[string]any)
					name := ""
					if creator != nil {
						name = stringField(creator, "name")
					}
					task.Comments = append(task.Comments, TaskComment{
						CreatorName: name,
						CreatedAt:   stringField(m, "createdAt"),
						Message:     stringField(m, "message"),
					})
				}
			}
		}
	}

	if task.IdTask == 0 {
		return Task{}, fmt.Errorf("missing idTask")
	}
	if task.Stage == "" {
		return Task{}, fmt.Errorf("missing stage")
	}
	return task, nil
}

func int64Field(m map[string]any, key string) int64 {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return int64(val)
	case int64:
		return val
	case int:
		return int64(val)
	}
	return 0
}

func stringField(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// branchOfWorktree returns the branch checked out in a worktree, or "" if
// detached HEAD or unreadable.
func branchOfWorktree(worktreePath string) (string, error) {
	cmd := exec.Command("git", "-C", worktreePath, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	branch := strings.TrimSpace(string(out))
	if branch == "HEAD" {
		return "", nil
	}
	return branch, nil
}
