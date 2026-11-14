package publicurl

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestResolve(t *testing.T) {
	tests := []struct {
		name   string
		base   string
		legacy string
		want   string
		errVar string
	}{
		{name: "both unset", want: ""},
		{name: "public base url", base: "https://tracker.example.com", want: "https://tracker.example.com"},
		{name: "trailing slash and whitespace are trimmed", base: "  https://tracker.example.com/ ", want: "https://tracker.example.com"},
		{name: "http is accepted", base: "http://tracker.lan:8080", want: "http://tracker.lan:8080"},
		{name: "legacy mcp variable is used when the new one is unset", legacy: "https://mcp.example.com/", want: "https://mcp.example.com"},
		{name: "legacy value keeps its old lenient handling", legacy: "https://tracker.example.com/rurdesk/", want: "https://tracker.example.com/rurdesk"},
		{name: "new variable wins over the legacy one", base: "https://tracker.example.com", legacy: "https://mcp.example.com", want: "https://tracker.example.com"},

		{name: "path", base: "https://tracker.example.com/app", errVar: EnvBaseUrl},
		{name: "query", base: "https://tracker.example.com?x=1", errVar: EnvBaseUrl},
		{name: "fragment", base: "https://tracker.example.com#x", errVar: EnvBaseUrl},
		{name: "userinfo", base: "https://user:pass@tracker.example.com", errVar: EnvBaseUrl},
		{name: "no scheme", base: "tracker.example.com", errVar: EnvBaseUrl},
		{name: "other scheme", base: "ftp://tracker.example.com", errVar: EnvBaseUrl},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Resolve(tt.base, tt.legacy)

			if tt.errVar != "" {
				require.ErrorContains(t, err, tt.errVar)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestLoad_ReadsBothVariables(t *testing.T) {
	previousBase, previousLegacy := viper.GetString(EnvBaseUrl), viper.GetString(EnvLegacyMcpBaseUrl)
	defer func() {
		viper.Set(EnvBaseUrl, previousBase)
		viper.Set(EnvLegacyMcpBaseUrl, previousLegacy)
	}()
	viper.Set(EnvBaseUrl, "")
	viper.Set(EnvLegacyMcpBaseUrl, "https://mcp.example.com")

	got, err := Load()

	require.NoError(t, err)
	require.Equal(t, "https://mcp.example.com", got)
}
