package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/gin-gonic/gin"
)

// runTokenProjectRoute roots the per-project routes, whose project role checks the run scope
// restricts to the run's projects.
const runTokenProjectRoute = "/api/private/project/:idProject"

// runTokenCompleteRoute is the only write a run token makes: completing its task.
const runTokenCompleteRoute = "/api/private/agent/task/:idTask/complete"

// runTokenMessageRoute lists messages; a run token reads project and issue comments only, not
// direct or team messages.
const runTokenMessageRoute = "/api/private/message"

// runTokenReadRoutes are the reads outside a project a run token may make; each lists only the
// caller's visible projects or the caller itself. Every other route outside a project is refused.
var runTokenReadRoutes = map[string]bool{
	"/api/private/user":         true,
	"/api/private/project":      true,
	"/api/private/state":        true,
	"/api/private/severity":     true,
	"/api/private/issue-type":   true,
	"/api/private/tag":          true,
	"/api/private/custom-field": true,
}

// authenticateRunToken admits a run token on the allowed routes only and puts its scope on the request context.
func authenticateRunToken(c *gin.Context, runTokenAuth RunTokenAuthenticator, bearer string) {
	user, scope, err := runTokenAuth.LookupRunToken(c.Request.Context(), bearer)
	if err != nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	if user == nil || scope == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	if !isRunTokenRouteAllowed(c.Request, c.FullPath()) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	ctx := extctx.WithRunScope(extctx.WithApiKeyAuth(extctx.WithUser(c.Request.Context(), *user)), *scope)
	c.Request = c.Request.WithContext(ctx)
	c.Next()
}

// isRunTokenRouteAllowed matches the route pattern, not the URL, so an unmatched route is refused too.
func isRunTokenRouteAllowed(request *http.Request, route string) bool {
	switch request.Method {
	case http.MethodGet:
		if route == runTokenMessageRoute {
			return isProjectMessageQuery(request)
		}
		return runTokenReadRoutes[route] || route == runTokenProjectRoute || strings.HasPrefix(route, runTokenProjectRoute+"/")
	case http.MethodPost:
		return route == runTokenCompleteRoute
	default:
		return false
	}
}

// isProjectMessageQuery reads idMessageRecipientType like the message handler (first value), so
// both agree on the listed recipient type.
func isProjectMessageQuery(request *http.Request) bool {
	idType, err := strconv.ParseInt(request.URL.Query().Get("idMessageRecipientType"), 10, 64)
	if err != nil {
		return false
	}
	recipientType := model.MessageRecipientType(idType)
	return recipientType == model.ProjectRecipientType || recipientType == model.IssueRecipientType
}
