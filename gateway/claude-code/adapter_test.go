package claudecode

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/bitmaster-sk/rurdesk/gateway/common"
)

func TestMapTerminalStatus(t *testing.T) {
	tests := []struct {
		name     string
		exitCode int
		subtype  string
		isError  bool
		wantErr  bool
		wantMsg  string
	}{
		{"success subtype", 0, "success", false, false, ""},
		{"empty subtype zero exit", 0, "", false, false, ""},
		{"max turns", 1, "error_max_turns", true, true, "error_max_turns"},
		{"execution error", 1, "error_during_execution", true, true, "error_during_execution"},
		{"is_error no subtype", 1, "", true, true, "reported an error"},
		{"nonzero exit only", 2, "", false, true, "exited with code 2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mapTerminalStatus(tt.exitCode, tt.subtype, tt.isError)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected nil, got %v", err)
			}
			if tt.wantMsg != "" && !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantMsg)
			}
		})
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
	drainEvents(r, 42,
		func(s string, e bool) { subtype = s; isError = e },
		func(n int, _ bool) { toolCalls += n },
		func(tk int) {
			if tk > tokens {
				tokens = tk
			}
		},
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
			drainEvents(strings.NewReader(tt.line+"\n"), 7,
				nil,
				func(_ int, sawCompleteStage bool) {
					if sawCompleteStage {
						saw = true
					}
				},
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
	const mcpURL = "http://issue.proxy/mcp/plan/sse"
	const botKey = "deadbeef"

	path, err := writeMCPSettings(configDir, mcpURL, botKey)
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
	if tracker.Type != "sse" {
		t.Errorf("type = %q, want sse", tracker.Type)
	}
	if tracker.URL != mcpURL {
		t.Errorf("url = %q, want %q", tracker.URL, mcpURL)
	}
	if got := tracker.Headers["Authorization"]; got != "Bearer "+botKey {
		t.Errorf("Authorization = %q, want %q", got, "Bearer "+botKey)
	}
}

func TestChildEnvScrubsAPIKeysAndSetsToken(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-should-be-removed")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "tok-should-be-removed")

	env := childEnv("my-oauth-token")

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
}

func TestChildEnvOmitsTokenWhenEmpty(t *testing.T) {
	env := childEnv("")
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
				BotApiKey:     "deadbeef",
				ClaudeCode: &common.ClaudeCodeConfig{
					MaxTurnsPlan:      25,
					MaxTurnsImplement: 60,
				},
			}
			adapter := NewClaudeCodeAdapter(cfg)
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

			stats, err := adapter.Run(context.Background(), task)
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
			// The MCP config carries the bot's bearer token and the agent
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
