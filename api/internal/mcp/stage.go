package mcp

// Stage controls which tools a register function exposes. Plan-stage MCP
// connections see only read-only context tools plus submit_plan and
// request_clarification — the bot cannot call any write tool during planning.
// Implement stage sees everything.
const (
	StageAll  = "all"
	StagePlan = "plan"
)

// allowsWrite returns true when tools that mutate tracker state should be
// registered for the given stage. Read-only tools are always registered.
func allowsWrite(stage string) bool {
	return stage == StageAll
}
