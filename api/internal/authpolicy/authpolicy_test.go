package authpolicy

import (
	"context"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestFreeAllowsEverythingAndOffersPasswordOnly(t *testing.T) {
	ctx := context.Background()
	assert.True(t, Free{}.Allow(ctx, &model.User{}))
	assert.NoError(t, Free{}.Check(ctx, UserChange{IdUser: 1, Kind: ChangeDelete}))
	assert.Equal(t, model.AuthMethodsRes{Password: true}, Free{}.Methods(ctx))
}
