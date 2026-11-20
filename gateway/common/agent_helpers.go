package common

import (
	"fmt"
	"slices"
	"strings"
)

// MCPURLForStage picks the right MCP endpoint for the stage: a chat turn gets
// the read-only /mcp/chat/sse, other non-implementation stages the restricted
// /mcp/plan/sse subset, implementation the full /mcp/sse. The base URL is
// expected to end in /mcp/sse.
func MCPURLForStage(baseURL, stage string) string {
	if stage == StageChat {
		return strings.Replace(baseURL, "/mcp/sse", "/mcp/chat/sse", 1)
	}
	if stage != StageImplementation {
		return strings.Replace(baseURL, "/mcp/sse", "/mcp/plan/sse", 1)
	}
	return baseURL
}

// ToStreamableHTTP converts a tracker MCP SSE URL to its streamable-HTTP path
// (".../sse" → ".../http"); other URLs pass through. Both adapters use it:
// goose ≥ v1.30 dropped SSE, and over SSE the claude CLI posts to the message
// URL the tracker advertises (built from PUBLIC_BASE_URL), which the gateway
// may not reach.
func ToStreamableHTTP(mcpURL string) string {
	if strings.HasSuffix(mcpURL, "/sse") {
		return strings.TrimSuffix(mcpURL, "/sse") + "/http"
	}
	return mcpURL
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

// TaskSessionUUID is the --session-id of the task's CLI process. A chat turn derives it from
// idTask rather than the attempt, which is capped at 9.
func TaskSessionUUID(task Task) string {
	if task.Stage == StageChat {
		return fmt.Sprintf("00000000-0000-4000-8600-%012d", task.IdTask)
	}
	return SessionUUID(task.IdRun, task.Stage, task.AttemptNo)
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
	case StageReleaseNotes:
		return "5"
	default:
		return "0"
	}
}

const TrackerRunHeader = "X-Rurdesk-Run"

// agentEnvSecrets are gateway secrets no agent process may inherit: the agent
// key, the webhook secret and a git token the gateway itself was given.
var agentEnvSecrets = []string{"GATEWAY_TO_TRACKER_TOKEN", "TRACKER_TO_GATEWAY_TOKEN", gitTokenEnvName}

// AgentEnv is the base env of an agent process: base without the gateway
// secrets, plus the git token of the run when there is one. A chat passes no
// git token, so its agent cannot push.
func AgentEnv(base []string, gitToken string) []string {
	env := make([]string, 0, len(base)+1)
	for _, entry := range base {
		name, _, _ := strings.Cut(entry, "=")
		if slices.Contains(agentEnvSecrets, name) {
			continue
		}
		env = append(env, entry)
	}
	if gitToken != "" {
		env = append(env, GitTokenEnv(gitToken))
	}
	return env
}
