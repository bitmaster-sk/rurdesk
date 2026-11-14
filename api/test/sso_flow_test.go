package test

import (
	"context"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/flow"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/login"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSsoFlowStore_EntriesAreSingleUseAndExpire(t *testing.T) {
	Setup(t)
	cache, err := injector.GetCache()
	require.NoError(t, err)
	store := flow.NewStore(cache)
	ctx := context.Background()

	stateKey, err := store.SaveState(ctx, flow.State{Nonce: "n", ReturnUrl: "/x"})
	require.NoError(t, err)
	assert.InDelta(t, flow.StateTtl.Seconds(), cache.TTL(ctx, "sso:state:"+stateKey).Val().Seconds(), 2)
	state, err := store.TakeState(ctx, stateKey)
	require.NoError(t, err)
	assert.Equal(t, "/x", state.ReturnUrl)
	_, err = store.TakeState(ctx, stateKey)
	assert.ErrorIs(t, err, flow.ErrNotFound)

	codeKey, err := store.SaveCode(ctx, flow.Code{IdUser: 5})
	require.NoError(t, err)
	assert.InDelta(t, flow.CodeTtl.Seconds(), cache.TTL(ctx, "sso:code:"+codeKey).Val().Seconds(), 2)
	code, err := store.TakeCode(ctx, codeKey)
	require.NoError(t, err)
	assert.Equal(t, int64(5), code.IdUser)
	_, err = store.TakeCode(ctx, codeKey)
	assert.ErrorIs(t, err, flow.ErrNotFound)

	idTest, err := store.SaveTestResult(ctx, login.FailedTest(login.TestErrorIdp, "denied"))
	require.NoError(t, err)
	for range 2 {
		result, err := store.LoadTestResult(ctx, idTest)
		require.NoError(t, err)
		assert.Equal(t, login.TestErrorIdp, result.Error)
	}
	_, err = store.LoadTestResult(ctx, "unknown")
	assert.ErrorIs(t, err, flow.ErrNotFound)
}
