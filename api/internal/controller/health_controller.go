package controller

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// readinessTimeout caps total wait across all dependency pings, so a hung
// dependency fails the probe fast instead of blocking the orchestrator.
const readinessTimeout = 2 * time.Second

// HealthCheck is a single named dependency probe used by the readiness
// endpoint. Ping returns nil when the dependency is reachable.
type HealthCheck struct {
	Name string
	Ping func(ctx context.Context) error
}

// HealthController serves liveness and readiness probes. Liveness is a pure
// process check; readiness pings each registered dependency (DB, cache).
type HealthController struct {
	checks []HealthCheck
}

func NewHealthController(checks []HealthCheck) *HealthController {
	return &HealthController{checks: checks}
}

// Live always reports OK as long as the process can serve HTTP. It must not
// touch dependencies — a transient DB blip shouldn't get the container killed.
func (ctrl *HealthController) Live(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Ready pings every registered dependency and returns 200 only when all are
// reachable, otherwise 503 with a per-dependency breakdown.
func (ctrl *HealthController) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), readinessTimeout)
	defer cancel()

	checks := make(map[string]string, len(ctrl.checks))
	allUp := true
	for _, check := range ctrl.checks {
		if err := check.Ping(ctx); err != nil {
			checks[check.Name] = "down"
			allUp = false
			continue
		}
		checks[check.Name] = "up"
	}

	status := "ok"
	code := http.StatusOK
	if !allUp {
		status = "unavailable"
		code = http.StatusServiceUnavailable
	}
	c.JSON(code, gin.H{"status": status, "checks": checks})
}
