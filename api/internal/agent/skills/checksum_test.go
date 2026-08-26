package skills

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChecksumDistinguishesEveryField(t *testing.T) {
	base := Checksum("name", "description", "content")

	tests := []struct {
		name        string
		checksum    string
		shouldMatch bool
	}{
		{"same input", Checksum("name", "description", "content"), true},
		{"changed name", Checksum("other", "description", "content"), false},
		{"changed description", Checksum("name", "other", "content"), false},
		{"changed content", Checksum("name", "description", "other"), false},
		{"field boundary shifted", Checksum("namedescription", "", "content"), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.shouldMatch, base == tc.checksum)
		})
	}
}

func TestBuiltinChecksumMatchesItsFields(t *testing.T) {
	builtin, ok := BuiltinByKey("verification-rules")
	require.True(t, ok)

	assert.Equal(t, Checksum(builtin.Name, builtin.Description, builtin.Content), builtin.Checksum())
}
