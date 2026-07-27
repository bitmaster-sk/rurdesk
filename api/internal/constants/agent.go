package constants

const (
	PhaseQueued           = "queued"
	PhaseInProgress       = "in_progress"
	PhaseAwaitingInput    = "awaiting_input"
	PhaseAwaitingApproval = "awaiting_approval"
	PhasePrOpen           = "pr_open"
	PhaseDone             = "done"
	PhaseFailed           = "failed"
	PhaseCancelled        = "cancelled"
)

const (
	StagePickup             = "pickup"
	StageBrainstorming      = "brainstorming"
	StageDesign             = "design"
	StageImplementationPlan = "implementation_plan"
	StageImplementation     = "implementation"
)

const (
	TaskStatusPending   = "pending"
	TaskStatusActive    = "active"
	TaskStatusCompleted = "completed"
	TaskStatusFailed    = "failed"
	TaskStatusCancelled = "cancelled"
)

const (
	StageOutcomeOutputSubmitted = "output_submitted"
	StageOutcomeQuestionAsked   = "question_asked"
	StageOutcomeNoActionNeeded  = "no_action_needed"
	StageOutcomeErrored         = "errored"
)

const (
	ActorTypeUser    = "user"
	ActorTypeAgent   = "agent"
	ActorTypeGateway = "gateway"
	ActorTypeSystem  = "system"
)

type StageDef struct {
	Name      string
	Skippable bool
}

// StageDefinitions is the canonical stage order. The skippable flag is materialized
// into agent_run.stage_plan at insert time and shipped to the gateway in the
// stage_execute event.
var StageDefinitions = []StageDef{
	{Name: StagePickup, Skippable: false},
	{Name: StageBrainstorming, Skippable: true},
	{Name: StageDesign, Skippable: true},
	{Name: StageImplementationPlan, Skippable: false},
	{Name: StageImplementation, Skippable: false},
}

var TerminalPhases = map[string]bool{
	PhaseDone:      true,
	PhaseFailed:    true,
	PhaseCancelled: true,
}

var PassivePhases = map[string]bool{
	PhaseAwaitingInput:    true,
	PhaseAwaitingApproval: true,
	PhasePrOpen:           true,
}

var HeartbeatTaskStatuses = map[string]bool{
	TaskStatusActive: true,
}

// Task failure reasons stamped by infrastructure recovery paths, not the agent
// itself — these mark a task orphaned by a restart, not a genuine agent/dispatch error.
const (
	FailReasonCrashRecovery  = "crash_recovery"  // API restart blanket-fail (RunCrashRecovery)
	FailReasonHeartbeatStale = "heartbeat_stale" // no heartbeat within the sweep window
	FailReasonGatewayRestart = "gateway_restart" // gateway (re)start ReportRecovered
)

// RecoverableFailReasons are error_reasons for which a late complete_stage from a
// still-live gateway is reconciled instead of rejected. heartbeat_stale is
// deliberately excluded — that's a genuinely wedged agent, handled via Continue/Restart.
var RecoverableFailReasons = map[string]bool{
	FailReasonCrashRecovery:  true,
	FailReasonGatewayRestart: true,
}

// RecoverableFailReasonList is RecoverableFailReasons as a slice, for SQL ANY($n).
var RecoverableFailReasonList = []string{FailReasonCrashRecovery, FailReasonGatewayRestart}
