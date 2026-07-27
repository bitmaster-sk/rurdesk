package model

import (
	"encoding/json"
	"time"
)

type AgentRun struct {
	IdRun             int64           `json:"idRun"            db:"id_run"`
	IdIssue           int64           `json:"idIssue"          db:"id_issue"`
	IdUserBot         int64           `json:"idUserBot"        db:"id_user_bot"`
	IdProject         int64           `json:"idProject"        db:"id_project"`
	IdGitIntegration  *int64          `json:"idGitIntegration" db:"id_git_integration"`
	Phase             string          `json:"phase"            db:"phase"`
	StagePlan         json.RawMessage `json:"stagePlan"        db:"stage_plan"`
	QueuePosition     *int            `json:"queuePosition"    db:"queue_position"`
	PrUrl             *string         `json:"prUrl"            db:"pr_url"`
	PrHostType        *string         `json:"prHostType"       db:"pr_host_type"`
	PrId              *string         `json:"prId"             db:"pr_id"`
	BranchName        *string         `json:"branchName"       db:"branch_name"`
	ErrorMessage      *string         `json:"errorMessage"     db:"error_message"`
	ApprovedMockupRef *string         `json:"approvedMockupRef" db:"approved_mockup_ref"`
	StartedAt         *time.Time      `json:"startedAt"        db:"started_at"`
	FinishedAt        *time.Time      `json:"finishedAt"       db:"finished_at"`
	CreatedAt         time.Time       `json:"createdAt"        db:"created_at"`
}

type AgentRunWithEvents struct {
	AgentRun
	Events []*AgentRunEvent     `json:"events"`
	Stages []AgentStageProgress `json:"stages"`
}

// AgentStageProgress is one row of the run timeline, derived server-side from
// the stage plan + tasks + events so the client doesn't reconstruct progression
// from raw phase transitions. Note is an i18n token, not display text: e.g.
// "no_clarifications" | "submitted" | "pr_opened".
type AgentStageProgress struct {
	Stage      string     `json:"stage"`
	Status     string     `json:"status"` // pending|active|done|awaiting_approval|failed|skipped
	Note       string     `json:"note,omitempty"`
	AttemptNo  int        `json:"attemptNo,omitempty"`
	IdUserBot  *int64     `json:"idUserBot,omitempty"`  // which bot executed this stage (provenance)
	At         *time.Time `json:"at,omitempty"`         // finishedAt (done/failed) or startedAt (active)
	ApprovedAt *time.Time `json:"approvedAt,omitempty"` // user approval waypoint
	// ErrorReason is a stable code (e.g. provider_credit_exhausted) translated via
	// i18n; ErrorDetail is the raw provider/agent message. Set only on failure.
	ErrorReason *string `json:"errorReason,omitempty"`
	ErrorDetail *string `json:"errorDetail,omitempty"`
}

type AgentRunEvent struct {
	IdEvent   int64     `json:"idEvent"   db:"id_event"`
	IdRun     int64     `json:"idRun"     db:"id_run"`
	FromPhase *string   `json:"fromPhase" db:"from_phase"`
	ToPhase   *string   `json:"toPhase"   db:"to_phase"`
	ActorType string    `json:"actorType" db:"actor_type"`
	IdUser    *int64    `json:"idUser"    db:"id_user"`
	Reason    *string   `json:"reason"    db:"reason"`
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
}

type AgentTask struct {
	IdTask          int64      `json:"idTask"          db:"id_task"`
	IdRun           int64      `json:"idRun"           db:"id_run"`
	IdUserBot       *int64     `json:"idUserBot"       db:"id_user_bot"`
	Stage           string     `json:"stage"           db:"stage"`
	AttemptNo       int        `json:"attemptNo"       db:"attempt_no"`
	Status          string     `json:"status"          db:"status"`
	IdOutputMessage *int64     `json:"idOutputMessage" db:"id_output_message"`
	ErrorReason     *string    `json:"errorReason"     db:"error_reason"`
	ErrorDetail     *string    `json:"errorDetail"     db:"error_detail"`
	TokensUsed      *int       `json:"tokensUsed"      db:"tokens_used"`
	DurationMs      *int       `json:"durationMs"      db:"duration_ms"`
	ToolCallsCount  *int       `json:"toolCallsCount"  db:"tool_calls_count"`
	StartedAt       *time.Time `json:"startedAt"       db:"started_at"`
	FinishedAt      *time.Time `json:"finishedAt"      db:"finished_at"`
	LastHeartbeatAt *time.Time `json:"lastHeartbeatAt" db:"last_heartbeat_at"`
	CreatedAt       time.Time  `json:"createdAt"       db:"created_at"`
}

type StagePlan struct {
	Stages []StagePlanEntry `json:"stages"`
}

type StagePlanEntry struct {
	Name      string `json:"name"`
	Skippable bool   `json:"skippable"`
	Skip      bool   `json:"skip"`
}

// CompleteStageReq is the body of POST /private/agent/task/:idTask/complete,
// also used as the MCP complete_stage tool parameter payload.
type CompleteStageReq struct {
	Outcome        string  `json:"outcome"        binding:"required,oneof=output_submitted question_asked no_action_needed errored"`
	Message        string  `json:"message"`
	MessageKind    string  `json:"messageKind"`
	PrUrl          string  `json:"prUrl"`
	BranchName     string  `json:"branchName"`
	PrTitle        string  `json:"prTitle"`
	PrBody         string  `json:"prBody"`
	TokensUsed     *int    `json:"tokensUsed"`
	DurationMs     *int    `json:"durationMs"`
	ToolCallsCount *int    `json:"toolCallsCount"`
	ErrorReason    *string `json:"errorReason"`
	ErrorDetail    *string `json:"errorDetail"`
}

// ReportRunRepoReq records which repo (`owner/repo`) a run pushes to, sent by
// the gateway at dispatch so the backend can resolve the git_integration
// before complete_stage.
type ReportRunRepoReq struct {
	RepoPath string `json:"repoPath" binding:"required"`
}

type CompleteStageRes struct {
	IdTask    int64  `json:"idTask"`
	Status    string `json:"status"`
	NextPhase string `json:"nextPhase"`
}

type RunStatsRes struct {
	TotalTokensUsed     int            `json:"totalTokensUsed"`
	TotalDurationMs     int            `json:"totalDurationMs"`
	TotalToolCallsCount int            `json:"totalToolCallsCount"`
	AttemptsPerStage    map[string]int `json:"attemptsPerStage"`
	FailedAttempts      int            `json:"failedAttempts"`
}

// AgentStatsNotice is the SubjectAgentStats broadcast payload: pushes
// freshly-aggregated run stats so the client patches state without a roundtrip.
type AgentStatsNotice struct {
	IdRun int64        `json:"idRun"`
	Stats *RunStatsRes `json:"stats"`
}

// SetRunPrReq is consumed by AgentRunRepository.SetPrInfo, populated from the
// PR the backend opened or reused on the git host at complete_stage time.
type SetRunPrReq struct {
	PrUrl            string `json:"prUrl"            binding:"required,url"`
	PrId             string `json:"prId"             binding:"required"`
	PrHostType       string `json:"prHostType"       binding:"required,oneof=github gitlab gitea"`
	BranchName       string `json:"branchName"       binding:"required"`
	IdGitIntegration int64  `json:"idGitIntegration" binding:"required"`
}
