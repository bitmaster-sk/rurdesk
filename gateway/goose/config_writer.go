package goose

import (
	"fmt"
	"os"
	"path/filepath"
)

// gooseConfigTemplate is config.yaml content for the tracker MCP server: a
// streamable-HTTP extension (goose ≥ v1.30 dropped SSE remote extensions).
// Goose does not reliably substitute ${ENV} in the extension `uri` (yields a
// reqwest "builder error" on initialize), so writeGooseConfig writes the
// stage-scoped URL and bearer token in literally, per run. %s order: uri,
// bearer token.
//
// enabled: false is the only headless-safe tool gate — goose run has no
// --allowed-tools flag, and GOOSE_MODE approve/smart_approve would hang
// waiting for an approval no headless user can give. `developer` (shell +
// text_editor) stays enabled by default (not listed here); `apps` is
// disabled because it builds goose apps instead of editing repo files, and
// computercontroller/memory/tutorial/todo are unused and would eat into
// goose's <25-tool budget. Per-tool gating inside `developer` isn't done —
// it needs the undocumented permission file.
const gooseConfigTemplate = `GOOSE_DISABLE_KEYRING: true
extensions:
  tracker:
    enabled: true
    type: streamable_http
    name: tracker
    uri: %s
    timeout: 300
    headers:
      Authorization: Bearer %s
  apps:
    type: builtin
    enabled: false
  computercontroller:
    type: builtin
    enabled: false
  memory:
    type: builtin
    enabled: false
  tutorial:
    type: builtin
    enabled: false
  todo:
    type: builtin
    enabled: false
`

// gooseConfigDir returns the directory goose reads config.yaml from:
// $XDG_CONFIG_HOME/goose if set, else $HOME/.config/goose (root → /root/.config/goose).
func gooseConfigDir() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "goose")
	}
	home := os.Getenv("HOME")
	if home == "" {
		home = "/root"
	}
	return filepath.Join(home, ".config", "goose")
}

// writeGooseConfig writes <configDir>/config.yaml with the tracker streamable-
// HTTP MCP server pointed at mcpURL and carrying the literal bot bearer token.
// Called once per run (the URL differs by stage), overwriting any previous
// config. Mode 0o600 since the file holds the bearer token.
func writeGooseConfig(configDir, mcpURL, botKey string) (string, error) {
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return "", fmt.Errorf("creating goose config dir: %w", err)
	}
	path := filepath.Join(configDir, "config.yaml")
	content := fmt.Sprintf(gooseConfigTemplate, mcpURL, botKey)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("writing goose config.yaml: %w", err)
	}
	return path, nil
}
