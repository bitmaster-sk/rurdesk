package extctx

import (
	"context"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
)

// RunTokenPrefix marks a run token, so auth never mistakes it for a session
// token or an API key.
const RunTokenPrefix = "rdr_"

// RunScope bounds a request made with a run token: the agent reaches only IdsProject, with
// at most MaxRole there. It lives on the request context and is never written into the agent's
// cached roles.
type RunScope struct {
	IdRun      int64
	IdsProject []int64
	MaxRole    model.Role
}

type runScopeKey struct{}

func WithRunScope(ctx context.Context, scope RunScope) context.Context {
	return context.WithValue(ctx, runScopeKey{}, scope)
}

func GetRunScope(ctx context.Context) (RunScope, bool) {
	scope, ok := ctx.Value(runScopeKey{}).(RunScope)
	return scope, ok
}
