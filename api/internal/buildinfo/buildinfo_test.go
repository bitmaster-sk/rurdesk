package buildinfo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Without -ldflags the binary is a local build; Get must say so rather than
// claim a release version that was never tagged.
func TestGetReturnsDevDefaultsWhenNotStamped(t *testing.T) {
	info := Get()

	assert.Equal(t, "dev", info.Version)
	assert.Equal(t, "unknown", info.Commit)
}

func TestGetReturnsStampedValues(t *testing.T) {
	originalVersion, originalCommit := version, commit
	t.Cleanup(func() { version, commit = originalVersion, originalCommit })

	version, commit = "1.0.0", "abc1234"

	info := Get()

	assert.Equal(t, "1.0.0", info.Version)
	assert.Equal(t, "abc1234", info.Commit)
}
