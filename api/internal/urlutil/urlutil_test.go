package urlutil

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func ctxWithQuery(rawQuery string) *gin.Context {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("GET", "/?"+rawQuery, nil)
	return ctx
}

// Repeated params are the canonical form (HTML form model, OpenAPI's default,
// Go's net/url) and what the MCP tools emit.
func TestParseInt64Array_RepeatedParams(t *testing.T) {
	ctx := ctxWithQuery("ids=1&ids=2")
	require.Equal(t, []int64{1, 2}, ParseInt64Array(ctx, "ids"))
}

// Comma-joined is the widely used convention and what the Angular client emits.
// Before this was supported, a multi-id filter was dropped in full.
func TestParseInt64Array_CommaJoined(t *testing.T) {
	ctx := ctxWithQuery("ids=1,2,3")
	require.Equal(t, []int64{1, 2, 3}, ParseInt64Array(ctx, "ids"))
}

// Both forms in one request: each repeated value may itself be a comma list.
func TestParseInt64Array_MixedForms(t *testing.T) {
	ctx := ctxWithQuery("ids=1,2&ids=3")
	require.Equal(t, []int64{1, 2, 3}, ParseInt64Array(ctx, "ids"))
}

// Unparseable elements are skipped, not fatal — a filter degrades, it does not
// reject. Whitespace around a comma is tolerated.
func TestParseInt64Array_SkipsInvalidElements(t *testing.T) {
	ctx := ctxWithQuery("ids=1,x&ids=%204%20")
	require.Equal(t, []int64{1, 4}, ParseInt64Array(ctx, "ids"))
}

func TestParseInt64Array_NegativeValues(t *testing.T) {
	ctx := ctxWithQuery("ids=-1,2")
	require.Equal(t, []int64{-1, 2}, ParseInt64Array(ctx, "ids"))
}

func TestParseInt64Array_ParamAbsent(t *testing.T) {
	ctx := ctxWithQuery("other=1")
	require.Nil(t, ParseInt64Array(ctx, "ids"))
}

// Present but empty, e.g. `?ids=` — no usable values, so nil like the absent case.
func TestParseInt64Array_PresentButEmpty(t *testing.T) {
	ctx := ctxWithQuery("ids=")
	require.Nil(t, ParseInt64Array(ctx, "ids"))
}

// The case the trailing nil-guard covers: the param IS present but nothing in it
// parses. This normalises "no usable filter" to a single return value; callers
// gate on len(...) > 0, so nil and empty behave identically for them.
func TestParseInt64Array_AllInvalid(t *testing.T) {
	ctx := ctxWithQuery("ids=x,y")
	require.Nil(t, ParseInt64Array(ctx, "ids"))
}

// Empty elements inside a list are ignored rather than counting as a zero —
// `1,,2` is two ids, not three.
func TestParseInt64Array_EmptyElementsIgnored(t *testing.T) {
	ctx := ctxWithQuery("ids=1,,2")
	require.Equal(t, []int64{1, 2}, ParseInt64Array(ctx, "ids"))
}
