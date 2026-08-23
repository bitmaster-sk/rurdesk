package goose

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bitmaster-sk/rurdesk/gateway/common"
	"github.com/rs/zerolog/log"
)

const defaultGooseBinary = "goose"

// gooseSession holds the running goose subprocess and a cancel hook so
// orchestrator-driven Cancel calls can SIGTERM the right child.
type gooseSession struct {
	cmd    *exec.Cmd
	cancel context.CancelFunc
}

// GooseAdapter implements common.Agent by driving Block's Goose agent
// (`goose run`, a pinned Rust binary) headlessly. Unlike the claude adapter it
// authenticates with a per-token API key (BYOK): GOOSE_PROVIDER
// selects the provider, and the matching ANTHROPIC_API_KEY / GOOGLE_API_KEY /
// OLLAMA_HOST passes through to the child. The tracker MCP server can't be
// configured per-run on the command line (goose flags can't carry an auth
// header), so writeGooseConfig writes the stage-scoped URL and bearer token
// literally into ~/.config/goose/config.yaml before each run.
type GooseAdapter struct {
	cfg      *common.Config
	mu       sync.Mutex
	sessions map[common.RunID]*gooseSession
}

func NewGooseAdapter(cfg *common.Config) *GooseAdapter {
	return &GooseAdapter{
		cfg:      cfg,
		sessions: make(map[common.RunID]*gooseSession),
	}
}

func (a *GooseAdapter) Run(ctx context.Context, task common.Task) (common.RunStats, error) {
	stats := common.RunStats{}
	task.MaxTurns = maxTurnsForStage(task.Stage, a.cfg.Goose)
	task.Vocab = common.ToolVocabGoose
	prompt, err := common.RenderPrompt(task)
	if err != nil {
		return stats, fmt.Errorf("rendering prompt: %w", err)
	}

	// Stage-scoped tracker MCP endpoint, converted to the streamable-HTTP path
	// goose ≥ v1.30 requires (it dropped SSE remote extensions).
	mcpURL := toStreamableHTTP(common.MCPURLForStage(a.cfg.TrackerMCPUrl, task.Stage))

	// Writes config.yaml with the stage URL + bearer token per run (not via
	// ${ENV} substitution, which goose doesn't apply to the extension uri).
	// Assumes MAX_CONCURRENT=1 (config path is shared); concurrent runs would
	// need per-run config dirs.
	if _, err := writeGooseConfig(gooseConfigDir(), mcpURL, a.cfg.BotApiKey); err != nil {
		return stats, fmt.Errorf("writing goose config: %w", err)
	}

	runCtx, cancel := context.WithCancel(ctx)
	session := &gooseSession{cancel: cancel}

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
		"run",
		// Stateless cold-start per stage — no session file written or resumed.
		"--no-session",
		// Streaming NDJSON: goose emits one event per token/tool, letting the
		// gateway log the agent's thinking live instead of a final blob. The
		// stream aggregator (stream.go) coalesces the firehose into grouped log
		// lines; correctness rides on the exit code + the agent's complete_stage
		// MCP call.
		"--output-format", "stream-json",
		// Hard per-stage turn cap. --max-turns means "turns without user input";
		// in --no-session headless mode there's no user, so it's an enforced
		// ceiling.
		"--max-turns", fmt.Sprintf("%d", task.MaxTurns),
		// Provider override (also set via env; the flag wins and is explicit).
		"--provider", a.cfg.Goose.Provider,
		"-t", prompt,
	}
	// Unset model omits --model so the provider default applies; a set value
	// passes through verbatim and an unrecognised name fails fast (mirrors the
	// claude adapter) rather than degrading silently.
	if m := a.cfg.Goose.Model; m != "" {
		args = append(args, "--model", m)
	}

	cmd := exec.CommandContext(runCtx, gooseBinary(), args...)
	cmd.Dir = task.WorktreePath
	cmd.Env = childEnv(a.cfg.Goose)

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
		Str("provider", a.cfg.Goose.Provider).
		Str("model", a.cfg.Goose.Model).
		Str("mcpURL", mcpURL).
		Int("maxTurns", task.MaxTurns).
		Msg("starting goose")

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return stats, fmt.Errorf("starting goose: %w", err)
	}

	// stream-json emits one NDJSON event per token/tool; scan line-by-line and
	// feed the aggregator, which coalesces the firehose into grouped log lines
	// and accumulates the terminal stats (tokens, tool count, provider error).
	agg := newStreamAggregator(task.IdRun, func(ev streamEvent) { logStreamEvent(ev, task.IdRun) })
	done := make(chan struct{})
	go func() {
		scanGooseStream(stdoutPipe, agg)
		close(done)
	}()
	stderrDone := make(chan struct{})
	go func() {
		drainStderr(stderrPipe, task.IdRun)
		close(stderrDone)
	}()

	// Both pipes must reach EOF before Wait: Wait closes them as soon as it sees
	// the child exit, which drops whatever the scanners had not read yet — the
	// trailing completion event and the complete_stage tool call land there.
	<-done
	<-stderrDone
	waitErr := cmd.Wait()

	result := agg.finish()
	status, tokensUsed, toolCalls, provErr := result.status, result.tokens, result.toolCalls, result.provErr
	turnLimitHit := result.turnLimitHit
	stats.TokensUsed = tokensUsed
	stats.ToolCallsCount = toolCalls
	stats.DurationMs = int(time.Since(start) / time.Millisecond)

	if runCtx.Err() != nil {
		return stats, fmt.Errorf("goose cancelled")
	}

	// Goose reports provider failures (low credit, bad request) as in-band
	// assistant text with exit 0 (block/goose#4612), so the exit code alone
	// can't be trusted. A detected provider error takes precedence and carries
	// a stable code the UI translates.
	if provErr != nil {
		log.Warn().Int64("idRun", task.IdRun).Str("code", provErr.Code).Str("detail", provErr.Detail).Msg("goose provider error")
		return stats, provErr
	}

	exitCode := 0
	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return stats, fmt.Errorf("goose wait failed: %w", waitErr)
		}
	}

	// Success rides entirely on the agent calling the `complete_stage` MCP
	// tool — the gateway never posts output itself. A weaker model sometimes
	// exits 0 after a plain final message without ever submitting, which would
	// hang the run. Turn that into an explicit failure so the tracker fails
	// the run and the user gets Continue/Restart.
	if termErr := mapTerminalStatus(exitCode, status); termErr != nil {
		return stats, termErr
	}
	if turnLimitHit && !result.completeStageCalled {
		log.Warn().Int64("idRun", task.IdRun).Str("stage", task.Stage).
			Msg("goose reached --max-turns before calling complete_stage")
		return stats, &common.AgentError{
			Code:   common.ErrCodeTurnLimitExhausted,
			Detail: "agent reached --max-turns before calling complete_stage",
		}
	}
	if !result.completeStageCalled {
		log.Warn().Int64("idRun", task.IdRun).Str("stage", task.Stage).
			Msg("goose exited 0 without calling complete_stage — failing stage")
		return stats, &common.AgentError{
			Code:   common.ErrCodeStageNotSubmitted,
			Detail: "agent ended without calling complete_stage",
		}
	}
	return stats, nil
}

// mapTerminalStatus converts (exit code, final status) into an error. A
// non-zero exit is authoritative; finalStatus (from the stream-json
// completion event) only refines the message.
func mapTerminalStatus(exitCode int, finalStatus string) error {
	if exitCode == 0 && isSuccessStatus(finalStatus) {
		return nil
	}
	if finalStatus != "" && !isSuccessStatus(finalStatus) {
		return fmt.Errorf("goose ended with status %q (exit %d)", finalStatus, exitCode)
	}
	if exitCode != 0 {
		return fmt.Errorf("goose exited with code %d", exitCode)
	}
	return nil
}

func isSuccessStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "success", "completed", "complete", "done", "ok":
		return true
	default:
		return false
	}
}

func isGooseErrorText(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "ran into this error") ||
		strings.Contains(lower, "session ended with error")
}

// classifyProviderError maps error text to a stable code: credit/quota
// exhaustion, rate limiting, or a generic provider error.
func classifyProviderError(text string) *common.AgentError {
	lower := strings.ToLower(text)
	detail := firstLine(text)
	switch {
	case strings.Contains(lower, "credit balance is too low"),
		strings.Contains(lower, "insufficient"),
		strings.Contains(lower, "quota"),
		strings.Contains(lower, "402"):
		return &common.AgentError{Code: common.ErrCodeProviderCreditExhausted, Detail: detail}
	case strings.Contains(lower, "rate limit"),
		strings.Contains(lower, "usage limit"),
		strings.Contains(lower, "too many requests"),
		strings.Contains(lower, "429"):
		return &common.AgentError{Code: common.ErrCodeProviderRateLimited, Detail: detail}
	default:
		return &common.AgentError{Code: common.ErrCodeProviderError, Detail: detail}
	}
}

// firstLine returns the first non-empty trimmed line of s — a concise UI
// detail instead of the full multi-paragraph provider error.
func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return strings.TrimSpace(s)
}

// toStreamableHTTP converts a tracker MCP SSE URL to the streamable-HTTP path
// goose ≥ v1.30 requires (".../sse" → ".../http"); other URLs pass through.
func toStreamableHTTP(mcpURL string) string {
	if strings.HasSuffix(mcpURL, "/sse") {
		return strings.TrimSuffix(mcpURL, "/sse") + "/http"
	}
	return mcpURL
}

// isToolRequestType reports whether a stream-json content-block type marks a
// tool call; alias names are tolerated so a goose rename doesn't zero the count.
func isToolRequestType(v any) bool {
	t, ok := v.(string)
	if !ok {
		return false
	}
	switch t {
	case "toolRequest", "tool_request", "tool_use":
		return true
	}
	return false
}

// tokensFromMap sums a token total, tolerating a couple of key layouts
// (total_tokens/total, or input+output). Returns 0 if none.
func tokensFromMap(m map[string]any) int {
	if t, ok := numberField(m, "total_tokens"); ok && t > 0 {
		return t
	}
	if t, ok := numberField(m, "total"); ok && t > 0 {
		return t
	}
	in, _ := numberField(m, "input_tokens")
	out, _ := numberField(m, "output_tokens")
	if in == 0 {
		in, _ = numberField(m, "input")
	}
	if out == 0 {
		out, _ = numberField(m, "output")
	}
	if in+out > 0 {
		return in + out
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

// forwardStderrLine maps goose's stderr lines to zerolog at the right
// severity by prefix-matching, so a real failure doesn't read as routine
// debug noise (parity with the claude adapter).
func forwardStderrLine(line string, idRun int64) {
	trim := strings.TrimLeft(line, " \t")
	switch {
	case hasPrefixFold(trim, "Error"),
		hasPrefixFold(trim, "Fatal"),
		hasPrefixFold(trim, "panic"):
		log.Error().Int64("idRun", idRun).Str("line", line).Msg("goose stderr")
	case hasPrefixFold(trim, "Warning"),
		hasPrefixFold(trim, "Warn"),
		hasPrefixFold(trim, "Deprecated"):
		log.Warn().Int64("idRun", idRun).Str("line", line).Msg("goose stderr")
	default:
		log.Debug().Int64("idRun", idRun).Str("line", line).Msg("goose stderr")
	}
}

func hasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}

func (a *GooseAdapter) Cancel(ctx context.Context, runID common.RunID) error {
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

// childEnv builds the goose subprocess env: inherited env plus goose wiring.
// Unlike the claude adapter it does NOT scrub ANTHROPIC_API_KEY — for goose
// the API key IS the BYOK billing path. GIT_PAGER=cat avoids the implement
// stage's `git log`/`git diff` blocking on a pager; GOOSE_DISABLE_KEYRING=1
// skips the OS keyring in headless mode.
//
// The tracker MCP URL + bearer are NOT set here — writeGooseConfig writes
// them literally into config.yaml per run (goose doesn't substitute ${ENV} in
// the extension uri).
func childEnv(cfg *common.GooseConfig) []string {
	out := append([]string{}, os.Environ()...)
	out = append(out,
		"GIT_PAGER=cat",
		"GOOSE_DISABLE_KEYRING=1",
		"GOOSE_PROVIDER="+cfg.Provider,
	)
	if cfg.AnthropicAPIKey != "" {
		out = append(out, "ANTHROPIC_API_KEY="+cfg.AnthropicAPIKey)
	}
	if cfg.GoogleAPIKey != "" {
		out = append(out, "GOOGLE_API_KEY="+cfg.GoogleAPIKey)
	}
	if cfg.OllamaHost != "" {
		out = append(out, "OLLAMA_HOST="+cfg.OllamaHost)
	}
	return out
}

func maxTurnsForStage(stage string, cfg *common.GooseConfig) int {
	if stage == common.StageImplementation {
		return cfg.MaxTurnsImplement
	}
	return cfg.MaxTurnsPlan
}

// gooseBinary returns the path to the goose CLI. The GOOSE_BINARY env var
// override lets the unit tests point at a stub script without the real binary.
func gooseBinary() string {
	if p := os.Getenv("GOOSE_BINARY"); p != "" {
		return p
	}
	return defaultGooseBinary
}
