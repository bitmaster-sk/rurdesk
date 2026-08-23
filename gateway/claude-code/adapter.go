package claudecode

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bitmaster-sk/rurdesk/gateway/common"
	"github.com/rs/zerolog/log"
)

const defaultClaudeBinary = "claude"

// claudeSession holds the running claude CLI subprocess and a cancel hook so
// orchestrator-driven Cancel calls can SIGTERM the right child.
type claudeSession struct {
	cmd    *exec.Cmd
	cancel context.CancelFunc
}

// ClaudeCodeAdapter implements common.Agent by driving the official Claude
// Code CLI (`claude`, installed via npm) headlessly. It authenticates against
// a Claude Pro/Max subscription via OAuth credentials in
// $HOME/.claude/.credentials.json (volume-mounted, seeded by an interactive
// login) or an optional CLAUDE_CODE_OAUTH_TOKEN override. ANTHROPIC_API_KEY is
// scrubbed from the child env so calls never silently bill the Console
// pay-per-token bucket.
type ClaudeCodeAdapter struct {
	cfg      *common.Config
	mu       sync.Mutex
	sessions map[common.RunID]*claudeSession
}

func NewClaudeCodeAdapter(cfg *common.Config) *ClaudeCodeAdapter {
	return &ClaudeCodeAdapter{
		cfg:      cfg,
		sessions: make(map[common.RunID]*claudeSession),
	}
}

func (a *ClaudeCodeAdapter) Run(ctx context.Context, task common.Task) (common.RunStats, error) {
	stats := common.RunStats{}
	task.MaxTurns = maxTurnsForStage(task.Stage, a.cfg.ClaudeCode)
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

	mcpURL := common.MCPURLForStage(a.cfg.TrackerMCPUrl, task.Stage)
	mcpConfigPath, err := writeMCPSettings(mcpConfigDir, mcpURL, a.cfg.BotApiKey)
	if err != nil {
		return stats, fmt.Errorf("writing claude MCP settings: %w", err)
	}

	runCtx, cancel := context.WithCancel(ctx)
	session := &claudeSession{cancel: cancel}

	runIDKey := common.RunID(fmt.Sprintf("%d", task.IdRun))
	a.mu.Lock()
	a.sessions[runIDKey] = session
	a.mu.Unlock()
	defer func() {
		cancel()
		a.mu.Lock()
		delete(a.sessions, runIDKey)
		a.mu.Unlock()
	}()

	args := []string{
		"-p", prompt,
		"--output-format", "stream-json",
		// stream-json on stdout requires --verbose in print mode, or the CLI
		// buffers one result object instead of emitting per-event lines.
		"--verbose",
		// Headless auto-approve; without it the CLI blocks on interactive
		// confirmation and the run hangs.
		"--dangerously-skip-permissions",
		"--max-turns", fmt.Sprintf("%d", task.MaxTurns),
		"--session-id", common.SessionUUID(task.IdRun, task.Stage, task.AttemptNo),
		"--mcp-config", mcpConfigPath,
	}
	// Unset model omits --model so the account default applies; a set value
	// passes through verbatim and an unrecognised name fails fast rather than
	// degrading silently.
	if m := a.cfg.ClaudeCode.Model; m != "" {
		args = append(args, "--model", m)
	}

	cmd := exec.CommandContext(runCtx, claudeBinary(), args...)
	cmd.Dir = task.WorktreePath
	cmd.Env = childEnv(a.cfg.ClaudeCode.OAuthToken)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return stats, fmt.Errorf("stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return stats, fmt.Errorf("stderr pipe: %w", err)
	}

	session.cmd = cmd

	log.Info().
		Int64("idRun", task.IdRun).
		Str("stage", task.Stage).
		Str("model", a.cfg.ClaudeCode.Model).
		Str("mcpURL", mcpURL).
		Int("maxTurns", task.MaxTurns).
		Msg("starting claude code")

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return stats, fmt.Errorf("starting claude: %w", err)
	}

	// finalSubtype/finalIsError are set by drainEvents and read after Wait. The
	// exit code is authoritative for success; the result event only refines
	// the error message (e.g. "error_max_turns") on non-zero exit. A mutex
	// guards the cross-goroutine writes.
	var statsMu sync.Mutex
	finalSubtype := ""
	finalIsError := false
	var toolCalls int
	var tokensUsed int
	completeStageCalled := false

	done := make(chan struct{})
	go func() {
		drainEvents(stdoutPipe, task.IdRun,
			func(subtype string, isError bool) {
				statsMu.Lock()
				finalSubtype = subtype
				finalIsError = isError
				statsMu.Unlock()
			},
			func(n int, sawCompleteStage bool) {
				statsMu.Lock()
				toolCalls += n
				if sawCompleteStage {
					completeStageCalled = true
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
		)
		close(done)
	}()
	stderrDone := make(chan struct{})
	go func() {
		drainStderr(stderrPipe, task.IdRun)
		close(stderrDone)
	}()

	// Both pipes must reach EOF before Wait: Wait closes them as soon as it sees
	// the child exit, which drops whatever the scanners had not read yet — the
	// trailing `result` event and the complete_stage tool_use land there.
	<-done
	<-stderrDone
	waitErr := cmd.Wait()

	statsMu.Lock()
	stats.ToolCallsCount = toolCalls
	stats.TokensUsed = tokensUsed
	stats.DurationMs = int(time.Since(start) / time.Millisecond)
	subtype := finalSubtype
	isError := finalIsError
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

	if termErr := mapTerminalStatus(exitCode, subtype, isError); termErr != nil {
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

// mapTerminalStatus converts (exit code, subtype, is_error) into an error.
// The CLI exits zero on success; the `result` event's subtype refines the
// message for soft failures (max-turns, execution error) on non-zero exit.
func mapTerminalStatus(exitCode int, subtype string, isError bool) error {
	if subtype == "error_max_turns" {
		return &common.AgentError{
			Code:   common.ErrCodeTurnLimitExhausted,
			Detail: "agent reached --max-turns before calling complete_stage",
		}
	}
	if exitCode == 0 && !isError && (subtype == "" || subtype == "success") {
		return nil
	}
	if subtype != "" {
		return fmt.Errorf("claude code ended with result %q (exit %d)", subtype, exitCode)
	}
	if isError {
		return fmt.Errorf("claude code reported an error (exit %d)", exitCode)
	}
	return fmt.Errorf("claude code exited with code %d", exitCode)
}

// drainEvents reads stream-json events from claude stdout, forwarding each
// well-formed line to zerolog so the operator sees the agent's timeline.
// tool_use blocks in `assistant` messages fire onToolCalls; usage totals fire
// onTokens (caller keeps the max); the final `result` event fires onFinal
// with its subtype/is_error for error mapping.
func drainEvents(
	r io.Reader,
	idRun int64,
	onFinal func(subtype string, isError bool),
	onToolCalls func(count int, sawCompleteStage bool),
	onTokens func(int),
) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev map[string]any
		if err := json.Unmarshal(line, &ev); err != nil {
			log.Warn().
				Int64("idRun", idRun).
				Str("line", string(line)).
				Msg("claude code stdout: non-JSON line")
			continue
		}
		evType, _ := ev["type"].(string)
		switch evType {
		case "assistant":
			if onToolCalls != nil {
				if n, sawCompleteStage := countToolUses(ev); n > 0 {
					onToolCalls(n, sawCompleteStage)
				}
			}
			if onTokens != nil {
				if t := tokensFromMessage(ev); t > 0 {
					onTokens(t)
				}
			}
		case "result":
			if onFinal != nil {
				subtype, _ := ev["subtype"].(string)
				isError, _ := ev["is_error"].(bool)
				onFinal(subtype, isError)
			}
			if onTokens != nil {
				if t := extractTotalTokens(ev); t > 0 {
					onTokens(t)
				}
			}
		}
		log.Info().
			Int64("idRun", idRun).
			Str("type", evType).
			Interface("event", ev).
			Msg("agent event")
	}
	if err := scanner.Err(); err != nil {
		log.Warn().Int64("idRun", idRun).Err(err).Msg("claude code stdout scan ended with error")
	}
}

// countToolUses counts tool_use content blocks in an `assistant` event.
func countToolUses(ev map[string]any) (count int, sawCompleteStage bool) {
	message, ok := ev["message"].(map[string]any)
	if !ok {
		return 0, false
	}
	content, ok := message["content"].([]any)
	if !ok {
		return 0, false
	}
	for _, block := range content {
		if blockMap, ok := block.(map[string]any); ok {
			if t, _ := blockMap["type"].(string); t == "tool_use" {
				count++
				if name, _ := blockMap["name"].(string); strings.Contains(name, "complete_stage") {
					sawCompleteStage = true
				}
			}
		}
	}
	return count, sawCompleteStage
}

// tokensFromMessage pulls a token total from an assistant event's message.usage.
func tokensFromMessage(ev map[string]any) int {
	message, ok := ev["message"].(map[string]any)
	if !ok {
		return 0
	}
	if usage, ok := message["usage"].(map[string]any); ok {
		return tokensFromMap(usage)
	}
	return 0
}

// extractTotalTokens hunts token totals in a claude `result` event, probing a
// few layouts since the schema can drift across CLI versions. Returns 0 if
// none found.
func extractTotalTokens(ev map[string]any) int {
	if usage, ok := ev["usage"].(map[string]any); ok {
		if n := tokensFromMap(usage); n > 0 {
			return n
		}
	}
	return tokensFromMap(ev)
}

func tokensFromMap(m map[string]any) int {
	if t, ok := numberField(m, "total_tokens"); ok && t > 0 {
		return t
	}
	in, _ := numberField(m, "input_tokens")
	out, _ := numberField(m, "output_tokens")
	cacheCreate, _ := numberField(m, "cache_creation_input_tokens")
	cacheRead, _ := numberField(m, "cache_read_input_tokens")
	total := in + out + cacheCreate + cacheRead
	if total > 0 {
		return total
	}
	return 0
}

func numberField(m map[string]any, key string) (int, bool) {
	switch v := m[key].(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	case int64:
		return int(v), true
	}
	return 0, false
}

func drainStderr(r io.Reader, idRun int64) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r\n")
		if line == "" {
			continue
		}
		forwardStderrLine(line, idRun)
	}
}

// forwardStderrLine maps claude CLI's stderr lines to zerolog at the right
// severity by prefix-matching, so a real failure doesn't read as routine
// debug noise.
func forwardStderrLine(line string, idRun int64) {
	trim := strings.TrimLeft(line, " \t")
	switch {
	case hasPrefixFold(trim, "Error"),
		hasPrefixFold(trim, "Fatal"),
		hasPrefixFold(trim, "panic"):
		log.Error().Int64("idRun", idRun).Str("line", line).Msg("claude code stderr")
	case hasPrefixFold(trim, "Warning"),
		hasPrefixFold(trim, "Warn"),
		hasPrefixFold(trim, "Deprecated"):
		log.Warn().Int64("idRun", idRun).Str("line", line).Msg("claude code stderr")
	default:
		log.Debug().Int64("idRun", idRun).Str("line", line).Msg("claude code stderr")
	}
}

func hasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}

func (a *ClaudeCodeAdapter) Cancel(ctx context.Context, runID common.RunID) error {
	a.mu.Lock()
	session, ok := a.sessions[runID]
	if ok {
		delete(a.sessions, runID)
	}
	a.mu.Unlock()

	if !ok || session.cmd == nil {
		return nil
	}

	session.cancel()
	if session.cmd.Process != nil {
		_ = session.cmd.Process.Signal(syscall.SIGTERM)
		done := make(chan struct{})
		go func() {
			_ = session.cmd.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			_ = session.cmd.Process.Kill()
		}
	}
	return nil
}

// childEnv builds the subprocess env: inherited env with ANTHROPIC_API_KEY
// and ANTHROPIC_AUTH_TOKEN removed (a stray host var can't flip billing to
// the Console pay-per-token bucket), plus GIT_PAGER=cat (the implement
// stage's `git log`/`git diff` would otherwise block on a pager), IS_SANDBOX=1,
// and an optional CLAUDE_CODE_OAUTH_TOKEN.
//
// IS_SANDBOX=1: the container runs as root, and the CLI refuses
// --dangerously-skip-permissions under uid 0 unless this is set. The
// container is a disposable per-run worktree sandbox where headless
// auto-approve is intended, so we opt into the root bypass instead of
// re-plumbing the image to non-root.
func childEnv(oauthToken string) []string {
	base := os.Environ()
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

// writeMCPSettings writes <dir>/mcp.json with the "tracker" MCP server entry
// (stage-scoped SSE endpoint, bot key as an Authorization: Bearer header — the
// tracker API strips the prefix and matches by shape) and returns its path for
// --mcp-config.
//
// dir MUST be outside the worktree: the file holds the bot's bearer token in
// cleartext and the agent commits from the worktree, so `git add -A` would push
// it to the user's remote.
func writeMCPSettings(dir, mcpURL, botKey string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("creating MCP config dir: %w", err)
	}
	settings := map[string]any{
		"mcpServers": map[string]any{
			"tracker": map[string]any{
				"type": "sse",
				"url":  mcpURL,
				"headers": map[string]string{
					"Authorization": "Bearer " + botKey,
				},
			},
		},
	}
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

func maxTurnsForStage(stage string, cfg *common.ClaudeCodeConfig) int {
	if stage == common.StageImplementation {
		return cfg.MaxTurnsImplement
	}
	return cfg.MaxTurnsPlan
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
