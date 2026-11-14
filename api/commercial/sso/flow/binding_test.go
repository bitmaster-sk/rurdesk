package flow

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBinding(t *testing.T) {
	first, err := NewBinding()
	require.NoError(t, err)
	second, err := NewBinding()
	require.NoError(t, err)

	assert.NotEqual(t, first.Value, second.Value)
	assert.Equal(t, HashBinding(first.Value), first.Hash)
	assert.True(t, strings.HasPrefix(CookieName(first.Hash), CookiePrefix))
	assert.Len(t, CookieName(first.Hash), len(CookiePrefix)+16)
	assert.NotEqual(t, CookieName(first.Hash), CookieName(second.Hash))

	assert.True(t, IsBindingValid(first.Value, first.Hash))
	assert.False(t, IsBindingValid(second.Value, first.Hash))
	assert.False(t, IsBindingValid("", first.Hash))
}

func TestNewState(t *testing.T) {
	state, err := NewState("hash", "//evil.com", true)
	require.NoError(t, err)

	assert.Equal(t, "/", state.ReturnUrl)
	assert.True(t, state.IsExtended)
	assert.NotEmpty(t, state.Nonce)
	assert.NotEmpty(t, state.Verifier)
	assert.False(t, state.IsTest)

	testState, err := NewTestState("draft-hash")
	require.NoError(t, err)
	assert.True(t, testState.IsTest)
	assert.Equal(t, "draft-hash", testState.TestHash)
	assert.Empty(t, testState.BindingHash)
}
