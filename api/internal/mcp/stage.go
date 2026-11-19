package mcp

// Stage controls which tools a register function exposes. Plan-stage MCP
// connections see only read-only context tools plus submit_plan and
// request_clarification — the agent cannot call any write tool during planning.
// Implement stage sees everything. Chat connections (authenticated with a chat turn's run token) get
// the read tools plus complete_stage, minus the reads that token cannot reach over REST.
const (
	StageAll  = "all"
	StagePlan = "plan"
	StageChat = "chat"
)

// chatExcludedReads are read tools a chat turn's run scope refuses over REST; exposing them would only
// yield 403s. Write tools need no entry because allowsWrite(StageChat) is false.
var chatExcludedReads = []string{"get_my_issues", "list_track_records"}

// allowsWrite returns true when tools that mutate tracker state should be
// registered for the given stage. Read-only tools are always registered.
func allowsWrite(stage string) bool {
	return stage == StageAll
}
