package extctx_test

import (
	"context"
	"io"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestGetUser_roundtrip(t *testing.T) {
	user := model.User{IdUser: 42, Name: "Alice"}
	ctx := extctx.WithUser(context.Background(), user)
	got, ok := extctx.GetUser(ctx)
	assert.True(t, ok)
	assert.Equal(t, user.IdUser, got.IdUser)
}

func TestGetUser_missing(t *testing.T) {
	_, ok := extctx.GetUser(context.Background())
	assert.False(t, ok)
}

func TestGetLogger_roundtrip(t *testing.T) {
	logger := zerolog.New(io.Discard)
	ctx := extctx.WithLogger(context.Background(), logger)
	got := extctx.GetLogger(ctx)
	assert.NotNil(t, got)
}

func TestGetLogger_fallback(t *testing.T) {
	// Bare context falls back to the zerolog global.
	got := extctx.GetLogger(context.Background())
	assert.NotNil(t, got)
}
