package entity

import "github.com/bitmaster-sk/rurdesk/gateway/common"

// StreamEvent is one grouped, human-facing unit the aggregator emits: a
// coalesced run of thinking/answer text, or a discrete tool call. The
// adapter's emit callback turns these into zerolog lines; tests collect them
// to assert the grouping.
type StreamEvent struct {
	Kind       string // "thinking" | "text" | "tool"
	Text       string // coalesced text for thinking/text kinds
	Chars      int    // len(Text), for the log
	Tool       string // tool name for the "tool" kind
	ToolDetail string // the argument that identifies the call (command, path, …)
}

// StreamResult holds the run outcome (status, tokens, toolCalls, providerError)
// built incrementally while scanning the stream. stream-json sends no
// terminal status field, so status stays "" — treated as success unless the
// exit code says otherwise.
type StreamResult struct {
	Status              string
	Tokens              int
	ToolCalls           int
	ProviderError       *common.AgentError
	RunErrorMessage     string
	TurnLimitHit        bool
	CompleteStageCalled bool
}
