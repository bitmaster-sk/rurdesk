package middleware

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveAllowOrigin(t *testing.T) {
	tests := []struct {
		name          string
		allowed       []string
		requestOrigin string
		want          string
	}{
		{
			name:          "unset allows any origin",
			requestOrigin: "https://anything.example",
			want:          "*",
		},
		{
			name:          "listed origin is echoed back, not the wildcard",
			allowed:       []string{"https://tracker.example.com"},
			requestOrigin: "https://tracker.example.com",
			want:          "https://tracker.example.com",
		},
		{
			name:          "unlisted origin gets no header",
			allowed:       []string{"https://tracker.example.com"},
			requestOrigin: "https://attacker.example.net",
			want:          "",
		},
		{
			name:          "scheme and host compare case-insensitively",
			allowed:       []string{"https://Tracker.Example.com"},
			requestOrigin: "https://tracker.example.com",
			want:          "https://tracker.example.com",
		},
		{
			name:          "a request without an Origin matches nothing once a list is set",
			allowed:       []string{"https://tracker.example.com"},
			requestOrigin: "",
			want:          "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, resolveAllowOrigin(tt.allowed, tt.requestOrigin))
		})
	}
}
