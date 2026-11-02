package goose

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bitmaster-sk/rurdesk/gateway/common"
)

// gooseConfigTemplate is config.yaml content for the tracker MCP server: a
// streamable-HTTP extension (goose ≥ v1.30 dropped SSE remote extensions).
// Goose does not reliably substitute ${ENV} in the extension `uri` (yields a
// reqwest "builder error" on initialize), so writeGooseConfig writes the
// stage-scoped URL and bearer token in literally, per run. Verb order: uri,
// bearer token, run header name, run id.
//
// enabled: false is the only headless-safe tool gate — goose run has no
// --allowed-tools flag, and GOOSE_MODE approve/smart_approve would hang
// waiting for an approval no headless user can give. `developer` (shell,
// edit, write, tree) stays enabled by default (not listed here); `apps` is
// disabled because it builds goose apps instead of editing repo files, and
// computercontroller/memory/tutorial/todo/scheduler/summon/extensionmanager
// are unused and would eat into goose's <25-tool budget. Per-tool gating
// inside `developer` isn't done — it needs the undocumented permission file.
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
      %s: "%d"
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
  scheduler:
    type: builtin
    enabled: false
  summon:
    type: builtin
    enabled: false
  extensionmanager:
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
// HTTP MCP server pointed at mcpURL and carrying the literal agent bearer token.
// Called once per run (the URL differs by stage), overwriting any previous
// config. Mode 0o600 since the file holds the bearer token. Every plugin in
// disabledPluginDirs is turned off, otherwise goose enables it on discovery.
func writeGooseConfig(configDir, mcpURL, agentKey string, idRun int64, disabledPluginDirs []string) (string, error) {
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return "", fmt.Errorf("creating goose config dir: %w", err)
	}
	path := filepath.Join(configDir, "config.yaml")
	var content strings.Builder
	fmt.Fprintf(&content, gooseConfigTemplate, mcpURL, agentKey, common.TrackerRunHeader, idRun)
	if len(disabledPluginDirs) > 0 {
		content.WriteString("plugins:\n")
		for _, pluginDir := range disabledPluginDirs {
			fmt.Fprintf(&content, "  %s:\n    enabled: false\n", strconv.Quote(pluginDir))
		}
	}
	if err := os.WriteFile(path, []byte(content.String()), 0o600); err != nil {
		return "", fmt.Errorf("writing goose config.yaml: %w", err)
	}
	return path, nil
}

// repoPluginDirs lists the plugins a repository ships in .agents/plugins.
// Goose keys plugins by the path under its working directory as getcwd
// reports it, so symlinks in the worktree path are resolved first.
func repoPluginDirs(worktreePath string) []string {
	if worktreePath == "" {
		return nil
	}
	root, err := filepath.EvalSymlinks(worktreePath)
	if err != nil {
		root = worktreePath
	}
	pluginsDir := filepath.Join(root, ".agents", "plugins")
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return nil
	}
	var pluginDirs []string
	for _, entry := range entries {
		pluginDir := filepath.Join(pluginsDir, entry.Name())
		if info, err := os.Stat(pluginDir); err == nil && info.IsDir() {
			pluginDirs = append(pluginDirs, pluginDir)
		}
	}
	return pluginDirs
}
