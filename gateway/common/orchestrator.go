package common

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
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

func (o *Orchestrator) handleCancelled(event WebhookEvent) error {
	if event.IdRun == 0 {
		return fmt.Errorf("cancelled event missing idRun")
	}
	o.mu.Lock()
	var toCancel []*activeTask
	for _, at := range o.activeTasks {
		if at.task.IdRun == event.IdRun {
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
	if !task.Repo.IsSet() {
		log.Warn().Int64("idTask", task.IdTask).Msg("rejecting a stage without a repo")
		o.failTask(task, ErrCodeTrackerMissingRepo, "the tracker sent no repository for this run; update the tracker")
		return
	}
	if err := o.applyModelAndTurns(&task); err != nil {
		log.Warn().Int64("idTask", task.IdTask).Err(err).Msg("rejecting stage the gateway does not allow")
		reason, detail := agentErrorReason(err)
		o.failTask(task, reason, detail)
		return
	}
	token, err := DecryptGitToken(o.cfg.WebhookSecret, task.Repo.EncryptedToken)
	if err != nil {
		log.Warn().Int64("idTask", task.IdTask).Err(err).Msg("rejecting a stage whose git token the gateway cannot read")
		o.failTask(task, ErrCodeGitTokenUnreadable, "the gateway could not decrypt the git token; check that TRACKER_TO_GATEWAY_TOKEN matches the token shown for this gateway in the tracker")
		return
	}
	task.Repo.Token = token
	o.trackerClient.RedactSecret(task.Repo.Token)
	defer o.trackerClient.ForgetSecret(task.Repo.Token)

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
		return
	}
	defer func() { <-o.sem }()

	repoPath, err := EnsureRepo(o.cfg.WorkspaceBase, task.Repo)
	if err != nil {
		log.Error().Int64("idTask", task.IdTask).Err(err).Msg("failed to prepare the repo")
		o.failTask(task, "worktree_error", err.Error())
		return
	}

	// A run without a worktree resumes the branch of the issue's open PR, or starts a new one.
	if WorktreeExists(repoPath, task.IdRun) {
		task.WorktreePath = WorktreePath(repoPath, task.IdRun)
		if branch, err := branchOfWorktree(task.WorktreePath); err == nil {
			task.Branch = branch
		}
	} else {
		path, branch, err := o.createRunWorktree(repoPath, task)
		if err != nil {
			log.Error().Int64("idTask", task.IdTask).Err(err).Msg("failed to create worktree")
			o.failTask(task, "worktree_error", err.Error())
			return
		}
		task.WorktreePath = path
		task.Branch = branch
	}

	log.Info().
		Int64("idTask", task.IdTask).
		Str("stage", task.Stage).
		Str("branch", task.Branch).
		Msg("dispatching agent")
	start := time.Now()
	stats, err := o.agent.Run(ctx, task)
	if errors.Is(context.Cause(ctx), errTaskGone) {
		_ = RemoveWorktree(repoPath, task.IdRun)
		return
	}
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

func (o *Orchestrator) applyModelAndTurns(task *Task) error {
	model, err := o.cfg.Models.Resolve(task.RequestedModel)
	if err != nil {
		return err
	}
	task.Model = model

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

func (o *Orchestrator) createRunWorktree(repoPath string, task Task) (path, branch string, err error) {
	if o.canResumePrBranch(repoPath, task) {
		path, err = ResumeWorktree(repoPath, task.Repo.Token, task.PrBranch, task.IdRun)
		return path, task.PrBranch, err
	}
	branch = GenerateBranchName(task.IdUserAgent, task.IdIssue)
	path, err = CreateWorktree(repoPath, task.Repo.Token, task.Repo.BaseBranch, branch, task.IdRun)
	return path, branch, err
}

func (o *Orchestrator) canResumePrBranch(repoPath string, task Task) bool {
	if task.PrBranch == "" {
		return false
	}
	if !IsAgentBranch(task.PrBranch) {
		log.Warn().Int64("idRun", task.IdRun).Str("branch", task.PrBranch).Msg("PR branch is not an agent branch — starting a new one")
		return false
	}
	hasBranch, err := remoteHasBranch(repoPath, task.Repo.Token, task.PrBranch)
	if err != nil {
		log.Warn().Int64("idRun", task.IdRun).Str("branch", task.PrBranch).Err(err).Msg("checking PR branch on origin failed — starting a new one")
		return false
	}
	return hasBranch
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

// payload types below mirror the API contract from api/internal/agent/dispatch.go
// and api/internal/agent/gateway_client.go. They live in the gateway module
// because it cannot import api/internal/model.

type stageExecutePayload struct {
	IdTask        int64                 `json:"idTask"`
	Stage         string                `json:"stage"`
	AttemptNo     int64                 `json:"attemptNo"`
	PrBranch      string                `json:"prBranch"`
	Repo          *repoPayload          `json:"repo"`
	Model         string                `json:"model"`
	MaxTurns      int64                 `json:"maxTurns"`
	ContextBundle *contextBundlePayload `json:"contextBundle"`
}

type repoPayload struct {
	CloneUrl       string `json:"cloneUrl"`
	Host           string `json:"host"`
	RepoPath       string `json:"repoPath"`
	BaseBranch     string `json:"baseBranch"`
	EncryptedToken string `json:"encryptedToken"`
}

type contextBundlePayload struct {
	Issue             *issuePayload     `json:"issue"`
	ReviewThread      []json.RawMessage `json:"reviewThread"`
	ApprovedDesign    *artifactPayload  `json:"approvedDesign"`
	ApprovedImplPlan  *artifactPayload  `json:"approvedImplPlan"`
	RejectedOutput    *artifactPayload  `json:"rejectedOutput"`
	ApprovedMockupRef *string           `json:"approvedMockupRef"`
	Skills            []json.RawMessage `json:"skills"`
	Wiki              *wikiPayload      `json:"wiki"`
	WikiProposals     []json.RawMessage `json:"wikiProposals"`
	IsShortWorkflow   bool              `json:"isShortWorkflow"`
}

type issuePayload struct {
	IdIssuePublic int64  `json:"idIssuePublic"`
	Title         string `json:"title"`
	Description   string `json:"description"`
}

type artifactPayload struct {
	Message     string `json:"message"`
	MessageKind string `json:"messageKind"`
}

type skillPayload struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type wikiPayload struct {
	Pages     []json.RawMessage `json:"pages"`
	Index     []json.RawMessage `json:"index"`
	IndexMore int               `json:"indexMore"`
}

type wikiPagePayload struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	Version int    `json:"version"`
	Reason  string `json:"reason"`
	Body    string `json:"body"`
}

type wikiIndexEntryPayload struct {
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	Summary   string `json:"summary"`
	OverLimit bool   `json:"overLimit"`
}

type wikiProposalPayload struct {
	Kind   string  `json:"kind"`
	Slug   string  `json:"slug"`
	Title  string  `json:"title"`
	Parent *string `json:"parent,omitempty"`
	Reason string  `json:"reason"`
}

type messagePayload struct {
	Message     string          `json:"message"`
	MessageKind string          `json:"messageKind"`
	CreatedAt   string          `json:"createdAt"`
	Creator     *creatorPayload `json:"creator"`
}

type creatorPayload struct {
	Name string `json:"name"`
}

func parseStageExecutePayload(event WebhookEvent) (Task, error) {
	if event.IdRun == 0 {
		return Task{}, fmt.Errorf("missing idRun")
	}
	if len(event.Payload) == 0 {
		return Task{}, fmt.Errorf("empty payload")
	}

	var p stageExecutePayload
	if err := json.Unmarshal(event.Payload, &p); err != nil {
		return Task{}, fmt.Errorf("unmarshal stage_execute payload: %w", err)
	}

	task := Task{
		IdRun:          event.IdRun,
		IdIssue:        event.IdIssue,
		IdUserAgent:    event.IdUserAgent,
		IdProject:      event.IdProject,
		IdTask:         p.IdTask,
		Stage:          p.Stage,
		AttemptNo:      int(p.AttemptNo),
		PrBranch:       p.PrBranch,
		RequestedModel: p.Model,
		MaxTurns:       int(p.MaxTurns),
	}

	if p.Repo != nil {
		task.Repo = AgentRunGitRepo{
			CloneUrl:       p.Repo.CloneUrl,
			Host:           p.Repo.Host,
			RepoPath:       p.Repo.RepoPath,
			BaseBranch:     p.Repo.BaseBranch,
			EncryptedToken: p.Repo.EncryptedToken,
		}
	}

	if p.ContextBundle != nil {
		ctx := p.ContextBundle
		if ctx.Issue != nil {
			task.IdIssuePublic = ctx.Issue.IdIssuePublic
			task.IssueTitle = ctx.Issue.Title
			task.IssueDesc = ctx.Issue.Description
		}
		if ctx.ApprovedDesign != nil {
			task.ApprovedDesign = ctx.ApprovedDesign.Message
		}
		if ctx.ApprovedImplPlan != nil {
			task.ApprovedImplPlan = ctx.ApprovedImplPlan.Message
		}
		if ctx.RejectedOutput != nil {
			task.RejectedOutput = ctx.RejectedOutput.Message
		}
		if ctx.ApprovedMockupRef != nil {
			task.ApprovedMockupRef = *ctx.ApprovedMockupRef
		}
		task.IsShortWorkflow = ctx.IsShortWorkflow
		for _, raw := range ctx.Skills {
			var s skillPayload
			if err := json.Unmarshal(raw, &s); err != nil {
				continue
			}
			if s.Name == "" || s.Content == "" {
				continue
			}
			task.Skills = append(task.Skills, Skill(s))
		}
		task.Wiki = parseWiki(ctx.Wiki)
		task.WikiProposals = parseWikiProposals(ctx.WikiProposals)
		for _, raw := range ctx.ReviewThread {
			var m messagePayload
			if err := json.Unmarshal(raw, &m); err != nil {
				continue
			}
			name := ""
			if m.Creator != nil {
				name = m.Creator.Name
			}
			task.Conversation = append(task.Conversation, TaskMessage{
				CreatorName: name,
				CreatedAt:   m.CreatedAt,
				Kind:        m.MessageKind,
				Message:     conversationBody(m.MessageKind, m.Message),
			})
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

// conversationBody drops the body of artifacts the prompt already renders in
// full sections of their own, keeping only their place in the thread.
func conversationBody(kind, message string) string {
	switch kind {
	case MessageKindDesign:
		return "(design document — body omitted here)"
	case MessageKindImplementationPlan:
		return "(implementation plan — body omitted here)"
	default:
		return message
	}
}

func parseWiki(w *wikiPayload) *WikiContext {
	if w == nil {
		return nil
	}
	wiki := &WikiContext{IndexMore: w.IndexMore}
	for _, raw := range w.Pages {
		var page wikiPagePayload
		if err := json.Unmarshal(raw, &page); err != nil {
			continue
		}
		if page.Slug == "" || page.Body == "" {
			continue
		}
		wiki.Pages = append(wiki.Pages, WikiPage(page))
	}
	for _, raw := range w.Index {
		var entry wikiIndexEntryPayload
		if err := json.Unmarshal(raw, &entry); err != nil {
			continue
		}
		if entry.Slug == "" {
			continue
		}
		wiki.Index = append(wiki.Index, WikiIndexEntry(entry))
	}
	if len(wiki.Pages) == 0 && len(wiki.Index) == 0 && wiki.IndexMore == 0 {
		return nil
	}
	return wiki
}

func parseWikiProposals(rawProposals []json.RawMessage) []WikiProposal {
	if len(rawProposals) == 0 {
		return nil
	}
	proposals := make([]WikiProposal, 0, len(rawProposals))
	for _, raw := range rawProposals {
		var proposal wikiProposalPayload
		if err := json.Unmarshal(raw, &proposal); err != nil {
			continue
		}
		if proposal.Slug == "" {
			continue
		}
		parent := ""
		if proposal.Parent != nil {
			parent = *proposal.Parent
		}
		proposals = append(proposals, WikiProposal{
			Kind:   proposal.Kind,
			Slug:   proposal.Slug,
			Title:  proposal.Title,
			Parent: parent,
			Reason: proposal.Reason,
		})
	}
	return proposals
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
