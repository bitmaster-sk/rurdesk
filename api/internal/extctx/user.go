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
