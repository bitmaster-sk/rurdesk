package skills

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuiltinsParseFrontmatterAndContent(t *testing.T) {
	all := Builtins()
	require.NotEmpty(t, all)
	keys := map[string]bool{}
	for _, builtin := range all {
		assert.NotEmpty(t, builtin.Key, "key derives from filename")
		assert.NotEmpty(t, builtin.Name, "frontmatter name")
		assert.NotEmpty(t, builtin.Description, "frontmatter description")
		assert.NotEmpty(t, builtin.Content, "body after frontmatter")
		assert.NotContains(t, builtin.Content, "---\nname:", "frontmatter must be stripped")
		assert.False(t, keys[builtin.Key], "keys must be unique")
		keys[builtin.Key] = true
	}
	assert.True(t, keys["verification-rules"])
	assert.True(t, keys["repository-rules"])
	assert.True(t, keys["testing-rules"])
	assert.True(t, keys["pr-rules"])
}

func TestBuiltinByKey(t *testing.T) {
	builtin, ok := BuiltinByKey("verification-rules")
	require.True(t, ok)
	assert.Equal(t, "Verification rules", builtin.Name)

	_, ok = BuiltinByKey("nope")
	assert.False(t, ok)
}

func TestBuiltinDefaultStages(t *testing.T) {
	tests := []struct {
		key      string
		expected []string
	}{
		{"repository-rules", []string{"design", "implementation_plan", "implementation"}},
		{"verification-rules", []string{"implementation"}},
		{"testing-rules", nil},
		{"pr-rules", nil},
	}
	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			builtin, ok := BuiltinByKey(tc.key)
			require.True(t, ok)
			assert.Equal(t, tc.expected, builtin.DefaultStages)
		})
	}
}

func TestParseRejectsIncompleteFrontmatter(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{"no frontmatter", "# Just content\n"},
		{"unclosed frontmatter", "---\nname: X\ndescription: Y\n"},
		{"missing name", "---\ndescription: Y\n---\ncontent\n"},
		{"missing content", "---\nname: X\ndescription: Y\n---\n\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parse("some-key", tc.raw)
			assert.Error(t, err)
		})
	}
}

func TestParseRejectsUnknownStage(t *testing.T) {
	_, err := parse("k", "---\nname: X\ndescription: Y\nstages: pickup\n---\nbody\n")
	assert.Error(t, err, "pickup is never a skill stage")
}
