package flow

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSafeReturnUrl(t *testing.T) {
	tests := []struct {
		raw      string
		expected string
	}{
		{raw: "/issue/5?tab=comments#c1", expected: "/issue/5?tab=comments#c1"},
		{raw: "/", expected: "/"},
		{raw: "", expected: "/"},
		{raw: "//evil.com", expected: "/"},
		{raw: "/\\evil.com", expected: "/"},
		{raw: "/a//b", expected: "/"},
		{raw: "https://evil.com", expected: "/"},
		{raw: "evil.com", expected: "/"},
		{raw: "/a\nb", expected: "/"},
		{raw: "/" + strings.Repeat("a", 2000), expected: "/"},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			assert.Equal(t, tt.expected, SafeReturnUrl(tt.raw))
		})
	}
}
