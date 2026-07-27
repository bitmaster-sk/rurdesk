package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/bitmaster-sk/rurdesk/api/internal/buildinfo"
)

// VersionController serves the build identity of the running binary. Mounted
// admin-only: a self-hosted instance's version is as useful for CVE-matching
// as it is for an operator.
type VersionController struct{}

func NewVersionController() *VersionController {
	return &VersionController{}
}

// Get returns the version and commit stamped into the binary at build time.
func (ctrl *VersionController) Get(c *gin.Context) {
	c.JSON(http.StatusOK, buildinfo.Get())
}
