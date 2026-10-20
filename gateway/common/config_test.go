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
	t.Setenv("MODELS", "openai/kimi")
	t.Setenv("MODEL_DEFAULT", "")
	t.Setenv("MAX_TURNS_LIMIT", "")
}

func TestLoadConfigClaudeCodeNoAPIKeyRequired(t *testing.T) {
	setBaseEnv(t)
	// Explicitly ensure no claude-specific vars leak in from the host.
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "")
	t.Setenv("MODELS", "sonnet")

	cfg, err := LoadConfig("claude-code")
	if err != nil {
		t.Fatalf("LoadConfig(claude-code) returned error: %v", err)
	}
	if cfg.ClaudeCode == nil {
		t.Fatal("ClaudeCode config is nil")
	}
	if cfg.ClaudeCode.OAuthToken != "" {
		t.Errorf("OAuthToken = %q, want empty", cfg.ClaudeCode.OAuthToken)
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

func TestLoadConfigClaudeCodeOAuthTokenFlowsThrough(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "sk-oauth-token")

	cfg, err := LoadConfig("claude-code")
	if err != nil {
		t.Fatalf("LoadConfig(claude-code) returned error: %v", err)
	}
	if cfg.ClaudeCode.OAuthToken != "sk-oauth-token" {
		t.Errorf("OAuthToken = %q, want sk-oauth-token", cfg.ClaudeCode.OAuthToken)
	}
}

func TestLoadConfigReadsTheModelCatalog(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("MODELS", "openai/kimi-k2.7-code,ollama/qwen3:8b")
	t.Setenv("MODEL_DEFAULT", "ollama/qwen3:8b")

	cfg, err := LoadConfig("goose")
	if err != nil {
		t.Fatalf("LoadConfig(goose) returned error: %v", err)
	}
	if got := cfg.Models.Ids(); len(got) != 2 || got[0] != "openai/kimi-k2.7-code" || got[1] != "ollama/qwen3:8b" {
		t.Errorf("Models = %v", got)
	}
	if cfg.Models.Default != (Model{Provider: "ollama", Name: "qwen3:8b"}) {
		t.Errorf("Default = %+v", cfg.Models.Default)
	}
}

func TestLoadConfigRefusesToStartWithoutModels(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("MODELS", "")

	if _, err := LoadConfig("goose"); err == nil || !strings.Contains(err.Error(), "MODELS") {
		t.Fatalf("expected a MODELS error, got %v", err)
	}
}

func TestLoadConfigRefusesADefaultOutsideTheList(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("MODEL_DEFAULT", "openai/opus")

	if _, err := LoadConfig("goose"); err == nil || !strings.Contains(err.Error(), "MODEL_DEFAULT") {
		t.Fatalf("expected a MODEL_DEFAULT error, got %v", err)
	}
}

func TestLoadConfigMaxTurnsLimit(t *testing.T) {
	setBaseEnv(t)

	cfg, err := LoadConfig("goose")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.MaxTurnsLimit != 0 {
		t.Errorf("MaxTurnsLimit = %d, want 0 (no limit) when unset", cfg.MaxTurnsLimit)
	}

	t.Setenv("MAX_TURNS_LIMIT", "600")
	cfg, err = LoadConfig("goose")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.MaxTurnsLimit != 600 {
		t.Errorf("MaxTurnsLimit = %d, want 600", cfg.MaxTurnsLimit)
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

func TestLoadConfigGooseCredentialsFlowThrough(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-xyz")
	t.Setenv("GOOGLE_API_KEY", "g-key")
	t.Setenv("OLLAMA_HOST", "http://gpu-box:11434")

	cfg, err := LoadConfig("goose")
	if err != nil {
		t.Fatalf("LoadConfig(goose) returned error: %v", err)
	}
	g := cfg.Goose
	if g.AnthropicAPIKey != "sk-ant-xyz" {
		t.Errorf("AnthropicAPIKey = %q", g.AnthropicAPIKey)
	}
	if g.GoogleAPIKey != "g-key" {
		t.Errorf("GoogleAPIKey = %q", g.GoogleAPIKey)
	}
	if g.OllamaHost != "http://gpu-box:11434" {
		t.Errorf("OllamaHost = %q", g.OllamaHost)
	}
}
