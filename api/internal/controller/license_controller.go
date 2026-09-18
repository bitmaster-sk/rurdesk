package controller

import (
	"net/http"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/license"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/gin-gonic/gin"
)

type LicenseController struct {
	licenses license.Provider
}

func NewLicenseController(licenses license.Provider) *LicenseController {
	return &LicenseController{licenses: licenses}
}

// Get returns the expiry state to every authenticated user. It is not
// admin-only on purpose: the admin who can renew is often not the one using
// the app.
func (lc *LicenseController) Get(c *gin.Context) {
	ctx := c.Request.Context()

	status := lc.licenses.Status(ctx)
	now := time.Now()
	severity := status.ExpirySeverity(now)
	if severity == license.SeverityNone {
		c.JSON(http.StatusOK, model.LicenseRes{Severity: string(license.SeverityNone)})
		return
	}

	c.JSON(http.StatusOK, model.LicenseRes{
		Severity:      string(severity),
		DaysRemaining: status.DaysRemaining(now),
		ExpiresAt:     status.ExpiresAt,
		IsDismissible: severity.IsDismissible(),
	})
}
