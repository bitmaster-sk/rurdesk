package common

import (
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Tracker endpoint paths are fixed by the tracker's routing, so only the base
// URL is configured via TRACKER_URL and these are appended.
const (
	trackerMCPPath = "/mcp/sse"
	trackerAPIPath = "/api/private"
)

type Config struct {
	// TrackerMCPUrl / TrackerAPIUrl are derived from TRACKER_URL + the fixed
	// paths above.
	TrackerMCPUrl  string
	TrackerAPIUrl  string
	BotApiKey      string
	WebhookSecret  []byte
	ListenPort     int
	RepoUrl        string
	RepoBranchBase string
	GitAccessToken string
	MaxConcurrent  int
	AdapterType    string
	LogLevel       string
	WorkspaceBase  string
	ClaudeCode     *ClaudeCodeConfig
	Goose          *GooseConfig
}

// ClaudeCodeConfig configures the Claude Code CLI adapter, which authenticates
// against a Claude Pro/Max subscription rather than a pay-per-token API key.
// In the CLI's native precedence order, an optional CLAUDE_CODE_OAUTH_TOKEN
// (from `claude setup-token`) overrides the mounted
// ~/.claude/.credentials.json from an interactive `claude` login.
// ANTHROPIC_API_KEY is deliberately not read here and is scrubbed from the
// child env by the adapter — if set it would take precedence over both and
// silently switch billing to the Console pay-per-token bucket.
type ClaudeCodeConfig struct {
	// OAuthToken is an optional long-lived subscription token from
	// `claude setup-token`. Empty means rely on the mounted login creds.
	OAuthToken string
	// Model is the claude model id. Empty string lets the CLI/account pick.
	Model             string
	MaxTurnsPlan      int
	MaxTurnsImplement int
}

// GooseConfig configures the Goose adapter. Unlike claude, Goose authenticates
// with a per-token API key (BYOK), read from the environment and passed
// through to the child process. GOOSE_PROVIDER selects which provider Goose
// drives (anthropic / google / ollama); the matching key (ANTHROPIC_API_KEY /
// GOOGLE_API_KEY) or OLLAMA_HOST must be set. Not pre-validated here — it
// fails clearly at first invocation if the chosen provider's credential is
// missing.
type GooseConfig struct {
	// Provider is the GOOSE_PROVIDER value (e.g. "anthropic", "google", "ollama").
	Provider string
	// Model is the GOOSE_MODEL id. Empty lets Goose/the provider pick a default.
	Model string
	// AnthropicAPIKey / GoogleAPIKey / OllamaHost are provider credentials
	// passed through to the child env; only the one matching Provider is used.
	AnthropicAPIKey string
	GoogleAPIKey    string
	OllamaHost      string
	// MaxTurns* are per-stage caps passed to `goose run --max-turns`.
	MaxTurnsPlan      int
	MaxTurnsImplement int
}

func LoadConfig(adapterType string) (*Config, error) {
	trackerURL := strings.TrimRight(os.Getenv("TRACKER_URL"), "/")
	botApiKey := os.Getenv("GATEWAY_TO_TRACKER_TOKEN")
	webhookSecretHex := os.Getenv("TRACKER_TO_GATEWAY_TOKEN")
	repoUrl := strings.TrimSpace(os.Getenv("REPO_URL"))
	gitAccessToken := os.Getenv("GIT_ACCESS_TOKEN")

	var missing []string
	for _, v := range []struct{ name, val string }{
		{"TRACKER_URL", trackerURL},
		{"GATEWAY_TO_TRACKER_TOKEN", botApiKey},
		{"TRACKER_TO_GATEWAY_TOKEN", webhookSecretHex},
		{"REPO_URL", repoUrl},
		{"GIT_ACCESS_TOKEN", gitAccessToken},
	} {
		if v.val == "" {
			missing = append(missing, v.name)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}

	webhookSecret, err := hex.DecodeString(webhookSecretHex)
	if err != nil {
		return nil, fmt.Errorf("decoding TRACKER_TO_GATEWAY_TOKEN: %w", err)
	}

	listenPort := envInt("LISTEN_PORT", 9090)
	maxConcurrent := envInt("MAX_CONCURRENT", 1)
	repoBranchBase := envOrDefault("REPO_BRANCH_BASE", "main")
	logLevel := envOrDefault("LOG_LEVEL", "info")
	workspaceBase := envOrDefault("WORKSPACE_BASE", "/worktrees")
	cfg := &Config{
		TrackerMCPUrl:  trackerURL + trackerMCPPath,
		TrackerAPIUrl:  trackerURL + trackerAPIPath,
		BotApiKey:      botApiKey,
		WebhookSecret:  webhookSecret,
		ListenPort:     listenPort,
		RepoUrl:        repoUrl,
		RepoBranchBase: repoBranchBase,
		GitAccessToken: gitAccessToken,
		MaxConcurrent:  maxConcurrent,
		AdapterType:    adapterType,
		LogLevel:       logLevel,
		WorkspaceBase:  workspaceBase,
	}

	switch adapterType {
	case "claude-code":
		// Subscription auth only: mounted ~/.claude creds, optionally overridden
		// by CLAUDE_CODE_OAUTH_TOKEN. No API key is read or required — the CLI
		// fails hard at first invocation if no usable credential is present.
		cfg.ClaudeCode = &ClaudeCodeConfig{
			OAuthToken:        os.Getenv("CLAUDE_CODE_OAUTH_TOKEN"),
			Model:             os.Getenv("CLAUDE_MODEL"),
			MaxTurnsPlan:      envInt("CLAUDE_MAX_TURNS_PLAN", 50),
			MaxTurnsImplement: envInt("CLAUDE_MAX_TURNS_IMPLEMENT", 100),
		}
	case "goose":
		// BYOK: the API key (or OLLAMA_HOST) is passed through to the goose
		// subprocess. Turn caps mirror claude's 50/100, deliberately generous —
		// exhausting the cap kills the stage before the agent can call
		// complete_stage, so a too-low ceiling loses real work while a too-high
		// one costs nothing on runs that finish normally.
		cfg.Goose = &GooseConfig{
			Provider:          envOrDefault("GOOSE_PROVIDER", "anthropic"),
			Model:             os.Getenv("GOOSE_MODEL"),
			AnthropicAPIKey:   os.Getenv("ANTHROPIC_API_KEY"),
			GoogleAPIKey:      os.Getenv("GOOGLE_API_KEY"),
			OllamaHost:        os.Getenv("OLLAMA_HOST"),
			MaxTurnsPlan:      envInt("GOOSE_MAX_TURNS_PLAN", 50),
			MaxTurnsImplement: envInt("GOOSE_MAX_TURNS_IMPLEMENT", 100),
		}
	default:
		return nil, fmt.Errorf("unknown adapter type: %q (must be 'claude-code' or 'goose')", adapterType)
	}

	return cfg, nil
}

func envOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func envInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			return parsed
		}
	}
	return defaultVal
}
