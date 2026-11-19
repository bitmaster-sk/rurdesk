package agent

import (
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMintRunToken_FormatAndHash(t *testing.T) {
	raw, hash, err := MintRunToken()
	require.NoError(t, err)

	assert.Regexp(t, "^rdr_[0-9a-f]{64}$", raw)
	assert.Len(t, raw, 68)
	sum := sha256.Sum256([]byte(raw))
	assert.Equal(t, sum[:], hash)
	assert.Equal(t, hash, HashRunToken(raw))

	other, otherHash, err := MintRunToken()
	require.NoError(t, err)
	assert.NotEqual(t, raw, other)
	assert.NotEqual(t, hash, otherHash)
}
