package claudecode

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/gateway/claude-code/model"
	"github.com/bitmaster-sk/rurdesk/gateway/common"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func TestClaudeCodeReportsEveryModelAsThinking(t *testing.T) {
	isThinkingByModel, unanswered := NewClaudeCodeAdapter(&common.Config{}, nil).ModelThinking(context.Background(), []common.Model{{Name: "sonnet"}, {Name: "opus"}})

	if len(isThinkingByModel) != 2 || !isThinkingByModel["sonnet"] || !isThinkingByModel["opus"] {
		t.Errorf("thinking by model = %v, want sonnet and opus thinking", isThinkingByModel)
	}
	if len(unanswered) != 0 {
		t.Errorf("unanswered = %v, want none", unanswered)
	}
}

func TestMapTerminalStatus(t *testing.T) {
	const workspaceError = "API Error: 400 This API key is not scoped to a workspace, so this request must include the anthropic-workspace-id header."
	tests := []struct {
		name     string
		exitCode int
		final    finalResult
		wantErr  bool
		wantCode string
		wantMsg  string
		denyMsg  string
	}{
		{"success subtype", 0, finalResult{Subtype: "success"}, false, "", "", ""},
		{"empty subtype zero exit", 0, finalResult{}, false, "", "", ""},
		{"max turns", 1, finalResult{Subtype: "error_max_turns", IsError: true}, true, common.ErrCodeTurnLimitExhausted, "", ""},
		{"execution error", 1, finalResult{Subtype: "error_during_execution", IsError: true}, true, "", "error_during_execution", ""},
		{"is_error no subtype", 1, finalResult{IsError: true}, true, "", "reported an error", ""},
		{"nonzero exit only", 2, finalResult{}, true, "", "exited with code 2", ""},
		{
			"API error shows its text, not the success subtype", 1,
			finalResult{Subtype: "success", IsError: true, Text: workspaceError, ApiErrorStatus: 400},
			true, common.ErrCodeProviderError, "not scoped to a workspace", `"success"`,
		},
		{"API auth error", 1, finalResult{Subtype: "success", IsError: true, Text: "API Error: 401 invalid x-api-key", ApiErrorStatus: 401}, true, common.ErrCodeProviderAuthFailed, "invalid x-api-key", ""},
		{"API forbidden", 1, finalResult{Subtype: "success", IsError: true, Text: "API Error: 403 forbidden", ApiErrorStatus: 403}, true, common.ErrCodeProviderAuthFailed, "forbidden", ""},
		{"API rate limit", 1, finalResult{Subtype: "success", IsError: true, Text: "API Error: 429 rate_limit_error", ApiErrorStatus: 429}, true, common.ErrCodeProviderRateLimited, "rate_limit_error", ""},
		{"error text without API status", 1, finalResult{Subtype: "success", IsError: true, Text: "Invalid API key · Please run /login\nmore"}, true, "", "Invalid API key · Please run /login", `"success"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mapTerminalStatus(tt.exitCode, tt.final)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected nil, got %v", err)
			}
			if tt.wantCode != "" {
				var agentErr *common.AgentError
				if !errors.As(err, &agentErr) || agentErr.Code != tt.wantCode {
					t.Errorf("error %v, want code %q", err, tt.wantCode)
				}
			}
			if tt.wantMsg != "" && !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantMsg)
			}
			if tt.denyMsg != "" && strings.Contains(err.Error(), tt.denyMsg) {
				t.Errorf("error %q still contains %q", err.Error(), tt.denyMsg)
			}
			if strings.Contains(tt.final.Text, "\n") && strings.Contains(err.Error(), "\n") {
				t.Errorf("error %q is not a single line", err.Error())
			}
		})
	}
}

func TestDrainEventsPassesTheResultErrorText(t *testing.T) {
	line := `{"type":"result","subtype":"success","is_error":true,"api_error_status":400,"result":"API Error: 400 bad request"}`
	var final finalResult
	drainEvents(strings.NewReader(line+"\n"), common.DefaultMaxOutputLineBytes, 7,
		func(result finalResult) { final = result }, nil, nil, nil, nil)

	want := finalResult{Subtype: "success", IsError: true, Text: "API Error: 400 bad request", ApiErrorStatus: 400}
	if final != want {
		t.Errorf("final = %+v, want %+v", final, want)
	}
}

func TestExtractResultUsage(t *testing.T) {
	ev := func(line string) model.StreamLine {
		var event model.StreamLine
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		return event
	}

	withCost := extractResultUsage(ev(
		`{"type":"result","subtype":"success","is_error":false,"total_cost_usd":0.4213,` +
			`"usage":{"input_tokens":12,"output_tokens":3456,"cache_creation_input_tokens":20000,"cache_read_input_tokens":150000}}`,
	))
	if withCost.InputTokens != 12 || withCost.OutputTokens != 3456 ||
		withCost.CacheReadTokens != 150000 || withCost.CacheWriteTokens != 20000 {
		t.Errorf("usage = %+v, want 12/3456/150000/20000", withCost)
	}
	if withCost.CostUsd == nil || *withCost.CostUsd != 0.4213 {
		t.Errorf("costUsd = %v, want 0.4213", withCost.CostUsd)
	}

	withoutCost := extractResultUsage(ev(
		`{"type":"result","subtype":"success","is_error":false,"usage":{"input_tokens":1,"output_tokens":2}}`,
	))
	if withoutCost.CostUsd != nil {
		t.Errorf("costUsd = %v, want nil when total_cost_usd is absent", *withoutCost.CostUsd)
	}
}

func TestDrainEventsExtractsStatsFromStreamJSON(t *testing.T) {
	// An assistant turn with two tool_use blocks and a usage record, followed by
	// a final result event carrying a larger usage total.
	lines := []string{
		`{"type":"system","subtype":"init","session_id":"abc"}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"hi"},{"type":"tool_use","id":"t1","name":"Bash"},{"type":"tool_use","id":"t2","name":"Read"}],"usage":{"input_tokens":100,"output_tokens":40}}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"t1"}]}}`,
		`{"type":"result","subtype":"success","is_error":false,"usage":{"input_tokens":300,"output_tokens":120,"cache_read_input_tokens":80},"total_cost_usd":0.01}`,
	}
	r := strings.NewReader(strings.Join(lines, "\n") + "\n")

	var subtype string
	var isError bool
	var toolCalls int
	var tokens int
	drainEvents(r, common.DefaultMaxOutputLineBytes, 42,
		func(final finalResult) { subtype = final.Subtype; isError = final.IsError },
		func(n int, _ bool) { toolCalls += n },
		func(tk int) {
			if tk > tokens {
				tokens = tk
			}
		},
		nil,
		nil,
	)

	if toolCalls != 2 {
		t.Errorf("toolCalls = %d, want 2", toolCalls)
	}
	// result usage total (300+120+80=500) is larger than the assistant usage
	// (140), so the max-tracking caller keeps 500.
	if tokens != 500 {
		t.Errorf("tokens = %d, want 500", tokens)
	}
	if subtype != "success" {
		t.Errorf("subtype = %q, want success", subtype)
	}
	if isError {
		t.Errorf("isError = true, want false")
	}
}

func TestDrainEventsDetectsCompleteStage(t *testing.T) {
	tests := []struct {
		name string
		line string
		want bool
	}{
		{
			name: "namespaced tracker tool counts as submitted",
			line: `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1","name":"mcp__tracker__complete_stage"}]}}`,
			want: true,
		},
		{
			name: "other tools do not",
			line: `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1","name":"Read"},{"type":"tool_use","id":"t2","name":"Bash"}]}}`,
			want: false,
		},
		{
			name: "text-only turn does not",
			line: `{"type":"assistant","message":{"content":[{"type":"text","text":"here is my plan"}]}}`,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			saw := false
			drainEvents(strings.NewReader(tt.line+"\n"), common.DefaultMaxOutputLineBytes, 7,
				nil,
				func(_ int, sawCompleteStage bool) {
					if sawCompleteStage {
						saw = true
					}
				},
				nil,
				nil,
				nil,
			)
			if saw != tt.want {
				t.Errorf("sawCompleteStage = %v, want %v", saw, tt.want)
			}
		})
	}
}

func TestWriteMCPSettings(t *testing.T) {
	configDir := t.TempDir()
	const mcpURL = "http://issue.proxy/mcp/plan/http"
	const agentKey = "deadbeef"

	path, err := writeMCPSettings(configDir, mcpURL, agentKey, 42)
	if err != nil {
		t.Fatalf("writeMCPSettings: %v", err)
	}
	if path != filepath.Join(configDir, "mcp.json") {
		t.Errorf("unexpected path %q", path)
	}

	// The file carries the bearer token in cleartext — keep it owner-only.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat settings: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("settings mode = %o, want 600", perm)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading settings: %v", err)
	}
	var parsed struct {
		McpServers map[string]struct {
			Type    string            `json:"type"`
			URL     string            `json:"url"`
			Headers map[string]string `json:"headers"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	tracker, ok := parsed.McpServers["tracker"]
	if !ok {
		t.Fatal("missing tracker server entry")
	}
	if tracker.Type != "http" {
		t.Errorf("type = %q, want http", tracker.Type)
	}
	if tracker.URL != mcpURL {
		t.Errorf("url = %q, want %q", tracker.URL, mcpURL)
	}
	if got := tracker.Headers[common.TrackerRunHeader]; got != "42" {
		t.Errorf("run header = %q, want 42", got)
	}
	if got := tracker.Headers["Authorization"]; got != "Bearer "+agentKey {
		t.Errorf("Authorization = %q, want %q", got, "Bearer "+agentKey)
	}
}

func TestChildEnvScrubsAPIKeysAndSetsToken(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-should-be-removed")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "tok-should-be-removed")

	env := childEnv("my-oauth-token", "git-token-of-the-run", "")

	for _, kv := range env {
		if strings.HasPrefix(kv, "ANTHROPIC_API_KEY=") {
			t.Errorf("ANTHROPIC_API_KEY leaked into child env: %q", kv)
		}
		if strings.HasPrefix(kv, "ANTHROPIC_AUTH_TOKEN=") {
			t.Errorf("ANTHROPIC_AUTH_TOKEN leaked into child env: %q", kv)
		}
	}
	if !hasEnv(env, "GIT_PAGER=cat") {
		t.Error("GIT_PAGER=cat not set")
	}
	if !hasEnv(env, "CLAUDE_CODE_OAUTH_TOKEN=my-oauth-token") {
		t.Error("CLAUDE_CODE_OAUTH_TOKEN not set")
	}
	if !hasEnv(env, "RURDESK_GIT_TOKEN=git-token-of-the-run") {
		t.Error("the git token of the run is not in the agent env")
	}
}

func TestChildEnvOmitsTokenWhenEmpty(t *testing.T) {
	env := childEnv("", "", "")
	for _, kv := range env {
		if strings.HasPrefix(kv, "CLAUDE_CODE_OAUTH_TOKEN=") {
			t.Errorf("CLAUDE_CODE_OAUTH_TOKEN should be absent, got %q", kv)
		}
	}
}

func hasEnv(env []string, want string) bool {
	for _, kv := range env {
		if kv == want {
			return true
		}
	}
	return false
}

// TestRunWithStubBinary drives Run against a stub `claude` script that emits
// canned stream-json and exits, exercising the full subprocess path without the
// real CLI.
func TestRunWithStubBinary(t *testing.T) {
	tests := []struct {
		name         string
		resultLine   string
		exitCode     int
		submitsStage bool
		wantErr      bool
		wantCode     string
		wantTools    int
	}{
		{
			name:         "success",
			resultLine:   `{"type":"result","subtype":"success","is_error":false,"usage":{"input_tokens":10,"output_tokens":5}}`,
			exitCode:     0,
			submitsStage: true,
			wantErr:      false,
			wantTools:    2,
		},
		{
			name:       "max turns",
			resultLine: `{"type":"result","subtype":"error_max_turns","is_error":true,"usage":{"input_tokens":10,"output_tokens":5}}`,
			exitCode:   1,
			wantErr:    true,
			wantCode:   common.ErrCodeTurnLimitExhausted,
			wantTools:  1,
		},
		{
			name:       "exit 0 without complete_stage",
			resultLine: `{"type":"result","subtype":"success","is_error":false,"usage":{"input_tokens":10,"output_tokens":5}}`,
			exitCode:   0,
			wantErr:    true,
			wantCode:   common.ErrCodeStageNotSubmitted,
			wantTools:  1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			worktree := t.TempDir()
			stub := writeStubClaude(t, tt.resultLine, tt.exitCode, tt.submitsStage)
			t.Setenv("CLAUDE_CODE_BINARY", stub)

			cfg := &common.Config{
				TrackerMCPUrl: "http://issue.proxy/mcp/sse",
				AgentApiKey:   "deadbeef",
				ClaudeCode:    &common.ClaudeCodeConfig{},
			}
			adapter := NewClaudeCodeAdapter(cfg, nil)
			task := common.Task{
				IdRun:        7,
				IdTask:       1,
				IdIssue:      2,
				IdProject:    3,
				Stage:        common.StageImplementation,
				AttemptNo:    1,
				WorktreePath: worktree,
				IssueTitle:   "Test",
				IssueDesc:    "Body",
			}
			isReportedSubmitted := false
			task.OnStageSubmitted = func() { isReportedSubmitted = true }

			stats, err := adapter.Run(context.Background(), task)
			if isReportedSubmitted != tt.submitsStage {
				t.Errorf("reported stage submitted = %v, want %v", isReportedSubmitted, tt.submitsStage)
			}
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected nil, got %v", err)
			}
			if tt.wantCode != "" {
				var agentErr *common.AgentError
				if !errors.As(err, &agentErr) {
					t.Fatalf("error %v is not an *AgentError", err)
				}
				if agentErr.Code != tt.wantCode {
					t.Errorf("error code = %q, want %q", agentErr.Code, tt.wantCode)
				}
			}
			if stats.ToolCallsCount != tt.wantTools {
				t.Errorf("ToolCallsCount = %d, want %d", stats.ToolCallsCount, tt.wantTools)
			}
			if stats.TokensUsed != 15 {
				t.Errorf("TokensUsed = %d, want 15", stats.TokensUsed)
			}
			if stats.DurationMs < 0 {
				t.Errorf("DurationMs = %d, want >= 0", stats.DurationMs)
			}
			// The MCP config carries the agent's bearer token and the agent
			// commits from this worktree, so the run must leave nothing
			// token-bearing behind for `git add -A` to pick up.
			entries, readErr := os.ReadDir(worktree)
			if readErr != nil {
				t.Fatalf("reading worktree: %v", readErr)
			}
			for _, entry := range entries {
				if entry.Name() == ".claude" || entry.Name() == "mcp.json" {
					t.Errorf("run left %q in the worktree", entry.Name())
				}
			}
		})
	}
}

// writeStubClaude writes an executable shell script that mimics the claude CLI
// stream-json output: one assistant event with a single tool_use, then the
// provided result line, exiting with the given code.
func writeStubClaude(t *testing.T, resultLine string, exitCode int, submitsStage bool) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "claude-stub.sh")
	script := "#!/bin/sh\n" +
		`echo '{"type":"system","subtype":"init"}'` + "\n" +
		`echo '{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1","name":"Bash"}],"usage":{"input_tokens":8,"output_tokens":4}}}'` + "\n"
	if submitsStage {
		script += `echo '{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t2","name":"mcp__tracker__complete_stage"}]}}'` + "\n"
	}
	script += "echo '" + resultLine + "'\n" +
		"exit " + strconv.Itoa(exitCode) + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("writing stub: %v", err)
	}
	return path
}

func TestCancelAlwaysEndsARunningStage(t *testing.T) {
	stub, startedFile := writeHangingStub(t, "claude-stub.sh")
	t.Setenv("CLAUDE_CODE_BINARY", stub)
	cfg := &common.Config{
		TrackerMCPUrl: "http://issue.proxy/mcp/sse",
		AgentApiKey:   "deadbeef",
		ClaudeCode:    &common.ClaudeCodeConfig{},
	}
	adapter := NewClaudeCodeAdapter(cfg, nil)

	for attempt := range 20 {
		_ = os.Remove(startedFile)
		ctx, cancel := context.WithCancel(context.Background())
		task := common.Task{IdRun: 7, IdTask: int64(attempt + 1), Stage: common.StageImplementation, WorktreePath: t.TempDir()}
		result := make(chan error, 1)
		go func() {
			_, err := adapter.Run(ctx, task)
			result <- err
		}()
		waitForFile(t, startedFile)

		cancel()
		if err := adapter.Cancel(context.Background(), "7"); err != nil {
			t.Fatalf("Cancel: %v", err)
		}
		select {
		case err := <-result:
			if err == nil || !strings.Contains(err.Error(), "claude code cancelled") {
				t.Fatalf("attempt %d: Run returned %v, want claude code cancelled", attempt, err)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("attempt %d: Run did not return after Cancel", attempt)
		}
	}
}

func writeHangingStub(t *testing.T, name string) (stub, startedFile string) {
	t.Helper()
	dir := t.TempDir()
	startedFile = filepath.Join(dir, "started")
	stub = filepath.Join(dir, name)
	script := "#!/bin/sh\ntouch " + startedFile + "\nexec sleep 30\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatalf("writing stub: %v", err)
	}
	return stub, startedFile
}

func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("stub never started (%s missing)", path)
}

func TestDrainEventsKeepsReadingAfterALineOverTheLimit(t *testing.T) {
	input := strings.Repeat("x", 300*1024) + "\n" +
		`{"type":"result","subtype":"success","is_error":false}` + "\n"
	var subtype string
	drainEvents(strings.NewReader(input), 1024, 42,
		func(final finalResult) { subtype = final.Subtype },
		nil,
		nil,
		nil,
		nil,
	)
	if subtype != "success" {
		t.Errorf("subtype = %q, want the result event after the long line to be read", subtype)
	}
}

func TestRunPassesTheEffortOnlyWhenSet(t *testing.T) {
	for _, tt := range []struct {
		effort   string
		isPassed bool
	}{{"max", true}, {"", false}} {
		dir := t.TempDir()
		argsFile := filepath.Join(dir, "args.txt")
		stub := filepath.Join(dir, "claude-stub.sh")
		script := "#!/bin/sh\n" +
			`printf '%s\n' "$@" > ` + argsFile + "\n" +
			"cat > /dev/null\n" +
			`echo '{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1","name":"mcp__tracker__complete_stage"}]}}'` + "\n" +
			`echo '{"type":"result","subtype":"success","is_error":false}'` + "\n"
		if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
			t.Fatalf("writing stub: %v", err)
		}
		t.Setenv("CLAUDE_CODE_BINARY", stub)
		cfg := &common.Config{TrackerMCPUrl: "http://issue.proxy/mcp/sse", AgentApiKey: "deadbeef", ClaudeCode: &common.ClaudeCodeConfig{}}
		task := common.Task{IdRun: 7, IdTask: 1, Stage: common.StageDesign, WorktreePath: t.TempDir(), Model: common.Model{Name: "opus"}, MaxTurns: 60, Effort: tt.effort}

		if _, err := NewClaudeCodeAdapter(cfg, nil).Run(context.Background(), task); err != nil {
			t.Fatalf("Run: %v", err)
		}

		args, err := os.ReadFile(argsFile)
		if err != nil {
			t.Fatalf("reading args: %v", err)
		}
		if got := strings.Contains(string(args), "--effort\n"+tt.effort+"\n"); got != tt.isPassed {
			t.Errorf("effort %q passed = %v, want %v\n--- args ---\n%s", tt.effort, got, tt.isPassed, args)
		}
		if !tt.isPassed && strings.Contains(string(args), "--effort") {
			t.Errorf("no effort was requested, but claude got --effort\n--- args ---\n%s", args)
		}
	}
}

func TestRunPassesPromptOnStdinNotInArgs(t *testing.T) {
	const marker = "vite-scaffold-marker"
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	stdinFile := filepath.Join(dir, "stdin.txt")
	stub := filepath.Join(dir, "claude-stub.sh")
	script := "#!/bin/sh\n" +
		`printf '%s\n' "$@" > ` + argsFile + "\n" +
		"cat > " + stdinFile + "\n" +
		`echo '{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1","name":"mcp__tracker__complete_stage"}]}}'` + "\n" +
		`echo '{"type":"result","subtype":"success","is_error":false}'` + "\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatalf("writing stub: %v", err)
	}
	t.Setenv("CLAUDE_CODE_BINARY", stub)
	cfg := &common.Config{
		TrackerMCPUrl: "http://issue.proxy/mcp/sse",
		AgentApiKey:   "deadbeef",
		ClaudeCode:    &common.ClaudeCodeConfig{},
	}
	adapter := NewClaudeCodeAdapter(cfg, nil)
	task := common.Task{
		IdRun:        7,
		IdTask:       1,
		Stage:        common.StageImplementation,
		WorktreePath: t.TempDir(),
		IssueTitle:   "Scaffold",
		IssueDesc:    "Run " + marker + " then stop it",
		Model:        common.Model{Name: "opus"},
		MaxTurns:     60,
	}

	if _, err := adapter.Run(context.Background(), task); err != nil {
		t.Fatalf("Run: %v", err)
	}

	args, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("reading args: %v", err)
	}
	if strings.Contains(string(args), marker) {
		t.Errorf("claude command line contains the prompt\n--- args ---\n%s", args)
	}
	for _, want := range []string{"--model\nopus\n", "--max-turns\n60\n"} {
		if !strings.Contains(string(args), want) {
			t.Errorf("claude args missing %q\n--- args ---\n%s", want, args)
		}
	}
	stdin, err := os.ReadFile(stdinFile)
	if err != nil {
		t.Fatalf("reading stdin: %v", err)
	}
	task.MaxTurns = 60
	task.Vocab = common.ToolVocabClaudeCode
	wantPrompt, err := common.RenderPrompt(task)
	if err != nil {
		t.Fatalf("rendering prompt: %v", err)
	}
	if string(stdin) != wantPrompt {
		t.Errorf("claude stdin is not the full prompt\n--- got ---\n%s\n--- want ---\n%s", stdin, wantPrompt)
	}
}

const testRunToken = "rdr_0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// writeRecordingStub writes a claude stub that copies its MCP config to
// mcpCopy, its env to envFile and its args to argsFile, then submits the stage.
func writeRecordingStub(t *testing.T, dir string) (stub, mcpCopy, envFile, argsFile string) {
	t.Helper()
	stub = filepath.Join(dir, "claude-stub.sh")
	mcpCopy = filepath.Join(dir, "mcp.json")
	envFile = filepath.Join(dir, "env.txt")
	argsFile = filepath.Join(dir, "args.txt")
	script := "#!/bin/sh\n" +
		`printf '%s\n' "$@" > ` + argsFile + "\n" +
		"env > " + envFile + "\n" +
		`while [ $# -gt 0 ]; do if [ "$1" = "--mcp-config" ]; then cp "$2" ` + mcpCopy + `; fi; shift; done` + "\n" +
		"cat > /dev/null\n" +
		`echo '{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1","name":"mcp__tracker__complete_stage"}]}}'` + "\n" +
		`echo '{"type":"result","subtype":"success","is_error":false}'` + "\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatalf("writing stub: %v", err)
	}
	return stub, mcpCopy, envFile, argsFile
}

func TestClaudeAdapter_ChatUsesRunTokenAndURL(t *testing.T) {
	stub, mcpCopy, envFile, argsFile := writeRecordingStub(t, t.TempDir())
	t.Setenv("CLAUDE_CODE_BINARY", stub)
	t.Setenv("GATEWAY_TO_TRACKER_TOKEN", "deadbeef")
	t.Setenv("TRACKER_TO_GATEWAY_TOKEN", "webhook-secret-hex")
	t.Setenv("RURDESK_GIT_TOKEN", "inherited-git-token")
	cfg := &common.Config{TrackerMCPUrl: "http://issue.proxy/mcp/sse", AgentApiKey: "deadbeef", ClaudeCode: &common.ClaudeCodeConfig{}}
	task := common.Task{
		IdRun: 7, IdTask: 31, Stage: common.StageChat, AttemptNo: 12,
		WorktreePath: t.TempDir(), Model: common.Model{Name: "opus"}, MaxTurns: 60,
		RunToken: testRunToken,
		Repo:     common.AgentRunGitRepo{Token: "run-git-token"},
		Chat:     &common.ChatContext{Mode: common.ChatModeReply},
	}

	if _, err := NewClaudeCodeAdapter(cfg, nil).Run(context.Background(), task); err != nil {
		t.Fatalf("Run: %v", err)
	}

	data, err := os.ReadFile(mcpCopy)
	if err != nil {
		t.Fatalf("reading the MCP config the CLI got: %v", err)
	}
	var parsed struct {
		McpServers map[string]struct {
			URL     string            `json:"url"`
			Headers map[string]string `json:"headers"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	tracker := parsed.McpServers["tracker"]
	if tracker.URL != "http://issue.proxy/mcp/chat/http" {
		t.Errorf("url = %q, want the chat endpoint", tracker.URL)
	}
	if got := tracker.Headers["Authorization"]; got != "Bearer "+testRunToken {
		t.Errorf("Authorization = %q, want the run token", got)
	}
	assertAgentEnvHasNoSecrets(t, envFile, "deadbeef", "webhook-secret-hex", "inherited-git-token", "run-git-token")
	if env, err := os.ReadFile(envFile); err != nil || !strings.Contains(string(env), "RURDESK_WORKTREE="+task.WorktreePath+"\n") {
		t.Errorf("chat env does not name the turn's worktree %q (err %v)", task.WorktreePath, err)
	}
	args, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	if want := "--session-id\n" + common.TaskSessionUUID(task) + "\n"; !strings.Contains(string(args), want) {
		t.Errorf("claude args lack %q\n%s", want, args)
	}
}

// The SSE transport makes the CLI post to the message URL the tracker
// advertises, built from PUBLIC_BASE_URL, which the gateway may not reach.
// Streamable HTTP only ever talks to the configured URL.
func TestClaudeAdapter_TrackerMCPUsesStreamableHTTP(t *testing.T) {
	tests := []struct {
		name    string
		stage   string
		wantURL string
	}{
		{"implementation gets the full endpoint", common.StageImplementation, "http://issue.proxy/mcp/http"},
		{"plan stage gets the plan subset", common.StageBrainstorming, "http://issue.proxy/mcp/plan/http"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub, mcpCopy, _, _ := writeRecordingStub(t, t.TempDir())
			t.Setenv("CLAUDE_CODE_BINARY", stub)
			cfg := &common.Config{TrackerMCPUrl: "http://issue.proxy/mcp/sse", AgentApiKey: "deadbeef", ClaudeCode: &common.ClaudeCodeConfig{}}
			task := common.Task{
				IdRun: 7, IdTask: 1, Stage: tt.stage,
				WorktreePath: t.TempDir(), Model: common.Model{Name: "opus"}, MaxTurns: 60,
			}

			if _, err := NewClaudeCodeAdapter(cfg, nil).Run(context.Background(), task); err != nil {
				t.Fatalf("Run: %v", err)
			}

			data, err := os.ReadFile(mcpCopy)
			if err != nil {
				t.Fatalf("reading the MCP config the CLI got: %v", err)
			}
			var parsed struct {
				McpServers map[string]struct {
					Type string `json:"type"`
					URL  string `json:"url"`
				} `json:"mcpServers"`
			}
			if err := json.Unmarshal(data, &parsed); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			tracker := parsed.McpServers["tracker"]
			if tracker.Type != "http" {
				t.Errorf("type = %q, want http", tracker.Type)
			}
			if tracker.URL != tt.wantURL {
				t.Errorf("url = %q, want %q", tracker.URL, tt.wantURL)
			}
		})
	}
}

func TestAgentEnv_StripsTrackerAndGitTokens(t *testing.T) {
	stub, _, envFile, _ := writeRecordingStub(t, t.TempDir())
	t.Setenv("CLAUDE_CODE_BINARY", stub)
	t.Setenv("GATEWAY_TO_TRACKER_TOKEN", "deadbeef")
	t.Setenv("TRACKER_TO_GATEWAY_TOKEN", "webhook-secret-hex")
	t.Setenv("RURDESK_GIT_TOKEN", "inherited-git-token")
	cfg := &common.Config{TrackerMCPUrl: "http://issue.proxy/mcp/sse", AgentApiKey: "deadbeef", ClaudeCode: &common.ClaudeCodeConfig{}}
	task := common.Task{
		IdRun: 7, IdTask: 1, Stage: common.StageImplementation,
		WorktreePath: t.TempDir(), Model: common.Model{Name: "opus"}, MaxTurns: 60,
		Repo: common.AgentRunGitRepo{Token: "run-git-token"},
	}

	if _, err := NewClaudeCodeAdapter(cfg, nil).Run(context.Background(), task); err != nil {
		t.Fatalf("Run: %v", err)
	}

	assertAgentEnvHasNoSecrets(t, envFile, "deadbeef", "webhook-secret-hex", "inherited-git-token")
	env, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(env), "RURDESK_GIT_TOKEN=run-git-token\n") {
		t.Error("an issue stage lost the git token of its run")
	}
	if !strings.Contains(string(env), "RURDESK_WORKTREE="+task.WorktreePath+"\n") {
		t.Errorf("agent env does not name the run's worktree %q", task.WorktreePath)
	}
}

func assertAgentEnvHasNoSecrets(t *testing.T, envFile string, secrets ...string) {
	t.Helper()
	env, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatalf("reading the agent env: %v", err)
	}
	for _, secret := range secrets {
		if strings.Contains(string(env), secret) {
			t.Errorf("the agent env holds %q", secret)
		}
	}
}

// A chat turn reads only: its built-in tools are an allow-list (Read, Grep, Glob, web) backed by a
// deny list, and it leaves no session transcript on the gateway host. Issue stages keep every tool and session.
func TestClaudeAdapter_ChatRestrictsToolsAndSkipsSessionPersistence(t *testing.T) {
	chatArgs := []string{
		"--no-session-persistence",
		"--tools", "Read,Grep,Glob,WebSearch,WebFetch",
		"--disallowedTools", "Bash,Write,Edit,MultiEdit,NotebookEdit,Task",
	}
	cfg := &common.Config{TrackerMCPUrl: "http://issue.proxy/mcp/sse", AgentApiKey: "deadbeef", ClaudeCode: &common.ClaudeCodeConfig{}}
	for _, tt := range []struct {
		name         string
		task         common.Task
		isRestricted bool
	}{
		{"chat", common.Task{Stage: common.StageChat, RunToken: testRunToken, Chat: &common.ChatContext{Mode: common.ChatModeReply}}, true},
		{"design", common.Task{Stage: common.StageDesign}, false},
		{"implementation", common.Task{Stage: common.StageImplementation}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stub, _, _, argsFile := writeRecordingStub(t, t.TempDir())
			t.Setenv("CLAUDE_CODE_BINARY", stub)
			task := tt.task
			task.IdRun, task.IdTask, task.WorktreePath, task.Model, task.MaxTurns = 7, 1, t.TempDir(), common.Model{Name: "opus"}, 60

			if _, err := NewClaudeCodeAdapter(cfg, nil).Run(context.Background(), task); err != nil {
				t.Fatalf("Run: %v", err)
			}

			raw, err := os.ReadFile(argsFile)
			if err != nil {
				t.Fatal(err)
			}
			args := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
			if tt.isRestricted {
				if len(args) < len(chatArgs) || !slices.Equal(args[len(args)-len(chatArgs):], chatArgs) {
					t.Errorf("chat argv must end with %q\n--- args ---\n%s", chatArgs, raw)
				}
				return
			}
			for _, flag := range []string{"--no-session-persistence", "--tools", "--disallowedTools"} {
				if slices.Contains(args, flag) {
					t.Errorf("an issue stage got %s\n--- args ---\n%s", flag, raw)
				}
			}
		})
	}
}

func TestClaudeAdapter_ChatPassesTheModelAndEffort(t *testing.T) {
	stub, _, _, argsFile := writeRecordingStub(t, t.TempDir())
	t.Setenv("CLAUDE_CODE_BINARY", stub)
	cfg := &common.Config{TrackerMCPUrl: "http://issue.proxy/mcp/sse", AgentApiKey: "deadbeef", ClaudeCode: &common.ClaudeCodeConfig{}}
	task := common.Task{
		IdRun: 7, IdTask: 1, Stage: common.StageChat, WorktreePath: t.TempDir(), MaxTurns: 60,
		RunToken: testRunToken, Chat: &common.ChatContext{Mode: common.ChatModeReply},
		Model: common.Model{Name: "sonnet"}, Effort: "high",
	}

	if _, err := NewClaudeCodeAdapter(cfg, nil).Run(context.Background(), task); err != nil {
		t.Fatalf("Run: %v", err)
	}

	args, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--model\nsonnet\n", "--effort\nhigh\n", "--tools\nRead,Grep,Glob,WebSearch,WebFetch\n"} {
		if !strings.Contains(string(args), want) {
			t.Errorf("chat args missing %q\n--- args ---\n%s", want, args)
		}
	}
}

const streamFixturePath = "testdata/stream_partial.jsonl"

// readStreamFixture returns the recorded `--include-partial-messages` stream
// (a thought, one Glob call, the answer) with the tool renamed to toolName.
func readStreamFixture(t *testing.T, toolName string) string {
	t.Helper()
	data, err := os.ReadFile(streamFixturePath)
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	return strings.ReplaceAll(string(data), `"name":"Glob"`, `"name":"`+toolName+`"`)
}

type recordingThinkingSender struct {
	mu     sync.Mutex
	events []common.ThinkingEvent
}

func (s *recordingThinkingSender) SendThinking(_ context.Context, _ int64, _ int, events []common.ThinkingEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, events...)
	return nil
}

func (s *recordingThinkingSender) received() []common.ThinkingEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]common.ThinkingEvent(nil), s.events...)
}

func TestDrainEvents_ForwardsThinkingDeltasAndToolUse(t *testing.T) {
	sender := &recordingThinkingSender{}
	batcher := common.NewThinkingBatcher(sender, 31)
	batcher.Start(context.Background())

	drainEvents(strings.NewReader(readStreamFixture(t, "Glob")), common.DefaultMaxOutputLineBytes, 7,
		nil, nil, nil, nil, batcher.Add)
	batcher.Stop()

	// The 13 recorded fragments arrive merged into one thought, before the tool.
	want := []common.ThinkingEvent{
		{Kind: common.ThinkingKindThinking, Text: "I'm going to call Glob once with the '*.txt' pattern and then answer with just the" +
			" file name. Since Glob may return several matches, I'll keep my reasoning short and just make the call.\n\n"},
		{Kind: common.ThinkingKindTool, Tool: "Glob", Text: "*.txt"},
	}

	got := sender.received()
	for index := range got {
		got[index].At = 0
	}
	if len(got) != len(want) {
		t.Fatalf("got %d events, want %d\n%+v", len(got), len(want), got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Errorf("event %d = %+v, want %+v", index, got[index], want[index])
		}
	}
}

func TestToolDetail(t *testing.T) {
	tests := []struct {
		name  string
		tool  string
		input map[string]any
		want  string
	}{
		{"bash command", "Bash", map[string]any{"command": "go test ./...", "description": "run tests"}, "go test ./..."},
		{"multi-line command keeps the first line", "Bash", map[string]any{"command": "cat <<EOF\nbody\nEOF"}, "cat <<EOF…"},
		{"read path", "Read", map[string]any{"file_path": "/w/main.go"}, "/w/main.go"},
		{"edit path", "Edit", map[string]any{"file_path": "/w/a.go", "old_string": "x"}, "/w/a.go"},
		{"write path", "Write", map[string]any{"file_path": "/w/b.go", "content": "y"}, "/w/b.go"},
		{"grep pattern", "Grep", map[string]any{"pattern": "func main", "path": "/w"}, "func main"},
		{"glob pattern", "Glob", map[string]any{"pattern": "**/*.go"}, "**/*.go"},
		{"other tool", "mcp__tracker__get_issue", map[string]any{"idIssue": 3}, ""},
		{"no input", "Bash", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var input json.RawMessage
			if tt.input != nil {
				encoded, err := json.Marshal(tt.input)
				if err != nil {
					t.Fatal(err)
				}
				input = encoded
			}
			if got := toolDetail(model.ContentBlock{Type: "tool_use", Name: tt.tool, Input: input}); got != tt.want {
				t.Errorf("toolDetail = %q, want %q", got, tt.want)
			}
		})
	}
}

func thinkingDeltaLine(fragment string) string {
	encoded, err := json.Marshal(fragment)
	if err != nil {
		panic(err)
	}
	return `{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":` + string(encoded) + `}}}`
}

func TestDrainEvents_CoalescesThinkingFragments(t *testing.T) {
	fiftyChars := strings.Repeat("x", 49) + " "
	const progress = `{"type":"system","subtype":"thinking_tokens","estimated_tokens":5}`
	const blockStop = `{"type":"stream_event","event":{"type":"content_block_stop","index":0}}`
	const toolCall = `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1","name":"Read","input":{"file_path":"/w/a.go"}}]}}`
	thinking := func(text string) common.ThinkingEvent {
		return common.ThinkingEvent{Kind: common.ThinkingKindThinking, Text: text}
	}
	tests := []struct {
		name  string
		lines []string
		want  []common.ThinkingEvent
	}{
		{
			name:  "flushes once the merged thought reaches 200 chars",
			lines: []string{thinkingDeltaLine(fiftyChars), progress, thinkingDeltaLine(fiftyChars), thinkingDeltaLine(fiftyChars), progress, thinkingDeltaLine(fiftyChars), thinkingDeltaLine("tail")},
			want:  []common.ThinkingEvent{thinking(strings.Repeat(fiftyChars, 4)), thinking("tail")},
		},
		{
			name:  "flushes when the thinking block ends",
			lines: []string{thinkingDeltaLine("first "), thinkingDeltaLine("block"), blockStop, thinkingDeltaLine("second")},
			want:  []common.ThinkingEvent{thinking("first block"), thinking("second")},
		},
		{
			name:  "sends the thought before the tool call that follows it",
			lines: []string{thinkingDeltaLine("read it"), toolCall, thinkingDeltaLine("done")},
			want: []common.ThinkingEvent{
				thinking("read it"),
				{Kind: common.ThinkingKindTool, Tool: "Read", Text: "/w/a.go"},
				thinking("done"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []common.ThinkingEvent
			drainEvents(strings.NewReader(strings.Join(tt.lines, "\n")+"\n"), common.DefaultMaxOutputLineBytes, 7,
				nil, nil, nil, nil, func(event common.ThinkingEvent) { got = append(got, event) })
			if len(got) != len(tt.want) {
				t.Fatalf("got %d events, want %d\n%+v", len(got), len(tt.want), got)
			}
			for index := range tt.want {
				if got[index] != tt.want[index] {
					t.Errorf("event %d = %+v, want %+v", index, got[index], tt.want[index])
				}
			}
		})
	}
}

func TestDrainEvents_LogsNoThinkingAtInfo(t *testing.T) {
	var buffer bytes.Buffer
	previous := log.Logger
	log.Logger = zerolog.New(&buffer).Level(zerolog.InfoLevel)
	t.Cleanup(func() { log.Logger = previous })

	drainEvents(strings.NewReader(readStreamFixture(t, "Glob")), common.DefaultMaxOutputLineBytes, 7,
		nil, nil, nil, nil, nil)

	logged := buffer.String()
	for _, thought := range []string{"keep my reasoning short", "going to call", "reasoning"} {
		if strings.Contains(logged, thought) {
			t.Errorf("the info log holds thinking text %q\n%s", thought, logged)
		}
	}
	for _, noise := range []string{`"type":"stream_event"`, `"subtype":"thinking_tokens"`} {
		if strings.Contains(logged, noise) {
			t.Errorf("the info log holds %s events", noise)
		}
	}
	if !strings.Contains(logged, `"type":"assistant"`) || !strings.Contains(logged, `"type":"result"`) {
		t.Errorf("the info log lost the assistant and result events\n%s", logged)
	}
}

func TestDrainEvents_SkipsEmptyThinkingDeltas(t *testing.T) {
	// Without --thinking-display summarized the CLI streams thinking as empty
	// fragments; they carry nothing worth sending.
	line := `{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":""}}}`
	var events []common.ThinkingEvent
	drainEvents(strings.NewReader(line+"\n"), common.DefaultMaxOutputLineBytes, 7,
		nil, nil, nil, nil, func(event common.ThinkingEvent) { events = append(events, event) })
	if len(events) != 0 {
		t.Errorf("events = %+v, want none", events)
	}
}

func TestDrainEvents_PartialMessagesDoNotDoubleCountTools(t *testing.T) {
	for _, tt := range []struct {
		tool                 string
		wantSawCompleteStage bool
	}{
		{"Glob", false},
		{"mcp__tracker__complete_stage", true},
	} {
		t.Run(tt.tool, func(t *testing.T) {
			withPartials := readStreamFixture(t, tt.tool)
			var withoutPartials strings.Builder
			for line := range strings.Lines(withPartials) {
				if !strings.Contains(line, `"type":"stream_event"`) {
					withoutPartials.WriteString(line)
				}
			}

			for name, stream := range map[string]string{"with partial messages": withPartials, "without": withoutPartials.String()} {
				toolCalls, sawCompleteStage := 0, false
				drainEvents(strings.NewReader(stream), common.DefaultMaxOutputLineBytes, 7,
					nil,
					func(count int, isCompleteStage bool) {
						toolCalls += count
						sawCompleteStage = sawCompleteStage || isCompleteStage
					},
					nil, nil, nil)
				if toolCalls != 1 {
					t.Errorf("%s: toolCalls = %d, want 1", name, toolCalls)
				}
				if sawCompleteStage != tt.wantSawCompleteStage {
					t.Errorf("%s: sawCompleteStage = %v, want %v", name, sawCompleteStage, tt.wantSawCompleteStage)
				}
			}
		})
	}
}

// writeStreamStub writes a claude stub that records its args and prints stream.
func writeStreamStub(t *testing.T, stream string) (stub, argsFile string) {
	t.Helper()
	dir := t.TempDir()
	streamFile := filepath.Join(dir, "stream.jsonl")
	if err := os.WriteFile(streamFile, []byte(stream), 0o600); err != nil {
		t.Fatalf("writing stream: %v", err)
	}
	stub = filepath.Join(dir, "claude-stub.sh")
	argsFile = filepath.Join(dir, "args.txt")
	script := "#!/bin/sh\n" +
		`printf '%s\n' "$@" > ` + argsFile + "\n" +
		"cat > /dev/null\n" +
		"cat " + streamFile + "\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatalf("writing stub: %v", err)
	}
	return stub, argsFile
}

func TestClaudeAdapter_NilThinkingSender_RunsStage(t *testing.T) {
	stub, _ := writeStreamStub(t, readStreamFixture(t, "mcp__tracker__complete_stage"))
	t.Setenv("CLAUDE_CODE_BINARY", stub)
	cfg := &common.Config{TrackerMCPUrl: "http://issue.proxy/mcp/sse", AgentApiKey: "deadbeef", ClaudeCode: &common.ClaudeCodeConfig{ThinkingDisplay: "summarized"}}
	task := common.Task{IdRun: 7, IdTask: 31, Stage: common.StageImplementation, WorktreePath: t.TempDir(), Model: common.Model{Name: "opus"}, MaxTurns: 60}

	stats, err := NewClaudeCodeAdapter(cfg, nil).Run(context.Background(), task)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if stats.ToolCallsCount != 1 {
		t.Errorf("ToolCallsCount = %d, want 1", stats.ToolCallsCount)
	}
}

func TestRunPassesTheThinkingDisplayOnlyWhenSet(t *testing.T) {
	for _, tt := range []struct {
		display  string
		wantArgs string
	}{
		{"summarized", "--thinking-display\nsummarized\n"},
		{"omitted", "--thinking-display\nomitted\n"},
		{"", ""},
	} {
		t.Run("display "+tt.display, func(t *testing.T) {
			stub, argsFile := writeStreamStub(t, readStreamFixture(t, "mcp__tracker__complete_stage"))
			t.Setenv("CLAUDE_CODE_BINARY", stub)
			cfg := &common.Config{TrackerMCPUrl: "http://issue.proxy/mcp/sse", AgentApiKey: "deadbeef", ClaudeCode: &common.ClaudeCodeConfig{ThinkingDisplay: tt.display}}
			task := common.Task{IdRun: 7, IdTask: 31, Stage: common.StageImplementation, WorktreePath: t.TempDir(), Model: common.Model{Name: "opus"}, MaxTurns: 60}

			if _, err := NewClaudeCodeAdapter(cfg, nil).Run(context.Background(), task); err != nil {
				t.Fatalf("Run: %v", err)
			}

			args, err := os.ReadFile(argsFile)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(args), "--include-partial-messages\n") {
				t.Errorf("claude args lack --include-partial-messages\n--- args ---\n%s", args)
			}
			if tt.wantArgs == "" && strings.Contains(string(args), "--thinking-display") {
				t.Errorf("no thinking display is configured, but claude got --thinking-display\n--- args ---\n%s", args)
			}
			if tt.wantArgs != "" && !strings.Contains(string(args), tt.wantArgs) {
				t.Errorf("claude args lack %q\n--- args ---\n%s", tt.wantArgs, args)
			}
		})
	}
}

func TestClaudeAdapter_ThinkingMasksSecrets(t *testing.T) {
	var mu sync.Mutex
	var thinkingBodies []string
	tracker := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		if request.URL.Path == "/agent/task/31/thinking" {
			mu.Lock()
			thinkingBodies = append(thinkingBodies, string(body))
			mu.Unlock()
		}
		writer.WriteHeader(http.StatusOK)
	}))
	defer tracker.Close()

	stream := `{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"the MCP token is ` + testRunToken + `"}}}` + "\n" +
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1","name":"mcp__tracker__complete_stage"}]}}` + "\n" +
		`{"type":"result","subtype":"success","is_error":false}` + "\n"
	stub, _ := writeStreamStub(t, stream)
	t.Setenv("CLAUDE_CODE_BINARY", stub)
	cfg := &common.Config{
		TrackerMCPUrl: "http://issue.proxy/mcp/sse", TrackerAPIUrl: tracker.URL,
		AgentApiKey: "deadbeef", WebhookSecret: []byte("webhook-secret"), ClaudeCode: &common.ClaudeCodeConfig{},
	}
	trackerClient := common.NewTrackerClient(cfg)
	trackerClient.RedactSecret(testRunToken)
	task := common.Task{
		IdRun: 7, IdTask: 31, Stage: common.StageChat, WorktreePath: t.TempDir(), Model: common.Model{Name: "opus"}, MaxTurns: 60,
		RunToken: testRunToken, Chat: &common.ChatContext{Mode: common.ChatModeReply},
	}

	if _, err := NewClaudeCodeAdapter(cfg, trackerClient).Run(context.Background(), task); err != nil {
		t.Fatalf("Run: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	sent := strings.Join(thinkingBodies, "\n")
	if !strings.Contains(sent, "the MCP token is ***") {
		t.Errorf("thinking sent to the tracker lacks the masked fragment\n%s", sent)
	}
	if strings.Contains(sent, testRunToken) {
		t.Errorf("thinking sent to the tracker holds the run token\n%s", sent)
	}
}
