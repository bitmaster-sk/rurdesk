package goose

import (
	"context"
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
		name        string
		exitCode    int
		finalStatus string
		wantErr     bool
		wantMsg     string
	}{
		{"success status", 0, "success", false, ""},
		{"empty status zero exit", 0, "", false, ""},
		{"completed status", 0, "completed", false, ""},
		{"error status", 1, "error", true, "error"},
		{"nonzero exit only", 2, "", true, "exited with code 2"},
		{"failed status nonzero", 1, "failed", true, "failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mapTerminalStatus(tt.exitCode, tt.finalStatus)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected nil, got %v", err)
			}
			if tt.wantMsg != "" && (err == nil || !strings.Contains(err.Error(), tt.wantMsg)) {
				t.Errorf("error %v does not contain %q", err, tt.wantMsg)
			}
		})
	}
}

func TestToStreamableHTTP(t *testing.T) {
	tests := []struct{ in, want string }{
		{"http://issue.proxy/mcp/sse", "http://issue.proxy/mcp/http"},
		{"http://issue.proxy/mcp/plan/sse", "http://issue.proxy/mcp/plan/http"},
		{"http://issue.proxy/mcp/http", "http://issue.proxy/mcp/http"},
	}
	for _, tt := range tests {
		if got := toStreamableHTTP(tt.in); got != tt.want {
			t.Errorf("toStreamableHTTP(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestWriteGooseConfig(t *testing.T) {
	dir := t.TempDir()
	const mcpURL = "http://issue.proxy/mcp/plan/http"
	const botKey = "deadbeef"

	path, err := writeGooseConfig(dir, mcpURL, botKey)
	if err != nil {
		t.Fatalf("writeGooseConfig: %v", err)
	}
	if path != filepath.Join(dir, "config.yaml") {
		t.Errorf("unexpected path %q", path)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("mode = %o, want 600", mode)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	content := string(data)
	for _, want := range []string{
		"type: streamable_http",
		"uri: " + mcpURL,
		"Authorization: Bearer " + botKey,
		"enabled: true",
		// Non-essential built-ins are disabled (headless tool gate); `apps`
		// caused agents to build goose apps instead of repo files.
		"apps:",
		"computercontroller:",
		"enabled: false",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("config.yaml missing %q\n--- got ---\n%s", want, content)
		}
	}
	// Literal values only — no unsubstituted ${...} placeholders.
	if strings.Contains(content, "${") {
		t.Errorf("config.yaml still has an unsubstituted token:\n%s", content)
	}
}

func TestChildEnvWiresProvider(t *testing.T) {
	cfg := &common.GooseConfig{
		Provider:        "anthropic",
		AnthropicAPIKey: "sk-ant-123",
		// GoogleAPIKey and OllamaHost intentionally empty → must be omitted.
	}
	env := childEnv(cfg)

	mustHave := []string{
		"GIT_PAGER=cat",
		"GOOSE_DISABLE_KEYRING=1",
		"GOOSE_PROVIDER=anthropic",
		"ANTHROPIC_API_KEY=sk-ant-123",
	}
	for _, want := range mustHave {
		if !hasEnv(env, want) {
			t.Errorf("childEnv missing %q", want)
		}
	}
	for _, kv := range env {
		if strings.HasPrefix(kv, "GOOGLE_API_KEY=") {
			t.Errorf("empty GoogleAPIKey leaked: %q", kv)
		}
		if strings.HasPrefix(kv, "OLLAMA_HOST=") && strings.TrimPrefix(kv, "OLLAMA_HOST=") == "" {
			t.Errorf("empty OllamaHost leaked: %q", kv)
		}
	}
}

// TestChildEnvInheritsProcessEnv documents the passthrough OpenAI-compatible
// providers rely on (e.g. Ollama Cloud via OPENAI_HOST): childEnv seeds from
// os.Environ(), so any container env var reaches goose unchanged.
func TestChildEnvInheritsProcessEnv(t *testing.T) {
	t.Setenv("OPENAI_HOST", "https://ollama.com")
	t.Setenv("OPENAI_API_KEY", "sk-passthrough")

	env := childEnv(&common.GooseConfig{Provider: "openai"})
	for _, want := range []string{"OPENAI_HOST=https://ollama.com", "OPENAI_API_KEY=sk-passthrough"} {
		if !hasEnv(env, want) {
			t.Errorf("childEnv did not inherit %q from the process env", want)
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

// TestRunWithStubBinary drives Run against a stub `goose` script that records
// args + cwd and emits canned stream-json, exercising the full subprocess
// path without the real binary or a provider.
func TestRunWithStubBinary(t *testing.T) {
	// The stub emits stream-json NDJSON: a banner, a thinking + text token, a
	// complete_stage tool call (the real submission signal), and a terminal
	// "complete" event carrying the token total (10+5 = 15).
	const streamOut = `{"type":"message","message":{"role":"assistant","content":[{"type":"thinking","thinking":"planning"}]}}` + "\n" +
		`{"type":"message","message":{"role":"assistant","content":[{"type":"text","text":"done"}]}}` + "\n" +
		`{"type":"message","message":{"role":"assistant","content":[{"type":"toolRequest","id":"c1","toolCall":{"status":"success","value":{"name":"tracker__complete_stage"}}}]}}` + "\n" +
		`{"type":"complete","total_tokens":15,"input_tokens":10,"output_tokens":5}`
	// Same run but the agent never calls complete_stage — it just wrote its
	// answer and stopped. Nothing was submitted.
	const streamOutNoSubmit = `{"type":"message","message":{"role":"assistant","content":[{"type":"thinking","thinking":"planning"}]}}` + "\n" +
		`{"type":"message","message":{"role":"assistant","content":[{"type":"text","text":"done"}]}}` + "\n" +
		`{"type":"complete","total_tokens":15,"input_tokens":10,"output_tokens":5}`
	tests := []struct {
		name      string
		streamOut string
		exitCode  int
		wantErr   bool
		wantCode  string // expected AgentError.Code when wantErr is set
	}{
		{
			name:      "success",
			streamOut: streamOut,
			exitCode:  0,
			wantErr:   false,
		},
		{
			name:      "crash",
			streamOut: streamOut,
			exitCode:  1,
			wantErr:   true,
		},
		{
			// Exit 0 but no complete_stage → must fail as stage_not_submitted
			// instead of silently succeeding and hanging the run.
			name:      "exit zero without complete_stage",
			streamOut: streamOutNoSubmit,
			exitCode:  0,
			wantErr:   true,
			wantCode:  common.ErrCodeStageNotSubmitted,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			worktree := t.TempDir()
			argsFile := filepath.Join(t.TempDir(), "args.txt")
			stub := writeStubGoose(t, argsFile, tt.streamOut, tt.exitCode)
			t.Setenv("GOOSE_BINARY", stub)
			// Isolate the goose config write (Run calls writeGooseConfig) into a
			// temp dir instead of the real ~/.config/goose.
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())

			cfg := &common.Config{
				TrackerMCPUrl: "http://issue.proxy/mcp/sse",
				BotApiKey:     "deadbeef",
				Goose: &common.GooseConfig{
					Provider:          "anthropic",
					AnthropicAPIKey:   "sk-ant",
					MaxTurnsPlan:      25,
					MaxTurnsImplement: 60,
				},
			}
			adapter := NewGooseAdapter(cfg)
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
					t.Fatalf("expected *common.AgentError, got %T (%v)", err, err)
				}
				if agentErr.Code != tt.wantCode {
					t.Errorf("error code = %q, want %q", agentErr.Code, tt.wantCode)
				}
			}
			if stats.TokensUsed != 15 {
				t.Errorf("TokensUsed = %d, want 15", stats.TokensUsed)
			}
			if stats.DurationMs < 0 {
				t.Errorf("DurationMs = %d, want >= 0", stats.DurationMs)
			}

			recorded, readErr := os.ReadFile(argsFile)
			if readErr != nil {
				t.Fatalf("reading args file: %v", readErr)
			}
			got := string(recorded)
			for _, want := range []string{
				"run", "--no-session", "--output-format", "stream-json",
				"--provider", "anthropic",
			} {
				if !strings.Contains(got, want) {
					t.Errorf("stub args missing %q\n--- got ---\n%s", want, got)
				}
			}
			// The stub ran in the worktree dir (it records pwd on the last line).
			if !strings.Contains(got, worktree) {
				t.Errorf("stub did not run in worktree %q\n--- got ---\n%s", worktree, got)
			}
		})
	}
}

// writeStubGoose writes an executable shell script that records its args and
// cwd to argsFile, mimics goose's stream-json output, and exits with the given code.
func writeStubGoose(t *testing.T, argsFile, streamOut string, exitCode int) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "goose-stub.sh")
	script := "#!/bin/sh\n" +
		`printf '%s\n' "$@" > ` + argsFile + "\n" +
		`pwd >> ` + argsFile + "\n" +
		`echo 'goose is ready'` + "\n" +
		"cat <<'GOOSE_EOF'\n" + streamOut + "\nGOOSE_EOF\n" +
		"exit " + strconv.Itoa(exitCode) + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("writing stub: %v", err)
	}
	return path
}
