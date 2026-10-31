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
	TrackerMCPUrl      string
	TrackerAPIUrl      string
	AgentApiKey        string
	WebhookSecret      []byte
	ListenPort         int
	MaxOutputLineBytes int
	AdapterType        string
	LogLevel           string
	WorkspaceBase      string
	Models             ModelCatalog
	MaxTurnsLimit      int
	ClaudeCode         *ClaudeCodeConfig
	Goose              *GooseConfig
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
}

// GooseConfig configures the Goose adapter. Unlike claude, Goose authenticates
// with a per-token API key (BYOK), read from the environment and passed
// through to the child process. The provider of the run's model selects which
// one Goose drives (anthropic / google / ollama / openai); the matching key
// (ANTHROPIC_API_KEY / GOOGLE_API_KEY) or OLLAMA_HOST must be set. Not
// pre-validated here — it fails clearly at first invocation if the chosen
// provider's credential is missing.
type GooseConfig struct {
	// AnthropicAPIKey / GoogleAPIKey / OllamaHost are provider credentials
	// passed through to the child env; only the one matching the provider is used.
	AnthropicAPIKey string
	GoogleAPIKey    string
	OllamaHost      string

	// DefaultExtensionTimeoutSeconds is how long one tool call (e.g. a shell
	// command) may run before goose kills it.
	DefaultExtensionTimeoutSeconds int
	// StopHookBlockCap is how many times the Stop hook may send the agent back
	// to call complete_stage before goose ends the run.
	StopHookBlockCap int
}

var ignoredRepoEnv = []string{"REPO_URL", "GIT_ACCESS_TOKEN", "REPO_BRANCH_BASE"}

func IgnoredRepoEnv() []string {
	var set []string
	for _, name := range ignoredRepoEnv {
		if os.Getenv(name) != "" {
			set = append(set, name)
		}
	}
	return set
}

func LoadConfig(adapterType string) (*Config, error) {
	trackerURL := strings.TrimRight(os.Getenv("TRACKER_URL"), "/")
	agentApiKey := os.Getenv("GATEWAY_TO_TRACKER_TOKEN")
	webhookSecretHex := os.Getenv("TRACKER_TO_GATEWAY_TOKEN")

	var missing []string
	for _, v := range []struct{ name, val string }{
		{"TRACKER_URL", trackerURL},
		{"GATEWAY_TO_TRACKER_TOKEN", agentApiKey},
		{"TRACKER_TO_GATEWAY_TOKEN", webhookSecretHex},
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
	logLevel := envOrDefault("LOG_LEVEL", "info")
	workspaceBase := envOrDefault("WORKSPACE_BASE", "/worktrees")
	maxOutputLineBytes := envInt("MAX_OUTPUT_LINE_MB", 0) * 1024 * 1024
	models, err := parseModelCatalog(adapterType, os.Getenv("MODELS"), os.Getenv("MODEL_DEFAULT"))
	if err != nil {
		return nil, err
	}
	cfg := &Config{
		TrackerMCPUrl:      trackerURL + trackerMCPPath,
		TrackerAPIUrl:      trackerURL + trackerAPIPath,
		AgentApiKey:        agentApiKey,
		WebhookSecret:      webhookSecret,
		ListenPort:         listenPort,
		MaxOutputLineBytes: maxOutputLineBytes,
		AdapterType:        adapterType,
		LogLevel:           logLevel,
		WorkspaceBase:      workspaceBase,
		Models:             models,
		MaxTurnsLimit:      envInt("MAX_TURNS_LIMIT", 0),
	}

	switch adapterType {
	case "claude-code":
		// Subscription auth only: mounted ~/.claude creds, optionally overridden
		// by CLAUDE_CODE_OAUTH_TOKEN. No API key is read or required — the CLI
		// fails hard at first invocation if no usable credential is present.
		cfg.ClaudeCode = &ClaudeCodeConfig{
			OAuthToken: os.Getenv("CLAUDE_CODE_OAUTH_TOKEN"),
		}
	case "goose":
		// BYOK: the API key (or OLLAMA_HOST) is passed through to the goose
		// subprocess.
		cfg.Goose = &GooseConfig{
			AnthropicAPIKey:                os.Getenv("ANTHROPIC_API_KEY"),
			GoogleAPIKey:                   os.Getenv("GOOGLE_API_KEY"),
			OllamaHost:                     os.Getenv("OLLAMA_HOST"),
			DefaultExtensionTimeoutSeconds: envInt("GOOSE_DEFAULT_EXTENSION_TIMEOUT", 1800),
			StopHookBlockCap:               envInt("GOOSE_STOP_HOOK_BLOCK_CAP", 3),
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
