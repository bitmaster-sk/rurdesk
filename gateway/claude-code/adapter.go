package claudecode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bitmaster-sk/rurdesk/gateway/claude-code/model"
	"github.com/bitmaster-sk/rurdesk/gateway/common"
	"github.com/rs/zerolog/log"
)

const defaultClaudeBinary = "claude"

// chatBuiltinTools is the allow-list of built-in tools for a chat turn: reading, searching and
// web research. MCP tools (the tracker) are not built-ins and stay available.
const chatBuiltinTools = "Read,Grep,Glob,WebSearch,WebFetch"

// chatDisallowedTools is the second fence behind the allow-list: no shell, file edits or subagents.
const chatDisallowedTools = "Bash,Write,Edit,MultiEdit,NotebookEdit,Task"

// ClaudeCodeAdapter implements common.Agent by driving the official Claude
// Code CLI (`claude`, installed via npm) headlessly. It authenticates against
// a Claude Console account, billed per token, via CLAUDE_CODE_OAUTH_TOKEN or a
// Console `/login` kept in the mounted $HOME/.claude volume. Claude
// subscriptions are not supported: Anthropic's terms forbid routing them
// through a shared service. ANTHROPIC_API_KEY and ANTHROPIC_AUTH_TOKEN are
// scrubbed from the child env so a stray key cannot switch the billed account.
type ClaudeCodeAdapter struct {
	cfg            *common.Config
	thinkingSender common.ThinkingSender
	sessions       *common.ProcessSessions
}

// Every model the Claude Code CLI runs thinks.
func (a *ClaudeCodeAdapter) ModelThinking(_ context.Context, models []common.Model) (map[string]bool, []common.Model) {
	isThinkingByModel := make(map[string]bool, len(models))
	for _, model := range models {
		isThinkingByModel[model.Id()] = true
	}
	return isThinkingByModel, nil
}

func NewClaudeCodeAdapter(cfg *common.Config, thinkingSender common.ThinkingSender) *ClaudeCodeAdapter {
	return &ClaudeCodeAdapter{
		cfg:            cfg,
		thinkingSender: thinkingSender,
		sessions:       common.NewProcessSessions(),
	}
}

func (a *ClaudeCodeAdapter) Run(ctx context.Context, task common.Task) (common.RunStats, error) {
	stats := common.RunStats{}
	task.Vocab = common.ToolVocabClaudeCode
	prompt, err := common.RenderPrompt(task)
	if err != nil {
		return stats, fmt.Errorf("rendering prompt: %w", err)
	}

	// Kept outside the worktree so the bearer token it carries can never be
	// swept into a commit, and removed with the run so it does not outlive it.
	mcpConfigDir, err := os.MkdirTemp("", fmt.Sprintf("rurdesk-mcp-%d-", task.IdRun))
	if err != nil {
		return stats, fmt.Errorf("creating claude MCP config dir: %w", err)
	}
	defer os.RemoveAll(mcpConfigDir)

	mcpURL := common.ToStreamableHTTP(common.MCPURLForStage(a.cfg.TrackerMCPUrl, task.Stage))
	mcpConfigPath, err := writeMCPSettings(mcpConfigDir, mcpURL, task.MCPBearer(a.cfg.AgentApiKey), task.IdRun)
	if err != nil {
		return stats, fmt.Errorf("writing claude MCP settings: %w", err)
	}

	runCtx, cancel := context.WithCancel(ctx)
	session := &common.ProcessSession{Cancel: cancel}

	runIDKey := common.RunID(fmt.Sprintf("%d", task.IdRun))
	a.sessions.Add(runIDKey, session)
	defer func() {
		cancel()
		a.sessions.Remove(runIDKey)
	}()

	args := []string{
		"-p",
		"--output-format", "stream-json",
		// stream-json on stdout requires --verbose in print mode, or the CLI
		// buffers one result object instead of emitting per-event lines.
		"--verbose",
		// Streams thinking as it is produced, so the tracker can show it live.
		"--include-partial-messages",
		// Headless auto-approve; without it the CLI blocks on interactive
		// confirmation and the run hangs.
		"--dangerously-skip-permissions",
		"--max-turns", fmt.Sprintf("%d", task.MaxTurns),
		"--session-id", common.TaskSessionUUID(task),
		"--mcp-config", mcpConfigPath,
		"--model", task.Model.Name,
	}
	if task.Effort != "" {
		args = append(args, "--effort", task.Effort)
	}
	// Without it the CLI streams thinking as empty fragments.
	if display := a.cfg.ClaudeCode.ThinkingDisplay; display != "" {
		args = append(args, "--thinking-display", display)
	}
	if task.Stage == common.StageChat {
		// A chat is private to its owner: the CLI must not keep the turn's transcript on the gateway
		// host. Each turn gets the whole history, so nothing is resumed.
		args = append(args, "--no-session-persistence",
			"--tools", chatBuiltinTools,
			"--disallowedTools", chatDisallowedTools)
	}

	cmd := exec.CommandContext(runCtx, claudeBinary(), args...)
	common.StopGracefully(cmd, common.ProcessKillGrace)
	cmd.Dir = task.WorktreePath
	cmd.Env = childEnv(a.cfg.ClaudeCode.OAuthToken, task.AgentGitToken(), task.WorktreePath)
	// The prompt must never be an argument: the agent's own `pkill -f <word>` would match it and kill claude.
	cmd.Stdin = strings.NewReader(prompt)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return stats, fmt.Errorf("stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return stats, fmt.Errorf("stderr pipe: %w", err)
	}

	log.Info().
		Int64("idRun", task.IdRun).
		Str("stage", task.Stage).
		Str("model", task.Model.Name).
		Str("mcpURL", mcpURL).
		Int("maxTurns", task.MaxTurns).
		Str("effort", task.Effort).
		Msg("starting claude code")

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return stats, fmt.Errorf("starting claude: %w", err)
	}

	// final is set by drainEvents and read after Wait. The exit code is
	// authoritative for success; the result event only refines the error
	// (e.g. "error_max_turns", the API error text) on failure. A mutex guards
	// the cross-goroutine writes.
	var statsMu sync.Mutex
	var final finalResult
	var toolCalls int
	var tokensUsed int
	var usage resultUsage
	completeStageCalled := false

	thinking := common.NewThinkingBatcher(a.thinkingSender, task.IdTask)
	thinking.Start(runCtx)
	done := make(chan struct{})
	go func() {
		drainEvents(stdoutPipe, a.cfg.MaxOutputLineBytes, task.IdRun,
			func(result finalResult) {
				statsMu.Lock()
				final = result
				statsMu.Unlock()
			},
			func(n int, sawCompleteStage bool) {
				statsMu.Lock()
				toolCalls += n
				if sawCompleteStage {
					completeStageCalled = true
					task.ReportStageSubmitted()
				}
				statsMu.Unlock()
			},
			func(t int) {
				statsMu.Lock()
				if t > tokensUsed {
					tokensUsed = t
				}
				statsMu.Unlock()
			},
			func(u resultUsage) {
				statsMu.Lock()
				usage = u
				statsMu.Unlock()
			},
			thinking.Add,
		)
		close(done)
	}()
	stderrDone := make(chan struct{})
	go func() {
		common.DrainStderr(stderrPipe, a.cfg.MaxOutputLineBytes, task.IdRun, "claude code")
		close(stderrDone)
	}()

	// Both pipes must reach EOF before Wait: Wait closes them as soon as it sees
	// the child exit, which drops whatever the scanners had not read yet — the
	// trailing `result` event and the complete_stage tool_use land there.
	<-done
	<-stderrDone
	waitErr := cmd.Wait()
	thinking.Stop()
	if dropped := thinking.DroppedCount(); dropped > 0 {
		log.Warn().Int64("idTask", task.IdTask).Int("dropped", dropped).Msg("thinking events dropped on overflow")
	}

	statsMu.Lock()
	stats.ToolCallsCount = toolCalls
	stats.TokensUsed = tokensUsed
	stats.DurationMs = int(time.Since(start) / time.Millisecond)
	stats.InputTokens = usage.InputTokens
	stats.OutputTokens = usage.OutputTokens
	stats.CacheReadTokens = usage.CacheReadTokens
	stats.CacheWriteTokens = usage.CacheWriteTokens
	stats.CostUsd = usage.CostUsd
	stats.CostEstimated = false
	terminal := final
	submitted := completeStageCalled
	statsMu.Unlock()

	if runCtx.Err() != nil {
		return stats, fmt.Errorf("claude code cancelled")
	}

	exitCode := 0
	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return stats, fmt.Errorf("claude code wait failed: %w", waitErr)
		}
	}

	if termErr := mapTerminalStatus(exitCode, terminal); termErr != nil {
		return stats, termErr
	}
	if !submitted {
		log.Warn().Int64("idRun", task.IdRun).Str("stage", task.Stage).
			Msg("claude code exited 0 without calling complete_stage — failing stage")
		return stats, &common.AgentError{
			Code:   common.ErrCodeStageNotSubmitted,
			Detail: "agent ended without calling complete_stage",
		}
	}
	return stats, nil
}

// finalResult is what the closing `result` event says about the run.
type finalResult struct {
	Subtype        string
	IsError        bool
	Text           string
	ApiErrorStatus int
}

// mapTerminalStatus converts the exit code and the `result` event into an
// error. The CLI exits zero on success. On failure an API error is classified
// by its HTTP status, and the error text the CLI reported is preferred over
// the subtype, which stays "success" for an API error.
func mapTerminalStatus(exitCode int, final finalResult) error {
	if final.Subtype == "error_max_turns" {
		return &common.AgentError{
			Code:   common.ErrCodeTurnLimitExhausted,
			Detail: "agent reached --max-turns before calling complete_stage",
		}
	}
	if exitCode == 0 && !final.IsError && (final.Subtype == "" || final.Subtype == "success") {
		return nil
	}
	detail := firstLine(final.Text)
	if final.ApiErrorStatus != 0 {
		return &common.AgentError{Code: apiErrorCode(final.ApiErrorStatus), Detail: detail}
	}
	if detail != "" {
		return fmt.Errorf("claude code failed (exit %d): %s", exitCode, detail)
	}
	if final.Subtype != "" && final.Subtype != "success" {
		return fmt.Errorf("claude code ended with result %q (exit %d)", final.Subtype, exitCode)
	}
	if final.IsError {
		return fmt.Errorf("claude code reported an error (exit %d)", exitCode)
	}
	return fmt.Errorf("claude code exited with code %d", exitCode)
}

func apiErrorCode(status int) string {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return common.ErrCodeProviderAuthFailed
	case http.StatusTooManyRequests:
		return common.ErrCodeProviderRateLimited
	default:
		return common.ErrCodeProviderError
	}
}

func firstLine(text string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	return strings.TrimSpace(line)
}

// drainEvents reads stream-json events from claude stdout, forwarding each
// well-formed line to zerolog so the operator sees the agent's timeline.
// tool_use blocks in `assistant` messages fire onToolCalls; usage totals fire
// onTokens (caller keeps the max); the final `result` event fires onFinal
// with its subtype, is_error, text and API status for error mapping. Thinking fragments (merged by thinkingCoalescer)
// and tool calls fire onThinking; answer text is skipped. Thinking text is logged only at debug level.
func drainEvents(
	r io.Reader,
	maxLineBytes int,
	idRun int64,
	onFinal func(finalResult),
	onToolCalls func(count int, sawCompleteStage bool),
	onTokens func(int),
	onUsage func(resultUsage),
	onThinking func(common.ThinkingEvent),
) {
	thoughts := &thinkingCoalescer{onThinking: onThinking}
	skipped, err := common.ReadLines(r, maxLineBytes, func(line []byte) {
		if len(line) == 0 {
			return
		}
		var ev model.StreamLine
		if err := json.Unmarshal(line, &ev); err != nil {
			log.Warn().
				Int64("idRun", idRun).
				Str("line", string(line)).
				Msg("claude code stdout: non-JSON line")
			return
		}
		isThinkingProgress := ev.Type == "system" && ev.Subtype == "thinking_tokens"
		fragment, isThinkingDelta := thinkingFragment(ev)
		switch {
		case isThinkingDelta:
			thoughts.add(fragment)
		case !isThinkingProgress:
			thoughts.flush()
		}
		logEvent := log.Info()
		// Partial events and thinking progress arrive per token, so they only
		// show at debug level.
		if ev.Type == "stream_event" || isThinkingProgress {
			logEvent = log.Debug()
		}
		loggedEvent := json.RawMessage(line)
		switch ev.Type {
		case "assistant":
			for _, call := range toolUses(ev) {
				thoughts.send(call)
			}
			if onToolCalls != nil {
				if count, sawCompleteStage := countToolUses(ev); count > 0 {
					onToolCalls(count, sawCompleteStage)
				}
			}
			if onTokens != nil && ev.Message != nil && ev.Message.Usage != nil {
				if tokens := ev.Message.Usage.Total(); tokens > 0 {
					onTokens(tokens)
				}
			}
			if hasThinkingText(ev) {
				loggedEvent = withoutThinkingText(ev)
			}
		case "result":
			if onFinal != nil {
				onFinal(finalResult{Subtype: ev.Subtype, IsError: ev.IsError, Text: ev.Result, ApiErrorStatus: ev.ApiErrorStatus})
			}
			if onTokens != nil {
				if tokens := totalTokens(ev); tokens > 0 {
					onTokens(tokens)
				}
			}
			if onUsage != nil {
				onUsage(extractResultUsage(ev))
			}
		}
		logEvent.
			Int64("idRun", idRun).
			Str("type", ev.Type).
			RawJSON("event", loggedEvent).
			Msg("agent event")
	})
	thoughts.flush()
	common.LogReadOutcome(idRun, "claude code stdout", skipped, err)
}

// thinkingFlushChars is the size at which merged thinking is sent, the same as
// the goose adapter's flushChars.
const thinkingFlushChars = 200

// thinkingCoalescer merges consecutive thinking fragments, which the CLI
// streams a few words at a time, into events of about thinkingFlushChars.
// A nil onThinking drops everything.
type thinkingCoalescer struct {
	onThinking func(common.ThinkingEvent)
	buffer     strings.Builder
}

func (c *thinkingCoalescer) add(fragment string) {
	if c.onThinking == nil {
		return
	}
	c.buffer.WriteString(fragment)
	if c.buffer.Len() >= thinkingFlushChars {
		c.flush()
	}
}

func (c *thinkingCoalescer) flush() {
	if c.onThinking == nil || c.buffer.Len() == 0 {
		return
	}
	c.onThinking(common.ThinkingEvent{Kind: common.ThinkingKindThinking, Text: c.buffer.String()})
	c.buffer.Reset()
}

// send passes a non-thinking event on after the thinking that preceded it.
func (c *thinkingCoalescer) send(event common.ThinkingEvent) {
	if c.onThinking == nil {
		return
	}
	c.flush()
	c.onThinking(event)
}

// thinkingFragment returns the text of a `thinking_delta` stream event and
// whether the event is one.
func thinkingFragment(ev model.StreamLine) (string, bool) {
	if ev.Type != "stream_event" || ev.Event == nil || ev.Event.Type != "content_block_delta" ||
		ev.Event.Delta.Type != "thinking_delta" {
		return "", false
	}
	return ev.Event.Delta.Thinking, true
}

func hasThinkingText(ev model.StreamLine) bool {
	return ev.Message != nil && slices.ContainsFunc(ev.Message.Content, func(block model.ContentBlock) bool {
		return block.Type == "thinking" && block.Thinking != ""
	})
}

// withoutThinkingText is the decoded `assistant` event with the text of its
// thinking blocks replaced by its length, so the info-level event log never
// holds thinking. Fields the decoded shape does not know are left out.
func withoutThinkingText(ev model.StreamLine) json.RawMessage {
	message := *ev.Message
	message.Content = slices.Clone(message.Content)
	for index, block := range message.Content {
		if block.Type == "thinking" && block.Thinking != "" {
			message.Content[index].Thinking = fmt.Sprintf("(%d chars, sent to the tracker)", len(block.Thinking))
		}
	}
	ev.Message = &message
	encoded, err := json.Marshal(ev)
	if err != nil {
		return json.RawMessage(`{"type":"assistant"}`)
	}
	return encoded
}

// toolUses turns the tool_use blocks of an `assistant` event into tool events.
func toolUses(ev model.StreamLine) []common.ThinkingEvent {
	if ev.Message == nil {
		return nil
	}
	var calls []common.ThinkingEvent
	for _, block := range ev.Message.Content {
		if block.Type != "tool_use" {
			continue
		}
		calls = append(calls, common.ThinkingEvent{Kind: common.ThinkingKindTool, Tool: block.Name, Text: toolDetail(block)})
	}
	return calls
}

// toolDetailMaxChars caps a tool detail like the goose adapter does.
const toolDetailMaxChars = 400

// toolDetail is the one line that identifies a tool call: the command, the
// file or the pattern; other tools show none.
func toolDetail(block model.ContentBlock) string {
	var input model.ToolInput
	if len(block.Input) > 0 {
		if err := json.Unmarshal(block.Input, &input); err != nil {
			return ""
		}
	}
	var value string
	switch block.Name {
	case "Bash":
		value = input.Command
	case "Read", "Edit", "Write":
		value = input.FilePath
	case "Grep", "Glob":
		value = input.Pattern
	default:
		return ""
	}
	if index := strings.IndexByte(value, '\n'); index >= 0 {
		value = strings.TrimRight(value[:index], "\r ") + "…"
	}
	if runes := []rune(value); len(runes) > toolDetailMaxChars {
		value = string(runes[:toolDetailMaxChars]) + "…"
	}
	return value
}

// countToolUses counts tool_use content blocks in an `assistant` event.
func countToolUses(ev model.StreamLine) (count int, sawCompleteStage bool) {
	if ev.Message == nil {
		return 0, false
	}
	for _, block := range ev.Message.Content {
		if block.Type == "tool_use" {
			count++
			if strings.Contains(block.Name, "complete_stage") {
				sawCompleteStage = true
			}
		}
	}
	return count, sawCompleteStage
}

// totalTokens is the token total of a claude `result` event: its usage, or
// the counts on the event itself, since the schema can drift across CLI
// versions. Returns 0 if none found.
func totalTokens(ev model.StreamLine) int {
	if ev.Usage != nil {
		if total := ev.Usage.Total(); total > 0 {
			return total
		}
	}
	return ev.Total()
}

type resultUsage struct {
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int
	CostUsd          *float64
}

func extractResultUsage(ev model.StreamLine) resultUsage {
	usage := resultUsage{CostUsd: ev.TotalCostUsd}
	if ev.Usage != nil {
		usage.InputTokens = ev.Usage.InputTokens
		usage.OutputTokens = ev.Usage.OutputTokens
		usage.CacheReadTokens = ev.Usage.CacheReadInputTokens
		usage.CacheWriteTokens = ev.Usage.CacheCreationInputTokens
	}
	return usage
}

func (a *ClaudeCodeAdapter) Cancel(_ context.Context, runID common.RunID) error {
	a.sessions.Terminate(runID)
	return nil
}

// childEnv builds the subprocess env: common.AgentEnv (inherited env without
// the gateway secrets, plus the run's git token unless it is empty and the
// run's worktree path) with
// ANTHROPIC_API_KEY and ANTHROPIC_AUTH_TOKEN removed (a stray host var can't
// flip billing to the Console pay-per-token bucket), plus GIT_PAGER=cat (the
// implement stage's `git log`/`git diff` would otherwise block on a pager),
// IS_SANDBOX=1, and an optional CLAUDE_CODE_OAUTH_TOKEN.
//
// IS_SANDBOX=1: the container runs as root, and the CLI refuses
// --dangerously-skip-permissions under uid 0 unless this is set. The
// container is a disposable per-run worktree sandbox where headless
// auto-approve is intended, so we opt into the root bypass instead of
// re-plumbing the image to non-root.
func childEnv(oauthToken, gitToken, worktreePath string) []string {
	base := common.AgentEnv(os.Environ(), gitToken, worktreePath)
	out := make([]string, 0, len(base)+3)
	for _, kv := range base {
		if strings.HasPrefix(kv, "ANTHROPIC_API_KEY=") ||
			strings.HasPrefix(kv, "ANTHROPIC_AUTH_TOKEN=") {
			continue
		}
		out = append(out, kv)
	}
	out = append(out, "GIT_PAGER=cat")
	out = append(out, "IS_SANDBOX=1")
	if oauthToken != "" {
		out = append(out, "CLAUDE_CODE_OAUTH_TOKEN="+oauthToken)
	}
	return out
}

// mcpSettings is the --mcp-config file of the Claude Code CLI.
type mcpSettings struct {
	McpServers map[string]mcpServer `json:"mcpServers"`
}

type mcpServer struct {
	Type    string            `json:"type"`
	Url     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

// writeMCPSettings writes <dir>/mcp.json with the "tracker" MCP server entry
// (stage-scoped streamable-HTTP endpoint, the agent key — or a chat turn's run token — as
// an Authorization: Bearer header; the tracker API strips the prefix and matches
// by shape) and returns its path for --mcp-config.
//
// dir MUST be outside the worktree: the file holds the agent's bearer token in
// cleartext and the agent commits from the worktree, so `git add -A` would push
// it to the user's remote.
func writeMCPSettings(dir, mcpURL, bearerToken string, idRun int64) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("creating MCP config dir: %w", err)
	}
	settings := mcpSettings{McpServers: map[string]mcpServer{
		"tracker": {
			Type: "http",
			Url:  mcpURL,
			Headers: map[string]string{
				"Authorization":         "Bearer " + bearerToken,
				common.TrackerRunHeader: strconv.FormatInt(idRun, 10),
			},
		},
	}}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "mcp.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// claudeBinary returns the path to the claude CLI. The CLAUDE_CODE_BINARY env
// var override lets the unit tests point at a stub script without needing the
// real binary on PATH.
func claudeBinary() string {
	if p := os.Getenv("CLAUDE_CODE_BINARY"); p != "" {
		return p
	}
	return defaultClaudeBinary
}
