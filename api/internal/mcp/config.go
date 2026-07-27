package mcp

import (
	"strings"

	"github.com/spf13/viper"
)

// Config holds MCP server config. MCP mounts on the shared HTTP engine (no
// standalone listener), so there is no address to configure.
type Config struct {
	// PublicBaseURL pins the origin advertised to MCP clients in SSE endpoint
	// events, e.g. "https://tracker.example.com". Empty (the default) derives it
	// per request from the Host header — see originRewriteHandler.clientOrigin.
	// Set it when a proxy cannot preserve Host, or to remove any dependence on
	// request headers for the URL that carries the client's bearer token.
	PublicBaseURL string
}

func LoadConfig() *Config {
	return &Config{
		PublicBaseURL: strings.TrimRight(strings.TrimSpace(viper.GetString("MCP_PUBLIC_BASE_URL")), "/"),
	}
}

// internalBaseSentinel is a placeholder origin embedded in SSE endpoint events
// sent to MCP clients. Never dialed — originRewriteHandler swaps it for the
// client's real origin before the response leaves. Fixed rather than derived
// from a listen port, since MCP has none on the shared engine; must match the
// value passed to the SSE server verbatim.
const internalBaseSentinel = "http://mcp.internal"

func (c *Config) internalBaseURL() string {
	return internalBaseSentinel
}
