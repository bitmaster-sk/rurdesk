package extctx

import (
	"context"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
)

type userKey struct{}

func WithUser(ctx context.Context, u model.User) context.Context {
	return context.WithValue(ctx, userKey{}, u)
}

func GetUser(ctx context.Context) (model.User, bool) {
	u, ok := ctx.Value(userKey{}).(model.User)
	return u, ok
}

type isApiKeyAuthKey struct{}

// Token management checks this so a leaked token cannot mint more tokens.
func WithApiKeyAuth(ctx context.Context) context.Context {
	return context.WithValue(ctx, isApiKeyAuthKey{}, true)
}

func IsApiKeyAuth(ctx context.Context) bool {
	marked, _ := ctx.Value(isApiKeyAuthKey{}).(bool)
	return marked
}
