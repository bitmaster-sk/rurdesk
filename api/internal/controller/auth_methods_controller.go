package controller

import (
	"net/http"

	"github.com/bitmaster-sk/rurdesk/api/internal/authpolicy"
	"github.com/gin-gonic/gin"
)

type AuthMethodsController struct {
	provider func() authpolicy.AuthMethodsProvider
}

func NewAuthMethodsController(provider func() authpolicy.AuthMethodsProvider) *AuthMethodsController {
	return &AuthMethodsController{provider: provider}
}

func (ac *AuthMethodsController) Get(c *gin.Context) {
	c.JSON(http.StatusOK, ac.provider().Methods(c.Request.Context()))
}
