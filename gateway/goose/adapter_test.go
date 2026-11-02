package goose

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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
	const agentKey = "deadbeef"

	path, err := writeGooseConfig(dir, mcpURL, agentKey, 42, nil)
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
		"Authorization: Bearer " + agentKey,
		common.TrackerRunHeader + `: "42"`,
		"enabled: true",
		// Non-essential built-ins are disabled (headless tool gate); `apps`
		// caused agents to build goose apps instead of repo files.
		"apps:",
		"computercontroller:",
		"scheduler:",
		"summon:",
		"extensionmanager:",
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
	if strings.Contains(content, "plugins:") {
		t.Errorf("config.yaml lists plugins although the repo has none:\n%s", content)
	}
}

// Goose enables every plugin a repository ships, so each one must be listed
// as disabled under its full path.
func TestWriteGooseConfigDisablesRepoPlugins(t *testing.T) {
	dir := t.TempDir()
	pluginDirs := []string{"/worktrees/repo/.agents/plugins/lint", "/worktrees/repo/.agents/plugins/deploy"}

	path, err := writeGooseConfig(dir, "http://issue.proxy/mcp/http", "deadbeef", 42, pluginDirs)
	if err != nil {
		t.Fatalf("writeGooseConfig: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := "plugins:\n" +
		"  \"/worktrees/repo/.agents/plugins/lint\":\n    enabled: false\n" +
		"  \"/worktrees/repo/.agents/plugins/deploy\":\n    enabled: false\n"
	if !strings.HasSuffix(string(data), want) {
		t.Errorf("config.yaml does not end with the disabled plugins\n--- got ---\n%s", data)
	}
}

func TestRepoPluginDirs(t *testing.T) {
	t.Run("lists plugin directories, including symlinked ones", func(t *testing.T) {
		worktree := t.TempDir()
		pluginsDir := filepath.Join(worktree, ".agents", "plugins")
		for _, name := range []string{"lint", "target"} {
			if err := os.MkdirAll(filepath.Join(pluginsDir, name), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.Symlink(filepath.Join(pluginsDir, "target"), filepath.Join(pluginsDir, "linked")); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pluginsDir, "README.md"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}

		resolvedPluginsDir := filepath.Join(mustEvalSymlinks(t, worktree), ".agents", "plugins")
		got := repoPluginDirs(worktree)
		want := []string{
			filepath.Join(resolvedPluginsDir, "linked"),
			filepath.Join(resolvedPluginsDir, "lint"),
			filepath.Join(resolvedPluginsDir, "target"),
		}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("repoPluginDirs = %v, want %v", got, want)
		}
	})
	t.Run("resolves a symlinked worktree path", func(t *testing.T) {
		realWorktree := t.TempDir()
		if err := os.MkdirAll(filepath.Join(realWorktree, ".agents", "plugins", "lint"), 0o755); err != nil {
			t.Fatal(err)
		}
		linkedWorktree := filepath.Join(t.TempDir(), "worktree")
		if err := os.Symlink(realWorktree, linkedWorktree); err != nil {
			t.Fatal(err)
		}

		got := repoPluginDirs(linkedWorktree)
		want := filepath.Join(mustEvalSymlinks(t, realWorktree), ".agents", "plugins", "lint")
		if len(got) != 1 || got[0] != want {
			t.Errorf("repoPluginDirs = %v, want [%s]", got, want)
		}
	})
	t.Run("no plugins directory", func(t *testing.T) {
		if got := repoPluginDirs(t.TempDir()); len(got) != 0 {
			t.Errorf("repoPluginDirs = %v, want none", got)
		}
	})
}

func mustEvalSymlinks(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func TestChildEnvWiresProvider(t *testing.T) {
	cfg := &common.GooseConfig{
		AnthropicAPIKey: "sk-ant-123",
		// GoogleAPIKey and OllamaHost intentionally empty → must be omitted.
		DefaultExtensionTimeoutSeconds: 1800,
		StopHookBlockCap:               3,
	}
	env := childEnv(cfg, "anthropic", "git-token-of-the-run", "/tmp/rurdesk-stage-9.submitted")

	mustHave := []string{
		"GIT_PAGER=cat",
		"GOOSE_DISABLE_KEYRING=1",
		"GOOSE_PROVIDER=anthropic",
		"ANTHROPIC_API_KEY=sk-ant-123",
		"RURDESK_GIT_TOKEN=git-token-of-the-run",
		"GOOSE_DISABLE_SESSION_NAMING=true",
		"GOOSE_DEFAULT_EXTENSION_TIMEOUT=1800",
		"GOOSE_STOP_HOOK_BLOCK_CAP=3",
		"RURDESK_STAGE_MARKER=/tmp/rurdesk-stage-9.submitted",
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

	env := childEnv(&common.GooseConfig{}, "openai", "", "")
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
		`{"type":"message","message":{"role":"user","content":[{"type":"toolResponse","id":"c1","toolResult":{"status":"success","value":{"content":[]}}}]}}` + "\n" +
		`{"type":"complete","total_tokens":15,"input_tokens":10,"output_tokens":5}`
	// Same run but the agent never calls complete_stage — it just wrote its
	// answer and stopped. Nothing was submitted.
	const streamOutNoSubmit = `{"type":"message","message":{"role":"assistant","content":[{"type":"thinking","thinking":"planning"}]}}` + "\n" +
		`{"type":"message","message":{"role":"assistant","content":[{"type":"text","text":"done"}]}}` + "\n" +
		`{"type":"complete","total_tokens":15,"input_tokens":10,"output_tokens":5}`
	// Agent hit --max-turns: emits the sentinel text, no complete_stage call.
	const streamOutTurnLimit = `{"type":"message","message":{"role":"assistant","content":[{"type":"text","text":"I've reached the maximum number of actions I can do without user input. Would you like me to continue?"}]}}` + "\n" +
		`{"type":"complete","total_tokens":15,"input_tokens":10,"output_tokens":5}`
	// Agent hit --max-turns sentinel but still called complete_stage (finished
	// despite the warning — should NOT be treated as a turn-limit failure).
	const streamOutTurnLimitButSubmitted = `{"type":"message","message":{"role":"assistant","content":[{"type":"text","text":"I've reached the maximum number of actions I can do without user input. Would you like me to continue?"}]}}` + "\n" +
		`{"type":"message","message":{"role":"assistant","content":[{"type":"toolRequest","id":"c1","toolCall":{"status":"success","value":{"name":"tracker__complete_stage"}}}]}}` + "\n" +
		`{"type":"message","message":{"role":"user","content":[{"type":"toolResponse","id":"c1","toolResult":{"status":"success","value":{"content":[]}}}]}}` + "\n" +
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
		{
			// Turn limit exhausted, exit 0, no complete_stage → must be
			// classified as turn_limit_exhausted, not stage_not_submitted.
			name:      "turn limit exhausted, exit 0",
			streamOut: streamOutTurnLimit,
			exitCode:  0,
			wantErr:   true,
			wantCode:  common.ErrCodeTurnLimitExhausted,
		},
		{
			// Sentinel present but complete_stage was called → the agent
			// finished despite the warning; must NOT be a turn-limit error.
			name:      "turn limit sentinel but complete_stage was called",
			streamOut: streamOutTurnLimitButSubmitted,
			exitCode:  0,
			wantErr:   false,
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
				AgentApiKey:   "deadbeef",
				Goose: &common.GooseConfig{
					AnthropicAPIKey: "sk-ant",
				},
			}
			adapter := NewGooseAdapter(cfg, common.NewTrackerClient(cfg))
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
				Model:        common.Model{Provider: "anthropic", Name: "claude-sonnet"},
				MaxTurns:     60,
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
				"--provider\nanthropic\n", "--model\nclaude-sonnet\n", "--max-turns\n60\n",
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

func newTestAdapter() *GooseAdapter {
	cfg := &common.Config{
		TrackerMCPUrl: "http://issue.proxy/mcp/sse",
		AgentApiKey:   "deadbeef",
		Goose:         &common.GooseConfig{},
	}
	return NewGooseAdapter(cfg, common.NewTrackerClient(cfg))
}

// writeFixtureStub writes a goose stub that prints a recorded stream-json
// fixture and exits with the given code.
func writeFixtureStub(t *testing.T, fixture string, exitCode int) string {
	t.Helper()
	fixturePath, err := filepath.Abs(filepath.Join("testdata", "stream-json", fixture))
	if err != nil {
		t.Fatal(err)
	}
	stub := filepath.Join(t.TempDir(), "goose-stub.sh")
	script := "#!/bin/sh\ncat " + fixturePath + "\nexit " + strconv.Itoa(exitCode) + "\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatalf("writing stub: %v", err)
	}
	return stub
}

// Real goose v1.56.0 runs: a provider failure must surface as its own code
// whatever the exit code, and a submitted stage must succeed.
func TestRunWithRecordedStreams(t *testing.T) {
	tests := []struct {
		fixture  string
		exitCode int
		wantCode string
		wantErr  bool
	}{
		{"v1.56-complete-stage-accepted.ndjson", 0, "", false},
		{"v1.56-complete-stage-rejected.ndjson", 0, common.ErrCodeStageNotSubmitted, true},
		{"v1.56-complete-stage-never-called.ndjson", 0, common.ErrCodeStageNotSubmitted, true},
		{"v1.56-provider-credits-402.ndjson", 0, common.ErrCodeProviderCreditExhausted, true},
		{"v1.56-provider-auth-401.ndjson", 1, common.ErrCodeProviderAuthFailed, true},
		{"v1.56-provider-rate-limit-429.ndjson", 1, common.ErrCodeProviderRateLimited, true},
		{"v1.56-provider-low-credit-400.ndjson", 1, common.ErrCodeProviderCreditExhausted, true},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			t.Setenv("GOOSE_BINARY", writeFixtureStub(t, tt.fixture, tt.exitCode))
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			task := common.Task{IdRun: 7, IdTask: 1, Stage: common.StageDesign, WorktreePath: t.TempDir()}

			_, err := newTestAdapter().Run(context.Background(), task)
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("Run error = %v, want nil", err)
				}
				return
			}
			var agentErr *common.AgentError
			if !errors.As(err, &agentErr) {
				t.Fatalf("expected *common.AgentError, got %T (%v)", err, err)
			}
			if agentErr.Code != tt.wantCode {
				t.Errorf("error code = %q, want %q", agentErr.Code, tt.wantCode)
			}
		})
	}
}

// A run goose ended with an error that is not a provider failure must report
// goose's own reason instead of a bare exit code.
func TestRunReportsGooseErrorText(t *testing.T) {
	stub := filepath.Join(t.TempDir(), "goose-stub.sh")
	script := "#!/bin/sh\nprintf '%s\\n' '{\"type\":\"error\",\"error\":\"Extension tracker failed to start\\nmore detail\"}'\nexit 1\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatalf("writing stub: %v", err)
	}
	t.Setenv("GOOSE_BINARY", stub)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	task := common.Task{IdRun: 7, IdTask: 1, Stage: common.StageDesign, WorktreePath: t.TempDir()}

	_, err := newTestAdapter().Run(context.Background(), task)
	if err == nil || err.Error() != "goose exited with code 1: Extension tracker failed to start" {
		t.Errorf("Run error = %v, want goose's first error line", err)
	}
}

// The Stop hook lets a turn end once the marker exists, so a marker left by an
// earlier attempt of the same task must be gone before goose starts, and the
// marker must not outlive the run.
func TestRunStartsWithoutStageMarker(t *testing.T) {
	const idTask = 987654
	marker := stageMarkerPath(idTask)
	if err := os.WriteFile(marker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(marker) })

	reportFile := filepath.Join(t.TempDir(), "marker.txt")
	stub := filepath.Join(t.TempDir(), "goose-stub.sh")
	script := "#!/bin/sh\n" +
		`echo "$RURDESK_STAGE_MARKER" > ` + reportFile + "\n" +
		`if [ -e "$RURDESK_STAGE_MARKER" ]; then echo present >> ` + reportFile + "; else echo absent >> " + reportFile + "; fi\n" +
		`touch "$RURDESK_STAGE_MARKER"` + "\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatalf("writing stub: %v", err)
	}
	t.Setenv("GOOSE_BINARY", stub)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	task := common.Task{IdRun: 7, IdTask: idTask, Stage: common.StageDesign, WorktreePath: t.TempDir()}

	_, _ = newTestAdapter().Run(context.Background(), task)

	report, err := os.ReadFile(reportFile)
	if err != nil {
		t.Fatalf("reading report: %v", err)
	}
	if want := marker + "\nabsent\n"; string(report) != want {
		t.Errorf("goose saw %q, want %q", report, want)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("marker still exists after the run (err=%v)", err)
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

func TestCancelAlwaysEndsARunningStage(t *testing.T) {
	stub, startedFile := writeHangingStub(t, "goose-stub.sh")
	t.Setenv("GOOSE_BINARY", stub)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := &common.Config{
		TrackerMCPUrl: "http://issue.proxy/mcp/sse",
		AgentApiKey:   "deadbeef",
		Goose:         &common.GooseConfig{},
	}
	adapter := NewGooseAdapter(cfg, common.NewTrackerClient(cfg))

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
			if err == nil || !strings.Contains(err.Error(), "goose cancelled") {
				t.Fatalf("attempt %d: Run returned %v, want goose cancelled", attempt, err)
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

func TestRunSkipsOutputLinesOverTheLimit(t *testing.T) {
	const completeStage = `{"type":"message","message":{"role":"assistant","content":[{"type":"toolRequest","id":"c1","toolCall":{"status":"success","value":{"name":"tracker__complete_stage"}}}]}}`
	const completeStageAccepted = `{"type":"message","message":{"role":"user","content":[{"type":"toolResponse","id":"c1","toolResult":{"status":"success","value":{"content":[]}}}]}}`
	stub := filepath.Join(t.TempDir(), "goose-stub.sh")
	script := "#!/bin/sh\n" +
		"head -c 300000 /dev/zero | tr '\\0' 'x'; echo\n" +
		"head -c 300000 /dev/zero | tr '\\0' 'y' >&2; echo >&2\n" +
		"echo '" + completeStage + "'\n" +
		"echo '" + completeStageAccepted + "'\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatalf("writing stub: %v", err)
	}
	t.Setenv("GOOSE_BINARY", stub)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := &common.Config{
		TrackerMCPUrl:      "http://issue.proxy/mcp/sse",
		AgentApiKey:        "deadbeef",
		MaxOutputLineBytes: 1024,
		Goose:              &common.GooseConfig{},
	}
	adapter := NewGooseAdapter(cfg, common.NewTrackerClient(cfg))
	var isReportedSubmitted atomic.Bool
	task := common.Task{IdRun: 7, IdTask: 1, Stage: common.StageImplementation, WorktreePath: t.TempDir()}
	task.OnStageSubmitted = func() { isReportedSubmitted.Store(true) }

	result := make(chan error, 1)
	go func() {
		_, err := adapter.Run(context.Background(), task)
		result <- err
	}()
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("Run error = %v, want the stage to finish", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run hung on an output line over the limit")
	}
	if !isReportedSubmitted.Load() {
		t.Error("complete_stage after the long line was not reported as submitted")
	}
}

func TestRunPassesPromptOnStdinNotInArgs(t *testing.T) {
	const marker = "vite-scaffold-marker"
	const completeStage = `{"type":"message","message":{"role":"assistant","content":[{"type":"toolRequest","id":"c1","toolCall":{"status":"success","value":{"name":"tracker__complete_stage"}}}]}}`
	const completeStageAccepted = `{"type":"message","message":{"role":"user","content":[{"type":"toolResponse","id":"c1","toolResult":{"status":"success","value":{"content":[]}}}]}}`
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	stdinFile := filepath.Join(dir, "stdin.txt")
	stub := filepath.Join(dir, "goose-stub.sh")
	script := "#!/bin/sh\n" +
		`printf '%s\n' "$@" > ` + argsFile + "\n" +
		"cat > " + stdinFile + "\n" +
		"echo '" + completeStage + "'\n" +
		"echo '" + completeStageAccepted + "'\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatalf("writing stub: %v", err)
	}
	t.Setenv("GOOSE_BINARY", stub)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := &common.Config{
		TrackerMCPUrl: "http://issue.proxy/mcp/sse",
		AgentApiKey:   "deadbeef",
		Goose:         &common.GooseConfig{},
	}
	adapter := NewGooseAdapter(cfg, common.NewTrackerClient(cfg))
	task := common.Task{
		IdRun:        7,
		IdTask:       1,
		Stage:        common.StageImplementation,
		WorktreePath: t.TempDir(),
		IssueTitle:   "Scaffold",
		IssueDesc:    "Run " + marker + " then stop it",
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
		t.Errorf("goose command line contains the prompt\n--- args ---\n%s", args)
	}
	stdin, err := os.ReadFile(stdinFile)
	if err != nil {
		t.Fatalf("reading stdin: %v", err)
	}
	task.MaxTurns = 60
	task.Vocab = common.ToolVocabGoose
	wantPrompt, err := common.RenderPrompt(task)
	if err != nil {
		t.Fatalf("rendering prompt: %v", err)
	}
	if string(stdin) != wantPrompt {
		t.Errorf("goose stdin is not the full prompt\n--- got ---\n%s\n--- want ---\n%s", stdin, wantPrompt)
	}
}
