package publicurl

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/viper"
)

const (
	EnvBaseUrl          = "PUBLIC_BASE_URL"
	EnvLegacyMcpBaseUrl = "MCP_PUBLIC_BASE_URL"
)

func Load() (string, error) {
	return Resolve(viper.GetString(EnvBaseUrl), viper.GetString(EnvLegacyMcpBaseUrl))
}

// Resolve keeps MCP_PUBLIC_BASE_URL exactly as lenient as it was before PUBLIC_BASE_URL existed.
func Resolve(baseUrl, legacyMcpBaseUrl string) (string, error) {
	value := normalize(baseUrl)
	if value == "" {
		return normalize(legacyMcpBaseUrl), nil
	}
	invalid := fmt.Errorf("%s must be an absolute http(s) URL without a path, e.g. https://tracker.example.com", EnvBaseUrl)
	if len(value) > 500 || strings.ContainsAny(value, "?#") {
		return "", invalid
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Path != "" {
		return "", invalid
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", invalid
	}
	return value, nil
}

func normalize(raw string) string {
	return strings.TrimRight(strings.TrimSpace(raw), "/")
}
