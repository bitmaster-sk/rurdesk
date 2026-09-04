package common

import "context"

type RunID string

// Stage names mirror api/internal/constants/agent.go StageDefinitions.
const (
	StagePickup             = "pickup"
	StageBrainstorming      = "brainstorming"
	StageDesign             = "design"
	StageImplementationPlan = "implementation_plan"
	StageImplementation     = "implementation"
)

// Outcome enum, mirroring api/internal/constants/agent.go.
const (
	OutcomeOutputSubmitted = "output_submitted"
	OutcomeQuestionAsked   = "question_asked"
	OutcomeNoActionNeeded  = "no_action_needed"
	OutcomeErrored         = "errored"
)

// Message kinds, mirroring api/internal/constants/message_kind.go.
const (
	MessageKindComment               = "comment"
	MessageKindBrainstormingQuestion = "brainstorming_question"
	MessageKindBrainstormingComplete = "brainstorming_complete"
	MessageKindDesign                = "design"
	MessageKindImplementationPlan    = "implementation_plan"
	MessageKindPullRequestPushed     = "pull_request_pushed"
	MessageKindImplementationDone    = "implementation_done"
	MessageKindReviewReply           = "review_reply"
)

type Task struct {
	IdTask        int64
	IdRun         int64
	IdIssue       int64
	IdUserBot     int64
	IdProject     int64
	IdIssuePublic int64
	Branch        string
	WorktreePath  string
	IssueTitle    string
	IssueDesc     string

	// Conversation is the whole issue thread oldest-first, the agent's own output
	// included. Never trim it to the current review round.
	Conversation []TaskMessage

	// ApprovedDesign / ApprovedImplPlan are the markdown bodies of the latest
	// approved design/implementation-plan messages, shipped in the
	// contextBundle so the agent doesn't re-fetch them each stage. Empty when
	// no such artifact exists yet.
	ApprovedDesign   string
	ApprovedImplPlan string

	// RejectedOutput is the markdown body of this stage's previous, rejected
	// attempt — shipped only when re-executing after rejection. Rendered with
	// an explicit "rejected, revise it" label; must never look approved.
	RejectedOutput string

	// ApprovedMockupRef is the reference (title or #N) of the mockup the user
	// picked when a design had multiple ```mockup blocks, so the agent
	// implements that variant instead of re-posting all of them. Empty when
	// none was chosen.
	ApprovedMockupRef string

	// Empty means the section is omitted entirely — which is also what an older
	// tracker (one that never sends the field) produces.
	Skills []Skill

	// Stage selects which instruction block the prompt template uses.
	Stage string

	// AttemptNo is the per-stage attempt counter (1 for the first attempt, 2
	// after a Continue, etc.), surfaced so the model knows it's revising
	// based on user feedback.
	AttemptNo int

	// MaxTurns is the per-stage hard cap on agent turns/iterations enforced
	// by the underlying runtime. Zero means "not set".
	MaxTurns int

	// Vocab is the per-adapter tool dictionary for the prompt's ALLOWED /
	// FORBIDDEN lists. Tool names belong to the harness, not the model —
	// goose driving Claude still calls goose's tools — so each adapter sets
	// its own vocab before RenderPrompt. Zero value falls back to
	// ToolVocabClaudeCode.
	Vocab ToolVocab
}

type Skill struct {
	Name    string
	Content string
}

type TaskMessage struct {
	CreatorName string
	CreatedAt   string
	Kind        string
	Message     string
}

type RunStatus struct {
	State string
	Error string
}

// RunStats are usage counters for one stage attempt. The agent can report
// these via the `complete_stage` MCP tool but in practice doesn't, so the
// adapter measures them from subprocess output (tool-call events, the CLI's
// final `result` token usage) plus wall-clock duration, reported via /stats
// after Run.
type RunStats struct {
	TokensUsed     int
	DurationMs     int
	ToolCallsCount int
}

// Agent is implemented by each adapter (Claude Code, Goose). Each Run is
// a short-lived subprocess executing ONE stage attempt; it reports
// completion by calling the `complete_stage` MCP tool from inside the
// process. If the subprocess errors out, the orchestrator posts a final
// complete_stage with outcome=errored on its behalf.
type Agent interface {
	Run(ctx context.Context, task Task) (RunStats, error)
	Cancel(ctx context.Context, runID RunID) error
}
