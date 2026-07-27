package common

import (
	"fmt"
	"strings"
)

// MCPURLForStage picks the right MCP endpoint for the stage: non-implementation
// stages get the restricted /mcp/plan/sse subset, implementation gets the full
// /mcp/sse. The base URL is expected to end in /mcp/sse.
func MCPURLForStage(baseURL, stage string) string {
	if stage != StageImplementation {
		return strings.Replace(baseURL, "/mcp/sse", "/mcp/plan/sse", 1)
	}
	return baseURL
}

// SessionUUID returns a deterministic, CLI-acceptable v4 UUID derived from
// (run, stage, attempt). The Claude CLI requires a UUID-shaped --session-id;
// mapping each tuple to a unique value keeps a retry from colliding with the
// attempt before it, which the CLI would otherwise resume instead of starting
// clean. Layout:
// 0000…-SA00-<12-hex idRun>, S = stage code, A = attempt nibble (0..9, capped).
func SessionUUID(idRun int64, stage string, attemptNo int) string {
	stageCode := stageCodeHex(stage)
	if attemptNo < 0 {
		attemptNo = 0
	}
	if attemptNo > 9 {
		attemptNo = 9
	}
	return fmt.Sprintf("00000000-0000-4000-8%s%d0-%012d",
		stageCode, attemptNo, idRun)
}

func stageCodeHex(stage string) string {
	switch stage {
	case StageBrainstorming:
		return "1"
	case StageDesign:
		return "2"
	case StageImplementationPlan:
		return "3"
	case StageImplementation:
		return "4"
	default:
		return "0"
	}
}
