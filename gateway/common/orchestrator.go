package common

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bitmaster-sk/rurdesk/gateway/model"
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
	isRepoless := task.Stage == StageReleaseNotes
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
	if stats.TokensUsed > 0 || stats.DurationMs > 0 || stats.ToolCallsCount > 0 || stats.CostUsd != nil {
		if err := o.trackerClient.UpdateTaskStats(context.Background(), task.IdTask, stats); err != nil {
			log.Warn().Int64("idTask", task.IdTask).Err(err).Msg("failed to update task stats")
		}
	}
}

// prepareWorkspace returns a cleanup only for the scratch directory of a
// repoless stage; a run's worktree outlives the stage attempt.
func (o *Orchestrator) prepareWorkspace(task *Task) (cleanup func(), ok bool) {
	if task.Stage == StageReleaseNotes {
		dir, err := os.MkdirTemp("", fmt.Sprintf("release-notes-%d-", task.IdRun))
		if err != nil {
			log.Error().Int64("idTask", task.IdTask).Err(err).Msg("failed to create the scratch directory")
			o.failTask(*task, "worktree_error", err.Error())
			return nil, false
		}
		task.WorktreePath = dir
		return func() { _ = os.RemoveAll(dir) }, true
	}

	repoPath, err := EnsureRepo(o.cfg.WorkspaceBase, task.Repo)
	if err != nil {
		log.Error().Int64("idTask", task.IdTask).Err(err).Msg("failed to prepare the repo")
		o.failTask(*task, "worktree_error", err.Error())
		return nil, false
	}

	isReused, ok := o.reuseRunWorktree(repoPath, task)
	if !ok {
		return nil, false
	}
	// A run without a worktree resumes the branch of the issue's open PR, or starts a new one.
	if !isReused {
		path, branch, err := o.createRunWorktree(repoPath, *task)
		if err != nil {
			log.Error().Int64("idTask", task.IdTask).Err(err).Msg("failed to create worktree")
			o.failTask(*task, "worktree_error", err.Error())
			return nil, false
		}
		task.WorktreePath = path
		task.Branch = branch
	}
	// Retention measures idleness by the root's mtime, which edits in
	// subdirectories never change.
	now := time.Now()
	if err := os.Chtimes(task.WorktreePath, now, now); err != nil {
		log.Warn().Int64("idRun", task.IdRun).Err(err).Msg("failed to mark the worktree as used")
	}
	return nil, true
}

// reuseRunWorktree points the task at the run's existing worktree. A clean
// detached worktree is removed so the caller creates a new one; any other
// worktree without a branch fails the stage and stays on disk for an operator.
func (o *Orchestrator) reuseRunWorktree(repoPath string, task *Task) (isReused, ok bool) {
	if !WorktreeExists(repoPath, task.IdRun) {
		return false, true
	}
	worktreePath := WorktreePath(repoPath, task.IdRun)
	branch, isDisposable, err := worktreeState(worktreePath)
	switch {
	case err == nil && branch != "":
		task.WorktreePath = worktreePath
		task.Branch = branch
		task.ResumedWorktree = true
		return true, true
	case err == nil && isDisposable:
		log.Info().Int64("idRun", task.IdRun).Msg("worktree is detached and holds no work — creating a new one")
		if err := RemoveWorktree(repoPath, task.IdRun); err != nil {
			log.Error().Int64("idTask", task.IdTask).Err(err).Msg("failed to remove the detached worktree")
			o.failTask(*task, "worktree_error", err.Error())
			return false, false
		}
		return false, true
	default:
		log.Warn().Int64("idTask", task.IdTask).Str("path", worktreePath).AnErr("cause", err).Msg("worktree cannot be reused")
		o.failTask(*task, ErrCodeWorktreeUnusable, fmt.Sprintf(
			"the worktree of run %d at %s has no branch checked out or is not a git worktree, so this run cannot continue on it; "+
				"another run has likely taken over its branch. To retry, remove that directory on the gateway host; anything not pushed is then lost",
			task.IdRun, worktreePath))
		return false, false
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

func parseStageExecutePayload(event WebhookEvent) (Task, error) {
	if event.IdRun == 0 {
		return Task{}, fmt.Errorf("missing idRun")
	}
	if len(event.Payload) == 0 {
		return Task{}, fmt.Errorf("empty payload")
	}

	var payload model.StageExecutePayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return Task{}, fmt.Errorf("decoding stage_execute payload: %w", err)
	}

	task := Task{
		IdRun:          event.IdRun,
		IdIssue:        event.IdIssue,
		IdUserAgent:    event.IdUserAgent,
		IdProject:      event.IdProject,
		IdTask:         payload.IdTask,
		Stage:          payload.Stage,
		AttemptNo:      payload.AttemptNo,
		PrBranch:       payload.PrBranch,
		RequestedModel: payload.Model,
		MaxTurns:       payload.MaxTurns,
		Effort:         payload.Effort,
	}

	if payload.Repo != nil {
		task.Repo = AgentRunGitRepo{
			CloneUrl:       payload.Repo.CloneUrl,
			Host:           payload.Repo.Host,
			RepoPath:       payload.Repo.RepoPath,
			BaseBranch:     payload.Repo.BaseBranch,
			EncryptedToken: payload.Repo.EncryptedToken,
		}
	}

	if bundle := payload.ContextBundle; bundle != nil {
		if bundle.Issue != nil {
			task.IdIssuePublic = bundle.Issue.IdIssuePublic
			task.IssueTitle = bundle.Issue.Title
			task.IssueDesc = bundle.Issue.Description
		}
		task.ApprovedDesign = messageBody(bundle.ApprovedDesign)
		task.ApprovedImplPlan = messageBody(bundle.ApprovedImplPlan)
		task.RejectedOutput = messageBody(bundle.RejectedOutput)
		if bundle.ApprovedMockupRef != nil {
			task.ApprovedMockupRef = *bundle.ApprovedMockupRef
		}
		task.IsShortWorkflow = bundle.IsShortWorkflow
		task.ResumedAfter = bundle.ResumedAfter
		task.IsRevision = bundle.IsRevision
		for _, skill := range decodeEach[model.Skill](bundle.Skills) {
			if skill.Name == "" || skill.Content == "" {
				continue
			}
			task.Skills = append(task.Skills, Skill(skill))
		}
		task.Wiki = parseWiki(bundle.Wiki)
		task.WikiProposals = parseWikiProposals(bundle.WikiProposals)
		task.Release = parseRelease(bundle.Release)
		for _, message := range decodeEach[model.Message](bundle.ReviewThread) {
			creatorName := ""
			if message.Creator != nil {
				creatorName = message.Creator.Name
			}
			task.Conversation = append(task.Conversation, TaskMessage{
				CreatorName: creatorName,
				CreatedAt:   message.CreatedAt,
				Kind:        message.MessageKind,
				Message:     conversationBody(message.MessageKind, message.Message),
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

// decodeEach skips elements that do not decode so one malformed entry never fails a stage.
func decodeEach[T any](rawItems []json.RawMessage) []T {
	var items []T
	for _, rawItem := range rawItems {
		var item T
		if err := json.Unmarshal(rawItem, &item); err != nil {
			continue
		}
		items = append(items, item)
	}
	return items
}

func messageBody(message *model.Message) string {
	if message == nil {
		return ""
	}
	return message.Message
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

func parseWiki(promptContext *model.WikiPromptContext) *WikiContext {
	if promptContext == nil {
		return nil
	}
	wiki := &WikiContext{IndexMore: promptContext.IndexMore}
	for _, page := range decodeEach[model.WikiPromptPage](promptContext.Pages) {
		if page.Slug == "" || page.Body == "" {
			continue
		}
		wiki.Pages = append(wiki.Pages, WikiPage(page))
	}
	for _, entry := range decodeEach[model.WikiPromptIndexEntry](promptContext.Index) {
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

func parseRelease(promptContext *model.ReleasePromptContext) *ReleaseContext {
	if promptContext == nil {
		return nil
	}
	release := &ReleaseContext{Name: promptContext.Name, Tag: promptContext.Tag}
	for _, issue := range promptContext.OpenIssues {
		release.OpenIssues = append(release.OpenIssues, ReleaseIssue(issue))
	}
	for _, section := range promptContext.Sections {
		name := ""
		if section.Name != nil {
			name = *section.Name
		}
		issues := make([]ReleaseIssue, len(section.Issues))
		for i, issue := range section.Issues {
			issues[i] = ReleaseIssue(issue)
		}
		release.Sections = append(release.Sections, ReleaseSection{Name: name, Issues: issues})
	}
	return release
}

func parseWikiProposals(rawProposals []json.RawMessage) []WikiProposal {
	var proposals []WikiProposal
	for _, proposal := range decodeEach[model.WikiProposalAgentView](rawProposals) {
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
