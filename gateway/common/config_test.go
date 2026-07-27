package common

import (
	"strings"
	"testing"
)

// setBaseEnv sets the required base env vars so adapter-specific tests can
// focus on their own branch.
func setBaseEnv(t *testing.T) {
	t.Helper()
	t.Setenv("TRACKER_URL", "http://rurdesk.proxy")
	t.Setenv("GATEWAY_TO_TRACKER_TOKEN", "botkey")
	t.Setenv("TRACKER_TO_GATEWAY_TOKEN", "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
	t.Setenv("REPO_URL", "https://github.com/org/repo.git")
	t.Setenv("GIT_ACCESS_TOKEN", "gat")
}

func TestLoadConfigClaudeCodeNoAPIKeyRequired(t *testing.T) {
	setBaseEnv(t)
	// Explicitly ensure no claude-specific vars leak in from the host.
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "")
	t.Setenv("CLAUDE_MODEL", "")
	t.Setenv("CLAUDE_MAX_TURNS_PLAN", "")
	t.Setenv("CLAUDE_MAX_TURNS_IMPLEMENT", "")

	cfg, err := LoadConfig("claude-code")
	if err != nil {
		t.Fatalf("LoadConfig(claude-code) returned error: %v", err)
	}
	if cfg.ClaudeCode == nil {
		t.Fatal("ClaudeCode config is nil")
	}
	if cfg.ClaudeCode.Model != "" {
		t.Errorf("Model = %q, want empty (CLI default)", cfg.ClaudeCode.Model)
	}
	if cfg.ClaudeCode.OAuthToken != "" {
		t.Errorf("OAuthToken = %q, want empty", cfg.ClaudeCode.OAuthToken)
	}
	if cfg.ClaudeCode.MaxTurnsPlan != 50 {
		t.Errorf("MaxTurnsPlan = %d, want 50", cfg.ClaudeCode.MaxTurnsPlan)
	}
	if cfg.ClaudeCode.MaxTurnsImplement != 100 {
		t.Errorf("MaxTurnsImplement = %d, want 100", cfg.ClaudeCode.MaxTurnsImplement)
	}
}

// TestLoadConfigDerivesTrackerURLs verifies TRACKER_URL is joined with the
// fixed endpoint paths and a trailing slash is trimmed to avoid a double slash.
func TestLoadConfigDerivesTrackerURLs(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("TRACKER_URL", "http://rurdesk.proxy/") // trailing slash on purpose

	cfg, err := LoadConfig("goose")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if got, want := cfg.TrackerMCPUrl, "http://rurdesk.proxy/mcp/sse"; got != want {
		t.Errorf("TrackerMCPUrl = %q, want %q", got, want)
	}
	if got, want := cfg.TrackerAPIUrl, "http://rurdesk.proxy/api/private"; got != want {
		t.Errorf("TrackerAPIUrl = %q, want %q", got, want)
	}
}

// TestLoadConfigMissingTrackerURL verifies TRACKER_URL is required.
func TestLoadConfigMissingTrackerURL(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("TRACKER_URL", "")

	if _, err := LoadConfig("goose"); err == nil || !strings.Contains(err.Error(), "TRACKER_URL") {
		t.Fatalf("expected missing-TRACKER_URL error, got %v", err)
	}
}

func TestLoadConfigClaudeCodeOptionalVarsFlowThrough(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "sk-oauth-token")
	t.Setenv("CLAUDE_MODEL", "claude-opus-4-20250101")
	t.Setenv("CLAUDE_MAX_TURNS_PLAN", "10")
	t.Setenv("CLAUDE_MAX_TURNS_IMPLEMENT", "90")

	cfg, err := LoadConfig("claude-code")
	if err != nil {
		t.Fatalf("LoadConfig(claude-code) returned error: %v", err)
	}
	if cfg.ClaudeCode.OAuthToken != "sk-oauth-token" {
		t.Errorf("OAuthToken = %q, want sk-oauth-token", cfg.ClaudeCode.OAuthToken)
	}
	if cfg.ClaudeCode.Model != "claude-opus-4-20250101" {
		t.Errorf("Model = %q", cfg.ClaudeCode.Model)
	}
	if cfg.ClaudeCode.MaxTurnsPlan != 10 {
		t.Errorf("MaxTurnsPlan = %d, want 10", cfg.ClaudeCode.MaxTurnsPlan)
	}
	if cfg.ClaudeCode.MaxTurnsImplement != 90 {
		t.Errorf("MaxTurnsImplement = %d, want 90", cfg.ClaudeCode.MaxTurnsImplement)
	}
}

func TestLoadConfigUnknownAdapterErrorListsAll(t *testing.T) {
	setBaseEnv(t)
	_, err := LoadConfig("bogus")
	if err == nil {
		t.Fatal("expected error for unknown adapter")
	}
	for _, want := range []string{"claude-code", "goose"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err.Error(), want)
		}
	}
}

func TestLoadConfigGooseDefaults(t *testing.T) {
	setBaseEnv(t)
	// Ensure no goose-specific vars leak in from the host.
	for _, k := range []string{
		"GOOSE_PROVIDER", "GOOSE_MODEL", "ANTHROPIC_API_KEY", "GOOGLE_API_KEY",
		"OLLAMA_HOST", "GOOSE_MAX_TURNS_PLAN", "GOOSE_MAX_TURNS_IMPLEMENT",
	} {
		t.Setenv(k, "")
	}

	cfg, err := LoadConfig("goose")
	if err != nil {
		t.Fatalf("LoadConfig(goose) returned error: %v", err)
	}
	if cfg.Goose == nil {
		t.Fatal("Goose config is nil")
	}
	if cfg.Goose.Provider != "anthropic" {
		t.Errorf("Provider = %q, want anthropic (default)", cfg.Goose.Provider)
	}
	if cfg.Goose.Model != "" {
		t.Errorf("Model = %q, want empty (provider default)", cfg.Goose.Model)
	}
	if cfg.Goose.MaxTurnsPlan != 50 {
		t.Errorf("MaxTurnsPlan = %d, want 50", cfg.Goose.MaxTurnsPlan)
	}
	if cfg.Goose.MaxTurnsImplement != 100 {
		t.Errorf("MaxTurnsImplement = %d, want 100", cfg.Goose.MaxTurnsImplement)
	}
}

func TestLoadConfigGooseOptionalVarsFlowThrough(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("GOOSE_PROVIDER", "ollama")
	t.Setenv("GOOSE_MODEL", "qwen3-coder")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-xyz")
	t.Setenv("GOOGLE_API_KEY", "g-key")
	t.Setenv("OLLAMA_HOST", "http://gpu-box:11434")
	t.Setenv("GOOSE_MAX_TURNS_PLAN", "12")
	t.Setenv("GOOSE_MAX_TURNS_IMPLEMENT", "80")

	cfg, err := LoadConfig("goose")
	if err != nil {
		t.Fatalf("LoadConfig(goose) returned error: %v", err)
	}
	g := cfg.Goose
	if g.Provider != "ollama" {
		t.Errorf("Provider = %q, want ollama", g.Provider)
	}
	if g.Model != "qwen3-coder" {
		t.Errorf("Model = %q, want qwen3-coder", g.Model)
	}
	if g.AnthropicAPIKey != "sk-ant-xyz" {
		t.Errorf("AnthropicAPIKey = %q", g.AnthropicAPIKey)
	}
	if g.GoogleAPIKey != "g-key" {
		t.Errorf("GoogleAPIKey = %q", g.GoogleAPIKey)
	}
	if g.OllamaHost != "http://gpu-box:11434" {
		t.Errorf("OllamaHost = %q", g.OllamaHost)
	}
	if g.MaxTurnsPlan != 12 {
		t.Errorf("MaxTurnsPlan = %d, want 12", g.MaxTurnsPlan)
	}
	if g.MaxTurnsImplement != 80 {
		t.Errorf("MaxTurnsImplement = %d, want 80", g.MaxTurnsImplement)
	}
}
